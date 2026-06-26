package scope_resolution

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// free_call_fallback.go — emit CALLS edges for free-call reference sites.
// Ported from TS scope-resolution/passes/free-call-fallback.ts (847 lines).
//
// Contract invariant I1: this pass runs AFTER EmitReceiverBoundCalls.
// Contract invariant I2: free calls collapse to one CALLS edge per
// (caller, target) pair regardless of how many call sites the caller contains.
// ---------------------------------------------------------------------------

// FreeCallOptions bundles per-language optional hooks for EmitFreeCallFallback.
// Mirrors TS options parameter in emitFreeCallFallback.
type FreeCallOptions struct {
	// AllowGlobalFallback enables workspace-wide unique-name fallback when
	// scope-chain lookup fails (gated per language).
	AllowGlobalFallback bool

	// ConstructorCallTargetsClass — when true, `Type(...)` constructor calls
	// link to the Class def itself rather than its explicit Constructor.
	// Swift opts in.
	ConstructorCallTargetsClass bool

	// IsFileLocalDef returns true for file-local defs (e.g. C static functions)
	// that are invisible from other files. nil means no filtering.
	IsFileLocalDef func(def *shared.SymbolDefinition) bool

	// IsCallableVisibleFromCaller gates candidate visibility from the caller's
	// perspective (e.g. PHP namespace + use-function import gating).
	// nil means no filtering.
	IsCallableVisibleFromCaller func(ctx CallableVisibilityContext) bool

	// ResolveAdlCandidates is the C++ argument-dependent lookup hook.
	// nil means ADL is not applicable.
	ResolveAdlCandidates func(
		site AdlSiteInfo,
		callerParsed *shared.ParsedFile,
		scopes *model.ScopeResolutionIndexes,
		parsedFiles []*shared.ParsedFile,
	) []*shared.SymbolDefinition

	// ConversionRankFn is the conversion-rank scoring for overload resolution.
	ConversionRankFn ConversionRankFn

	// ConstraintCompatibility drops candidates whose template constraints
	// provably fail at the call site. nil if not applicable.
	ConstraintCompatibility func(
		callsite shared.ReferenceSite,
		def shared.SymbolDefinition,
		ctx shared.ParameterTypeClass,
	) ArityVerdict
}

// CallableVisibilityContext is the argument to IsCallableVisibleFromCaller.
type CallableVisibilityContext struct {
	CallerParsed *shared.ParsedFile
	Candidate    *shared.SymbolDefinition
	CallerScope  shared.ScopeID
	Scopes       *model.ScopeResolutionIndexes
}

// AdlSiteInfo describes the call site for ADL resolution.
type AdlSiteInfo struct {
	Name          string
	Arity         *int
	ArgumentTypes []string
	AtRange       shared.Range
}

