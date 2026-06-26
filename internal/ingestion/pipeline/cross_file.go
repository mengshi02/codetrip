// Phase: crossFile
//
// Cross-file type propagation — resolves type references that span file
// boundaries using the exported type map and import edges.
//
// @deps    parse
// @reads   exportedTypeMap, parsedFiles (from parse), graph
// @writes  graph (EXTENDS/IMPLEMENTS/ACCESSES edges across files)
// @output  CrossFileOutput
//
// Ported from gitnexus pipeline-phases/cross-file.ts.
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// CrossFileOutput is the result of the crossFile phase.
type CrossFileOutput struct {
	// Number of cross-file type edges created
	CrossFileEdges int
}

// ── Phase implementation ─────────────────────────────────────────────────

// crossFilePhaseImpl implements the crossFile phase.
type crossFilePhaseImpl struct{ basePhase }

func (p *crossFilePhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	parseOut, err := GetPhaseOutputTyped[*ParseOutput](deps, "parse")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 70, "Resolving cross-file types...")
	}

	// TODO(Phase 3): Propagate exported types across file boundaries.
	// Use parseOut.ExportedTypeMap to resolve type references that point
	// to symbols defined in other files. Create EXTENDS/IMPLEMENTS/ACCESSES
	// edges in the graph.
	_ = parseOut.ExportedTypeMap
	_ = core.ImportKindDirect // will be used for import resolution

	return &CrossFileOutput{
		CrossFileEdges: 0,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var crossFilePhase = &crossFilePhaseImpl{basePhase{name: "crossFile", deps: []string{"parse"}}}