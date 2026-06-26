// fieldRegistry: Variable/Property/Const/Static resolution via lookupCore.
// Ported from gitnexus scope-resolution registries (field-registry pattern).
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// FieldRegistry resolves variable, property, const, and static references
// using the 7-step lookupCore algorithm with useReceiverTypeBinding=true.
// Field lookups walk the MRO chain to find inherited properties.
type FieldRegistry struct {
	ownerScopedContributor OwnerScopedContributor
}

// NewFieldRegistry creates a FieldRegistry with the given
// owner-scoped contributor callback for Step 3.
func NewFieldRegistry(contributor OwnerScopedContributor) *FieldRegistry {
	return &FieldRegistry{
		ownerScopedContributor: contributor,
	}
}

// Lookup resolves a variable/property/const/static name starting from the given scope.
// Uses FIELD_KINDS as the accepted kinds filter and enables type-binding MRO walk.
func (r *FieldRegistry) Lookup(
	name string,
	scope shared.ScopeID,
	explicitReceiver *string,
	ctx *RegistryContext,
) []shared.Resolution {
	params := CoreLookupParams{
		AcceptedKinds:           FIELD_KINDS,
		UseReceiverTypeBinding:  true,
		OwnerScopedContributor:  r.ownerScopedContributor,
		Callsite:                nil,
		ExplicitReceiver:        explicitReceiver,
	}
	return lookupCore(name, scope, params, ctx)
}

// LookupQualified resolves a field by its global qualified name.
func (r *FieldRegistry) LookupQualified(name string, ctx *RegistryContext) []shared.Resolution {
	return LookupQualified(name, LookupQualifiedParams{
		AcceptedKinds: FIELD_KINDS,
	}, ctx)
}