// EmitFreeCallFallback resolves call sites that were NOT handled by the
// receiver-bound pass. These are "free" calls — bare function calls like
// `foo()` without an explicit receiver.
//
// Mirrors TS scope-resolution/passes/free-call-fallback.ts emitFreeCallFallback.
//
// Contract invariant I1: this pass runs AFTER EmitReceiverBoundCalls
// (so handledSites already contains the sites the receiver pass resolved).
// Contract invariant I2: free calls collapse to one CALLS edge per
// (caller, target) pair regardless of how many call sites the caller contains.
func EmitFreeCallFallback(
	g shared.KnowledgeGraph,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
	handledSites map[string]bool,
	parsedFiles []*shared.ParsedFile,
	semModel model.SemanticModel,
	wsIndex *WorkspaceResolutionIndex,
	options *FreeCallOptions,
) int {
	if options == nil {
		options = &FreeCallOptions{}
	}

	emitted := 0
	seen := make(map[string]bool) // per-(caller,target) dedup (I2)

	// Build O(1) simple-name → callable defs index once per pass
	globalCallablesBySimpleName := buildGlobalCallableIndex(indexes)
	// Sibling index for constructor-form class fallback
	globalClassesBySimpleName := buildGlobalClassIndex(indexes)
	// Per-pass memo for pickUniqueGlobalCallable's post-filter candidate list
	var scopeDefsCache map[string][]*shared.SymbolDefinition
	if options.IsCallableVisibleFromCaller == nil {
		scopeDefsCache = make(map[string][]*shared.SymbolDefinition)
	}

	for _, parsed := range parsedFiles {
		for siteIdx := range parsed.ReferenceSites {
			site := &parsed.ReferenceSites[siteIdx]
			if site.Kind != shared.ReferenceCall {
				continue
			}
			if site.ExplicitReceiver != nil {
				continue
			}

			siteKeyStr := fmt.Sprintf("%s:%d:%d", parsed.FilePath, site.Range.StartLine, site.Range.StartCol)
			if handledSites[siteKeyStr] {
				continue
			}

			// Constructor form (`new User(...)`): resolve the class, then
			// emit CALLS to its explicit Constructor def (when present) or
			// to the Class node itself (implicit constructor).
			var fnDef *shared.SymbolDefinition
			if site.CallForm == shared.CallFormConstructor {
				classDef := ResolveInheritanceBaseInScope(site.InScope, site.SymbolName, indexes)
				if classDef != nil && classDef.Type != shared.LabelInterface {
					if options.ConstructorCallTargetsClass {
						fnDef = classDef
					} else {
						fnDef = pickConstructorOrClass(classDef, wsIndex, indexes, site.Arity)
					}
				} else if options.AllowGlobalFallback {
					globalClass := pickUniqueGlobalClass(site.SymbolName, globalClassesBySimpleName)
					if globalClass != nil {
						if globalClass.Type == shared.LabelInterface {
							fnDef = nil
						} else if options.ConstructorCallTargetsClass {
							fnDef = globalClass
						} else {
							fnDef = pickConstructorOrClass(globalClass, wsIndex, indexes, site.Arity)
						}
					}
				}
			}

			// Implicit-this overload narrowing: an unqualified call inside
			// a method body might be calling a sibling overload on the
			// enclosing class.
			fnDefFromImplicitThis := false
			if fnDef == nil {
				fnDef = pickImplicitThisOverload(site, indexes, wsIndex, semModel, options)
				if fnDef != nil {
					fnDefFromImplicitThis = true
				}
			}

			// Scope-chain callable lookup.
			if fnDef == nil {
				if options.ResolveAdlCandidates == nil {
					// Non-ADL path: first-match preserves scope-chain precedence
					fnDef = FindCallableBindingInScope(site.InScope, site.SymbolName, indexes)
					if fnDef != nil && options.ConversionRankFn != nil {
						allCallables := FindAllCallableBindingsInScope(site.InScope, site.SymbolName, indexes)
						if len(allCallables) > 1 {
							hookCtx := buildFreeCallHookCtx(site, options)
							narrowed := NarrowOverloadCandidates(
								ptrSliceToValSlice(allCallables),
								arityToInt(site.Arity),
								site.ArgumentTypes,
								hookCtx,
							)
							if len(narrowed) == 1 {
								fnDef = &narrowed[0]
							} else if len(narrowed) > 1 {
								// Multiple survivors after conversion-rank scoring.
								// Suppress when all candidates share the same file (true overloads).
								sameFile := true
								for _, d := range narrowed {
									if d.FilePath != narrowed[0].FilePath {
										sameFile = false
										break
									}
								}
								if sameFile {
									handledSites[siteKeyStr] = true
									continue
								}
							}
							// narrowed.length === 0: keep the first-match fnDef
						}
					}
				} else {
					// ADL path
					adlResult := FindCallableBindingsAndAdlBlocker(site.InScope, site.SymbolName, indexes)
					ordinary := adlResult.Callables
					adlSuppressed := adlResult.NonCallableFound || adlResult.BlockScopeDeclFound

					var adl []*shared.SymbolDefinition
					if !adlSuppressed {
						adl = options.ResolveAdlCandidates(
							AdlSiteInfo{
								Name:          site.SymbolName,
								Arity:         site.Arity,
								ArgumentTypes: site.ArgumentTypes,
								AtRange: shared.Range{
									StartLine: site.Range.StartLine,
									StartCol:  site.Range.StartCol,
								},
							},
							parsed,
							indexes,
							parsedFiles,
						)
					}

					key := siteKeyStr
					if adlSuppressed && len(ordinary) == 0 {
						handledSites[key] = true
						continue
					}
					if len(adl) == 0 {
						// No ADL contribution. Default: ordinary[0]
						hasConstraints := false
						for _, d := range ordinary {
							if d.TemplateArguments != nil && len(d.TemplateArguments) > 0 {
								hasConstraints = true
								break
							}
						}
						canNarrow := hasConstraints || options.ConversionRankFn != nil
						if len(ordinary) <= 1 || !canNarrow {
							if len(ordinary) > 0 {
								fnDef = ordinary[0]
							}
						} else {
							hookCtx := buildFreeCallHookCtx(site, options)
							narrowed := NarrowOverloadCandidates(
								ptrSliceToValSlice(ordinary),
								arityToInt(site.Arity),
								site.ArgumentTypes,
								hookCtx,
							)
							if len(narrowed) == 1 {
								fnDef = &narrowed[0]
							} else if len(narrowed) == 0 {
								handledSites[key] = true
								continue
							} else {
								// >1 survivors: same-file → suppress; cross-file → first-match
								sameFile := true
								for _, d := range narrowed {
									if d.FilePath != narrowed[0].FilePath {
										sameFile = false
										break
									}
								}
								if sameFile {
									handledSites[key] = true
									continue
								}
								if len(ordinary) > 0 {
									fnDef = ordinary[0]
								}
							}
						}
					} else {
						// ADL + ordinary merge
						var merged []shared.SymbolDefinition
						seenMerge := make(map[string]bool)
						push := func(defs []*shared.SymbolDefinition) {
							for _, d := range defs {
								if seenMerge[d.NodeID] {
									continue
								}
								seenMerge[d.NodeID] = true
								merged = append(merged, *d)
							}
						}
						push(ordinary)
						push(adl)

						hookCtx := buildFreeCallHookCtx(site, options)
						narrowed := NarrowOverloadCandidates(
							merged,
							arityToInt(site.Arity),
							site.ArgumentTypes,
							hookCtx,
						)
						if len(narrowed) == 1 {
							fnDef = &narrowed[0]
						} else if len(narrowed) == 0 {
							handledSites[key] = true
							continue
						} else {
							handledSites[key] = true
							continue
						}
					}
				}
			}

			// Global fallback: unique workspace-wide callable by simple name
			if fnDef == nil && options.AllowGlobalFallback {
				fnDef = pickUniqueGlobalCallable(
					site.SymbolName,
					semModel,
					globalCallablesBySimpleName,
					parsed.FilePath,
					options,
					site,
					scopeDefsCache,
				)
			}

			if fnDef == nil {
				continue
			}

			// Suppress deleted call targets
			if fnDef.IsDeleted {
				handledSites[siteKeyStr] = true
				continue
			}

			// Visibility check for implicit-this / method / constructor
			if (fnDefFromImplicitThis || fnDef.Type == shared.LabelMethod || fnDef.Type == shared.LabelConstructor) &&
				options.IsCallableVisibleFromCaller != nil &&
				!options.IsCallableVisibleFromCaller(CallableVisibilityContext{
					CallerParsed: parsed,
					Candidate:    fnDef,
					CallerScope:  site.InScope,
					Scopes:       indexes,
				}) {
				handledSites[siteKeyStr] = true
				continue
			}

			callerGraphId := ResolveCallerGraphID(site.InScope, indexes, lookup, &site.Range)
			if callerGraphId == "" {
				continue
			}
			tgtGraphId := ResolveDefGraphID(fnDef.FilePath, fnDef, lookup)
			if tgtGraphId == "" {
				continue
			}

			// Always mark the site as handled (I2 contract)
			handledSites[siteKeyStr] = true
			relId := fmt.Sprintf("rel:CALLS:%s->%s", callerGraphId, tgtGraphId)
			if seen[relId] {
				continue
			}
			seen[relId] = true

			reason := "local-call"
			if fnDef.FilePath != parsed.FilePath {
				reason = "import-resolved"
			}

			edge := &graph.Edge{
				ID:     relId,
				Type:   graph.RelCalls,
				Source: callerGraphId,
				Target: tgtGraphId,
			}
			edge.Props.SetProp("confidence", 0.85)
			edge.Props.SetProp("reason", reason)
			shared.SetEdgeEvidence(edge, []shared.Evidence{
				{Kind: "free-call-fallback", Weight: 0.85, Note: reason},
			})
			g.AddEdge(edge)
			emitted++
		}
	}
	return emitted
}

