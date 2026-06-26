package scope_resolution

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// ---------------------------------------------------------------------------
// receiver_bound_calls.go — 7-case dispatcher for receiver-bound CALLS/ACCESSES.
// Ported from TS scope-resolution/passes/receiver-bound-calls.ts (1460 lines).
//
// Contract Invariant I4 — case order is load-bearing.
// Contract Invariant I5 — pre-seeding `seen` is forbidden.
// ---------------------------------------------------------------------------

// pickResult encodes the three possible outcomes of pickOverload /
// pickFirstNonStaticOnly, replacing the TS Symbol sentinels
// OVERLOAD_AMBIGUOUS and STATIC_ONLY_FILTERED.
type pickResult struct {
	Def                *shared.SymbolDefinition
	Ambiguous          bool // narrowing left >1 ambiguous candidate
	StaticOnlyFiltered bool // all candidates were filtered by isStaticOnly
}

// pickOverload resolves a member by name on a class def, narrowing by argument
// types when multiple overloads share the name. Falls back to the first-seen
// def when narrowing empties the set.
// Mirrors TS pickOverload.
func pickOverload(
	ownerID string,
	memberName string,
	site shared.ReferenceSite,
	semModel model.SemanticModel,
	provider ScopeResolver,
) pickResult {
	overloads := semModel.Methods().LookupAllByOwner(ownerID, memberName)
	if len(overloads) == 0 {
		field := semModel.Fields().LookupFieldByOwner(ownerID, memberName)
		if field != nil {
			return pickResult{Def: field}
		}
		return pickResult{}
	}
	if len(overloads) == 1 {
		return pickResult{Def: overloads[0]}
	}

	// Multiple overloads — narrow by arity / argument types
	hookCtx := buildOverloadHookCtx(site, provider)
	valOverloads := ptrSliceToValSlice(overloads)
	candidates := NarrowOverloadCandidates(valOverloads, arityToInt(site.Arity), site.ArgumentTypes, hookCtx)

	if IsOverloadAmbiguousAfterNormalization(candidates, arityToInt(site.Arity)) {
		return pickResult{Ambiguous: true}
	}
	if len(candidates) > 1 {
		return pickResult{Ambiguous: true}
	}
	if len(candidates) == 1 {
		return pickResult{Def: &candidates[0]}
	}
	// Fallback to first overload when narrowing empties the set
	return pickResult{Def: overloads[0]}
}

// pickFirstNonStaticOnly is receiver-bound member lookup that filters
// static-only candidates BEFORE arity narrowing.
// Mirrors TS pickFirstNonStaticOnly.
func pickFirstNonStaticOnly(
	ownerID string,
	memberName string,
	site shared.ReferenceSite,
	semModel model.SemanticModel,
	provider ScopeResolver,
) pickResult {
	rawOverloads := semModel.Methods().LookupAllByOwner(ownerID, memberName)
	if len(rawOverloads) == 0 {
		// Non-callable — delegate to field lookup (no static-only filter)
		field := semModel.Fields().LookupFieldByOwner(ownerID, memberName)
		if field != nil {
			return pickResult{Def: field}
		}
		return pickResult{}
	}

	isStaticOnly := provider.IsStaticOnly()
	var overloads []*shared.SymbolDefinition
	filteredAny := false
	if isStaticOnly != nil {
		for _, candidate := range rawOverloads {
			if isStaticOnly(candidate) {
				filteredAny = true
				continue
			}
			overloads = append(overloads, candidate)
		}
	} else {
		overloads = rawOverloads
	}

	if len(overloads) == 0 {
		if filteredAny {
			return pickResult{StaticOnlyFiltered: true}
		}
		return pickResult{}
	}
	if len(overloads) == 1 {
		return pickResult{Def: overloads[0]}
	}

	hookCtx := buildOverloadHookCtx(site, provider)
	valOverloads := ptrSliceToValSlice(overloads)
	candidates := NarrowOverloadCandidates(valOverloads, arityToInt(site.Arity), site.ArgumentTypes, hookCtx)

	if IsOverloadAmbiguousAfterNormalization(candidates, arityToInt(site.Arity)) {
		return pickResult{Ambiguous: true}
	}
	if len(candidates) > 1 {
		return pickResult{Ambiguous: true}
	}
	if len(candidates) == 1 {
		return pickResult{Def: &candidates[0]}
	}
	return pickResult{Def: overloads[0]}
}

