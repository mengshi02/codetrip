package cfg

import (
	"github.com/mengshi02/codetrip/internal/graph"
)

// CollectCFGs walks the graph for all function nodes in the given repo
// and builds a FunctionCFG for each one using the provided builder.
//
// Mirrors TS cfg/collect.ts.
//
// Current status: skeleton — full implementation deferred.
func CollectCFGs(
	gs *graph.GraphStore,
	builder CFGBuilder,
	repo string,
) ([]*FunctionCFG, error) {
	_ = gs
	_ = builder
	_ = repo
	// TODO: query graph for all function nodes in repo
	// For each function node, call builder.Build(node, sourceCode)
	// Return collected FunctionCFGs
	return nil, nil
}