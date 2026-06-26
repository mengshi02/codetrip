package graph

import (
	"context"
	"fmt"
	"testing"
)

func TestBFS(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	nD := NewNode("testrepo", LabelFunction, "D")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addN(gs, nD)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nC.ID))
	addE(gs, NewEdge(RelCalls, nA.ID, nD.ID))

	// 验证邻接索引
	edges, _ := gs.GetAllOutEdges(nA.ID)
	t.Logf("A out edges: %d", len(edges))
	edgesB, _ := gs.GetAllOutEdges(nB.ID)
	t.Logf("B out edges: %d", len(edgesB))

	if result, _ := gs.BFS(context.Background(), nA.ID, TraverseOut, 3, nil); len(result) != 3 {
		t.Errorf("BFS = %d, want 3", len(result))
	}
}

func TestBFS_MaxDepth(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nC.ID))
	if result, _ := gs.BFS(context.Background(), nA.ID, TraverseOut, 1, nil); len(result) != 1 {
		t.Errorf("BFS depth=1 = %d, want 1", len(result))
	}
}

func TestBFS_WithFilter(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	e1 := NewEdge(RelCalls, nA.ID, nB.ID)
	e2 := NewEdge(RelContains, nA.ID, nC.ID)
	addE(gs, e1)
	addE(gs, e2)

	// 验证所有边都存在
	allEdges, _ := gs.GetAllOutEdges(nA.ID)
	if len(allEdges) != 2 {
		t.Fatalf("all out edges = %d, want 2", len(allEdges))
	}

	result, _ := gs.BFS(context.Background(), nA.ID, TraverseOut, 2, func(e *Edge) bool { return e.Type == RelCalls })
	if len(result) != 1 {
		t.Errorf("got %d, want 1", len(result))
	}
}

func TestShortestPath(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nC.ID))
	path, err := gs.ShortestPath(context.Background(), nA.ID, nC.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(path) != 2 {
		t.Errorf("path len = %d, want 2", len(path))
	}
}

func TestShortestPath_NoPath(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	addN(gs, nA)
	addN(gs, nB)
	if _, err := gs.ShortestPath(context.Background(), nA.ID, nB.ID); err == nil {
		t.Error("expected error")
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	addN(gs, nA)
	addN(gs, nB)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	if cycles, _ := gs.DetectCycles(context.Background(), "testrepo"); len(cycles) != 0 {
		t.Errorf("got %d cycles, want 0", len(cycles))
	}
}

func TestDetectCycles_WithCycle(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nC.ID))
	addE(gs, NewEdge(RelCalls, nC.ID, nA.ID))
	cycles, _ := gs.DetectCycles(context.Background(), "testrepo")
	if len(cycles) == 0 {
		t.Error("should detect cycle")
	}
	// After deduplication, the same cycle A→B→C should only be reported once
	if len(cycles) > 1 {
		t.Errorf("expected 1 unique cycle after dedup, got %d", len(cycles))
	}
}

// TestDetectCycles_SupersetCycleRemoved verifies that a long detour cycle
// (A→B→X1→X2→C→A) is removed when a shorter core cycle (A→B→C→A) exists.
// This addresses the "22+ node monster cycles" issue from P2-10.
func TestDetectCycles_SupersetCycleRemoved(t *testing.T) {
	gs := openTestGS(t)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	nD := NewNode("testrepo", LabelFunction, "D") // detour node
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addN(gs, nD)

	// Core cycle: A→B→C→A (3 nodes)
	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nC.ID))
	addE(gs, NewEdge(RelCalls, nC.ID, nA.ID))
	// Detour: B→D→C creates longer path A→B→D→C→A
	addE(gs, NewEdge(RelCalls, nB.ID, nD.ID))
	addE(gs, NewEdge(RelCalls, nD.ID, nC.ID))

	cycles, _ := gs.DetectCycles(context.Background(), "testrepo")
	// Should only report the shortest cycle (A,B,C), not the detour (A,B,D,C)
	for _, c := range cycles {
		t.Logf("cycle: %v (len=%d)", c, len(c))
	}
	for _, c := range cycles {
		if len(c) > 3 {
			t.Errorf("superset cycle not removed: %v (len=%d)", c, len(c))
		}
	}
}

// TestDetectCycles_LongPathDedup verifies that DFS cycle detection with
// deduplication works correctly on a diamond graph. Note: three-color DFS
// may only discover one path through a diamond due to the visited marking,
// so we verify that at least one minimal cycle is found and no superset
// cycles remain.
func TestDetectCycles_LongPathDedup(t *testing.T) {
	gs := openTestGS(t)
	// Create a diamond: A→B→D and A→C→D, plus D→A (two potential cycles: A,B,D and A,C,D)
	nA := NewNode("testrepo", LabelFunction, "A")
	nB := NewNode("testrepo", LabelFunction, "B")
	nC := NewNode("testrepo", LabelFunction, "C")
	nD := NewNode("testrepo", LabelFunction, "D")
	addN(gs, nA)
	addN(gs, nB)
	addN(gs, nC)
	addN(gs, nD)

	addE(gs, NewEdge(RelCalls, nA.ID, nB.ID))
	addE(gs, NewEdge(RelCalls, nA.ID, nC.ID))
	addE(gs, NewEdge(RelCalls, nB.ID, nD.ID))
	addE(gs, NewEdge(RelCalls, nC.ID, nD.ID))
	addE(gs, NewEdge(RelCalls, nD.ID, nA.ID))

	cycles, _ := gs.DetectCycles(context.Background(), "testrepo")
	t.Logf("found %d cycles", len(cycles))
	for _, c := range cycles {
		t.Logf("  cycle: %v (len=%d)", c, len(c))
	}
	// DFS may find 1 or 2 cycles depending on traversal order,
	// but no cycle should be longer than 3 (the minimal cycle length)
	if len(cycles) == 0 {
		t.Error("expected at least 1 cycle")
	}
	for _, c := range cycles {
		if len(c) > 3 {
			t.Errorf("expected max 3 nodes per cycle, got %d: %v", len(c), c)
		}
	}
}

