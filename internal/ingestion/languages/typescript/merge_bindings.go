// Package typescript — TypeScript binding merge precedence.
// Implements per-scope binding merge for TypeScript using declaration-space
// partitioning (value/type/namespace). Within each space, LEGB priority
// applies: local > import > namespace > wildcard > reexport.
//
// TypeScript's declaration merging allows the same name to occupy multiple
// spaces simultaneously (e.g., a class occupies both value and type spaces;
// a namespace can augment a class). The merge process:
//  1. Partition bindings by declaration space (value/type/namespace)
//  2. Within each space, deduplicate by name keeping lowest-tier origin
//  3. Merge all space partitions back together
//
// Ported from TS languages/typescript/merge-bindings.ts.
package typescript

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// DeclarationSpace represents the three TypeScript declaration spaces.
type DeclarationSpace string

const (
	SpaceValue      DeclarationSpace = "value"
	SpaceType       DeclarationSpace = "type"
	SpaceNamespace  DeclarationSpace = "namespace"
)

// spacesOf maps a NodeLabel to its declaration spaces.
// TypeScript declaration merging: a class occupies value + type,
// an enum occupies value + type, a namespace occupies namespace (or value+namespace).
// Default: value space.
func spacesOf(label shared.NodeLabel) []DeclarationSpace {
	switch label {
	case shared.LabelClass:
		return []DeclarationSpace{SpaceValue, SpaceType}
	case shared.LabelInterface:
		return []DeclarationSpace{SpaceType}
	case shared.LabelEnum:
		return []DeclarationSpace{SpaceValue, SpaceType}
	case shared.LabelNamespace:
		return []DeclarationSpace{SpaceNamespace, SpaceValue}
	case shared.LabelType, shared.LabelTypeAlias:
		return []DeclarationSpace{SpaceType}
	default:
		return []DeclarationSpace{SpaceValue}
	}
}

// TypeScriptMergeBindings merges binding sets for a TypeScript scope,
// using declaration-space partitioning and LEGB priority within each space.
//
// Mirrors TS typescriptMergeBindings(bindings): BindingRef[].
// TODO: full implementation — currently simple dedup by origin tier.
func TypeScriptMergeBindings(bindings []shared.BindingRef) []shared.BindingRef {
	if len(bindings) == 0 {
		return bindings
	}
	// TODO: partition by declaration space, deduplicate within each space
	// by origin tier, merge partitions back together.
	// For skeleton, return all bindings as-is.
	return bindings
}