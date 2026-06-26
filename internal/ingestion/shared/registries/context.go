// Package registries implements the scope-resolution registry system:
// ClassRegistry, MethodRegistry, FieldRegistry, MacroRegistry,
// and the shared lookupCore algorithm.
// Ported from gitnexus-shared scope-resolution/registries/.
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CLASS_KINDS are the NodeLabels accepted by ClassRegistry.
var CLASS_KINDS = []shared.NodeLabel{
	shared.LabelClass, shared.LabelInterface, shared.LabelEnum,
	shared.LabelStruct, shared.LabelUnion, shared.LabelTrait,
	shared.LabelTypeAlias, shared.LabelTypedef, shared.LabelRecord,
	shared.LabelDelegate, shared.LabelAnnotation, shared.LabelTemplate,
	shared.LabelNamespace,
}

// METHOD_KINDS are the NodeLabels accepted by MethodRegistry.
var METHOD_KINDS = []shared.NodeLabel{
	shared.LabelMethod, shared.LabelFunction, shared.LabelConstructor,
}

// FIELD_KINDS are the NodeLabels accepted by FieldRegistry.
var FIELD_KINDS = []shared.NodeLabel{
	shared.LabelVariable, shared.LabelProperty, shared.LabelConst,
	shared.LabelStatic,
}

// MACRO_KINDS are the NodeLabels accepted by MacroRegistry.
var MACRO_KINDS = []shared.NodeLabel{
	shared.LabelMacro,
}

// OwnerScopedContributor is the per-owner lookup callback used by Step 3
// of the 7-step lookup algorithm. It returns defs owned by a given
// owner DefID that match the lookup name.
type OwnerScopedContributor func(ownerDefID shared.DefID, name string) []shared.SymbolDefinition

// OwnedMembersByOwnerLookup is the reverse-index callback for Step 3.
// Given an interface DefID, returns all DefIDs that implement it.
type OwnedMembersByOwnerLookup func(interfaceDefID shared.DefID) []shared.DefID

// RegistryProviders holds the language-specific callback functions needed
// by the registry lookup algorithm.
type RegistryProviders struct {
	// ArityCompatibility checks if a call site's arity is compatible
	// with a definition's parameter signature.
	ArityCompatibility func(def *shared.SymbolDefinition, callsite *shared.Callsite) shared.ArityVerdict
	// ComputeMro computes the MRO for a given class DefID.
	ComputeMro func(defID shared.DefID) []shared.DefID
	// ComputeImplsOf returns DefIDs implementing the given interface.
	ComputeImplsOf func(interfaceDefID shared.DefID) []shared.DefID
}

// ConstraintContext carries additional context for constraint-based filtering.
type ConstraintContext struct {
	// FilePath of the current file being resolved.
	FilePath string
	// AccessibleNamespaces lists namespaces visible from the current scope.
	AccessibleNamespaces []string
}

// RegistryContext is the shared context used by all registry lookups.
// It bundles the immutable indexes built during finalize plus the
// language-specific provider callbacks.
type RegistryContext struct {
	// ScopeTree is the hierarchical scope tree.
	ScopeTree *shared.ScopeTree
	// Defs is the definition index.
	Defs *shared.DefIndex
	// QualifiedNames is the qualified name index.
	QualifiedNames *shared.QualifiedNameIndex
	// ModuleScopes is the module scope index.
	ModuleScopes *shared.ModuleScopeIndex
	// MethodDispatch is the MRO + implements materialized view.
	MethodDispatch *shared.MethodDispatchIndex
	// Bindings is the finalized bindings per scope.
	Bindings map[shared.ScopeID]map[string][]*shared.BindingRef
	// BindingAugmentations is the post-finalize binding channel.
	BindingAugmentations map[shared.ScopeID]map[string][]*shared.BindingRef
	// TypeBindings is the type bindings per scope.
	TypeBindings map[shared.ScopeID]map[string]shared.TypeRef
	// Providers holds the language-specific callbacks.
	Providers *RegistryProviders
	// ConstraintCtx carries additional constraint context.
	ConstraintCtx *ConstraintContext
}