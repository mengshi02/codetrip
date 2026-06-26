package taint

import (
	"github.com/mengshi02/codetrip/internal/graph"
)

// EmitTaintFindings writes taint analysis results to the graph as
// TAINT edges between source and sink nodes.
//
// Mirrors TS taint/emit.ts.
//
// Current status: skeleton — full implementation deferred.
func EmitTaintFindings(gs *graph.GraphStore, results []TaintResult) error {
	_ = gs
	_ = results
	// TODO: for each TaintResult, create TAINT edge from source to sink
	// Include sanitized flag and confidence as edge properties
	return nil
}