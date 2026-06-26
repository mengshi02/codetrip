package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// LookupOwnedMembersByOwner -- RFC #909 / PR #1656 Step 2 hook.
// Merges methods + fields + nestedTypes (owner, name) lookup results.
// Returns authoritative empty slice (not nil) -- callers can concatenate without nil checks.
//
// If only one registry has results, returns that registry's slice (zero-copy);
// if multiple registries have results, merges into a new slice.
func LookupOwnedMembersByOwner(
	model SemanticModel,
	ownerDefID shared.DefID,
	memberName string,
) []*shared.SymbolDefinition {
	methods := model.Methods().LookupAllByOwner(ownerDefID, memberName)
	fields := model.Fields().LookupAllByOwner(ownerDefID, memberName)
	nestedTypes := model.Types().LookupAllByOwner(ownerDefID, memberName)

	methodCount := len(methods)
	fieldCount := len(fields)
	typeCount := len(nestedTypes)
	total := methodCount + fieldCount + typeCount

	if total == 0 {
		return []*shared.SymbolDefinition{}
	}
	// Single source -> zero-copy direct return
	if methodCount == total {
		return methods
	}
	if fieldCount == total {
		return fields
	}
	if typeCount == total {
		return nestedTypes
	}

	// Multiple sources -> merge into new slice
	merged := make([]*shared.SymbolDefinition, total)
	i := 0
	for _, d := range methods {
		merged[i] = d
		i++
	}
	for _, d := range fields {
		merged[i] = d
		i++
	}
	for _, d := range nestedTypes {
		merged[i] = d
		i++
	}
	return merged
}
