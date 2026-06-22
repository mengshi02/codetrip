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
	if cycles, _ := gs.DetectCycles(context.Background(), "testrepo"); len(cycles) == 0 {
		t.Error("should detect cycle")
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