// buildOverloadHookCtx adapts ScopeResolver's ConversionRankFn/ConstraintCompatibility
// signatures to the OverloadNarrowingHookCtx interface.
func buildOverloadHookCtx(site shared.ReferenceSite, provider ScopeResolver) *OverloadNarrowingHookCtx {
	hookCtx := &OverloadNarrowingHookCtx{
		ArgumentTypeClasses: site.ArgumentTypeClasses,
	}
	if crf := provider.ConversionRankFn(); crf != nil {
		hookCtx.ConversionRankFn = func(argType, paramType string, _argTC, _paramTC *shared.ParameterTypeClass) float64 {
			return float64(crf(argType, paramType))
		}
	}
	if cc := provider.ConstraintCompatibility(); cc != nil {
		// Bridge: ScopeResolver.ConstraintCompatibility expects (ReferenceSite, ...) but
		// OverloadNarrowingHookCtx passes (Callsite, ...). We use the captured `site`
		// (ReferenceSite) instead of the callsite parameter, since Callsite is a subset
		// of ReferenceSite's data (just Arity + ArgumentTypes which site already has).
		hookCtx.ConstraintCompatibility = func(_callsite shared.Callsite, def shared.SymbolDefinition, _argTypes []string, argTypeClasses []shared.ParameterTypeClass) ArityVerdict {
			var ptc shared.ParameterTypeClass
			if len(argTypeClasses) > 0 {
				ptc = argTypeClasses[0]
			}
			return cc(site, def, ptc)
		}
	}
	return hookCtx
}

// ptrSliceToValSlice converts []*SymbolDefinition → []SymbolDefinition for
// NarrowOverloadCandidates which takes a value slice.
func ptrSliceToValSlice(ptrs []*shared.SymbolDefinition) []shared.SymbolDefinition {
	out := make([]shared.SymbolDefinition, len(ptrs))
	for i, p := range ptrs {
		if p != nil {
			out[i] = *p
		}
	}
	return out
}

func arityToInt(a *int) int {
	if a == nil {
		return -1
	}
	return *a
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }

// suppressDeletedCallTarget returns true if site is a call and the target
// def is marked IsDeleted. The callsite is suppressed (no edge emitted).
func suppressDeletedCallTarget(site *shared.ReferenceSite, target *shared.SymbolDefinition) bool {
	if site == nil || target == nil {
		return false
	}
	if site.Kind == shared.ReferenceCall && target.IsDeleted {
		return true
	}
	return false
}

// findOwnedMemberViaModel looks up a member by name on an owner using
// SemanticModel (methods first, then fields). Returns the first match.
func findOwnedMemberViaModel(ownerID, memberName string, semModel model.SemanticModel) *shared.SymbolDefinition {
	if semModel == nil {
		return nil
	}
	overloads := semModel.Methods().LookupAllByOwner(ownerID, memberName)
	if len(overloads) > 0 {
		return overloads[0]
	}
	return semModel.Fields().LookupFieldByOwner(ownerID, memberName)
}

// buildMROChain returns the method resolution order for a class defID.
func buildMROChain(defID string, indexes *model.ScopeResolutionIndexes, mro map[string][]string) []string {
	if indexes != nil {
		if md := indexes.MethodDispatch(); md != nil {
			if chain := md.MROByOwner(defID); len(chain) > 0 {
				return chain
			}
		}
	}
	if mro != nil {
		if chain, ok := mro[defID]; ok {
			return chain
		}
	}
	return []string{defID}
}

// mroChain returns the MRO chain from the legacy mro map.
func mroChain(defID string, mro map[string][]string) []string {
	if mro != nil {
		if chain, ok := mro[defID]; ok {
			return chain
		}
	}
	return []string{defID}
}

