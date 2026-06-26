package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RunScopeResolutionInput holds the inputs to RunScopeResolution.
// Mirrors TS scope-resolution/pipeline/run.ts RunScopeResolutionInput.
type RunScopeResolutionInput struct {
	// Graph is the writable knowledge graph.
	Graph shared.KnowledgeGraph
	// Provider is the language-specific ScopeResolver.
	Provider ScopeResolver
	// ParsedFiles is the set of pre-extracted parsed files for this language.
	ParsedFiles []*shared.ParsedFile
	// NodeLookup is the graph node lookup index.
	// If nil, it will be built from the graph.
	NodeLookup *GraphNodeLookup
	// RepoPath is the root path of the repository.
	RepoPath string
	// ResolutionConfig is the opaque per-workspace config from LoadResolutionConfig.
	ResolutionConfig interface{}
	// AllFilePaths is the workspace's file set (for import resolution).
	AllFilePaths map[string]bool
	// SemanticModel is the mutable semantic model for ownership reconciliation.
	SemanticModel model.MutableSemanticModel
	// OnWarn is an optional callback for warnings.
	OnWarn func(msg string)
	// RecordResolutionOutcome is an optional diagnostics sink.
	RecordResolutionOutcome func(outcome ResolutionOutcome)
}

// RunScopeResolutionResult holds the output of the scope-resolution orchestrator.
type RunScopeResolutionResult struct {
	FilesProcessed        int
	ImportsEmitted        int
	ReferenceEdgesEmitted int
	Outcomes              []ResolutionOutcome
}

