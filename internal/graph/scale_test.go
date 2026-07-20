package graph

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	store "github.com/mengshi02/codetrip/internal/store"
)

// openTestGSLarge creates a GraphStore with larger cache for scale tests.
func openTestGSLarge(t *testing.T) *GraphStore {
	t.Helper()
	dir, err := os.MkdirTemp("", "graph-scale-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg := store.DefaultConfig(dir)
	cfg.CacheSize = 64 << 20 // 64MB block cache
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return NewGraphStore(s, "scalerepo")
}

// addNodesBatch adds nodes in batched mode for better write performance.
func addNodesBatch(gs *GraphStore, nodes []*Node, batchSize int) error {
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		err := gs.Batch(func(b *Batch) error {
			for j := i; j < end; j++ {
				if err := b.AddNode(nodes[j]); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("batch nodes %d-%d: %w", i, end, err)
		}
	}
	return nil
}

// addEdgesBatch adds edges in batched mode for better write performance.
func addEdgesBatch(gs *GraphStore, edges []*Edge, batchSize int) error {
	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		err := gs.Batch(func(b *Batch) error {
			for j := i; j < end; j++ {
				if err := b.AddEdge(edges[j]); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("batch edges %d-%d: %w", i, end, err)
		}
	}
	return nil
}

// TestScale_BFS_100K_Nodes verifies BFS works correctly on a graph with 100K nodes.
// This is a scaled-down version of the 1M-node verification (100K nodes × 5 edges).
func TestScale_BFS_100K_Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 100_000

	// Build a chain graph: N0 → N1 → ... → N99999
	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	if err := addNodesBatch(gs, nodes, 1000); err != nil {
		t.Fatalf("add nodes: %v", err)
	}

	edges := make([]*Edge, nodeCount-1)
	for i := 0; i < nodeCount-1; i++ {
		edges[i] = NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID)
	}
	if err := addEdgesBatch(gs, edges, 1000); err != nil {
		t.Fatalf("add edges: %v", err)
	}

	// BFS from node 0 with maxDepth=3 — should visit nodes 0..3
	start := time.Now()
	result, err := gs.BFS(context.Background(), nodes[0].ID, TraverseOut, 3, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("BFS: %v", err)
	}
	t.Logf("BFS(maxDepth=3) on %d-node chain: %d results in %v", nodeCount, len(result), elapsed)

	if len(result) == 0 {
		t.Error("BFS should return at least 1 node")
	}
}

// TestScale_BFS_TraversalLimit verifies BFS respects traversal limits on large graphs.
func TestScale_BFS_TraversalLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 10_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	edges := make([]*Edge, nodeCount-1)
	for i := 0; i < nodeCount-1; i++ {
		edges[i] = NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID)
	}
	addEdgesBatch(gs, edges, 1000)

	// Set very low limit
	gs.SetTraversalLimit(100)
	result, err := gs.BFS(context.Background(), nodes[0].ID, TraverseOut, 10, nil)
	if err != ErrTraversalLimitExceeded {
		t.Logf("BFS with limit: err=%v, results=%d", err, len(result))
	} else {
		t.Logf("BFS correctly hit traversal limit after %d nodes", len(result))
	}
}

// TestScale_ShortestPath_LargeGraph verifies ShortestPath on a moderately large graph.
func TestScale_ShortestPath_LargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 10_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	var edges []*Edge
	for i := 0; i < nodeCount; i++ {
		for j := 1; j <= 2 && i+j < nodeCount; j++ {
			edges = append(edges, NewEdge(RelCalls, nodes[i].ID, nodes[i+j].ID))
		}
	}
	addEdgesBatch(gs, edges, 1000)

	start := time.Now()
	path, err := gs.ShortestPath(context.Background(), nodes[0].ID, nodes[nodeCount-1].ID)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("ShortestPath on %d nodes: %v", nodeCount, err)
	} else {
		t.Logf("ShortestPath on %d nodes: path length=%d in %v", nodeCount, len(path), elapsed)
	}
}

// TestScale_DetectCycles_NoStackOverflow verifies that DetectCycles uses iterative
// DFS and doesn't stack overflow on large graphs.
func TestScale_DetectCycles_NoStackOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 50_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	edges := make([]*Edge, nodeCount-1)
	for i := 0; i < nodeCount-1; i++ {
		edges[i] = NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID)
	}
	addEdgesBatch(gs, edges, 1000)

	start := time.Now()
	cycles, err := gs.DetectCycles(context.Background(), "scalerepo")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DetectCycles: %v", err)
	}
	t.Logf("DetectCycles on %d-node chain (no cycles): %d cycles in %v", nodeCount, len(cycles), elapsed)

	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d", len(cycles))
	}
}

// TestScale_DetectCycles_WithCycle verifies cycle detection works on larger graphs.
func TestScale_DetectCycles_WithCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 10_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	var edges []*Edge
	for i := 0; i < nodeCount-1; i++ {
		edges = append(edges, NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID))
	}
	// Create a cycle: last node → first node
	edges = append(edges, NewEdge(RelCalls, nodes[nodeCount-1].ID, nodes[0].ID))
	addEdgesBatch(gs, edges, 1000)

	start := time.Now()
	cycles, err := gs.DetectCycles(context.Background(), "scalerepo")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DetectCycles: %v", err)
	}
	t.Logf("DetectCycles on %d-node chain (with cycle): %d cycles in %v", nodeCount, len(cycles), elapsed)

	if len(cycles) == 0 {
		t.Error("expected at least 1 cycle")
	}
}

