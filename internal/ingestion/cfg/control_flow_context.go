package cfg

// ControlFlowContext holds the mutable state used during CFG construction.
// It tracks the current block, pending edges, loop/branch stack, and
// exception handler scope.
//
// Mirrors TS cfg/control-flow-context.ts.
//
// Key responsibilities:
//   - Maintain current block ID and pending edge list
//   - Push/pop loop/branch contexts for break/continue resolution
//   - Track exception handler scopes for try/catch/finally
//   - Manage break/continue target stacks
//
// Current status: skeleton — full implementation deferred.
type ControlFlowContext struct {
	CurrentBlockID string
	PendingEdges   []pendingEdge
	LoopStack      []loopContext
	BranchStack    []branchContext
	// ExceptionHandlerStack tracks try/catch scopes
	ExceptionHandlerStack []exceptionHandlerContext
}

type loopContext struct {
	BreakTargetID    string
	ContinueTargetID string
}

type branchContext struct {
	MergeBlockID string
}

type exceptionHandlerContext struct {
	TryBlockID    string
	CatchBlockID  string
	FinallyBlockID string
}

// NewControlFlowContext creates a new empty control flow context.
func NewControlFlowContext() *ControlFlowContext {
	return &ControlFlowContext{
		PendingEdges:           make([]pendingEdge, 0, 16),
		LoopStack:              make([]loopContext, 0, 8),
		BranchStack:            make([]branchContext, 0, 8),
		ExceptionHandlerStack: make([]exceptionHandlerContext, 0, 4),
	}
}