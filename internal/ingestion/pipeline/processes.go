// Phase: processes
//
// Extracts high-level process abstractions from the codebase graph.
// Processes represent end-to-end flows (e.g., HTTP handlers, event listeners,
// CLI commands) and are scored using entry-point heuristics and framework
// detection.
//
// @deps    communities
// @reads   graph (Community nodes, cross-community edges), framework hints
// @writes  graph (Process nodes, PART_OF edges)
// @output  ProcessesOutput
//
// Ported from gitnexus pipeline-phases/processes.ts (160 lines).
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// ProcessesOutput is the result of the processes phase.
type ProcessesOutput struct {
	ProcessResult interface{} // *core.ProcessProcessorResult
	ProcessCount  int
}

// ── Phase implementation ─────────────────────────────────────────────────

// processesPhaseImpl implements the processes phase.
type processesPhaseImpl struct{ basePhase }

func (p *processesPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 95, "Detecting processes...")
	}

	// Detect framework hints from all paths
	// TODO(Phase 3): collect framework hints from parse phase results
	var frameworkHints []core.FrameworkHintExt

	result, err := core.DetectProcesses(ctx.Graph, frameworkHints)
	if err != nil {
		return nil, err
	}

	return &ProcessesOutput{
		ProcessResult: result,
		ProcessCount:  len(result.Processes),
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var processesPhase = &processesPhaseImpl{basePhase{name: "processes", deps: []string{"communities"}}}