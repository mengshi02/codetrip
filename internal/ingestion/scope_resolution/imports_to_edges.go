package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ImportsToEdges converts finalized ImportEdge entries into graph IMPORTS edges.
// Mirrors TS scope-resolution/graph-bridge/imports-to-edges.ts emitImportEdges.
//
// Deduplicates by (sourceFile, targetFile) so multi-symbol imports
// from the same module collapse to a single edge — matching the
// legacy schema.
//
// Returns the number of edges emitted.
func ImportsToEdges(
	g shared.KnowledgeGraph,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) int {
	reason := "scope-resolution: import"
	if provider != nil {
		reason = provider.ImportEdgeReason()
	}

	seen := make(map[string]bool)
	emitted := 0
	scopeTree := indexes.ScopeTree()

	for scopeID, edges := range indexes.Imports() {
		scope := scopeTree.GetScope(scopeID)
		if scope == nil {
			continue
		}
		sourceFile := scope.FilePath
		if sourceFile == "" {
			continue
		}

		for _, edge := range edges {
			if edge.TargetFile == nil {
				continue
			}
			targetFile := *edge.TargetFile
			if targetFile == "" {
				continue
			}
			if targetFile == sourceFile {
				continue
			}

			dedupKey := fmt.Sprintf("%s->%s", sourceFile, targetFile)
			if seen[dedupKey] {
				continue
			}
			seen[dedupKey] = true

			EmitImportEdge(g, sourceFile, targetFile, edge.LocalName, reason, nil)
			emitted++
		}
	}

	return emitted
}

// EmitImportEdge emits a single IMPORTS edge from source file to target file.
// Mirrors TS emitImportEdge.
//
// Generates File→File IMPORTS edges with the given reason and evidence.
func EmitImportEdge(
	g shared.KnowledgeGraph,
	fromFile string,
	toFile string,
	localName string,
	reason string,
	evidence []shared.Evidence,
) {
	sourceID := fmt.Sprintf("File:%s", fromFile)
	targetID := fmt.Sprintf("File:%s", toFile)

	dedupKey := fmt.Sprintf("IMPORTS:%s->%s", fromFile, toFile)
	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:%s", dedupKey),
		Type:   graph.RelImports,
		Source: sourceID,
		Target: targetID,
	}
	edge.Props.SetProp("confidence", 1.0)
	edge.Props.SetProp("reason", reason)
	if localName != "" {
		edge.Props.SetProp("localName", localName)
	}
	if len(evidence) > 0 {
		shared.SetEdgeEvidence(edge, evidence)
	} else {
		shared.SetEdgeEvidence(edge, []shared.Evidence{
			{Kind: "scope-resolution", Weight: 1.0, Note: reason},
		})
	}

	g.AddEdge(edge)
}