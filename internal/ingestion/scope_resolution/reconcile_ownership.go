package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ReconcileStats holds the counters from a ReconcileOwnership pass.
type ReconcileStats struct {
	MethodsRegistered      int
	FieldsRegistered       int
	NestedTypesRegistered  int
	SkippedAlreadyPresent  int
}

// nestedTypeKinds are the NodeLabel values that represent nested type definitions.
var nestedTypeKinds = map[shared.NodeLabel]bool{
	shared.LabelClass:     true,
	shared.LabelInterface: true,
	shared.LabelEnum:      true,
	shared.LabelStruct:    true,
	shared.LabelUnion:     true,
	shared.LabelTrait:     true,
	shared.LabelTypeAlias: true,
	shared.LabelTypedef:   true,
	shared.LabelRecord:    true,
	shared.LabelDelegate:  true,
	shared.LabelAnnotation: true,
	shared.LabelTemplate:  true,
	shared.LabelNamespace: true,
}

// ReconcileOwnership ensures that every parsed definition has the correct
// owner ID in the SemanticModel. After the provider's PopulateOwners hook
// corrects any ownership that the legacy parse pass missed, this function
// registers the corrected owner in the model's registries.
//
// Mirrors TS scope-resolution/pipeline/reconcile-ownership.ts reconcileOwnership.
//
// Contract invariant I9: SemanticModel is the single authoritative symbol store.
// This pass is a transitional shim — the end state is for every language's
// parse-time extractor to emit the correct ownerId directly.
//
// Idempotent: skips registration when (ownerId, simpleName) already
// contains a def with matching nodeId.
func ReconcileOwnership(
	parsedFiles []*shared.ParsedFile,
	semanticModel model.MutableSemanticModel,
	onWarn func(string),
) ReconcileStats {
	stats := ReconcileStats{}

	methods := semanticModel.MethodsMut()
	fields := semanticModel.FieldsMut()
	types := semanticModel.TypesMut()
	symbols := semanticModel.SymbolsMut()

	for _, parsed := range parsedFiles {
		for i := range parsed.LocalDefs {
			def := &parsed.LocalDefs[i]
			simple := SimpleQualifiedName(def)
			if simple == "" {
				continue
			}

			ownerID := ""
			if def.OwnerID != nil {
				ownerID = *def.OwnerID
			}

			if isCallableLabel(def.Type) {
				// Method / Function / Constructor
				if ownerID == "" {
					// No owner — handle deleted defs without owner
					if def.IsDeleted {
						existing := symbols.LookupExactAll(def.FilePath, simple)
						idx := findCallableSignatureMatch(existing, def)
						if idx >= 0 {
							existing[idx].IsDeleted = true
							stats.SkippedAlreadyPresent++
							continue
						}
						// Register as deleted in symbol table
						symbols.Add(def.FilePath, simple, def.NodeID, def.Type, &model.AddMetadata{
							ParameterCount:         def.ParameterCount,
							RequiredParameterCount: def.RequiredParameterCount,
							ParameterTypes:         def.ParameterTypes,
							ParameterTypeClasses:   def.ParameterTypeClasses,
							ReturnType:             def.ReturnType,
							QualifiedName:          def.QualifiedName,
							IsDeleted:              true,
						})
					}
					continue
				}

				// Check for existing registration (idempotent)
				existing := methods.LookupAllByOwner(ownerID, simple)
				existingIdx := findMatchingNodeIdOrCallableSignature(existing, def)
				if existingIdx >= 0 {
					if def.IsDeleted {
						existing[existingIdx].IsDeleted = true
					}
					stats.SkippedAlreadyPresent++
					continue
				}

				methods.Register(ownerID, simple, def)
				stats.MethodsRegistered++

			} else if IsOwnableValueLabel(def.Type) {
				// Property / Variable / Const / Static
				if ownerID == "" {
					continue
				}
				existing := fields.LookupAllByOwner(ownerID, simple)
				if hasMatchingNodeId(existing, def.NodeID) {
					stats.SkippedAlreadyPresent++
					continue
				}
				fields.Register(ownerID, simple, def)
				stats.FieldsRegistered++

			} else if nestedTypeKinds[def.Type] {
				// Nested type definitions
				if ownerID == "" {
					continue
				}
				existing := types.LookupAllByOwner(ownerID, simple)
				if hasMatchingNodeId(existing, def.NodeID) {
					stats.SkippedAlreadyPresent++
					continue
				}
				types.RegisterByOwner(ownerID, simple, def)
				stats.NestedTypesRegistered++
			}
		}
	}

	return stats
}

// callableSignatureMatches checks if two definitions have matching callable signatures.
// Matches on filePath, parameterCount, requiredParameterCount, and parameterTypes.
func callableSignatureMatches(left, right *shared.SymbolDefinition) bool {
	if left.FilePath != right.FilePath {
		return false
	}
	if left.ParameterCount != right.ParameterCount {
		return false
	}
	if left.RequiredParameterCount != right.RequiredParameterCount {
		return false
	}
	leftTypes := left.ParameterTypes
	rightTypes := right.ParameterTypes
	if len(leftTypes) != len(rightTypes) {
		return false
	}
	for i := range leftTypes {
		if leftTypes[i] != rightTypes[i] {
			return false
		}
	}
	return true
}

// findMatchingNodeIdOrCallableSignature finds a def in the list that matches
// by nodeId or, if the incoming def is deleted, by callable signature.
func findMatchingNodeIdOrCallableSignature(list []*shared.SymbolDefinition, def *shared.SymbolDefinition) int {
	for i, candidate := range list {
		if candidate.NodeID == def.NodeID {
			return i
		}
		if def.IsDeleted && callableSignatureMatches(candidate, def) {
			return i
		}
	}
	return -1
}

// findCallableSignatureMatch finds a def in the list matching by callable signature.
func findCallableSignatureMatch(list []*shared.SymbolDefinition, def *shared.SymbolDefinition) int {
	for i, candidate := range list {
		if callableSignatureMatches(candidate, def) {
			return i
		}
	}
	return -1
}

// hasMatchingNodeId checks if any def in the list has the given nodeId.
func hasMatchingNodeId(list []*shared.SymbolDefinition, nodeID string) bool {
	for _, candidate := range list {
		if candidate.NodeID == nodeID {
			return true
		}
	}
	return false
}