// RunScopeResolution is the generic per-language orchestrator.
// Mirrors TS scope-resolution/pipeline/run.ts runScopeResolution.
//
// Execution order (load-bearing per contract invariant I1):
//  1. PopulateOwners per file
//  2. ReconcileOwnership + ValidateOwnershipParity
//  3. Build graph node lookup
//  4. FinalizeScopeModel → ScopeResolutionIndexes
//  5. preEmitInheritanceEdges (inheritance pre-pass)
//  6. EmitHeritageEdges (provider hook)
//  7. EmitImplicitImportEdges (provider hook)
//  8. Rebuild node lookup if heritage edges emitted
//  9. emitDetectedInterfaceImplementations
//
// 10. BuildMro + BuildExtendsOnlyMro
// 11. Build populated MethodDispatch index
// 12. BuildWorkspaceResolutionIndex
// 13. PopulateNamespaceSiblings (provider hook)
// 14. MirrorNamespaceTypeBindings (provider hook)
// 15. PropagateImportedReturnTypes
// 16. ValidateBindingsImmutability
// 17. EmitReceiverBoundCalls (FIRST — I1)
// 18. EmitFreeCallFallback (THEN)
// 19. EmitReferencesViaLookup (LAST)
// 20. EmitImportEdges
func RunScopeResolution(input *RunScopeResolutionInput) (*RunScopeResolutionResult, error) {
	result := &RunScopeResolutionResult{
		Outcomes: []ResolutionOutcome{},
	}

	onWarn := input.OnWarn
	if onWarn == nil {
		onWarn = func(string) {}
	}

	provider := input.Provider
	parsedFiles := input.ParsedFiles
	g := input.Graph

	// ── Step 1: PopulateOwners per file ────────────────────────────────
	for _, parsed := range parsedFiles {
		provider.PopulateOwners(parsed)
	}

	// ── Step 2: ReconcileOwnership + ValidateOwnershipParity ────────────
	// PHASE BOUNDARY: SemanticModel is MutableSemanticModel up to this
	// point (write phase: reconciliation). After this, no further writes
	// are expected — downstream passes consume SemanticModel (read-only).
	ReconcileOwnership(parsedFiles, input.SemanticModel, onWarn)
	ValidateOwnershipParity(parsedFiles, input.SemanticModel, onWarn)
	readonlyModel := model.SemanticModel(input.SemanticModel)

	if len(parsedFiles) == 0 {
		return result, nil
	}

	// ── Step 3: Build graph node lookup ────────────────────────────────
	nodeLookup := input.NodeLookup
	if nodeLookup == nil {
		nodeLookup = BuildGraphNodeLookup(g)
	}

	// ── Step 4: FinalizeScopeModel → ScopeResolutionIndexes ────────────
	// Build the allFilePaths set from parsedFiles for import resolution.
	allFilePaths := input.AllFilePaths
	if allFilePaths == nil {
		allFilePaths = make(map[string]bool, len(parsedFiles))
		for _, pf := range parsedFiles {
			allFilePaths[pf.FilePath] = true
		}
	}

	// Finalize the scope model using the core orchestrator.
	// This produces the ScopeResolutionIndexes with scope tree,
	// bindings, imports, SCCs, etc.
	resolutionConfig := input.ResolutionConfig
	indexes := FinalizeScopeModel(parsedFiles, provider, allFilePaths, resolutionConfig)

	// ── Step 5: preEmitInheritanceEdges ────────────────────────────────
	// Resolve inheritance reference sites early and pre-emit their
	// EXTENDS/IMPLEMENTS edges before MRO construction. This lets
	// template-base captures contribute to the graph in time for buildMro.
	handledSites := PreEmitInheritanceEdges(g, indexes, nodeLookup)

	// ── Step 6: EmitHeritageEdges (provider hook) ─────────────────────
	// Call-based heritage hook (e.g. Ruby include/extend/prepend) — emits
	// IMPLEMENTS edges that preEmitInheritanceEdges cannot produce because
	// the heritage declarations are syntactic method calls, not grammar-
	// level heritage clauses. Must run BEFORE buildMro so MRO construction
	// sees the freshly-emitted IMPLEMENTS edges.
	if provider.EmitHeritageEdges() != nil {
		provider.EmitHeritageEdges()(g, parsedFiles, nodeLookup, indexes)
	}

	// ── Step 7: EmitImplicitImportEdges (provider hook) ───────────────
	if provider.EmitImplicitImportEdges() != nil {
		provider.EmitImplicitImportEdges()(g, parsedFiles, nodeLookup, resolutionConfig)
	}

	// ── Step 8: Rebuild node lookup if heritage edges were emitted ────
	// Languages like Ruby create Property graph nodes inside
	// emitHeritageEdges; those nodes must be visible to downstream passes.
	postHeritageNodeLookup := nodeLookup
	if provider.EmitHeritageEdges() != nil {
		postHeritageNodeLookup = BuildGraphNodeLookup(g)
	}

	// ── Step 9: emitDetectedInterfaceImplementations ──────────────────
	// Emit language-inferred structural interface implementations before
	// MRO and interface dispatch are built.
	EmitDetectedInterfaceImplementations(g, parsedFiles, postHeritageNodeLookup, provider, indexes, readonlyModel)

	// ── Step 10: BuildMro + BuildExtendsOnlyMro ───────────────────────
	mroByClassDefId := provider.BuildMro(g, parsedFiles, postHeritageNodeLookup)

	var extendsOnlyMroByClassDefId map[string][]string
	if provider.BuildExtendsOnlyMro() != nil {
		extendsOnlyMroByClassDefId = provider.BuildExtendsOnlyMro()(g, parsedFiles, postHeritageNodeLookup)
	}

	// ── Step 11: Build populated MethodDispatch index ──────────────────
	// Replace the empty MethodDispatchIndex that finalizeScopeModel builds
	// with the populated one derived from the language's MRO.
	methodDispatch := BuildPopulatedMethodDispatch(mroByClassDefId, extendsOnlyMroByClassDefId)
	indexes = InjectMethodDispatch(indexes, methodDispatch)

	// ── Step 12: BuildWorkspaceResolutionIndex ─────────────────────────
	workspaceIndex := BuildWorkspaceResolutionIndex(parsedFiles)

	// ── Step 13: PopulateNamespaceSiblings (provider hook) ─────────────
	// Cross-file implicit-namespace visibility (C#). Must run before
	// propagateImportedReturnTypes so the latter pass sees siblings'
	// class bindings when chasing return-type chains across files.
	if provider.PopulateNamespaceSiblings() != nil {
		provider.PopulateNamespaceSiblings()(g, parsedFiles, nodeLookup, indexes)
	}

	// ── Step 14: MirrorNamespaceTypeBindings (provider hook) ──────────
	// Cross-package namespace typeBinding mirroring. Runs before
	// propagateImportedReturnTypes so the SCC-ordered pass sees the
	// mirrored bindings.
	// (This hook would be on the provider if it existed; not in current
	// ScopeResolver interface — future extension point.)

	// ── Step 15: PropagateImportedReturnTypes ──────────────────────────
	// Cross-file return-type propagation (Contract Invariant I3 timing:
	// after finalize, before resolve).
	if provider.PropagatesReturnTypesAcrossImports() {
		PropagateImportedReturnTypes(parsedFiles, provider, indexes)
	}

	// ── Step 16: ValidateBindingsImmutability ──────────────────────────
	// Opt-in I8 invariant guard. Runs once after all post-finalize hooks
	// have had a chance to drift.
	// In Go we pass nil snapshot (dev-mode validation not yet wired);
	// the function returns 0 immediately when snap is nil.
	ValidateBindingsImmutability(indexes, nil, onWarn)

	// ── Step 17: EmitReceiverBoundCalls (FIRST — I1) ──────────────────
	receiverExtras := EmitReceiverBoundCallsFull(
		provider,
		indexes,
		mroByClassDefId,
		postHeritageNodeLookup,
		g,
		parsedFiles,
		handledSites,
		readonlyModel,
	)

	// ── Step 18: EmitFreeCallFallback (THEN) ───────────────────────────
	freeCallOptions := &FreeCallOptions{
		AllowGlobalFallback:         provider.PropagatesReturnTypesAcrossImports(),
		ConstructorCallTargetsClass: false,
	}
	if provider.ConversionRankFn() != nil {
		crf := provider.ConversionRankFn()
		freeCallOptions.ConversionRankFn = func(argType, paramType string, _argTC, _paramTC *shared.ParameterTypeClass) float64 {
			return float64(crf(argType, paramType))
		}
	}
	if provider.ConstraintCompatibility() != nil {
		freeCallOptions.ConstraintCompatibility = provider.ConstraintCompatibility()
	}

	freeCallExtras := EmitFreeCallFallback(
		g,
		provider,
		postHeritageNodeLookup,
		indexes,
		handledSites,
		parsedFiles,
		readonlyModel,
		workspaceIndex,
		freeCallOptions,
	)

	// ── Step 19: EmitReferencesViaLookup (LAST) ────────────────────────
	// Uses handledSites from the previous passes.
	refEdgesEmitted := EmitReferencesViaLookup(
		g,
		provider,
		postHeritageNodeLookup,
		indexes,
		handledSites,
		mroByClassDefId,
	)

	// ── Step 20: ImportsToEdges ───────────────────────────────────────
	importsEmitted := ImportsToEdges(
		g,
		provider,
		postHeritageNodeLookup,
		indexes,
	)

	// ── Assemble result ────────────────────────────────────────────────
	result.FilesProcessed = len(parsedFiles)
	result.ImportsEmitted = importsEmitted
	result.ReferenceEdgesEmitted = receiverExtras + freeCallExtras + refEdgesEmitted

	return result, nil
}

