package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// BuildMroPass computes the method-resolution order for every class in the
// workspace using the provider's BuildMro strategy.
//
// Mirrors TS scope-resolution/passes/mro.ts.
//
// The MRO is used by the receiver-bound calls pass to dispatch method calls:
// given a receiver type, walk its MRO to find the class that owns the method.
//
// Different languages use different linearization strategies:
//   - Python: depth-first first-seen (simplified)
//   - C#: single inheritance only
//   - Java: single inheritance only
//   - C++: multiple inheritance, virtual base dedup
//   - Go: interface satisfaction (no classical MRO)
//
// Fully implemented — delegates to provider.BuildMro.
func BuildMroPass(
	provider ScopeResolver,
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	lookup *GraphNodeLookup,
) map[string][]string {
	// Delegate to the provider's BuildMro implementation
	return provider.BuildMro(graph, parsedFiles, lookup)
}