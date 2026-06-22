package search

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/coder/hnsw"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/util"
	"github.com/mengshi02/codetrip/internal/vecfile"
)

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// perfEmbedder is a deterministic mock embedder for performance testing.
type perfEmbedder struct {
	dim int
}

func (e *perfEmbedder) Dimensions() int { return e.dim }
func (e *perfEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	r := rand.New(rand.NewSource(42))
	result := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, e.dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		result[i] = vec
	}
	return result, nil
}

// openPerfVectorSearch creates a VectorSearch instance for performance benchmarks.
func openPerfVectorSearch(b *testing.B) (*VectorSearch, *graph.GraphStore, *store.Store, string, func()) {
	b.Helper()
	dir, err := os.MkdirTemp("", "perf-vector-*")
	if err != nil {
		b.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = 64 << 20
	s, err := store.Open(cfg)
	if err != nil {
		b.Fatal(err)
	}
	gs := graph.NewGraphStore(s, "perfrepo")
	embedder := &perfEmbedder{dim: 384}
	vs := NewVectorSearchWithDir(embedder, s, gs, dir)
	cleanup := func() {
		vs.Close()
		s.Close()
		os.RemoveAll(dir)
	}
	return vs, gs, s, dir, cleanup
}

// buildHNSWIndex builds an HNSW index with the given number of nodes.
// Each node gets a random 384-dimensional vector stored in Pebble.
func buildHNSWIndex(b *testing.B, vs *VectorSearch, gs *graph.GraphStore, s *store.Store, nodeCount int) {
	b.Helper()
	r := rand.New(rand.NewSource(42))
	dim := 384
	repo := gs.Repo()

	// Store dual-modal vectors and build embedding index
	var nodeIDs []string
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("vecnode%d", i)
		nodeIDs = append(nodeIDs, nodeID)

		// Store vector in Pebble
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		vecData := util.EncodeFloat32Vec(vec)
		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), vecData); err != nil {
			b.Fatalf("store desc vector: %v", err)
		}
		if err := s.Set([]byte(codeKey), vecData); err != nil {
			b.Fatalf("store code vector: %v", err)
		}
	}

	// Store dual-modal embedding indices
	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	idxData := util.EncodeStringList(nodeIDs)
	if err := s.Set([]byte(descIdxKey), idxData); err != nil {
		b.Fatalf("store desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), idxData); err != nil {
		b.Fatalf("store code index: %v", err)
	}

	// Build dual-modal HNSW index
	if err := vs.BuildDualModalHNSWIndex(); err != nil {
		b.Fatalf("BuildDualModalHNSWIndex: %v", err)
	}
}

