// Phase: communities
//
// Detects communities (clusters) in the knowledge graph using the
// Leiden algorithm. Communities help organize the graph for search
// and navigation.
//
// @deps    mro
// @reads   graph (all edges)
// @writes  graph (Community nodes, BELONGS_TO edges)
// @output  CommunitiesOutput
//
// Ported from gitnexus pipeline-phases/communities.ts.
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// CommunitiesOutput is the result of the communities phase.
type CommunitiesOutput struct {
	CommunityResult interface{} // *core.CommunityProcessorResult
	NumCommunities  int
	Modularity      float64
}

// ── Phase implementation ─────────────────────────────────────────────────

// communitiesPhaseImpl implements the communities phase.
type communitiesPhaseImpl struct{ basePhase }

func (p *communitiesPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 90, "Detecting communities...")
	}

	result, err := core.DetectCommunities(ctx.Graph, core.DefaultCommunityProcessorOptions)
	if err != nil {
		return nil, err
	}

	return &CommunitiesOutput{
		CommunityResult: result,
		NumCommunities:  result.NumCommunities,
		Modularity:      result.Modularity,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var communitiesPhase = &communitiesPhaseImpl{basePhase{name: "communities", deps: []string{"mro"}}}