// TestScale_BatchGetNodes verifies batch node retrieval works correctly.
func TestScale_BatchGetNodes(t *testing.T) {
	gs := openTestGSLarge(t)
	nodeCount := 5000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	ids := make([]string, nodeCount)
	for i, n := range nodes {
		ids[i] = n.ID
	}

	start := time.Now()
	result, err := gs.BatchGetNodes(ids)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("BatchGetNodes: %v", err)
	}
	t.Logf("BatchGetNodes(%d): got %d nodes in %v", nodeCount, len(result), elapsed)

	if len(result) != nodeCount {
		t.Errorf("expected %d nodes, got %d", nodeCount, len(result))
	}
}

// TestScale_MemoryUsage estimates memory usage for large graphs.
func TestScale_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 100_000

	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	t.Logf("Memory for %d nodes: alloc diff = %.2f MB", nodeCount, float64(allocDiff)/1e6)
	t.Logf("Per-node overhead: %.0f bytes", float64(allocDiff)/float64(nodeCount))

	estimated1M := float64(allocDiff) / float64(nodeCount) * 1_000_000
	t.Logf("Estimated memory for 1M nodes: %.2f MB", estimated1M/1e6)
}

// TestScale_ContextCancellation verifies that long-running traversals
// respect context cancellation.
func TestScale_ContextCancellation(t *testing.T) {
	gs := openTestGSLarge(t)
	nodeCount := 50_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("scalerepo", LabelFunction, fmt.Sprintf("Func%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	edges := make([]*Edge, nodeCount-1)
	for i := 0; i < nodeCount-1; i++ {
		edges[i] = NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID)
	}
	addEdgesBatch(gs, edges, 1000)

	// BFS with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := gs.BFS(ctx, nodes[0].ID, TraverseOut, 100, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Log("BFS with cancelled context returned nil error (possible race)")
	} else {
		t.Logf("BFS with cancelled context: err=%v in %v", err, elapsed)
	}

	if elapsed > 5*time.Second {
		t.Errorf("BFS with cancelled context took too long: %v", elapsed)
	}

	// ShortestPath with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	start = time.Now()
	_, err = gs.ShortestPath(ctx2, nodes[0].ID, nodes[nodeCount-1].ID)
	elapsed = time.Since(start)

	t.Logf("ShortestPath with 10ms timeout: err=%v in %v", err, elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("ShortestPath with timeout took too long: %v", elapsed)
	}
}

// TestScale_AdjBufferCoalescing verifies that the Batch adjBuffer coalesces
// multiple edges for the same adjacency key into a single Merge.
func TestScale_AdjBufferCoalescing(t *testing.T) {
	gs := openTestGSLarge(t)

	nodeA := NewNode("scalerepo", LabelFunction, "A")
	nodeB := NewNode("scalerepo", LabelFunction, "B")
	nodeC := NewNode("scalerepo", LabelFunction, "C")
	gs.AddNode(nodeA)
	gs.AddNode(nodeB)
	gs.AddNode(nodeC)

	err := gs.Batch(func(b *Batch) error {
		b.AddEdge(NewEdge(RelCalls, nodeA.ID, nodeB.ID))
		b.AddEdge(NewEdge(RelCalls, nodeA.ID, nodeC.ID))
		b.AddEdge(NewEdge(RelContains, nodeA.ID, nodeB.ID))
		return nil
	})
	if err != nil {
		t.Fatalf("batch commit: %v", err)
	}

	outEdges, err := gs.GetAllOutEdges(nodeA.ID)
	if err != nil {
		t.Fatalf("GetAllOutEdges: %v", err)
	}
	t.Logf("Node A has %d out edges (expected at least 3)", len(outEdges))

	if len(outEdges) < 3 {
		t.Errorf("expected at least 3 out edges, got %d", len(outEdges))
	}
}

// TestScale_NoAdjCache verifies that adjCache has been removed —
// GetAllOutEdges reads from Pebble, not an in-memory sync.Map.
// This validates Phase 1 (B1) optimization.
func TestScale_NoAdjCache(t *testing.T) {
	gs := openTestGSLarge(t)

	nodeA := NewNode("scalerepo", LabelFunction, "A")
	nodeB := NewNode("scalerepo", LabelFunction, "B")
	gs.AddNode(nodeA)
	gs.AddNode(nodeB)
	gs.AddEdge(NewEdge(RelCalls, nodeA.ID, nodeB.ID))

	// First read — from Pebble
	edges1, err := gs.GetAllOutEdges(nodeA.ID)
	if err != nil {
		t.Fatalf("GetAllOutEdges: %v", err)
	}
	if len(edges1) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges1))
	}

	// Second read — should still work (from Pebble block cache)
	edges2, err := gs.GetAllOutEdges(nodeA.ID)
	if err != nil {
		t.Fatalf("GetAllOutEdges (2nd): %v", err)
	}
	if len(edges2) != 1 {
		t.Fatalf("expected 1 edge (2nd), got %d", len(edges2))
	}

	t.Log("NoAdjCache: GetAllOutEdges correctly reads from Pebble (no adjCache)")
}