// buildImplementorsMap builds the map of interface defID → []*SymbolDefinition
// for interface dispatch. Uses MethodDispatchIndex.ImplsByInterface when available.
func buildImplementorsMap(
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	lookup *GraphNodeLookup,
) map[string][]*shared.SymbolDefinition {
	result := make(map[string][]*shared.SymbolDefinition)
	if indexes == nil {
		return result
	}

	// Collect all class-like defs with their NodeIDs
	classDefsByNodeID := make(map[string]*shared.SymbolDefinition)
	for _, pf := range parsedFiles {
		for i := range pf.LocalDefs {
			d := &pf.LocalDefs[i]
			if IsClassLike(d.Type) {
				classDefsByNodeID[d.NodeID] = d
			}
		}
	}

	md := indexes.MethodDispatch()
	if md == nil {
		return result
	}

	// For each interface in classDefsByNodeID, get implementors
	for defID, def := range classDefsByNodeID {
		if def.Type == shared.LabelInterface {
			impls := md.ImplsByInterface(defID)
			for _, implID := range impls {
				if implDef, ok := classDefsByNodeID[implID]; ok {
					result[defID] = append(result[defID], implDef)
				}
			}
		}
	}

	return result
}

// resolveClassBindingForName resolves a class name to its defID, using
// FindClassBindingInScope + qualified name matching with template argument
// normalization.
func resolveClassBindingForName(
	scopeID shared.ScopeID,
	rawClassName string,
	indexes *model.ScopeResolutionIndexes,
	wsIndex *WorkspaceResolutionIndex,
) string {
	classDef := FindClassBindingInScope(scopeID, rawClassName, indexes)
	if classDef != nil {
		return classDef.NodeID
	}

	// Fallback: strip template arguments and try qualified name matching
	strippedName := utils.StripTemplateArguments(rawClassName)
	templateArgs := utils.ExtractTemplateArguments(rawClassName)

	if indexes != nil {
		qn := indexes.QualifiedNames()
		if qn != nil {
			if defIDs := qn.Get(strippedName); len(defIDs) == 1 {
				return defIDs[0]
			}
			if len(templateArgs) > 0 {
				_ = normalizeTemplateArgs(templateArgs) // reserved for future matching
			}
		}
	}

	return ""
}

// normalizeTemplateArgs normalizes template arguments by trimming whitespace.
func normalizeTemplateArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.TrimSpace(a)
	}
	return out
}

// isSuperReceiver checks if the receiver name represents a super call.
func isSuperReceiver(provider ScopeResolver, receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	if fn := provider.IsSuperReceiverInContext(); fn != nil {
		return fn(receiverName, scopeID, indexes)
	}
	return provider.IsSuperReceiver(receiverName)
}

// EmitReceiverBoundCalls resolves method calls of the form `obj.method()`
// where the receiver type is known. Returns the number of edges emitted.
func EmitReceiverBoundCalls(
	provider ScopeResolver,
	indexes *model.ScopeResolutionIndexes,
	mro map[string][]string,
	lookup *GraphNodeLookup,
	graph_ shared.KnowledgeGraph,
) int {
	return EmitReceiverBoundCallsFull(provider, indexes, mro, lookup, graph_, nil, nil, nil)
}