// ---------------------------------------------------------------------------
// Helper functions for the orchestrator
// ---------------------------------------------------------------------------

// FinalizeScopeModel builds the ScopeResolutionIndexes from parsed files.
// This is a simplified placeholder that constructs indexes from the parsed
// file data. The full implementation would delegate to the finalize algorithm.
//
// In the TS codebase this is `finalizeScopeModel(parsedFiles, { hooks })`.
// In Go we use `core.RunFinalize` + construct indexes from its output.
func FinalizeScopeModel(
	parsedFiles []*shared.ParsedFile,
	provider ScopeResolver,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) *model.ScopeResolutionIndexes {
	// Build the finalize input from parsed files.
	// FinalizeFile has: FilePath, ModuleScope, ParsedImports, LocalDefs
	files := make([]shared.FinalizeFile, 0, len(parsedFiles))
	for _, pf := range parsedFiles {
		// Find the module scope ID for this file.
		moduleScopeID := findModuleScopeID(pf)
		files = append(files, shared.FinalizeFile{
			FilePath:      pf.FilePath,
			ModuleScope:   moduleScopeID,
			ParsedImports: pf.ParsedImports,
			LocalDefs:     pf.LocalDefs,
		})
	}

	// Create an import target adapter that delegates to the provider.
	// providerImportAdapter implements shared.FinalizeHooks.
	adapter := &providerImportAdapter{
		provider:         provider,
		allFilePaths:     allFilePaths,
		resolutionConfig: resolutionConfig,
	}

	// Run the finalize algorithm.
	// WorkspaceIndex is interface{} — just pass allFilePaths as the workspace context.
	finalizeResult := shared.Finalize(
		shared.FinalizeInput{
			Files:          files,
			WorkspaceIndex: allFilePaths, // WorkspaceIndex is interface{}
		},
		adapter, // adapter implements shared.FinalizeHooks
	)

	// Collect all scopes and reference sites from parsed files.
	var allScopes []*shared.Scope
	var referenceSites []*shared.ReferenceSite
	var allDefs []shared.SymbolDefinition
	for _, pf := range parsedFiles {
		allScopes = append(allScopes, pf.Scopes...)
		for i := range pf.ReferenceSites {
			referenceSites = append(referenceSites, &pf.ReferenceSites[i])
		}
		allDefs = append(allDefs, pf.LocalDefs...)
	}

	// Build the scope tree from all scopes.
	scopeTree, _ := shared.BuildScopeTree(allScopes)

	// Build the DefIndex from all defs.
	defIndex := shared.BuildDefIndex(allDefs)

	// Build qualified name index.
	qnIndex := shared.BuildQualifiedNameIndex(allDefs)

	// Build module scope index.
	// BuildModuleScopeIndex takes []ParsedFile (value slice), but we have []*ParsedFile.
	pfValues := make([]shared.ParsedFile, len(parsedFiles))
	for i, pf := range parsedFiles {
		pfValues[i] = *pf
	}
	moduleScopeIndex := shared.BuildModuleScopeIndex(pfValues)

	// Convert finalize result bindings to the indexes format.
	bindings := make(map[shared.ScopeID]map[string][]*shared.BindingRef)
	for scopeID, bucketMap := range finalizeResult.Bindings {
		inner := make(map[string][]*shared.BindingRef)
		for name, refs := range bucketMap {
			ptrRefs := make([]*shared.BindingRef, len(refs))
			for i := range refs {
				ptrRefs[i] = &refs[i]
			}
			inner[name] = ptrRefs
		}
		bindings[scopeID] = inner
	}

	// Convert imports.
	imports := make(map[shared.ScopeID][]*shared.ImportEdge)
	for scopeID, edges := range finalizeResult.Imports {
		ptrEdges := make([]*shared.ImportEdge, len(edges))
		for i := range edges {
			ptrEdges[i] = &edges[i]
		}
		imports[scopeID] = ptrEdges
	}

	// Convert SCCs.
	sccs := make([]*shared.FinalizedScc, len(finalizeResult.Sccs))
	for i := range finalizeResult.Sccs {
		sccs[i] = &shared.FinalizedScc{
			FilePaths: finalizeResult.Sccs[i].Files,
		}
	}

	// Convert stats.
	stats := shared.FinalizeStats{
		ScopeCount:      finalizeResult.Stats.TotalFiles,
		DefCount:        finalizeResult.Stats.TotalFiles,
		ImportEdgeCount: finalizeResult.Stats.TotalEdges,
		BindingCount:    finalizeResult.Stats.LinkedEdges,
	}

	// Build the indexes using the constructor.
	return model.NewScopeResolutionIndexes(
		scopeTree,
		defIndex,
		qnIndex,
		moduleScopeIndex,
		shared.NewMethodDispatchIndex(), // empty — populated later via InjectMethodDispatch
		imports,
		bindings,
		nil, // bindingAugmentations — populated by PopulateNamespaceSiblings
		nil, // workspaceFqnBindings
		nil, // workspaceTypeBindings
		nil, // namespaceFqnBindings
		nil, // namespaceTypeBindings
		nil, // accessibleNamespacesByScope
		referenceSites,
		sccs,
		stats,
	)
}