// LookupFreeCall resolves a bare function call (no receiver) by walking
// the scope chain and import bindings to find the matching definition.
// Mirrors TS lookupFreeCall.
func LookupFreeCall(
	symbolName string,
	scopeID shared.ScopeID,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) *ReceiverMemberResolution {
	def := FindCallableBindingInScope(scopeID, symbolName, indexes)
	if def != nil {
		return &ReceiverMemberResolution{
			Kind:       "resolved",
			Definition: def,
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildGlobalCallableIndex builds a simpleName → callable defs index from
// indexes.Defs(). Mirrors TS buildGlobalCallableIndex.
func buildGlobalCallableIndex(indexes *model.ScopeResolutionIndexes) map[string][]*shared.SymbolDefinition {
	out := make(map[string][]*shared.SymbolDefinition)
	if indexes == nil {
		return out
	}
	defs := indexes.Defs()
	if defs == nil {
		return out
	}
	for _, def := range defs.Entries() {
		if def.Type != shared.LabelFunction && def.Type != shared.LabelMethod && def.Type != shared.LabelConstructor {
			continue
		}
		if def.QualifiedName == nil || *def.QualifiedName == "" {
			continue
		}
		simple := *def.QualifiedName
		if idx := strings.LastIndex(*def.QualifiedName, "."); idx != -1 {
			simple = (*def.QualifiedName)[idx+1:]
		}
		d := def // capture loop variable
		out[simple] = append(out[simple], &d)
	}
	return out
}

// buildGlobalClassIndex builds a simpleName → class-like defs index from
// indexes.Defs(). Mirrors TS buildGlobalClassIndex.
// Kind filter: Class | Struct | Interface (KTD5).
func buildGlobalClassIndex(indexes *model.ScopeResolutionIndexes) map[string][]*shared.SymbolDefinition {
	out := make(map[string][]*shared.SymbolDefinition)
	if indexes == nil {
		return out
	}
	defs := indexes.Defs()
	if defs == nil {
		return out
	}
	for _, def := range defs.Entries() {
		if def.Type != shared.LabelClass && def.Type != shared.LabelStruct && def.Type != shared.LabelInterface {
			continue
		}
		if def.QualifiedName == nil || *def.QualifiedName == "" {
			continue
		}
		simple := *def.QualifiedName
		if idx := strings.LastIndex(*def.QualifiedName, "."); idx != -1 {
			simple = (*def.QualifiedName)[idx+1:]
		}
		d := def
		out[simple] = append(out[simple], &d)
	}
	return out
}

// pickUniqueGlobalCallable resolves a free call to a globally-unique
// callable def by simple name. Mirrors TS pickUniqueGlobalCallable.
func pickUniqueGlobalCallable(
	name string,
	semModel model.SemanticModel,
	globalCallablesBySimpleName map[string][]*shared.SymbolDefinition,
	callerFilePath string,
	options *FreeCallOptions,
	site *shared.ReferenceSite,
	scopeDefsCache map[string][]*shared.SymbolDefinition,
) *shared.SymbolDefinition {
	// Build scope-index candidate list with file-local + visibility filtering
	var cacheKey string
	if scopeDefsCache != nil && options.IsCallableVisibleFromCaller == nil {
		cacheKey = name + "\x00" + callerFilePath
	}

	var scopeDefs []*shared.SymbolDefinition
	if cacheKey != "" {
		if cached, ok := scopeDefsCache[cacheKey]; ok {
			scopeDefs = cached
		}
	}

	if scopeDefs == nil {
		var built []*shared.SymbolDefinition
		scopeSeen := make(map[string]bool)
		for _, def := range globalCallablesBySimpleName[name] {
			// Skip file-local defs from a different file
			if options.IsFileLocalDef != nil && def.FilePath != callerFilePath && options.IsFileLocalDef(def) {
				continue
			}
			// Caller-side visibility filter
			if options.IsCallableVisibleFromCaller != nil &&
				!options.IsCallableVisibleFromCaller(CallableVisibilityContext{
					Candidate:   def,
					CallerScope: site.InScope,
					Scopes:      nil, // indexes not available in this path
				}) {
				continue
			}
			key := logicalCallableKey(def)
			if scopeSeen[key] {
				continue
			}
			scopeSeen[key] = true
			built = append(built, def)
		}
		scopeDefs = built
		if cacheKey != "" && scopeDefsCache != nil {
			scopeDefsCache[cacheKey] = built
		}
	}

	if len(scopeDefs) == 1 {
		return scopeDefs[0]
	}

	// Arity narrowing
	if len(scopeDefs) > 1 && site.Arity != nil {
		arityMatch := narrowByArity(scopeDefs, *site.Arity)
		if arityMatch != nil {
			return arityMatch
		}
	}

	// Overload narrowing with argument types + conversion ranking
	if len(scopeDefs) > 1 {
		hookCtx := buildFreeCallHookCtx(site, options)
		narrowed := NarrowOverloadCandidates(
			ptrSliceToValSlice(scopeDefs),
			arityToInt(site.Arity),
			site.ArgumentTypes,
			hookCtx,
		)
		if len(narrowed) == 1 {
			return &narrowed[0]
		}
	}

	// SemanticModel fallback
	if semModel == nil {
		return nil
	}
	var defs []*shared.SymbolDefinition
	seenSet := make(map[string]bool)
	push := func(pool []*shared.SymbolDefinition) {
		for _, def := range pool {
			if options.IsFileLocalDef != nil && def.FilePath != callerFilePath && options.IsFileLocalDef(def) {
				continue
			}
			if options.IsCallableVisibleFromCaller != nil &&
				!options.IsCallableVisibleFromCaller(CallableVisibilityContext{
					Candidate:   def,
					CallerScope: site.InScope,
				}) {
				continue
			}
			key := logicalCallableKey(def)
			if seenSet[key] {
				continue
			}
			seenSet[key] = true
			defs = append(defs, def)
		}
	}

	push(semModel.Symbols().LookupCallableByName(name))
	push(semModel.Methods().LookupMethodByName(name))

	if len(defs) == 1 {
		return defs[0]
	}

	// Arity narrowing on model pool
	if len(defs) > 1 && site.Arity != nil {
		arityMatch := narrowByArity(defs, *site.Arity)
		if arityMatch != nil {
			return arityMatch
		}
	}

	// Overload narrowing on model pool
	if len(defs) > 1 {
		hookCtx := buildFreeCallHookCtx(site, options)
		narrowed := NarrowOverloadCandidates(
			ptrSliceToValSlice(defs),
			arityToInt(site.Arity),
			site.ArgumentTypes,
			hookCtx,
		)
		if len(narrowed) == 1 {
			return &narrowed[0]
		}
	}

	return nil
}

// narrowByArity narrows a list of callable candidates by call-site arity.
// A def is compatible when requiredParameterCount <= arity <= parameterCount.
// Defs with parameterCount undefined (nil) are always kept.
// Returns the single compatible def, or nil when zero or multiple match.
func narrowByArity(defs []*shared.SymbolDefinition, callArity int) *shared.SymbolDefinition {
	var compatible []*shared.SymbolDefinition
	for _, d := range defs {
		if d.ParameterCount == nil {
			compatible = append(compatible, d)
			continue
		}
		total := *d.ParameterCount
		required := total
		if d.RequiredParameterCount != nil {
			required = *d.RequiredParameterCount
		}
		if required <= callArity && callArity <= total {
			compatible = append(compatible, d)
		}
	}
	if len(compatible) == 1 {
		return compatible[0]
	}
	return nil
}

// logicalCallableKey produces a dedup key for callable defs.
func logicalCallableKey(def *shared.SymbolDefinition) string {
	qn := ""
	if def.QualifiedName != nil {
		qn = *def.QualifiedName
	}
	pc := ""
	if def.ParameterCount != nil {
		pc = fmt.Sprintf("%d", *def.ParameterCount)
	}
	pt := strings.Join(def.ParameterTypes, ",")
	return def.FilePath + "\x00" + qn + "\x00" + string(def.Type) + "\x00" + pc + "\x00" + pt
}

// pickConstructorOrClass returns the class's explicit Constructor def
// (when present) or the Class def itself when no explicit Constructor exists.
// Mirrors TS pickConstructorOrClass.
func pickConstructorOrClass(
	classDef *shared.SymbolDefinition,
	workspaceIndex *WorkspaceResolutionIndex,
	scopes *model.ScopeResolutionIndexes,
	callArity *int,
) *shared.SymbolDefinition {
	if workspaceIndex == nil {
		return classDef
	}
	classScope, ok := workspaceIndex.LookupClassScopeByDefId(classDef.NodeID)
	if !ok {
		return classDef
	}

	var ctors []*shared.SymbolDefinition
	for i := range classScope.OwnedDefs {
		def := &classScope.OwnedDefs[i]
		if def.Type == shared.LabelConstructor {
			ctors = append(ctors, def)
		}
	}

	if scopes != nil {
		scopeTree := scopes.ScopeTree()
		if scopeTree != nil {
			children := scopeTree.Children(classScope.ID)
			for _, childScope := range children {
				if childScope == nil || childScope.Kind == shared.ScopeKindClass {
					continue
				}
				for i := range childScope.OwnedDefs {
					def := &childScope.OwnedDefs[i]
					if def.Type == shared.LabelConstructor {
						ctors = append(ctors, def)
					}
				}
			}
		}
	}

	if len(ctors) == 0 {
		return classDef
	}
	if callArity != nil {
		narrowed := narrowByArity(ctors, *callArity)
		if narrowed != nil {
			return narrowed
		}
	}
	return ctors[0]
}

// pickUniqueGlobalClass finds a unique workspace-wide class-like def by simple name.
// Returns the def only when all matches share ONE qualified name — i.e. they
// are fragments of a single logical type (partial classes / extensions).
// Genuinely distinct types with the same simple name are ambiguous.
// Mirrors TS pickUniqueGlobalClass.
func pickUniqueGlobalClass(
	name string,
	index map[string][]*shared.SymbolDefinition,
) *shared.SymbolDefinition {
	defs, ok := index[name]
	if !ok {
		return nil
	}
	var found *shared.SymbolDefinition
	for _, def := range defs {
		if found != nil {
			if found.QualifiedName != nil && def.QualifiedName != nil && *found.QualifiedName != *def.QualifiedName {
				return nil // ambiguous
			}
		}
		if found == nil {
			found = def
		}
	}
	return found
}

// pickImplicitThisOverload walks up from the call-site scope to the
// enclosing class scope, then picks a method member by name with overload
// narrowing on arity + argument types.
// Mirrors TS pickImplicitThisOverload.
func pickImplicitThisOverload(
	site *shared.ReferenceSite,
	scopes *model.ScopeResolutionIndexes,
	workspaceIndex *WorkspaceResolutionIndex,
	semModel model.SemanticModel,
	options *FreeCallOptions,
) *shared.SymbolDefinition {
	if scopes == nil || workspaceIndex == nil || semModel == nil {
		return nil
	}
	scopeTree := scopes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	// Find enclosing Class scope by walking parents
	var classScopeID shared.ScopeID
	curID := site.InScope
	for curID != "" {
		sc := scopeTree.GetScope(curID)
		if sc == nil {
			break
		}
		if sc.Kind == shared.ScopeKindClass {
			classScopeID = sc.ID
			break
		}
		if sc.Parent == nil {
			break
		}
		curID = *sc.Parent
	}
	if classScopeID == "" {
		return nil
	}

	// Inverse lookup: class scope → class def
	classDefID, ok := workspaceIndex.LookupClassDefIdByScopeId(classScopeID)
	if !ok {
		return nil
	}

	overloads := semModel.Methods().LookupAllByOwner(classDefID, site.SymbolName)
	if len(overloads) == 0 {
		return nil
	}
	if len(overloads) == 1 {
		return overloads[0]
	}

	// Narrow on arity + argument types. Require a UNIQUE survivor.
	hookCtx := buildFreeCallHookCtx(site, options)
	candidates := NarrowOverloadCandidates(
		ptrSliceToValSlice(overloads),
		arityToInt(site.Arity),
		site.ArgumentTypes,
		hookCtx,
	)
	if len(candidates) != 1 {
		return nil
	}
	return &candidates[0]
}

// buildFreeCallHookCtx creates an OverloadNarrowingHookCtx from a ReferenceSite
// and FreeCallOptions, bridging the ConstraintCompatibility signature.
func buildFreeCallHookCtx(site *shared.ReferenceSite, options *FreeCallOptions) *OverloadNarrowingHookCtx {
	hookCtx := &OverloadNarrowingHookCtx{}
	if len(site.ArgumentTypeClasses) > 0 {
		hookCtx.ArgumentTypeClasses = site.ArgumentTypeClasses
	}
	if options.ConversionRankFn != nil {
		hookCtx.ConversionRankFn = options.ConversionRankFn
	}
	if options.ConstraintCompatibility != nil {
		siteCopy := *site // capture for closure
		hookCtx.ConstraintCompatibility = func(_callsite shared.Callsite, def shared.SymbolDefinition, argTypes []string, argTypeClasses []shared.ParameterTypeClass) ArityVerdict {
			var ptc shared.ParameterTypeClass
			if len(argTypeClasses) > 0 {
				ptc = argTypeClasses[0]
			}
			return options.ConstraintCompatibility(siteCopy, def, ptc)
		}
	}
	return hookCtx
}