// EmitReceiverBoundCallsFull is the full-arg version used by the orchestrator.
func EmitReceiverBoundCallsFull(
	provider ScopeResolver,
	indexes *model.ScopeResolutionIndexes,
	mro map[string][]string,
	lookup *GraphNodeLookup,
	graph_ shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	handledSites map[string]bool,
	semModel model.SemanticModel,
) int {
	if indexes == nil || parsedFiles == nil || handledSites == nil || semModel == nil {
		return 0
	}

	emitted := 0
	seen := make(map[string]bool) // I5: never pre-seed
	collapse := provider.CollapseMemberCallsByCallerTarget()
	hoistTypeBindingsToModule := provider.HoistTypeBindingsToModule()

	compoundOpts := ResolveCompoundReceiverOptions{
		FieldFallback:             provider.FieldFallbackOnMethodLookup(),
		HoistTypeBindingsToModule: hoistTypeBindingsToModule,
	}
	if unwrapFn := provider.UnwrapCollectionAccessor(); unwrapFn != nil {
		compoundOpts.UnwrapCollectionAccessor = func(receiverType, accessor string) string {
			syntheticDef := &shared.SymbolDefinition{NodeID: accessor, Type: shared.LabelMethod}
			qn := receiverType + "." + accessor
			syntheticDef.QualifiedName = &qn
			result := unwrapFn(syntheticDef)
			if result == nil {
				return ""
			}
			if result.QualifiedName != nil {
				return *result.QualifiedName
			}
			return result.NodeID
		}
	}

	implementorsByInterfaceDefId := buildImplementorsMap(indexes, parsedFiles, lookup)
	wsIndex := BuildWorkspaceResolutionIndex(parsedFiles)

	for _, parsed := range parsedFiles {
		namespaceTargets := BuildNamespaceTargets(parsed, indexes)

		for siteIdx := range parsed.ReferenceSites {
			site := &parsed.ReferenceSites[siteIdx]
			if site.Kind != shared.ReferenceCall && site.Kind != shared.ReferenceRead && site.Kind != shared.ReferenceWrite {
				continue
			}
			if site.ExplicitReceiver == nil {
				continue
			}

			receiverName := *site.ExplicitReceiver
			memberName := site.SymbolName
			siteKey := parsed.FilePath + ":" + itoa(site.Range.StartLine) + ":" + itoa(site.Range.StartCol)

			emittedThisSite := emitReceiverBoundCallForSite(
				provider, indexes, mro, lookup, graph_, semModel,
				parsed, site, receiverName, memberName, siteKey,
				seen, collapse, compoundOpts, wsIndex,
				namespaceTargets, implementorsByInterfaceDefId,
				handledSites, parsedFiles,
			)
			emitted += emittedThisSite
		}
	}

	return emitted
}

