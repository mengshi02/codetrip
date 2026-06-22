package graph

import (
	"testing"
)

// TestDeleteNode_ReverseAdjCleanup verifies that DeleteNode correctly cleans up
// reverse adjacency indexes on connected nodes (the core precision fix for incremental indexing).
//
// Scenario: A → CALLS → B. Delete A.
// Expected: B's in-adjacency no longer references A.
func TestDeleteNode_ReverseAdjCleanup(t *testing.T) {
	gs := openTestGS(t)

	// Create two nodes: A (caller) and B (callee)
	nodeA := NewNode("testrepo", LabelFunction, "caller").WithFile("caller.go")
	nodeB := NewNode("testrepo", LabelFunction, "callee").WithFile("callee.go")
	if err := addN(gs, nodeA); err != nil {
		t.Fatal(err)
	}
	if err := addN(gs, nodeB); err != nil {
		t.Fatal(err)
	}

	// Create edge: A → CALLS → B
	edge := NewEdge(RelCalls, nodeA.ID, nodeB.ID)
	if err := addE(gs, edge); err != nil {
		t.Fatal(err)
	}

	// Verify adjacency before deletion
	outEdgesA, _ := gs.GetAllOutEdges(nodeA.ID)
	if len(outEdgesA) != 1 {
		t.Fatalf("A should have 1 outgoing edge, got %d", len(outEdgesA))
	}
	inEdgesB, _ := gs.GetAllInEdges(nodeB.ID)
	if len(inEdgesB) != 1 {
		t.Fatalf("B should have 1 incoming edge, got %d", len(inEdgesB))
	}

	// Delete node A
	if err := gs.DeleteNode(nodeA.ID); err != nil {
		t.Fatal(err)
	}
	flushGS(gs)

	// Verify: B's incoming adjacency should no longer reference A
	inEdgesBAfter, _ := gs.GetAllInEdges(nodeB.ID)
	if len(inEdgesBAfter) != 0 {
		t.Errorf("B should have 0 incoming edges after A deletion, got %d: %v", len(inEdgesBAfter), inEdgesBAfter)
	}

	// Verify: A no longer exists
	if _, err := gs.GetNode(nodeA.ID); err == nil {
		t.Error("A should not exist after deletion")
	}

	// Verify: B still exists and is intact
	gotB, err := gs.GetNode(nodeB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotB.Name != "callee" {
		t.Errorf("B name = %q, want 'callee'", gotB.Name)
	}
}

// TestDeleteNode_BidirectionalAdjCleanup verifies cleanup for a node with both
// outgoing and incoming edges.
//
// Scenario: A → CALLS → B, C → DEFINES → A. Delete A.
// Expected: B's in-adjacency has no A; C's out-adjacency has no A.
func TestDeleteNode_BidirectionalAdjCleanup(t *testing.T) {
	gs := openTestGS(t)

	nodeA := NewNode("testrepo", LabelFunction, "middle").WithFile("mid.go")
	nodeB := NewNode("testrepo", LabelFunction, "target").WithFile("target.go")
	nodeC := NewNode("testrepo", LabelClass, "definer").WithFile("definer.go")
	addN(gs, nodeA)
	addN(gs, nodeB)
	addN(gs, nodeC)

	// A → CALLS → B (outgoing)
	addE(gs, NewEdge(RelCalls, nodeA.ID, nodeB.ID))
	// C → DEFINES → A (incoming to A)
	addE(gs, NewEdge(RelDefines, nodeC.ID, nodeA.ID))

	// Delete node A
	if err := gs.DeleteNode(nodeA.ID); err != nil {
		t.Fatal(err)
	}
	flushGS(gs)

	// B's incoming edges should not reference A
	inEdgesB, _ := gs.GetAllInEdges(nodeB.ID)
	if len(inEdgesB) != 0 {
		t.Errorf("B should have 0 incoming edges, got %d", len(inEdgesB))
	}

	// C's outgoing edges should not reference A
	outEdgesC, _ := gs.GetAllOutEdges(nodeC.ID)
	if len(outEdgesC) != 0 {
		t.Errorf("C should have 0 outgoing edges, got %d", len(outEdgesC))
	}
}

// TestDeleteNode_EdgeKVCleanup verifies that edge KV records are deleted
// when a node is removed.
func TestDeleteNode_EdgeKVCleanup(t *testing.T) {
	gs := openTestGS(t)

	nodeA := NewNode("testrepo", LabelFunction, "src").WithFile("src.go")
	nodeB := NewNode("testrepo", LabelFunction, "dst").WithFile("dst.go")
	addN(gs, nodeA)
	addN(gs, nodeB)

	edge := NewEdge(RelCalls, nodeA.ID, nodeB.ID).WithProp("confidence", 0.95)
	addE(gs, edge)

	// Verify edge exists before deletion
	outEdges, _ := gs.GetAllOutEdges(nodeA.ID)
	if len(outEdges) == 0 {
		t.Fatal("A should have outgoing edges before deletion")
	}

	// Delete node A (should also clean up edge KV)
	if err := gs.DeleteNode(nodeA.ID); err != nil {
		t.Fatal(err)
	}
	flushGS(gs)

	// Verify: scanning for outgoing edge KV from A should yield 0 results
	outEdgePrefix := "e:e:testrepo:" + nodeA.ID + ":"
	count := 0
	gs.store.ScanPrefix([]byte(outEdgePrefix), func(_, _ []byte) error {
		count++
		return nil
	})
	if count != 0 {
		t.Errorf("should have 0 outgoing edge KV records for deleted node A, got %d", count)
	}
}

// TestDeleteNode_FileAssociationCleanup verifies that deleting a node
// cleans up the file index association.
func TestDeleteNode_FileAssociationCleanup(t *testing.T) {
	gs := openTestGS(t)

	nodeA := NewNode("testrepo", LabelFunction, "func1").WithFile("main.go")
	addN(gs, nodeA)

	// Verify node is associated with file
	nodes, _ := gs.GetNodesByFile("testrepo", "main.go")
	if len(nodes) != 1 {
		t.Fatalf("should find 1 node for main.go, got %d", len(nodes))
	}

	// Delete node
	if err := gs.DeleteNode(nodeA.ID); err != nil {
		t.Fatal(err)
	}
	flushGS(gs)

	// Verify: file index should no longer reference the deleted node
	nodesAfter, _ := gs.GetNodesByFile("testrepo", "main.go")
	if len(nodesAfter) != 0 {
		t.Errorf("should find 0 nodes for main.go after deletion, got %d", len(nodesAfter))
	}
}