// findModuleScopeID returns the ScopeID of the first Module-kind scope
// in the parsed file, or an empty string if none found.
func findModuleScopeID(pf *shared.ParsedFile) shared.ScopeID {
	if pf.ModuleScope != nil {
		return pf.ModuleScope.ID
	}
	for _, scope := range pf.Scopes {
		if scope.Kind == shared.ScopeKindModule {
			return scope.ID
		}
	}
	return ""
}

// providerImportAdapter adapts ScopeResolver to the FinalizeHooks
// interface expected by the finalize algorithm.
type providerImportAdapter struct {
	provider         ScopeResolver
	allFilePaths     map[string]bool
	resolutionConfig interface{}
}

// ResolveImportTarget implements shared.FinalizeHooks.ResolveImportTarget.
func (a *providerImportAdapter) ResolveImportTarget(targetRaw string, fromFile string, workspace shared.WorkspaceIndex) []string {
	return a.provider.ResolveImportTarget(targetRaw, fromFile, a.allFilePaths, a.resolutionConfig)
}

// ExpandsWildcardTo implements shared.FinalizeHooks.ExpandsWildcardTo.
func (a *providerImportAdapter) ExpandsWildcardTo(targetModuleScope shared.ScopeID, workspace shared.WorkspaceIndex) []string {
	if fn := a.provider.ExpandsWildcardTo(); fn != nil {
		// The provider's ExpandsWildcardTo takes (ScopeID, []*ParsedFile)
		// We don't have parsedFiles here; return nil for now.
		return nil
	}
	return nil
}