// emitReceiverBoundCallForSite handles a single referenceSite through all 7 cases.
// Contract Invariant I4 — case order is load-bearing.
func emitReceiverBoundCallForSite(
	provider ScopeResolver,
	indexes *model.ScopeResolutionIndexes,
	mro map[string][]string,
	lookup *GraphNodeLookup,
	graph_ shared.KnowledgeGraph,
	semModel model.SemanticModel,
	parsed *shared.ParsedFile,
	site *shared.ReferenceSite,
	receiverName string,
	memberName string,
	siteKey string,
	seen map[string]bool,
	collapse bool,
	compoundOpts ResolveCompoundReceiverOptions,
	wsIndex *WorkspaceResolutionIndex,
	namespaceTargets map[string][]string,
	implementorsByInterfaceDefId map[string][]*shared.SymbolDefinition,
	handledSites map[string]bool,
	parsedFiles []*shared.ParsedFile,
) int {
	// ── super branch ─────────────────────────────────────────
	if isSuperReceiver(provider, receiverName, site.InScope, indexes) {
		enclosingClass := FindEnclosingClassDef(site.InScope, indexes)
		if enclosingClass != nil {
			var ancestors []string
			if md := indexes.MethodDispatch(); md != nil {
				if extMRO := md.ExtendsOnlyMRO(enclosingClass.NodeID); len(extMRO) > 0 {
					ancestors = extMRO
				} else {
					ancestors = md.MROByOwner(enclosingClass.NodeID)
				}
			} else {
				ancestors = mroChain(enclosingClass.NodeID, mro)
			}
			for _, ownerID := range ancestors {
				var picked pickResult
				if site.Kind == shared.ReferenceCall {
					picked = pickOverload(ownerID, memberName, *site, semModel, provider)
				} else {
					picked = pickResult{Def: findOwnedMemberViaModel(ownerID, memberName, semModel)}
				}
				if picked.Ambiguous {
					handledSites[siteKey] = true
					return 0
				}
				if picked.Def != nil {
					if suppressDeletedCallTarget(site, picked.Def) {
						handledSites[siteKey] = true
						return 0
					}
					si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
					if TryEmitEdge(graph_, indexes, lookup, si, picked.Def, "global", seen, 0.85, collapse) {
						handledSites[siteKey] = true
						return 1
					}
					handledSites[siteKey] = true
					return 0
				}
			}
		}
	}

	// ── Case 0: compound receiver ────────────────────────────
	if strings.Contains(receiverName, ".") || strings.Contains(receiverName, "(") {
		currentClass := ResolveCompoundReceiverClass(receiverName, site.InScope, indexes, wsIndex, compoundOpts, 0)
		if currentClass != nil {
			chain := buildMROChain(currentClass.NodeID, indexes, mro)
			for _, ownerID := range chain {
				var picked pickResult
				if site.Kind == shared.ReferenceCall {
					picked = pickFirstNonStaticOnly(ownerID, memberName, *site, semModel, provider)
				} else {
					picked = pickResult{Def: findOwnedMemberViaModel(ownerID, memberName, semModel)}
				}
				if picked.Ambiguous {
					handledSites[siteKey] = true
					return 0
				}
				if picked.StaticOnlyFiltered {
					continue
				}
				if picked.Def == nil {
					continue
				}
				if suppressDeletedCallTarget(site, picked.Def) {
					handledSites[siteKey] = true
					return 0
				}
				reason := "global"
				if picked.Def.FilePath != parsed.FilePath {
					reason = "import-resolved"
				}
				si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
				if TryEmitEdge(graph_, indexes, lookup, si, picked.Def, reason, seen, 0.85, collapse) {
					handledSites[siteKey] = true
					return 1
				}
				handledSites[siteKey] = true
				return 0
			}
		}
	}

	// ── Case 0.5: implicit `this` receiver ──────────────────
	if provider.ResolveThisViaEnclosingClass() && receiverName == "this" {
		enclosingClass := FindEnclosingClassDef(site.InScope, indexes)
		if enclosingClass != nil {
			if resolveReceiverMember := provider.ResolveReceiverMember(); resolveReceiverMember != nil {
				languageRes := resolveReceiverMember(enclosingClass, memberName, *site, indexes, semModel)
				if languageRes != nil && languageRes.Kind == "ambiguous" {
					handledSites[siteKey] = true
					return 0
				}
				if languageRes != nil && languageRes.Kind == "resolved" && languageRes.Definition != nil {
					memberDef := languageRes.Definition
					if suppressDeletedCallTarget(site, memberDef) {
						handledSites[siteKey] = true
						return 0
					}
					reason := "global"
					confidence := 0.85
					if site.Kind == shared.ReferenceWrite || site.Kind == shared.ReferenceRead {
						reason = string(site.Kind)
						confidence = 1.0
					} else if memberDef.FilePath != parsed.FilePath {
						reason = "import-resolved"
					}
					si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
					if TryEmitEdge(graph_, indexes, lookup, si, memberDef, reason, seen, confidence, collapse) {
						handledSites[siteKey] = true
						return 1
					}
					handledSites[siteKey] = true
					return 0
				}
			}
			// Generic MRO walk for Case 0.5
			chain := buildMROChain(enclosingClass.NodeID, indexes, mro)
			for _, ownerID := range chain {
				methodOverloads := semModel.Methods().LookupAllByOwner(ownerID, memberName)
				if len(methodOverloads) > 0 {
					hookCtx := buildOverloadHookCtx(*site, provider)
					valOv := ptrSliceToValSlice(methodOverloads)
					narrowed := NarrowOverloadCandidates(valOv, arityToInt(site.Arity), site.ArgumentTypes, hookCtx)
					if IsOverloadAmbiguousAfterNormalization(narrowed, arityToInt(site.Arity)) || len(narrowed) > 1 {
						handledSites[siteKey] = true
						return 0
					}
					var memberDef *shared.SymbolDefinition
					if len(narrowed) == 1 {
						memberDef = &narrowed[0]
					} else if len(narrowed) == 0 && len(methodOverloads) > 0 {
						handledSites[siteKey] = true
						return 0
					}
					if memberDef != nil {
						if suppressDeletedCallTarget(site, memberDef) {
							handledSites[siteKey] = true
							return 0
						}
						reason := "global"
						confidence := 0.85
						if site.Kind == shared.ReferenceWrite || site.Kind == shared.ReferenceRead {
							reason = string(site.Kind)
							confidence = 1.0
						}
						si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
						if TryEmitEdge(graph_, indexes, lookup, si, memberDef, reason, seen, confidence, collapse) {
							handledSites[siteKey] = true
							return 1
						}
					}
					handledSites[siteKey] = true
					return 0
				}
				if f := semModel.Fields().LookupFieldByOwner(ownerID, memberName); f != nil {
					if suppressDeletedCallTarget(site, f) {
						handledSites[siteKey] = true
						return 0
					}
					reason := "global"
					confidence := 0.85
					if site.Kind == shared.ReferenceWrite || site.Kind == shared.ReferenceRead {
						reason = string(site.Kind)
						confidence = 1.0
					}
					si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
					if TryEmitEdge(graph_, indexes, lookup, si, f, reason, seen, confidence, collapse) {
						handledSites[siteKey] = true
						return 1
					}
					handledSites[siteKey] = true
					return 0
				}
			}
		}
	}

	// ── Case 1: namespace receiver ──────────────────────────
	if targetFiles, ok := namespaceTargets[receiverName]; ok {
		if provider.ResolveQualifiedReceiverMember() == nil {
			for _, targetFile := range targetFiles {
				memberDef := FindExportedDef(targetFile, memberName, wsIndex)
				if memberDef != nil {
					if suppressDeletedCallTarget(site, memberDef) {
						handledSites[siteKey] = true
						return 0
					}
					reason := "global"
					if memberDef.FilePath != parsed.FilePath {
						reason = "import-resolved"
					}
					si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
					if TryEmitEdge(graph_, indexes, lookup, si, memberDef, reason, seen, 0.85, collapse) {
						handledSites[siteKey] = true
						return 1
					}
					handledSites[siteKey] = true
					return 0
				}
			}
		}
	}

	// ── Case 1.5: qualified receiver member (C++ qualified-name) ──
	if resolveQualifiedFn := provider.ResolveQualifiedReceiverMember(); resolveQualifiedFn != nil {
		qResult := resolveQualifiedFn(receiverName, memberName, site.InScope, indexes, parsedFiles, *site)
		if qResult != nil {
			if qResult.Ambiguous {
				handledSites[siteKey] = true
				return 0
			}
			if qResult.Def != nil {
				if suppressDeletedCallTarget(site, qResult.Def) {
					handledSites[siteKey] = true
					return 0
				}
				si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
				if TryEmitEdge(graph_, indexes, lookup, si, qResult.Def, "global", seen, 0.85, collapse) {
					handledSites[siteKey] = true
					return 1
				}
				handledSites[siteKey] = true
				return 0
			}
		}
	}

	// ── Case 2: bare class-name receiver ───────────────────
	classBinding := FindClassBindingInScope(site.InScope, receiverName, indexes)
	if classBinding != nil {
		classDefID := resolveClassBindingForName(site.InScope, receiverName, indexes, wsIndex)
		if classDefID != "" {
			chain := buildMROChain(classDefID, indexes, mro)
			var memberDef *shared.SymbolDefinition
			ambiguous := false
			for _, ownerID := range chain {
				if site.Kind == shared.ReferenceCall {
					picked := pickOverload(ownerID, memberName, *site, semModel, provider)
					if picked.Ambiguous {
						ambiguous = true
						break
					}
					if picked.Def != nil {
						memberDef = picked.Def
						break
					}
				} else {
					if f := findOwnedMemberViaModel(ownerID, memberName, semModel); f != nil {
						memberDef = f
						break
					}
				}
			}
			if ambiguous {
				handledSites[siteKey] = true
				return 0
			}
			if memberDef != nil {
				if suppressDeletedCallTarget(site, memberDef) {
					handledSites[siteKey] = true
					return 0
				}
				reason := "global"
				if memberDef.FilePath != parsed.FilePath {
					reason = "import-resolved"
				}
				si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
				if TryEmitEdge(graph_, indexes, lookup, si, memberDef, reason, seen, 0.85, collapse) {
					handledSites[siteKey] = true
					return 1
				}
				handledSites[siteKey] = true
				return 0
			}
		}
	}

	// ── Case 3: dotted type-binding via namespaceTargets ──
	if strings.Contains(receiverName, ".") {
		parts := strings.SplitN(receiverName, ".", 2)
		if len(parts) == 2 {
			nsPart := parts[0]
			classPart := parts[1]
			// Resolve namespace part via namespaceTargets → targetFiles
			if targetFiles, ok := namespaceTargets[nsPart]; ok {
				for _, targetFile := range targetFiles {
					classMemberDef := FindExportedDef(targetFile, classPart, wsIndex)
					if classMemberDef != nil {
						chain := buildMROChain(classMemberDef.NodeID, indexes, mro)
						for _, ownerID := range chain {
							picked := pickOverload(ownerID, memberName, *site, semModel, provider)
							if picked.Ambiguous {
								break
							}
							if picked.Def != nil {
								if suppressDeletedCallTarget(site, picked.Def) {
									handledSites[siteKey] = true
									return 0
								}
								si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
								if TryEmitEdge(graph_, indexes, lookup, si, picked.Def, "global", seen, 0.85, collapse) {
									handledSites[siteKey] = true
									return 1
								}
								handledSites[siteKey] = true
								return 0
							}
						}
					}
				}
			}
		}
	}

	// ── Case 3b: chain type-binding via compound resolver ──
	if strings.Contains(receiverName, ".") {
		currentClass := ResolveCompoundReceiverClass(receiverName, site.InScope, indexes, wsIndex, compoundOpts, 0)
		if currentClass != nil {
			chain := buildMROChain(currentClass.NodeID, indexes, mro)
			allFilteredStaticOnly := true
			for _, ownerID := range chain {
				var picked pickResult
				if site.Kind == shared.ReferenceCall {
					picked = pickFirstNonStaticOnly(ownerID, memberName, *site, semModel, provider)
				} else {
					picked = pickResult{Def: findOwnedMemberViaModel(ownerID, memberName, semModel)}
				}
				if picked.Ambiguous {
					handledSites[siteKey] = true
					return 0
				}
				if picked.StaticOnlyFiltered {
					continue
				}
				allFilteredStaticOnly = false
				if picked.Def == nil {
					continue
				}
				if suppressDeletedCallTarget(site, picked.Def) {
					handledSites[siteKey] = true
					return 0
				}
				reason := "global"
				if picked.Def.FilePath != parsed.FilePath {
					reason = "import-resolved"
				}
				si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
				if TryEmitEdge(graph_, indexes, lookup, si, picked.Def, reason, seen, 0.85, collapse) {
					handledSites[siteKey] = true
					return 1
				}
				handledSites[siteKey] = true
				return 0
			}
			if allFilteredStaticOnly {
				handledSites[siteKey] = true
				return 0
			}
		}
	}

	// ── Case 4: simple type-binding receiver ──────────────
	receiverTypeBinding := FindReceiverTypeBinding(site.InScope, receiverName, indexes)
	if receiverTypeBinding != nil && receiverTypeBinding.RawName != "" {
		ownerDefID := resolveClassBindingForName(site.InScope, receiverTypeBinding.RawName, indexes, wsIndex)
		if ownerDefID != "" {
			chain := buildMROChain(ownerDefID, indexes, mro)
			allFilteredStaticOnly := true
			for _, ownerID := range chain {
				var picked pickResult
				if site.Kind == shared.ReferenceCall {
					picked = pickFirstNonStaticOnly(ownerID, memberName, *site, semModel, provider)
				} else {
					picked = pickResult{Def: findOwnedMemberViaModel(ownerID, memberName, semModel)}
				}
				if picked.Ambiguous {
					handledSites[siteKey] = true
					return 0
				}
				if picked.StaticOnlyFiltered {
					continue
				}
				allFilteredStaticOnly = false
				if picked.Def == nil {
					continue
				}
				if suppressDeletedCallTarget(site, picked.Def) {
					handledSites[siteKey] = true
					return 0
				}
				reason := "global"
				confidence := 0.85
				if picked.Def.FilePath != parsed.FilePath {
					reason = "import-resolved"
				}
				si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
				if TryEmitEdge(graph_, indexes, lookup, si, picked.Def, reason, seen, confidence, collapse) {
					handledSites[siteKey] = true
					emitInterfaceDispatchFor(memberName, ownerDefID, indexes, implementorsByInterfaceDefId, graph_, lookup, seen, site, semModel, provider, collapse)
					return 1
				}
				handledSites[siteKey] = true
				return 0
			}
			if allFilteredStaticOnly {
				handledSites[siteKey] = true
				return 0
			}
		}
	}

	// ── Case 5: value-bridge (variable/parameter receiver) ──
	valueBinding := FindValueBindingInScope(site.InScope, receiverName, indexes)
	if valueBinding != nil {
		var memberDef *shared.SymbolDefinition
		if site.Kind == shared.ReferenceCall {
			picked := pickOverload(valueBinding.NodeID, memberName, *site, semModel, provider)
			if picked.Ambiguous {
				handledSites[siteKey] = true
				return 0
			}
			memberDef = picked.Def
		} else {
			memberDef = findOwnedMemberViaModel(valueBinding.NodeID, memberName, semModel)
		}
		if memberDef != nil {
			if isStaticOnlyFn := provider.IsStaticOnly(); isStaticOnlyFn != nil {
				if isStaticOnlyFn(memberDef) {
					handledSites[siteKey] = true
					return 0
				}
			}
			if suppressDeletedCallTarget(site, memberDef) {
				handledSites[siteKey] = true
				return 0
			}
			si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
			if lookup != nil {
				if targetID := ResolveDefGraphID(valueBinding.FilePath, valueBinding, lookup); targetID != "" {
					if TryEmitEdgeWithExplicitTargetId(graph_, indexes, lookup, si, targetID, "global", seen, 0.85, collapse) {
						handledSites[siteKey] = true
						return 1
					}
					handledSites[siteKey] = true
					return 0
				}
			}
			if TryEmitEdge(graph_, indexes, lookup, si, memberDef, "global", seen, 0.85, collapse) {
				handledSites[siteKey] = true
				return 1
			}
			handledSites[siteKey] = true
			return 0
		}
	}

	return 0 // no case matched
}

