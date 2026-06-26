// Phase: orm
//
// Extracts ORM query definitions from parsed source code.
// ORM queries represent database interactions (SQL, NoSQL, ORM calls).
//
// @deps    parse
// @reads   allORMQueries (from parse)
// @writes  graph (ORMQuery nodes, QUERIES edges)
// @output  ORMOutput
//
// Ported from gitnexus pipeline-phases/orm.ts.
package pipeline

// ── Output type ──────────────────────────────────────────────────────────

// ORMOutput is the result of the ORM phase.
type ORMOutput struct {
	ORMQueries []interface{}
}

// ── Phase implementation ─────────────────────────────────────────────────

// ormPhaseImpl implements the ORM phase.
type ormPhaseImpl struct{ basePhase }

func (p *ormPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	parseOut, err := GetPhaseOutputTyped[*ParseOutput](deps, "parse")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 65, "Extracting ORM queries...")
	}

	// TODO(Phase 3): Create ORMQuery nodes in graph for each ORM definition.
	// Each query gets a QUERIES edge linking to its containing symbol.

	return &ORMOutput{
		ORMQueries: parseOut.AllORMQueries,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var ormPhase = &ormPhaseImpl{basePhase{name: "orm", deps: []string{"parse"}}}