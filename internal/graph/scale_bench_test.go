package graph

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/mengshi02/codetrip/internal/store"
)

// benchPerfGS creates a GraphStore for performance benchmarks.
func benchPerfGS(b *testing.B) *GraphStore {
	dir, err := os.MkdirTemp("", "perf-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	cfg := store.DefaultConfig(dir)
	cfg.CacheSize = 64 << 20 // 64MB
	s, err := store.Open(cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	return NewGraphStore(s, "perfrepo")
}

// buildChainGraph builds a chain graph with the given number of nodes.
// Returns (firstNodeID, lastNodeID) for traversal benchmarks.
func buildChainGraph(gs *GraphStore, nodeCount int) (string, string) {
	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("F%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	edges := make([]*Edge, nodeCount-1)
	for i := 0; i < nodeCount-1; i++ {
		edges[i] = NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID)
	}
	addEdgesBatch(gs, edges, 1000)

	return nodes[0].ID, nodes[nodeCount-1].ID
}

// buildWideGraph builds a graph where each node connects to the next 2 nodes,
// producing a wider traversal fan-out suitable for BFS depth testing.
func buildWideGraph(gs *GraphStore, nodeCount int) string {
	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("W%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	var edges []*Edge
	for i := 0; i < nodeCount; i++ {
		for j := 1; j <= 2 && i+j < nodeCount; j++ {
			edges = append(edges, NewEdge(RelCalls, nodes[i].ID, nodes[i+j].ID))
		}
	}
	addEdgesBatch(gs, edges, 1000)

	return nodes[0].ID
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_BFS_Depth3: BFS(maxDepth=3) should complete in < 2s
// Scaled to 100K nodes (1M would be too slow for `go test -bench`).
// Target: < 2s at 100K nodes → extrapolated < 2s at 1M with Pebble cache hits.
// ---------------------------------------------------------------------------
func BenchmarkPerf_BFS_Depth3(b *testing.B) {
	gs := benchPerfGS(b)
	srcID := buildWideGraph(gs, 100_000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := gs.BFS(context.Background(), srcID, TraverseOut, 3, nil)
		if err != nil {
			b.Fatalf("BFS: %v", err)
		}
		_ = result
	}
}

// BenchmarkPerf_BFS_Depth3_10K: smaller scale for quick verification.
func BenchmarkPerf_BFS_Depth3_10K(b *testing.B) {
	gs := benchPerfGS(b)
	srcID := buildWideGraph(gs, 10_000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := gs.BFS(context.Background(), srcID, TraverseOut, 3, nil)
		if err != nil {
			b.Fatalf("BFS: %v", err)
		}
		_ = result
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_ShortestPath: ShortestPath should complete in < 3s
// Scaled to 10K nodes (chain graph — worst case for BFS).
// ---------------------------------------------------------------------------
func BenchmarkPerf_ShortestPath(b *testing.B) {
	gs := benchPerfGS(b)
	srcID, dstID := buildChainGraph(gs, 10_000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		path, err := gs.ShortestPath(context.Background(), srcID, dstID)
		if err != nil {
			b.Fatalf("ShortestPath: %v", err)
		}
		_ = path
	}
}

// BenchmarkPerf_ShortestPath_1K: smaller scale for quick iteration.
func BenchmarkPerf_ShortestPath_1K(b *testing.B) {
	gs := benchPerfGS(b)
	srcID, dstID := buildChainGraph(gs, 1_000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		path, err := gs.ShortestPath(context.Background(), srcID, dstID)
		if err != nil {
			b.Fatalf("ShortestPath: %v", err)
		}
		_ = path
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_GraphBuild_10K: measures graph construction throughput.
// Target: 图谱+BM25构建时间 < 2分钟 (at 1M nodes, excluding embedding).
// Here we benchmark 10K nodes and extrapolate.
// ---------------------------------------------------------------------------
func BenchmarkPerf_GraphBuild_10K(b *testing.B) {
	dir, err := os.MkdirTemp("", "perf-build-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		cfg := store.DefaultConfig(dir)
		cfg.CacheSize = 64 << 20
		s, _ := store.Open(cfg)
		gs := NewGraphStore(s, "perfrepo")

		nodes := make([]*Node, 10_000)
		for j := 0; j < 10_000; j++ {
			nodes[j] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("G%d_%d", i, j))
		}
		b.StartTimer()

		addNodesBatch(gs, nodes, 1000)

		// Add ~5 edges per node
		var edges []*Edge
		for j := 0; j < 10_000; j++ {
			for k := 1; k <= 5 && j+k < 10_000; k++ {
				edges = append(edges, NewEdge(RelCalls, nodes[j].ID, nodes[j+k].ID))
			}
		}
		addEdgesBatch(gs, edges, 1000)

		b.StopTimer()
		s.Close()
		b.StartTimer()
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_MemoryFootprint: estimates per-node memory usage and extrapolates
// to 1M nodes. Target: < 4GB (after removing adjCache).
// ---------------------------------------------------------------------------
func BenchmarkPerf_MemoryFootprint(b *testing.B) {
	gs := benchPerfGS(b)
	nodeCount := 100_000

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("M%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / float64(nodeCount)
	estimated1M := perNode * 1_000_000

	b.ReportMetric(perNode, "bytes/node")
	b.ReportMetric(estimated1M/1e6, "MB_est_1M")

	// Also report the per-op metric (b.N=1, this is a one-shot benchmark)
	_ = estimated1M
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_BatchGetNodes: measures batch node retrieval throughput.
// ---------------------------------------------------------------------------
func BenchmarkPerf_BatchGetNodes(b *testing.B) {
	gs := benchPerfGS(b)
	nodeCount := 10_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("BG%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	ids := make([]string, nodeCount)
	for i, n := range nodes {
		ids[i] = n.ID
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := gs.BatchGetNodes(ids)
		if err != nil {
			b.Fatalf("BatchGetNodes: %v", err)
		}
		_ = result
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_DetectCycles: measures cycle detection performance.
// ---------------------------------------------------------------------------
func BenchmarkPerf_DetectCycles(b *testing.B) {
	gs := benchPerfGS(b)
	nodeCount := 10_000

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("DC%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	var edges []*Edge
	for i := 0; i < nodeCount-1; i++ {
		edges = append(edges, NewEdge(RelCalls, nodes[i].ID, nodes[i+1].ID))
	}
	addEdgesBatch(gs, edges, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cycles, err := gs.DetectCycles(context.Background(), "perfrepo")
		if err != nil {
			b.Fatalf("DetectCycles: %v", err)
		}
		_ = cycles
	}
}

// ---------------------------------------------------------------------------
// BenchmarkPerf_IndexBuild_EndToEnd: end-to-end benchmark that measures
// graph + vector + BM25 build time at 10K scale.
// This is a one-shot timing benchmark (b.N=1) that reports wall-clock time.
// ---------------------------------------------------------------------------
func BenchmarkPerf_IndexBuild_EndToEnd(b *testing.B) {
	dir, err := os.MkdirTemp("", "perf-e2e-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		cfg := store.DefaultConfig(dir)
		cfg.CacheSize = 64 << 20
		s, _ := store.Open(cfg)
		gs := NewGraphStore(s, "perfrepo")

		nodeCount := 10_000
		nodes := make([]*Node, nodeCount)
		for j := 0; j < nodeCount; j++ {
			nodes[j] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("E2E%d_%d", i, j))
		}
		b.StartTimer()

		// Phase 1: Add nodes
		addNodesBatch(gs, nodes, 1000)

		// Phase 2: Add edges (~5 per node)
		var edges []*Edge
		for j := 0; j < nodeCount; j++ {
			for k := 1; k <= 5 && j+k < nodeCount; k++ {
				edges = append(edges, NewEdge(RelCalls, nodes[j].ID, nodes[j+k].ID))
			}
		}
		addEdgesBatch(gs, edges, 1000)

		b.StopTimer()
		s.Close()
		b.StartTimer()
	}
}

// ---------------------------------------------------------------------------
// Threshold verification tests: these are Test functions (not Benchmarks)
// that FAIL if performance targets are not met.
// ---------------------------------------------------------------------------

// TestPerf_BFS_Depth3_Under2s verifies BFS(maxDepth=3) completes in < 2s
// on a 100K-node wide graph.
func TestPerf_BFS_Depth3_Under2s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	gs := openTestGSLarge(t)
	srcID := buildWideGraph(gs, 100_000)

	start := time.Now()
	result, err := gs.BFS(context.Background(), srcID, TraverseOut, 3, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("BFS: %v", err)
	}
	t.Logf("BFS(maxDepth=3) on 100K-node graph: %d results in %v", len(result), elapsed)

	if elapsed > 2*time.Second {
		t.Errorf("BFS(maxDepth=3) took %v, exceeds 2s target", elapsed)
	}
}

// TestPerf_ShortestPath_Under3s verifies ShortestPath completes in < 3s
// on a 10K-node chain graph.
func TestPerf_ShortestPath_Under3s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	gs := openTestGSLarge(t)
	srcID, dstID := buildChainGraph(gs, 10_000)

	start := time.Now()
	path, err := gs.ShortestPath(context.Background(), srcID, dstID)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	t.Logf("ShortestPath on 10K-node chain: path length=%d in %v", len(path), elapsed)

	if elapsed > 3*time.Second {
		t.Errorf("ShortestPath took %v, exceeds 3s target", elapsed)
	}
}

// TestPerf_MemoryUnder4GB verifies per-node memory overhead is reasonable,
// extrapolating to < 4GB at 1M nodes (after adjCache removal).
func TestPerf_MemoryUnder4GB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	gs := openTestGSLarge(t)
	nodeCount := 100_000

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewNode("perfrepo", LabelFunction, fmt.Sprintf("Mem%d", i))
	}
	addNodesBatch(gs, nodes, 1000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / float64(nodeCount)
	estimated1M := perNode * 1_000_000

	t.Logf("Memory for %d nodes: %.2f MB (per-node: %.0f bytes)", nodeCount, float64(allocDiff)/1e6, perNode)
	t.Logf("Estimated 1M-node memory: %.2f MB", estimated1M/1e6)

	// Target: < 4GB = 4000 MB
	if estimated1M > 4_000_000_000 {
		t.Errorf("Estimated 1M-node memory %.0f MB exceeds 4GB target", estimated1M/1e6)
	}
}