// MergeBindings implements shared.FinalizeHooks.MergeBindings.
func (a *providerImportAdapter) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scope shared.ScopeID) []shared.BindingRef {
	return a.provider.MergeBindings(existing, incoming, scope)
}

// PreEmitInheritanceEdges resolves inheritance reference sites early and
// pre-emits their EXTENDS edges before MRO construction.
// Returns site keys to seed the downstream handled-site skip set.
//
// Mirrors TS preEmitInheritanceEdges.
func PreEmitInheritanceEdges(
	g shared.KnowledgeGraph,
	indexes *model.ScopeResolutionIndexes,
	nodeLookup *GraphNodeLookup,
) map[string]bool {
	handledSites := make(map[string]bool)
	seen := make(map[string]bool)
	existing := make(map[string]bool)

	for _, site := range indexes.ReferenceSites() {
		if site.Kind != shared.ReferenceInherits {
			continue
		}

		scopeTree := indexes.ScopeTree()
		if scopeTree == nil {
			continue
		}

		scope := scopeTree.GetScope(site.InScope)
		if scope == nil {
			continue
		}

		siteKey := fmt.Sprintf("%s:%d:%d", scope.FilePath, site.Range.StartLine, site.Range.StartCol)
		// Intentionally suppress every inherits site from the generic
		// reference bridge, even when this pre-pass can't emit an edge.
		handledSites[siteKey] = true

		// Resolve the deriving (caller) class
		callerClass := FindEnclosingClassDef(site.InScope, indexes)
		if callerClass == nil {
			continue
		}

		targetDef := ResolveInheritanceBaseInScope(site.InScope, site.SymbolName, indexes)
		if targetDef == nil {
			continue
		}

		callerGraphId := ResolveDefGraphID(callerClass.FilePath, callerClass, nodeLookup)
		targetGraphId := ResolveDefGraphID(targetDef.FilePath, targetDef, nodeLookup)
		if callerGraphId == "" || targetGraphId == "" {
			continue
		}

		// Discriminate EXTENDS vs IMPLEMENTS by the resolved target's symbol kind
		edgeType := graph.RelExtends
		if targetDef.Type == shared.LabelInterface || targetDef.Type == shared.LabelTrait {
			edgeType = graph.RelImplements
		}

		edgeKey := fmt.Sprintf("%s:%s->%s", edgeType, callerGraphId, targetGraphId)
		dedupKey := fmt.Sprintf("%s:%d:%d", edgeKey, site.Range.StartLine, site.Range.StartCol)
		if existing[edgeKey] || seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true
		existing[edgeKey] = true

		edge := &graph.Edge{
			ID:     fmt.Sprintf("rel:%s", dedupKey),
			Type:   edgeType,
			Source: callerGraphId,
			Target: targetGraphId,
		}
		edge.Props.SetProp("confidence", 0.85)
		edge.Props.SetProp("reason", "scope-resolution: inherits")
		shared.SetEdgeEvidence(edge, []shared.Evidence{
			{Kind: "scope-resolution", Weight: 0.85, Note: "inherits"},
		})
		g.AddEdge(edge)
	}

	return handledSites
}