// buildHNSWIndexQuantized builds an HNSW index with int8 quantization.
func buildHNSWIndexQuantized(b *testing.B, vs *VectorSearch, gs *graph.GraphStore, s *store.Store, dataDir string, nodeCount int) {
	b.Helper()
	r := rand.New(rand.NewSource(42))
	dim := 384
	repo := gs.Repo()

	// Generate vectors and compute quantization parameters
	vecs := make([][]float32, nodeCount)
	for i := 0; i < nodeCount; i++ {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		vecs[i] = vec
	}

	// Compute scale and offset for quantization
	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for j := 0; j < dim; j++ {
		minVal, maxVal := float32(1e9), float32(-1e9)
		for i := 0; i < nodeCount; i++ {
			if vecs[i][j] < minVal {
				minVal = vecs[i][j]
			}
			if vecs[i][j] > maxVal {
				maxVal = vecs[i][j]
			}
		}
		offset[j] = minVal
		if maxVal > minVal {
			scale[j] = (maxVal - minVal) / 254.0
		}
	}

	// Store dual-modal vectors in Pebble
	var nodeIDs []string
	qp := hnsw.QuantParams{Scale: scale, Offset: offset}
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("qvecnode%d", i)
		nodeIDs = append(nodeIDs, nodeID)

		vecData := util.EncodeFloat32Vec(vecs[i])
		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), vecData); err != nil {
			b.Fatalf("store desc vector: %v", err)
		}
		if err := s.Set([]byte(codeKey), vecData); err != nil {
			b.Fatalf("store code vector: %v", err)
		}
	}

	// Store dual-modal embedding indices
	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	idxData := util.EncodeStringList(nodeIDs)
	if err := s.Set([]byte(descIdxKey), idxData); err != nil {
		b.Fatalf("store desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), idxData); err != nil {
		b.Fatalf("store code index: %v", err)
	}

	// Write quantized vector file to the dataDir
	vfWriter := vecfile.NewVectorFileWriter(dim, scale, offset)
	for i := 0; i < nodeCount; i++ {
		qvec := hnsw.Quantize(vecs[i], qp)
		vfWriter.AddNodeVector(qvec)
	}
	vecFilePath := vecfile.VectorFilePath(dataDir, repo)
	if err := vfWriter.Write(vecFilePath); err != nil {
		b.Fatalf("write vector file: %v", err)
	}

	// Load vector file and rebuild dual-modal HNSW index
	if err := vs.LoadVectorFile(); err != nil {
		b.Fatalf("LoadVectorFile: %v", err)
	}
	if err := vs.BuildDualModalHNSWIndex(); err != nil {
		b.Fatalf("BuildDualModalHNSWIndex: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HNSW Search Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkPerf_HNSW_Float32Search: measures HNSW search with float32 vectors.
// Target: 语义搜索 < 100ms
func BenchmarkPerf_HNSW_Float32Search(b *testing.B) {
	vs, gs, s, _, cleanup := openPerfVectorSearch(b)
	defer cleanup()

	buildHNSWIndex(b, vs, gs, s, 10_000)

	// Generate a query vector
	queryVec := make([]float32, 384)
	r := rand.New(rand.NewSource(99))
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

// BenchmarkPerf_HNSW_Float32Search_1K: smaller index for quick verification.
func BenchmarkPerf_HNSW_Float32Search_1K(b *testing.B) {
	vs, gs, s, _, cleanup := openPerfVectorSearch(b)
	defer cleanup()

	buildHNSWIndex(b, vs, gs, s, 1_000)

	queryVec := make([]float32, 384)
	r := rand.New(rand.NewSource(99))
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

// BenchmarkPerf_HNSW_Int8Search: measures HNSW search with int8 quantized vectors.
// Target: 语义搜索（int8量化） < 100ms
func BenchmarkPerf_HNSW_Int8Search(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openPerfVectorSearch(b)
	defer cleanup()

	buildHNSWIndexQuantized(b, vs, gs, s, dataDir, 10_000)

	queryVec := make([]float32, 384)
	r := rand.New(rand.NewSource(99))
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

// BenchmarkPerf_HNSW_TwoStageSearch: measures two-stage search (int8 coarse + float32 refine).
// Target: 语义搜索（两阶段精排） < 200ms
func BenchmarkPerf_HNSW_TwoStageSearch(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openPerfVectorSearch(b)
	defer cleanup()

	buildHNSWIndexQuantized(b, vs, gs, s, dataDir, 10_000)
	vs.SetTwoStageSearch(true)

	queryVec := make([]float32, 384)
	r := rand.New(rand.NewSource(99))
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

// ---------------------------------------------------------------------------
// HNSW Build Benchmark
// ---------------------------------------------------------------------------

// BenchmarkPerf_BuildDualModalHNSWIndex: measures dual-modal HNSW index build time.
// Target: BuildDualModalHNSWIndex构建时间 < 15分钟
func BenchmarkPerf_BuildDualModalHNSWIndex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vs, gs, s, _, cleanup := openPerfVectorSearch(b)

		// Pre-store 10K dual-modal vectors
		r := rand.New(rand.NewSource(42))
		dim := 384
		repo := gs.Repo()
		var nodeIDs []string
		for j := 0; j < 10_000; j++ {
			nodeID := fmt.Sprintf("buildnode%d", j)
			nodeIDs = append(nodeIDs, nodeID)
			vec := make([]float32, dim)
			for k := range vec {
				vec[k] = r.Float32()
			}
			vecData := util.EncodeFloat32Vec(vec)
			descKey := graph.EmbDescKey(repo, nodeID)
			codeKey := graph.EmbCodeKey(repo, nodeID)
			s.Set([]byte(descKey), vecData)
			s.Set([]byte(codeKey), vecData)
		}
		descIdxKey := graph.EmbDescIdxKey(repo)
		codeIdxKey := graph.EmbCodeIdxKey(repo)
		idxData := util.EncodeStringList(nodeIDs)
		s.Set([]byte(descIdxKey), idxData)
		s.Set([]byte(codeIdxKey), idxData)
		b.StartTimer()

		if err := vs.BuildDualModalHNSWIndex(); err != nil {
			b.Fatalf("BuildDualModalHNSWIndex: %v", err)
		}

		b.StopTimer()
		cleanup()
		b.StartTimer()
	}
}

// ---------------------------------------------------------------------------
// HNSW Memory Footprint Benchmark
// ---------------------------------------------------------------------------

// BenchmarkPerf_HNSW_MemoryFootprint: measures HNSW index memory usage.
// Target: HNSW向量内存（QVec+邻接图） < 800MB
func BenchmarkPerf_HNSW_MemoryFootprint(b *testing.B) {
	vs, gs, s, _, cleanup := openPerfVectorSearch(b)
	defer cleanup()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	buildHNSWIndex(b, vs, gs, s, 10_000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / 10_000
	estimated1M := perNode * 1_000_000

	b.ReportMetric(perNode, "bytes/node_hnsw")
	b.ReportMetric(estimated1M/1e6, "MB_hnsw_est_1M")
}

// ---------------------------------------------------------------------------
// BM25 Search Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkPerf_BM25Search_Large: measures BM25 search on a larger index.
// Target: BM25搜索 < 1s
func BenchmarkPerf_BM25Search_Large(b *testing.B) {
	dir, err := os.MkdirTemp("", "perf-bm25-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = 64 << 20
	s, err := store.Open(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	idx, err := NewBM25IndexWithDir(dir, "perfrepo", s)
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()

	// Index 10K nodes
	nodes := make([]*graph.Node, 10_000)
	for i := 0; i < 10_000; i++ {
		n := graph.NewNode("perfrepo", graph.LabelFunction, fmt.Sprintf("handleRequest%d", i))
		n.FilePath = fmt.Sprintf("pkg/handler%d.go", i/100)
		nodes[i] = n
	}
	if err := idx.BatchIndex(nodes); err != nil {
		b.Fatalf("BatchIndex: %v", err)
	}

	if err := idx.FinalizeBuild(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := idx.Search("handleRequest", 10)
		if err != nil {
			b.Fatalf("BM25 Search: %v", err)
		}
		_ = results
	}
}

// BenchmarkPerf_BM25BatchIndex: measures BM25 batch index throughput.
// Target: 图谱+BM25构建时间 < 2分钟 (at 1M nodes).
func BenchmarkPerf_BM25BatchIndex_10K(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir, _ := os.MkdirTemp("", "perf-bm25idx-*")
		cfg := store.DefaultConfig(filepath.Join(dir, "db"))
		cfg.CacheSize = 64 << 20
		s, _ := store.Open(cfg)
		idx, _ := NewBM25IndexWithDir(dir, "perfrepo", s)

		nodes := make([]*graph.Node, 10_000)
		for j := 0; j < 10_000; j++ {
			n := graph.NewNode("perfrepo", graph.LabelFunction, fmt.Sprintf("idxFunc%d_%d", i, j))
			n.FilePath = fmt.Sprintf("pkg/file%d.go", j/100)
			nodes[j] = n
		}
		b.StartTimer()

		if err := idx.BatchIndex(nodes); err != nil {
			b.Fatalf("BatchIndex: %v", err)
		}

		b.StopTimer()
		idx.Close()
		s.Close()
		os.RemoveAll(dir)
		b.StartTimer()
	}
}

// ---------------------------------------------------------------------------
// Int8 Quantization Overhead Benchmark
// ---------------------------------------------------------------------------

// BenchmarkPerf_Int8Quantize: measures int8 quantization throughput.
func BenchmarkPerf_Int8Quantize(b *testing.B) {
	dim := 384
	r := rand.New(rand.NewSource(42))
	vec := make([]float32, dim)
	for j := range vec {
		vec[j] = r.Float32()
	}

	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for j := 0; j < dim; j++ {
		scale[j] = 0.01
		offset[j] = -1.0
	}
	qp := hnsw.QuantParams{Scale: scale, Offset: offset}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hnsw.Quantize(vec, qp)
	}
}

// BenchmarkPerf_Int8CosineDistance: measures int8 cosine distance throughput.
func BenchmarkPerf_Int8CosineDistance(b *testing.B) {
	dim := 384
	r := rand.New(rand.NewSource(42))
	qvec := make([]byte, dim)
	vvec := make([]byte, dim)
	for j := 0; j < dim; j++ {
		qvec[j] = byte(r.Intn(256))
		vvec[j] = byte(r.Intn(256))
	}

	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for j := 0; j < dim; j++ {
		scale[j] = 0.01
		offset[j] = -1.0
	}
	qp := hnsw.QuantParams{Scale: scale, Offset: offset}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hnsw.Int8CosineDistance(qvec, vvec, qp)
	}
}

// ---------------------------------------------------------------------------
// Direct HNSW Package Benchmarks (no VectorSearch wrapper)
// These measure the raw HNSW performance at scale.
// ---------------------------------------------------------------------------

// BenchmarkPerf_HNSW_Direct_10K: direct HNSW benchmark with 10K 384-dim vectors.
func BenchmarkPerf_HNSW_Direct_10K(b *testing.B) {
	g := hnsw.NewGraph[string]()
	g.M = 16
	g.Ml = 0.25
	g.EfSearch = 20
	g.Distance = hnsw.CosineDistance

	r := rand.New(rand.NewSource(42))
	dim := 384
	nodes := make([]hnsw.Node[string], 10_000)
	for i := 0; i < 10_000; i++ {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		nodes[i] = hnsw.MakeNode(fmt.Sprintf("n%d", i), vec)
		if err := g.Add(nodes[i]); err != nil {
			b.Fatalf("add: %v", err)
		}
	}

	queryVec := make([]float32, dim)
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Search(queryVec, 10)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
	}
}

// BenchmarkPerf_HNSW_Direct_Int8_10K: direct HNSW benchmark with int8 quantized vectors.
func BenchmarkPerf_HNSW_Direct_Int8_10K(b *testing.B) {
	g := hnsw.NewGraph[string]()
	g.M = 16
	g.Ml = 0.25
	g.EfSearch = 20
	g.QuantType = hnsw.QuantInt8

	r := rand.New(rand.NewSource(42))
	dim := 384

	// Generate vectors and compute quantization params
	vecs := make([][]float32, 10_000)
	for i := 0; i < 10_000; i++ {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		vecs[i] = vec
	}

	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for j := 0; j < dim; j++ {
		minVal, maxVal := float32(1e9), float32(-1e9)
		for i := 0; i < 10_000; i++ {
			if vecs[i][j] < minVal {
				minVal = vecs[i][j]
			}
			if vecs[i][j] > maxVal {
				maxVal = vecs[i][j]
			}
		}
		offset[j] = minVal
		if maxVal > minVal {
			scale[j] = (maxVal - minVal) / 254.0
		}
	}

	qp := hnsw.QuantParams{Scale: scale, Offset: offset}
	g.QuantParams = qp

	for i := 0; i < 10_000; i++ {
		node := hnsw.MakeNode(fmt.Sprintf("qn%d", i), vecs[i])
		if err := g.Add(node); err != nil {
			b.Fatalf("add: %v", err)
		}
	}

	queryVec := make([]float32, dim)
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Search(queryVec, 10)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Threshold verification tests for search
// ---------------------------------------------------------------------------

// TestPerf_HNSW_Search_Under100ms verifies HNSW search completes in < 100ms.
func TestPerf_HNSW_Search_Under100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	dir, _ := os.MkdirTemp("", "perf-hnsw-*")
	defer os.RemoveAll(dir)

	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = 64 << 20
	s, _ := store.Open(cfg)
	defer s.Close()

	gs := graph.NewGraphStore(s, "perfrepo")
	embedder := &perfEmbedder{dim: 384}
	vs := NewVectorSearchWithDir(embedder, s, gs, dir)
	defer vs.Close()

	// Build dual-modal index with 10K vectors
	r := rand.New(rand.NewSource(42))
	dim := 384
	repo := gs.Repo()
	var nodeIDs []string
	for i := 0; i < 10_000; i++ {
		nodeID := fmt.Sprintf("perfnode%d", i)
		nodeIDs = append(nodeIDs, nodeID)
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = r.Float32()
		}
		vecData := util.EncodeFloat32Vec(vec)
		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		s.Set([]byte(descKey), vecData)
		s.Set([]byte(codeKey), vecData)
	}
	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	idxData := util.EncodeStringList(nodeIDs)
	s.Set([]byte(descIdxKey), idxData)
	s.Set([]byte(codeIdxKey), idxData)

	if err := vs.BuildDualModalHNSWIndex(); err != nil {
		t.Fatalf("BuildDualModalHNSWIndex: %v", err)
	}

	// Search benchmark
	queryVec := make([]float32, dim)
	for j := range queryVec {
		queryVec[j] = r.Float32()
	}

	start := time.Now()
	for i := 0; i < 100; i++ {
		_, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			t.Fatalf("searchDualHNSW: %v", err)
		}
	}
	elapsed := time.Since(start)
	avgMs := elapsed.Milliseconds() / 100

	t.Logf("HNSW search avg: %d ms (100 queries on 10K index)", avgMs)
	if avgMs > 100 {
		t.Errorf("HNSW search avg %d ms exceeds 100ms target", avgMs)
	}
}

// TestPerf_BM25_Search_Under1s verifies BM25 search completes in < 1s.
func TestPerf_BM25_Search_Under1s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	dir, _ := os.MkdirTemp("", "perf-bm25-*")
	defer os.RemoveAll(dir)

	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = 64 << 20
	s, _ := store.Open(cfg)
	defer s.Close()

	idx, err := NewBM25IndexWithDir(dir, "perfrepo", s)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// Index 10K nodes
	nodes := make([]*graph.Node, 10_000)
	for i := 0; i < 10_000; i++ {
		n := graph.NewNode("perfrepo", graph.LabelFunction, fmt.Sprintf("processRequest%d", i))
		n.FilePath = fmt.Sprintf("pkg/service%d.go", i/100)
		nodes[i] = n
	}
	if err := idx.BatchIndex(nodes); err != nil {
		t.Fatalf("BatchIndex: %v", err)
	}

	if err := idx.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	for i := 0; i < 100; i++ {
		_, err := idx.Search("processRequest", 10)
		if err != nil {
			t.Fatalf("BM25 Search: %v", err)
		}
	}
	elapsed := time.Since(start)
	avgMs := elapsed.Milliseconds() / 100

	t.Logf("BM25 search avg: %d ms (100 queries on 10K index)", avgMs)
	if elapsed > 1*time.Second {
		t.Errorf("BM25 search took %v, exceeds 1s target", elapsed)
	}
}

// TestPerf_RecallAt10 verifies that int8 quantized HNSW recall@10 is within 10%
// of float32 HNSW recall (measured against brute force ground truth).
// Uses structured vectors in low dimension for meaningful recall measurement.
func TestPerf_RecallAt10(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in short mode")
	}

	rng := rand.New(rand.NewSource(42))
	dim := 8  // low dimension for meaningful HNSW recall
	n := 2000 // node count
	k := 10   // top-k

	// Generate structured vectors: each has a strong signal in one dimension
	vectors := make([][]float32, n)
	for i := range vectors {
		vec := make([]float32, dim)
		vec[i%dim] = float32(i) / float32(n) * 2.0
		for j := range vec {
			if j != i%dim {
				vec[j] = (rng.Float32() - 0.5) * 0.1
			}
		}
		vectors[i] = vec
	}

	params := hnsw.TrainQuantParams(vectors)

	// Brute force ground truth
	bruteForceTopK := func(queryVec []float32, queryIdx int) map[int]bool {
		type distEntry struct {
			idx  int
			dist float32
		}
		entries := make([]distEntry, n)
		for i, vec := range vectors {
			entries[i] = distEntry{idx: i, dist: hnsw.CosineDistance(queryVec, vec)}
		}
		topK := make(map[int]bool)
		for i := 0; i < k+1 && i < n; i++ {
			minIdx := i
			for j := i + 1; j < n; j++ {
				if entries[j].dist < entries[minIdx].dist {
					minIdx = j
				}
			}
			entries[i], entries[minIdx] = entries[minIdx], entries[i]
			if entries[i].idx != queryIdx {
				topK[entries[i].idx] = true
			}
		}
		return topK
	}

	computeRecall := func(g *hnsw.Graph[int]) float64 {
		totalRecall := 0.0
		queries := 100
		for q := 0; q < queries; q++ {
			queryIdx := rng.Intn(n)
			queryVec := vectors[queryIdx]
			topK := bruteForceTopK(queryVec, queryIdx)
			results, err := g.Search(queryVec, k)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			hits := 0
			for _, r := range results {
				if topK[r.Key] {
					hits++
				}
			}
			if len(topK) > 0 {
				totalRecall += float64(hits) / float64(len(topK))
			}
		}
		return totalRecall / float64(queries)
	}

	// Float32 HNSW baseline
	gFloat := hnsw.NewGraph[int]()
	gFloat.M = 16
	gFloat.Ml = 0.5
	gFloat.EfSearch = 200
	gFloat.Distance = hnsw.CosineDistance
	for i, vec := range vectors {
		if err := gFloat.Add(hnsw.MakeNode(i, vec)); err != nil {
			t.Fatalf("add float: %v", err)
		}
	}
	float32Recall := computeRecall(gFloat)

	// Int8 quantized HNSW
	gInt8 := hnsw.NewGraph[int]()
	gInt8.M = 16
	gInt8.Ml = 0.5
	gInt8.EfSearch = 200
	gInt8.Distance = hnsw.CosineDistance
	gInt8.QuantType = hnsw.QuantInt8
	gInt8.QuantParams = params
	for i, vec := range vectors {
		if err := gInt8.Add(hnsw.MakeNode(i, vec)); err != nil {
			t.Fatalf("add int8: %v", err)
		}
	}
	int8Recall := computeRecall(gInt8)

	t.Logf("Float32 recall@%d (dim=%d, n=%d): %.4f", k, dim, n, float32Recall)
	t.Logf("Int8    recall@%d (dim=%d, n=%d): %.4f", k, dim, n, int8Recall)
	t.Logf("Int8/Float32 recall ratio: %.2f%%", int8Recall/float32Recall*100)

	// Int8 recall should be within 10% of float32 recall
	if int8Recall < float32Recall*0.90 {
		t.Errorf("int8 recall %.4f is below 90%% of float32 recall %.4f", int8Recall, float32Recall)
	}
}
