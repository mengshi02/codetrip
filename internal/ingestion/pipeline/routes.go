// Phase: routes
//
// Extracts HTTP route definitions from parsed source code.
// Consumes route extraction results from the parse phase and builds
// a unified route map for the repository.
//
// @deps    parse
// @reads   allExtractedRoutes, allDecoratorRoutes (from parse)
// @writes  graph (Route nodes, CALLS edges for route handlers)
// @output  RoutesOutput
//
// Ported from gitnexus pipeline-phases/routes.ts (456 lines).
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Output type ──────────────────────────────────────────────────────────

// RoutesOutput is the result of the routes phase.
type RoutesOutput struct {
	RouteMap interface{} // typed later — map of route path → route info
	// Number of routes extracted
	RouteCount int
}

// ── Phase implementation ─────────────────────────────────────────────────

// routesPhaseImpl implements the routes phase.
type routesPhaseImpl struct{ basePhase }

func (p *routesPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	parseOut, err := GetPhaseOutputTyped[*ParseOutput](deps, "parse")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 55, "Building route map...")
	}

	// Merge extracted routes + decorator routes
	routes := parseOut.AllExtractedRoutes
	routes = append(routes, parseOut.AllDecoratorRoutes...)

	// TODO(Phase 3): Build unified route map from extracted routes.
	// For each route, create a Route node in the graph and link
	// handler symbols with HANDLES edges.
	// This requires the route_extractors to produce TaintFunctionCfg
	// results, which come from the taint pipeline (Phase 3).

	_ = shared.GenerateID // will be used for Route node creation

	return &RoutesOutput{
		RouteCount: len(routes),
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var routesPhase = &routesPhaseImpl{basePhase{name: "routes", deps: []string{"parse"}}}