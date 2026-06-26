// Phase: tools
//
// Extracts tool definitions (MCP tools, CLI commands) from parsed source.
// Tool definitions describe callable operations with their parameters.
//
// @deps    parse
// @reads   allToolDefs (from parse)
// @writes  graph (Tool nodes, DEFINED_IN edges)
// @output  ToolsOutput
//
// Ported from gitnexus pipeline-phases/tools.ts.
package pipeline

// ── Output type ──────────────────────────────────────────────────────────

// ToolsOutput is the result of the tools phase.
type ToolsOutput struct {
	ToolDefs []interface{}
}

// ── Phase implementation ─────────────────────────────────────────────────

// toolsPhaseImpl implements the tools phase.
type toolsPhaseImpl struct{ basePhase }

func (p *toolsPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	parseOut, err := GetPhaseOutputTyped[*ParseOutput](deps, "parse")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 60, "Extracting tool definitions...")
	}

	// TODO(Phase 3): Create Tool nodes in graph for each tool definition.
	// Each tool gets a DEFINED_IN edge linking to its containing symbol.

	return &ToolsOutput{
		ToolDefs: parseOut.AllToolDefs,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var toolsPhase = &toolsPhaseImpl{basePhase{name: "tools", deps: []string{"parse"}}}