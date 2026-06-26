// classRegistry: Class/Interface/Namespace resolution via lookupCore.
// Ported from gitnexus scope-resolution registries (class-registry pattern).
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ClassRegistry resolves class, interface, and namespace references
// using the 7-step lookupCore algorithm with useReceiverTypeBinding=false.
// Class lookups never need MRO walk — they resolve purely from lexical scope.
type ClassRegistry struct{}

// NewClassRegistry creates a ClassRegistry.
func NewClassRegistry() *ClassRegistry {
	return &ClassRegistry{}
}

// Lookup resolves a class/interface/namespace name starting from the given scope.
// Uses CLASS_KINDS as the accepted kinds filter and disables type-binding MRO walk.
func (r *ClassRegistry) Lookup(name string, scope shared.ScopeID, ctx *RegistryContext) []shared.Resolution {
	params := CoreLookupParams{
		AcceptedKinds:           CLASS_KINDS,
		UseReceiverTypeBinding:  false,
		OwnerScopedContributor:  nil,
		Callsite:                nil,
		ExplicitReceiver:        nil,
	}
	return lookupCore(name, scope, params, ctx)
}

// LookupQualified resolves a class by its global qualified name.
func (r *ClassRegistry) LookupQualified(name string, ctx *RegistryContext) []shared.Resolution {
	return LookupQualified(name, LookupQualifiedParams{
		AcceptedKinds: CLASS_KINDS,
	}, ctx)
}