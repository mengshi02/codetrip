// methodRegistry: Method/Constructor/Function resolution via lookupCore.
// Ported from gitnexus scope-resolution registries (method-registry pattern).
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// MethodRegistry resolves method, constructor, and function references
// using the 7-step lookupCore algorithm with useReceiverTypeBinding=true.
// Method lookups walk the MRO chain to find inherited methods.
type MethodRegistry struct {
	ownerScopedContributor OwnerScopedContributor
}

// NewMethodRegistry creates a MethodRegistry with the given
// owner-scoped contributor callback for Step 3.
func NewMethodRegistry(contributor OwnerScopedContributor) *MethodRegistry {
	return &MethodRegistry{
		ownerScopedContributor: contributor,
	}
}

// Lookup resolves a method/constructor/function name starting from the given scope.
// Uses METHOD_KINDS as the accepted kinds filter and enables type-binding MRO walk.
func (r *MethodRegistry) Lookup(
	name string,
	scope shared.ScopeID,
	callsite *shared.Callsite,
	explicitReceiver *string,
	ctx *RegistryContext,
) []shared.Resolution {
	params := CoreLookupParams{
		AcceptedKinds:           METHOD_KINDS,
		UseReceiverTypeBinding:  true,
		OwnerScopedContributor:  r.ownerScopedContributor,
		Callsite:                callsite,
		ExplicitReceiver:        explicitReceiver,
	}
	return lookupCore(name, scope, params, ctx)
}

// LookupQualified resolves a method by its global qualified name.
func (r *MethodRegistry) LookupQualified(name string, ctx *RegistryContext) []shared.Resolution {
	return LookupQualified(name, LookupQualifiedParams{
		AcceptedKinds: METHOD_KINDS,
	}, ctx)
}