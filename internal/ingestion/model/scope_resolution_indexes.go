package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// ScopeResolutionIndexes -- immutable index bundle produced by finalize
// ---------------------------------------------------------------------------

// ScopeResolutionIndexes -- RFC #909 Ring 2 PKG #921 product.
// Produced by finalizeScopeModel(parsedFiles, hooks),
// written to SemanticModel via model.AttachScopeIndexes() in a single shot.
//
// Lifecycle:
//   1. Pipeline collects ParsedFile[]
//   2. finalizeScopeModel -> ScopeResolutionIndexes
//   3. model.AttachScopeIndexes(indexes) -- one-time write; subsequent calls panic
//   4. Resolver reads via model.Scopes()
//
// Go has no Object.freeze; immutability is enforced by interface constraints
// (SemanticModel only exposes read methods).
// Internal fields are lowercase; external access only through interface methods.
type ScopeResolutionIndexes struct {
	// scopeTree -- hierarchical scope tree
	scopeTree *shared.ScopeTree
	// defs -- definition index
	defs *shared.DefIndex
	// qualifiedNames -- qualified name index
	qualifiedNames *shared.QualifiedNameIndex
	// moduleScopes -- module scope index
	moduleScopes *shared.ModuleScopeIndex
	// methodDispatch -- MRO + implements materialized view (#914)
	methodDispatch *shared.MethodDispatchIndex

	// imports -- resolved ImportEdge per module scope
	imports map[shared.ScopeID][]*shared.ImportEdge
	// bindings -- bindings per module scope (local + import + wildcard)
	bindings map[shared.ScopeID]map[string][]*shared.BindingRef
	// bindingAugmentations -- post-finalize append channel (C# namespace siblings, etc.)
	bindingAugmentations map[shared.ScopeID]map[string][]*shared.BindingRef
	// workspaceFqnBindings -- workspace-level bindings (global namespace types)
	workspaceFqnBindings map[string][]*shared.BindingRef
	// workspaceTypeBindings -- workspace-level type bindings (C# global namespace method return types)
	workspaceTypeBindings map[string]*shared.TypeRef
	// namespaceFqnBindings -- per-namespace bindings (namespace visibility gating)
	namespaceFqnBindings map[string]map[string][]*shared.BindingRef
	// namespaceTypeBindings -- per-namespace type bindings
	namespaceTypeBindings map[string]map[string]*shared.TypeRef
	// accessibleNamespacesByScope -- per-file accessible namespace list
	accessibleNamespacesByScope map[shared.ScopeID][]string

	// referenceSites -- pre-resolved use sites
	referenceSites []*shared.ReferenceSite
	// sccs -- file-level import graph SCC condensation
	sccs []*shared.FinalizedScc
	// stats -- coarse-grained statistics
	stats shared.FinalizeStats
}

// --- Read methods (replacing TS readonly properties) ---

func (s *ScopeResolutionIndexes) ScopeTree() *shared.ScopeTree               { return s.scopeTree }
func (s *ScopeResolutionIndexes) Defs() *shared.DefIndex                     { return s.defs }
func (s *ScopeResolutionIndexes) QualifiedNames() *shared.QualifiedNameIndex { return s.qualifiedNames }
func (s *ScopeResolutionIndexes) ModuleScopes() *shared.ModuleScopeIndex     { return s.moduleScopes }
func (s *ScopeResolutionIndexes) MethodDispatch() *shared.MethodDispatchIndex { return s.methodDispatch }
func (s *ScopeResolutionIndexes) Imports() map[shared.ScopeID][]*shared.ImportEdge {
	return s.imports
}
func (s *ScopeResolutionIndexes) Bindings() map[shared.ScopeID]map[string][]*shared.BindingRef {
	return s.bindings
}
func (s *ScopeResolutionIndexes) BindingAugmentations() map[shared.ScopeID]map[string][]*shared.BindingRef {
	return s.bindingAugmentations
}
func (s *ScopeResolutionIndexes) WorkspaceFqnBindings() map[string][]*shared.BindingRef {
	return s.workspaceFqnBindings
}
func (s *ScopeResolutionIndexes) WorkspaceTypeBindings() map[string]*shared.TypeRef {
	return s.workspaceTypeBindings
}
func (s *ScopeResolutionIndexes) NamespaceFqnBindings() map[string]map[string][]*shared.BindingRef {
	return s.namespaceFqnBindings
}
func (s *ScopeResolutionIndexes) NamespaceTypeBindings() map[string]map[string]*shared.TypeRef {
	return s.namespaceTypeBindings
}
func (s *ScopeResolutionIndexes) AccessibleNamespacesByScope() map[shared.ScopeID][]string {
	return s.accessibleNamespacesByScope
}
func (s *ScopeResolutionIndexes) ReferenceSites() []*shared.ReferenceSite { return s.referenceSites }
func (s *ScopeResolutionIndexes) Sccs() []*shared.FinalizedScc            { return s.sccs }
func (s *ScopeResolutionIndexes) Stats() shared.FinalizeStats              { return s.stats }

