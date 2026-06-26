// Phase: pruneLocalSymbols
//
// Removes non-exported, unreferenced symbols from the graph.
// After scope resolution, symbols that are neither exported nor
// referenced from other files can be pruned to reduce graph size
// by ~60-80% on large repos.
//
// @deps    scopeResolution
// @reads   graph (symbol export/reference status)
// @writes  graph (removes pruned Symbol nodes)
// @output  PruneLocalSymbolsOutput
//
// Ported from gitnexus pipeline-phases/prune-local-symbols.ts.
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// PruneLocalSymbolsOutput is the result of the pruneLocalSymbols phase.
type PruneLocalSymbolsOutput struct {
	PrunedCount int
}

// ── Phase implementation ─────────────────────────────────────────────────

// pruneLocalSymbolsPhaseImpl implements the pruneLocalSymbols phase.
type pruneLocalSymbolsPhaseImpl struct{ basePhase }

func (p *pruneLocalSymbolsPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 80, "Pruning local symbols...")
	}

	pruned := core.PruneLocalSymbols(ctx.Graph, core.PruneLocalSymbolsOptions{
		KeepExported:    true,
		KeepHeritage:    true,
		ProtectedLabels: nil, // use defaults
	})

	return &PruneLocalSymbolsOutput{
		PrunedCount: pruned,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var pruneLocalSymbolsPhase = &pruneLocalSymbolsPhaseImpl{basePhase{name: "pruneLocalSymbols", deps: []string{"scopeResolution"}}}