// TestDeduplicateAndSortCycles verifies that cycles with the same nodes
// but different start points are deduplicated, superset cycles are removed,
// and results are sorted by length.
func TestDeduplicateAndSortCycles(t *testing.T) {
	tests := []struct {
		name   string
		input  [][]string
		expect [][]string
	}{
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
		{
			name:   "single cycle",
			input:  [][]string{{"A", "B", "C"}},
			expect: [][]string{{"A", "B", "C"}},
		},
		{
			name:   "duplicate rotations",
			input:  [][]string{{"A", "B", "C"}, {"B", "C", "A"}, {"C", "A", "B"}},
			expect: [][]string{{"A", "B", "C"}},
		},
		{
			name:   "different cycles sorted by length",
			input:  [][]string{{"D", "E"}, {"A", "B", "C"}},
			expect: [][]string{{"D", "E"}, {"A", "B", "C"}},
		},
		{
			name:   "superset removal: A→B→C vs A→B→D→C (C is subset of A,B,C,D)",
			input:  [][]string{{"A", "B", "C"}, {"A", "B", "D", "C"}},
			expect: [][]string{{"A", "B", "C"}},
		},
		{
			name:   "superset removal: longer cycle with detour is removed",
			input:  [][]string{{"A", "B", "C"}, {"A", "B", "D", "E", "C"}},
			expect: [][]string{{"A", "B", "C"}},
		},
		{
			name:   "no superset: independent cycles kept",
			input:  [][]string{{"A", "B", "C"}, {"D", "E", "F"}},
			expect: [][]string{{"A", "B", "C"}, {"D", "E", "F"}},
		},
		{
			name:   "no superset: overlapping but not subset",
			input:  [][]string{{"A", "B", "C"}, {"B", "C", "D"}},
			expect: [][]string{{"A", "B", "C"}, {"B", "C", "D"}},
		},
		{
			name:   "22-node monster cycle removed by shorter core",
			input:  [][]string{{"A", "B", "C"}, {"A", "B", "X1", "X2", "X3", "X4", "X5", "X6", "X7", "X8", "X9", "X10", "X11", "X12", "X13", "X14", "X15", "X16", "X17", "X18", "X19", "C"}},
			expect: [][]string{{"A", "B", "C"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateAndSortCycles(tt.input)
			if len(result) != len(tt.expect) {
				t.Fatalf("expected %d cycles, got %d (result=%v)", len(tt.expect), len(result), result)
			}
			// Verify sorted by length
			for i := 1; i < len(result); i++ {
				if len(result[i]) < len(result[i-1]) {
					t.Errorf("result not sorted by length: cycle %d len=%d > cycle %d len=%d",
						i-1, len(result[i-1]), i, len(result[i]))
				}
			}
			for i, exp := range tt.expect {
				if len(result[i]) != len(exp) {
					t.Fatalf("cycle %d: expected length %d, got %d", i, len(exp), len(result[i]))
				}
				for j, node := range exp {
					if result[i][j] != node {
						t.Errorf("cycle %d pos %d: expected %s, got %s", i, j, node, result[i][j])
					}
				}
			}
		})
	}
}

func TestBFS_TraversalLimit(t *testing.T) {
	gs := openTestGS(t)
	// Create a chain: N0 -> N1 -> ... -> N9
	nodes := make([]*Node, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = NewNode("testrepo", LabelFunction, fmt.Sprintf("N%d", i))
		addN(gs, nodes[i])
	}
	for i := 0; i < 9; i++ {
		addE(gs, NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID))
	}
	// Set very low limit — only allow 2 nodes visited
	gs.SetTraversalLimit(2)
	result, err := gs.BFS(context.Background(), nodes[0].ID, TraverseOut, 10, nil)
	if err != ErrTraversalLimitExceeded {
		t.Errorf("expected ErrTraversalLimitExceeded, got %v", err)
	}
	if len(result) == 0 {
		t.Error("should have partial results before limit")
	}
}

func TestBFS_ContextCancellation(t *testing.T) {
	gs := openTestGS(t)
	// Create graph
	for i := 0; i < 10; i++ {
		addN(gs, NewNode("testrepo", LabelFunction, fmt.Sprintf("N%d", i)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	result, err := gs.BFS(ctx, NewNode("testrepo", LabelFunction, "N0").ID, TraverseOut, 10, nil)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	_ = result
}