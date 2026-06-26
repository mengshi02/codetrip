// Phase: mro
//
// Computes Method Resolution Order (C3-linearization) for type hierarchies.
// After cross-file resolution establishes IMPLEMENTS/EXTENDS edges,
// MRO computes deterministic linearization order so method dispatch
// can be resolved unambiguously.
//
// @deps    pruneLocalSymbols
// @reads   graph (EXTENDS/IMPLEMENTS edges)
// @writes  graph (MRO edges)
// @output  MROOutput
//
// Ported from gitnexus pipeline-phases/mro.ts.
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// MROOutput is the result of the MRO phase.
type MROOutput struct {
	// Number of types with computed linearizations
	LinearizedTypes int
	// Types with cyclic heritage (broken by removing duplicates)
	CyclicTypes []string
}

// ── Phase implementation ─────────────────────────────────────────────────

// mroPhaseImpl implements the MRO phase.
type mroPhaseImpl struct{ basePhase }

func (p *mroPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 85, "Computing MRO...")
	}

	result, err := core.ComputeMRO(ctx.Graph)
	if err != nil {
		return nil, err
	}

	return &MROOutput{
		LinearizedTypes: len(result.Linearizations),
		CyclicTypes:     result.CyclicTypes,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var mroPhase = &mroPhaseImpl{basePhase{name: "mro", deps: []string{"pruneLocalSymbols"}}}