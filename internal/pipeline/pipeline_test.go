package pipeline

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ============ Test helpers ============

// mockPhase is a simple Phase implementation for testing.
type mockPhase struct {
	name     string
	deps     []string
	runCalls atomic.Int32
	runErr   error
	delay    time.Duration
}

func (m *mockPhase) Name() string          { return m.name }
func (m *mockPhase) Dependencies() []string { return m.deps }
func (m *mockPhase) Run(ctx context.Context, input *PhaseInput) (*PhaseOutput, error) {
	m.runCalls.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.runErr != nil {
		return nil, m.runErr
	}
	return &PhaseOutput{}, nil
}

// ============ Core tests ============

// TestTopologicalSort verifies that Kahn's algorithm produces a correct
// topological order respecting all dependency edges.
func TestTopologicalSort(t *testing.T) {
	p := NewPipeline()
	p.Register(&mockPhase{name: "scan"})
	p.Register(&mockPhase{name: "parse", deps: []string{"scan"}})
	p.Register(&mockPhase{name: "scopeResolution", deps: []string{"parse"}})
	p.Register(&mockPhase{name: "crossFile", deps: []string{"parse"}})
	p.Register(&mockPhase{name: "embeddings", deps: []string{"parse"}})
	p.Register(&mockPhase{name: "index", deps: []string{"embeddings"}})

	p.ensureOrder()
	order := p.order

	// Verify: scan must come before parse, parse before everything else that depends on it
	pos := make(map[string]int)
	for i, name := range order {
		pos[name] = i
	}

	if pos["scan"] >= pos["parse"] {
		t.Errorf("scan (pos=%d) should come before parse (pos=%d)", pos["scan"], pos["parse"])
	}
	if pos["parse"] >= pos["scopeResolution"] {
		t.Errorf("parse (pos=%d) should come before scopeResolution (pos=%d)", pos["parse"], pos["scopeResolution"])
	}
	if pos["parse"] >= pos["crossFile"] {
		t.Errorf("parse (pos=%d) should come before crossFile (pos=%d)", pos["parse"], pos["crossFile"])
	}
	if pos["parse"] >= pos["embeddings"] {
		t.Errorf("parse (pos=%d) should come before embeddings (pos=%d)", pos["parse"], pos["embeddings"])
	}
	if pos["embeddings"] >= pos["index"] {
		t.Errorf("embeddings (pos=%d) should come before index (pos=%d)", pos["embeddings"], pos["index"])
	}
}

// TestComputeLayers verifies that phases with the same in-degree zero
// (no pending dependencies) are grouped into the same layer for parallel execution.
func TestComputeLayers(t *testing.T) {
	p := NewPipeline()
	p.Register(&mockPhase{name: "a"})
	p.Register(&mockPhase{name: "b"})
	p.Register(&mockPhase{name: "c", deps: []string{"a", "b"}})
	p.Register(&mockPhase{name: "d", deps: []string{"c"}})

	p.ensureOrder()
	layers := p.computeLayers()

	// Expected: Layer 1 = {a, b}, Layer 2 = {c}, Layer 3 = {d}
	if len(layers) < 3 {
		t.Fatalf("expected at least 3 layers, got %d", len(layers))
	}

	// First layer should have exactly 2 phases (a and b)
	if len(layers[0]) != 2 {
		t.Errorf("layer 0: expected 2 phases, got %d: %v", len(layers[0]), layers[0])
	}
	// Second layer should have exactly 1 phase (c)
	if len(layers[1]) != 1 {
		t.Errorf("layer 1: expected 1 phase, got %d: %v", len(layers[1]), layers[1])
	}
	// Third layer should have exactly 1 phase (d)
	if len(layers[2]) != 1 {
		t.Errorf("layer 2: expected 1 phase, got %d: %v", len(layers[2]), layers[2])
	}
}

// TestLayerParallelExecution verifies that phases in the same layer execute
// concurrently (in parallel) rather than sequentially.
func TestLayerParallelExecution(t *testing.T) {
	p := NewPipeline()
	// Two independent phases with 100ms delay each
	p.Register(&mockPhase{name: "a", delay: 100 * time.Millisecond})
	p.Register(&mockPhase{name: "b", delay: 100 * time.Millisecond})
	p.Register(&mockPhase{name: "c", deps: []string{"a", "b"}})

	input := &PhaseInput{Repo: "test"}
	start := time.Now()
	err := p.Run(context.Background(), input)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// If a and b ran sequentially, total would be ~200ms+.
	// If they ran in parallel, total should be ~100ms+ (within tolerance).
	if duration > 250*time.Millisecond {
		t.Errorf("phases a and b should have run in parallel; total duration %v suggests sequential execution", duration)
	}
}

// TestContextCancel verifies that the pipeline respects context cancellation
// and stops executing phases when the context is canceled.
func TestContextCancel(t *testing.T) {
	p := NewPipeline()
	p.Register(&mockPhase{name: "scan"})
	p.Register(&mockPhase{name: "parse", deps: []string{"scan"}})
	p.Register(&mockPhase{name: "slowPhase", deps: []string{"parse"}, delay: 5 * time.Second})
	p.Register(&mockPhase{name: "neverReached", deps: []string{"slowPhase"}})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := &PhaseInput{Repo: "test"}
	err := p.Run(ctx, input)

	if err == nil {
		t.Error("expected error from context cancellation, got nil")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", ctx.Err())
	}
}

// TestPhaseDependencyMissing verifies that a phase with a dependency on a
// non-existent phase still executes (the missing dependency is skipped).
func TestPhaseDependencyMissing(t *testing.T) {
	p := NewPipeline()
	p.Register(&mockPhase{name: "a", deps: []string{"nonexistent"}})
	p.Register(&mockPhase{name: "b"})

	input := &PhaseInput{Repo: "test"}
	err := p.Run(context.Background(), input)

	// Missing dependencies are simply ignored — both phases should still run
	if err != nil {
		t.Fatalf("pipeline should tolerate missing deps, got error: %v", err)
	}

	phaseA := p.phases["a"].(*mockPhase)
	phaseB := p.phases["b"].(*mockPhase)
	if phaseA.runCalls.Load() != 1 {
		t.Errorf("phase a should have been called once, got %d", phaseA.runCalls.Load())
	}
	if phaseB.runCalls.Load() != 1 {
		t.Errorf("phase b should have been called once, got %d", phaseB.runCalls.Load())
	}
}

// TestPhaseRunError verifies that a phase error propagates and stops
// subsequent phases from executing.
func TestPhaseRunError(t *testing.T) {
	p := NewPipeline()
	p.Register(&mockPhase{name: "a", runErr: context.DeadlineExceeded})
	p.Register(&mockPhase{name: "b", deps: []string{"a"}})

	input := &PhaseInput{Repo: "test"}
	err := p.Run(context.Background(), input)

	if err == nil {
		t.Error("expected error from phase a, got nil")
	}

	phaseB := p.phases["b"].(*mockPhase)
	if phaseB.runCalls.Load() != 0 {
		t.Errorf("phase b should not have been called after phase a failed, got %d calls", phaseB.runCalls.Load())
	}
}

// TestEmptyPipeline verifies that running an empty pipeline succeeds without error.
func TestEmptyPipeline(t *testing.T) {
	p := NewPipeline()
	input := &PhaseInput{Repo: "test"}
	err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("empty pipeline should succeed, got: %v", err)
	}
}