// --- Constructors ---

// NewScopeResolutionIndexes builds a ScopeResolutionIndexes from the individual
// sub-indexes produced by finalize + post-finalize augmentation.
// This is the Go equivalent of the TS finalizeScopeModel constructor.
func NewScopeResolutionIndexes(
	scopeTree *shared.ScopeTree,
	defs *shared.DefIndex,
	qualifiedNames *shared.QualifiedNameIndex,
	moduleScopes *shared.ModuleScopeIndex,
	methodDispatch *shared.MethodDispatchIndex,
	imports map[shared.ScopeID][]*shared.ImportEdge,
	bindings map[shared.ScopeID]map[string][]*shared.BindingRef,
	bindingAugmentations map[shared.ScopeID]map[string][]*shared.BindingRef,
	workspaceFqnBindings map[string][]*shared.BindingRef,
	workspaceTypeBindings map[string]*shared.TypeRef,
	namespaceFqnBindings map[string]map[string][]*shared.BindingRef,
	namespaceTypeBindings map[string]map[string]*shared.TypeRef,
	accessibleNamespacesByScope map[shared.ScopeID][]string,
	referenceSites []*shared.ReferenceSite,
	sccs []*shared.FinalizedScc,
	stats shared.FinalizeStats,
) *ScopeResolutionIndexes {
	return &ScopeResolutionIndexes{
		scopeTree:                   scopeTree,
		defs:                        defs,
		qualifiedNames:              qualifiedNames,
		moduleScopes:                moduleScopes,
		methodDispatch:              methodDispatch,
		imports:                     imports,
		bindings:                    bindings,
		bindingAugmentations:        bindingAugmentations,
		workspaceFqnBindings:        workspaceFqnBindings,
		workspaceTypeBindings:       workspaceTypeBindings,
		namespaceFqnBindings:        namespaceFqnBindings,
		namespaceTypeBindings:       namespaceTypeBindings,
		accessibleNamespacesByScope: accessibleNamespacesByScope,
		referenceSites:              referenceSites,
		sccs:                        sccs,
		stats:                       stats,
	}
}

// NewScopeResolutionIndexesWithDispatch returns a copy of the given indexes
// with the MethodDispatchIndex replaced. This is used by the orchestrator
// after BuildMro populates the dispatch data — the empty dispatch from
// finalize is replaced with the populated one.
func NewScopeResolutionIndexesWithDispatch(
	base *ScopeResolutionIndexes,
	md *shared.MethodDispatchIndex,
) *ScopeResolutionIndexes {
	return &ScopeResolutionIndexes{
		scopeTree:                   base.scopeTree,
		defs:                        base.defs,
		qualifiedNames:              base.qualifiedNames,
		moduleScopes:                base.moduleScopes,
		methodDispatch:              md,
		imports:                     base.imports,
		bindings:                    base.bindings,
		bindingAugmentations:        base.bindingAugmentations,
		workspaceFqnBindings:        base.workspaceFqnBindings,
		workspaceTypeBindings:       base.workspaceTypeBindings,
		namespaceFqnBindings:        base.namespaceFqnBindings,
		namespaceTypeBindings:       base.namespaceTypeBindings,
		accessibleNamespacesByScope: base.accessibleNamespacesByScope,
		referenceSites:              base.referenceSites,
		sccs:                        base.sccs,
		stats:                       base.stats,
	}
}
