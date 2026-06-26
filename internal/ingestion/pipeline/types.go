// Package pipeline — dependency-ordered ingestion pipeline.
//
// The pipeline is composed of named phases with explicit dependencies.
// Each phase is defined in its own file under pipeline/phases/.
// The runner executes phases in topological order, passing typed outputs
// from upstream phases as inputs to downstream phases.
//
// Ported from gitnexus pipeline-phases/types.ts (100 lines).
package pipeline

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Shared context ─────────────────────────────────────────────────────────

// PipelineContext holds immutable context available to every phase.
type PipelineContext struct {
	// RepoPath is the absolute path to the repository root.
	RepoPath string
	// Graph is the mutable knowledge graph — the single shared accumulator.
	Graph shared.KnowledgeGraph
	// OnProgress is the progress callback for UI updates.
	OnProgress func(phase string, percent int, message string)
	// Options controls pipeline behaviour (skipGraphPhases, etc.).
	Options *PipelineOptions
	// PipelineStart is the pipeline start timestamp (for elapsed-time logging).
	PipelineStart int64
}

// ── Phase result wrapper ───────────────────────────────────────────────────

// PhaseResult wraps a phase's output with timing metadata.
type PhaseResult struct {
	// PhaseName matches the phase's Name field.
	PhaseName string
	// Output is the typed output of the phase.
	Output interface{}
	// DurationMs is the wall-clock duration in milliseconds.
	DurationMs int64
}

// ── Phase definition ───────────────────────────────────────────────────────

// PipelinePhase is a single phase in the ingestion pipeline.
//
// Each phase declares its dependencies; the runner guarantees those have
// completed before Execute is called. Phases are independently testable
// with mocked inputs.
type PipelinePhase interface {
	// Name is the unique name for logging and result lookup.
	Name() string

	// Deps returns the names of phases this phase depends on.
	// The runner guarantees these have completed before Execute is called.
	Deps() []string

	// Execute runs the phase.
	//
	// ctx is the shared pipeline context (graph, repoPath, progress, options).
	// deps is a map of dependency name → PhaseResult (typed outputs from upstream phases).
	// Returns the phase's typed output.
	Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error)
}

// ── Phase output extraction ────────────────────────────────────────────────

// GetPhaseOutput extracts the typed output of a dependency phase.
//
// This is the Go equivalent of the TS getPhaseOutput<T> helper.
// Callers should type-assert the result to the expected output type.
// Returns an error if the phase is not found in the dependency map.
func GetPhaseOutput(deps map[string]*PhaseResult, phaseName string) (interface{}, error) {
	result, ok := deps[phaseName]
	if !ok {
		return nil, fmt.Errorf("phase %q not found in resolved dependencies", phaseName)
	}
	return result.Output, nil
}

// GetPhaseOutputTyped extracts and type-asserts the output of a dependency phase.
// This is a convenience wrapper that combines GetPhaseOutput with a type assertion.
func GetPhaseOutputTyped[T any](deps map[string]*PhaseResult, phaseName string) (T, error) {
	var zero T
	out, err := GetPhaseOutput(deps, phaseName)
	if err != nil {
		return zero, err
	}
	typed, ok := out.(T)
	if !ok {
		return zero, fmt.Errorf("phase %q output type mismatch: got %T, want %T", phaseName, out, zero)
	}
	return typed, nil
}