// emitInterfaceDispatchFor emits secondary CALLS edges from the caller to
// implementations of an interface, when the primary target is an interface method.
func emitInterfaceDispatchFor(
	memberName string,
	ownerDefID string,
	indexes *model.ScopeResolutionIndexes,
	implementorsByInterfaceDefId map[string][]*shared.SymbolDefinition,
	g shared.KnowledgeGraph,
	lookup *GraphNodeLookup,
	seen map[string]bool,
	site *shared.ReferenceSite,
	semModel model.SemanticModel,
	provider ScopeResolver,
	collapse bool,
) {
	// Check that the owner is actually an interface via DefIndex
	isInterface := false
	if indexes != nil {
		if defIdx := indexes.Defs(); defIdx != nil {
			if def := defIdx.Get(ownerDefID); def != nil && def.Type == shared.LabelInterface {
				isInterface = true
			}
		}
	}
	if !isInterface {
		return
	}

	impls, ok := implementorsByInterfaceDefId[ownerDefID]
	if !ok {
		return
	}
	for _, implDef := range impls {
		picked := pickOverload(implDef.NodeID, memberName, *site, semModel, provider)
		if picked.Ambiguous || picked.Def == nil {
			continue
		}
		si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
		TryEmitEdge(g, indexes, lookup, si, picked.Def, "interface-dispatch", seen, 0.7, collapse)
	}
}

// ResolveReceiverType resolves the type of a receiver name in a given scope.
func ResolveReceiverType(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) string {
	if indexes == nil {
		return ""
	}
	tb := FindReceiverTypeBinding(scopeID, receiverName, indexes)
	if tb != nil {
		return tb.RawName
	}
	cb := FindClassBindingInScope(scopeID, receiverName, indexes)
	if cb != nil {
		return cb.NodeID
	}
	return ""
}