// EmitDetectedInterfaceImplementations emits language-inferred structural
// interface implementations before MRO and interface dispatch are built.
//
// Mirrors TS emitDetectedInterfaceImplementations.
func EmitDetectedInterfaceImplementations(
	g shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *GraphNodeLookup,
	provider ScopeResolver,
	indexes *model.ScopeResolutionIndexes,
	semModel model.SemanticModel,
) int {
	// This requires the provider to implement detectInterfaceImplementations.
	// Since ScopeResolver doesn't currently expose this hook, this is a
	// no-op placeholder. Future extension point for Go structural implements.
	return 0
}

// BuildPopulatedMethodDispatch builds the MethodDispatchIndex from MRO data.
// Mirrors TS buildPopulatedMethodDispatch.
func BuildPopulatedMethodDispatch(
	mroByClassDefId map[string][]string,
	extendsOnlyMroByClassDefId map[string][]string,
) *shared.MethodDispatchIndex {
	idx := shared.NewMethodDispatchIndex()
	// Populate MRO from the provider's BuildMro output.
	for defID, ancestors := range mroByClassDefId {
		idx.SetMRO(shared.DefID(defID), sharedIDsToDefIDs(ancestors))
	}
	// Populate extends-only MRO if present.
	if extendsOnlyMroByClassDefId != nil {
		for defID, ancestors := range extendsOnlyMroByClassDefId {
			idx.SetExtendsOnlyMRO(shared.DefID(defID), sharedIDsToDefIDs(ancestors))
		}
	}
	return idx
}

// sharedIDsToDefIDs converts []string to []DefID.
func sharedIDsToDefIDs(ids []string) []shared.DefID {
	result := make([]shared.DefID, len(ids))
	for i, id := range ids {
		result[i] = shared.DefID(id)
	}
	return result
}

// InjectMethodDispatch returns a new ScopeResolutionIndexes with the given
// MethodDispatchIndex, replacing the empty one that finalize built.
// Since ScopeResolutionIndexes fields are unexported, we use a model-level
// function for this.
func InjectMethodDispatch(indexes *model.ScopeResolutionIndexes, md *shared.MethodDispatchIndex) *model.ScopeResolutionIndexes {
	return model.NewScopeResolutionIndexesWithDispatch(indexes, md)
}

// ValidateOwnershipParity checks that every def in parsedFiles with an ownerId
// is reachable via the SemanticModel's registries. Development-mode only.
// Mirrors TS validateOwnershipParity.
func ValidateOwnershipParity(
	parsedFiles []*shared.ParsedFile,
	semModel model.SemanticModel,
	onWarn func(string),
) int {
	// In production this is a no-op. In development mode it surfaces drift
	// between parsed.localDefs and SemanticModel that would otherwise
	// silently produce wrong edges.
	// For now, always run — the Go build has no NODE_ENV equivalent.
	mismatches := 0

	for _, parsed := range parsedFiles {
		for i := range parsed.LocalDefs {
			def := &parsed.LocalDefs[i]
			if def.OwnerID == nil {
				continue
			}
			ownerID := *def.OwnerID
			simple := SimpleQualifiedName(def)
			if simple == "" {
				continue
			}

			if isCallableLabel(def.Type) {
				found := semModel.Methods().LookupAllByOwner(ownerID, simple)
				if !hasMatchingNodeId(found, def.NodeID) {
					onWarn(fmt.Sprintf(
						"semantic-model parity: %s %s (%s) owned by %s as %q not in MethodRegistry",
						def.Type, def.NodeID, parsed.FilePath, ownerID, simple))
					mismatches++
				}
			} else if IsOwnableValueLabel(def.Type) {
				found := semModel.Fields().LookupAllByOwner(ownerID, simple)
				if !hasMatchingNodeId(found, def.NodeID) {
					onWarn(fmt.Sprintf(
						"semantic-model parity: %s %s (%s) owned by %s as %q not in FieldRegistry",
						def.Type, def.NodeID, parsed.FilePath, ownerID, simple))
					mismatches++
				}
			} else if nestedTypeKinds[def.Type] {
				found := semModel.Types().LookupAllByOwner(ownerID, simple)
				if !hasMatchingNodeId(found, def.NodeID) {
					onWarn(fmt.Sprintf(
						"semantic-model parity: %s %s (%s) owned by %s as %q not in TypeRegistry owner index",
						def.Type, def.NodeID, parsed.FilePath, ownerID, simple))
					mismatches++
				}
			}
		}
	}

	return mismatches
}
