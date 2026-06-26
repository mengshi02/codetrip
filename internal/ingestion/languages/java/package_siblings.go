// Package java — Java package sibling population.
// In Java, all classes in the same package have implicit visibility to each
// other's public/protected declarations without explicit import statements.
// This hook populates those cross-file sibling bindings so that the
// scope-resolution pipeline can resolve intra-package references.
// Ported from TS languages/java/package-siblings.ts.
package java

import (
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PopulateJavaPackageSiblings adds implicit sibling visibility for Java
// files in the same package directory. All public top-level defs from one
// file become import-visible to sibling files in the same package.
//
// Mirrors TS populateJavaPackageSiblings(graph, parsedFiles, nodeLookup, indexes).
// TODO: full implementation — currently no-op.
func PopulateJavaPackageSiblings(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	// TODO: group parsedFiles by package declaration, for each group
	// add sibling ImportEdges and binding augmentations.
}