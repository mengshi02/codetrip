// Phase: scopeResolution
//
// Runs the per-language scope-resolution orchestrator to build
// the fully-resolved symbol graph: import edges, heritage edges,
// interface implementations, and MRO.
//
// @deps    crossFile
// @reads   parsedFiles, graph
// @writes  graph (IMPORTS/EXTENDS/IMPLEMENTS/CALLS/ACCESSES edges)
// @output  ScopeResolutionOutput
//
// Ported from gitnexus pipeline-phases/scope-resolution.ts.
package pipeline

import (
	scoperesolution "github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
)

// ── Output type ──────────────────────────────────────────────────────────

// ScopeResolutionOutput is the result of the scopeResolution phase.
type ScopeResolutionOutput struct {
	ResolutionOutcomes interface{} // typed later — []scoperesolution.ResolutionOutcome
	FilesProcessed     int
	ImportsEmitted     int
	ReferenceEdges     int
}

// ── Phase implementation ─────────────────────────────────────────────────

// scopeResolutionPhaseImpl implements the scopeResolution phase.
type scopeResolutionPhaseImpl struct{ basePhase }

func (p *scopeResolutionPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 75, "Resolving scopes...")
	}

	// TODO(Phase 2): Wire up scope resolution per language.
	// For each supported language:
	//   1. Get the language provider's ScopeResolver
	//   2. Build RunScopeResolutionInput from parse results + graph
	//   3. Call scoperesolution.RunScopeResolution(input)
	//   4. Collect outcomes

	_ = scoperesolution.RunScopeResolutionInput{} // will be used

	return &ScopeResolutionOutput{
		FilesProcessed: 0,
		ImportsEmitted: 0,
		ReferenceEdges: 0,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var scopeResolutionPhase = &scopeResolutionPhaseImpl{basePhase{name: "scopeResolution", deps: []string{"crossFile"}}}