package cfg

// TraversalResult holds the output of a CFG traversal (e.g. topological
// sort or reverse postorder) used by reaching-definitions and taint
// analysis passes.
//
// Mirrors TS cfg/traversal-result.ts.
//
// Current status: skeleton — full implementation deferred.
type TraversalResult struct {
	// Order is the block IDs in traversal order (reverse postorder).
	Order []string
	// Dominators maps block ID → immediate dominator block ID.
	Dominators map[string]string
	// PostOrder is the block IDs in postorder.
	PostOrder []string
}

// ComputeTraversal computes reverse postorder and dominators for the
// given FunctionCFG.
//
// Current status: skeleton — full implementation deferred.
func ComputeTraversal(fcfg *FunctionCFG) *TraversalResult {
	_ = fcfg
	// TODO: implement reverse postorder traversal
	// TODO: implement iterative dominator computation
	return &TraversalResult{
		Order:      make([]string, 0),
		Dominators: make(map[string]string),
		PostOrder:  make([]string, 0),
	}
}