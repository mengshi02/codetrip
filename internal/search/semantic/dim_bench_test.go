package semantic

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
	store "github.com/mengshi02/codetrip/internal/storage"
	"github.com/mengshi02/codetrip/internal/util"
)

// ---------------------------------------------------------------------------
// Multi-dimension vector performance benchmarks (384, 768, 1536)
// Tests search latency, build time, memory footprint, and recall
// across different embedding dimensions.
// ---------------------------------------------------------------------------

// dimEmbedder is a configurable mock embedder for dimension-specific tests.
type dimEmbedder struct {
	dim int
}

func (e *dimEmbedder) Dimensions() int { return e.dim }
func (e *dimEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
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

// openDimVectorSearch creates a VectorSearch instance for the given dimension.
func openDimVectorSearch(b *testing.B, dim int) (*VectorSearch, *graph.GraphStore, *store.Store, string, func()) {
	b.Helper()
	dir, err := os.MkdirTemp("", fmt.Sprintf("dim%d-vector-*", dim))
	if err != nil {
		b.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = 64 << 20
	s, err := store.Open(cfg)
	if err != nil {
		b.Fatal(err)
	}
	gs := graph.NewGraphStore(s, fmt.Sprintf("dim%drepo", dim))
	embedder := &dimEmbedder{dim: dim}
	vs := NewVectorSearchWithDir(embedder, s, gs, dir)
	cleanup := func() {
		vs.Close()
		s.Close()
		os.RemoveAll(dir)
	}
	return vs, gs, s, dir, cleanup
}

// generateClusteredVectors generates vectors with cluster structure.
// This avoids the "distance concentration" problem in high dimensions
// where uniform random vectors all become roughly equidistant.
// Returns vectors and quantization parameters.
func generateClusteredVectors(seed int64, dim, n, numClusters int) ([][]float32, hnsw.QuantParams) {
	r := rand.New(rand.NewSource(seed))

	// Generate cluster centers
	centers := make([][]float32, numClusters)
	for c := range centers {
		center := make([]float32, dim)
		for j := range center {
			center[j] = r.Float32()*2.0 - 1.0 // range [-1, 1]
		}
		centers[c] = center
	}

	// Generate vectors near their cluster center with controlled spread
	spread := float32(0.3) // smaller spread = tighter clusters = more meaningful distances
	vectors := make([][]float32, n)
	for i := range vectors {
		centerIdx := i % numClusters
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = centers[centerIdx][j] + (r.Float32()*2.0-1.0)*spread
		}
		vectors[i] = vec
	}

	// Compute quantization parameters
	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for j := 0; j < dim; j++ {
		minVal, maxVal := float32(1e9), float32(-1e9)
		for i := 0; i < n; i++ {
			if vectors[i][j] < minVal {
				minVal = vectors[i][j]
			}
			if vectors[i][j] > maxVal {
				maxVal = vectors[i][j]
			}
		}
		offset[j] = minVal
		if maxVal > minVal {
			scale[j] = (maxVal - minVal) / 254.0
		}
	}
	qp := hnsw.QuantParams{Scale: scale, Offset: offset}

	return vectors, qp
}

// buildDimHNSWIndex builds a float32 HNSW index for the given dimension.
func buildDimHNSWIndex(b *testing.B, vs *VectorSearch, gs *graph.GraphStore, s *store.Store, dim, nodeCount int) {
	b.Helper()
	vectors, _ := generateClusteredVectors(42, dim, nodeCount, 20)
	repo := gs.Repo()

	var nodeIDs []string
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("dim%dnode%d", dim, i)
		nodeIDs = append(nodeIDs, nodeID)

		vecData := util.EncodeFloat32Vec(vectors[i])
		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), vecData); err != nil {
			b.Fatalf("store desc vector: %v", err)
		}
		if err := s.Set([]byte(codeKey), vecData); err != nil {
			b.Fatalf("store code vector: %v", err)
		}
	}

	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	idxData := util.EncodeStringList(nodeIDs)
	if err := s.Set([]byte(descIdxKey), idxData); err != nil {
		b.Fatalf("store desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), idxData); err != nil {
		b.Fatalf("store code index: %v", err)
	}

	if err := vs.BuildSemanticIndex(); err != nil {
		b.Fatalf("BuildSemanticIndex: %v", err)
	}
}

// buildDimHNSWIndexQuantized builds an int8 quantized HNSW index for the given dimension.
func buildDimHNSWIndexQuantized(b *testing.B, vs *VectorSearch, gs *graph.GraphStore, s *store.Store, dataDir string, dim, nodeCount int) {
	b.Helper()
	vectors, qp := generateClusteredVectors(42, dim, nodeCount, 20)
	repo := gs.Repo()

	var nodeIDs []string
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("dim%dqnode%d", dim, i)
		nodeIDs = append(nodeIDs, nodeID)

		vecData := util.EncodeFloat32Vec(vectors[i])
		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), vecData); err != nil {
			b.Fatalf("store desc vector: %v", err)
		}
		if err := s.Set([]byte(codeKey), vecData); err != nil {
			b.Fatalf("store code vector: %v", err)
		}
	}

	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	idxData := util.EncodeStringList(nodeIDs)
	if err := s.Set([]byte(descIdxKey), idxData); err != nil {
		b.Fatalf("store desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), idxData); err != nil {
		b.Fatalf("store code index: %v", err)
	}

	// Write quantized vector file
	vfWriter := NewVectorFileWriter(dim, qp.Scale, qp.Offset)
	for i := 0; i < nodeCount; i++ {
		qvec := hnsw.Quantize(vectors[i], qp)
		vfWriter.AddNodeVector(qvec)
	}
	vecFilePath := VectorFilePath(dataDir, repo)
	if err := vfWriter.Write(vecFilePath); err != nil {
		b.Fatalf("write vector file: %v", err)
	}

	if err := vs.LoadVectorFile(); err != nil {
		b.Fatalf("LoadVectorFile: %v", err)
	}
	if err := vs.BuildSemanticIndex(); err != nil {
		b.Fatalf("BuildSemanticIndex: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Float32 Search Benchmarks per dimension
// ---------------------------------------------------------------------------

func BenchmarkPerf_Dim384_Float32Search(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 384)
	defer cleanup()
	buildDimHNSWIndex(b, vs, gs, s, 384, 10_000)

	vectors, _ := generateClusteredVectors(99, 384, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim768_Float32Search(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 768)
	defer cleanup()
	buildDimHNSWIndex(b, vs, gs, s, 768, 10_000)

	vectors, _ := generateClusteredVectors(99, 768, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim1536_Float32Search(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 1536)
	defer cleanup()
	buildDimHNSWIndex(b, vs, gs, s, 1536, 10_000)

	vectors, _ := generateClusteredVectors(99, 1536, 1, 20)
	queryVec := vectors[0]

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
// Int8 Quantized Search Benchmarks per dimension
// ---------------------------------------------------------------------------

func BenchmarkPerf_Dim384_Int8Search(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 384)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 384, 10_000)

	vectors, _ := generateClusteredVectors(99, 384, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim768_Int8Search(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 768)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 768, 10_000)

	vectors, _ := generateClusteredVectors(99, 768, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim1536_Int8Search(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 1536)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 1536, 10_000)

	vectors, _ := generateClusteredVectors(99, 1536, 1, 20)
	queryVec := vectors[0]

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
// Two-Stage Search Benchmarks per dimension
// ---------------------------------------------------------------------------

func BenchmarkPerf_Dim384_TwoStageSearch(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 384)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 384, 10_000)
	vs.SetTwoStageSearch(true)

	vectors, _ := generateClusteredVectors(99, 384, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim768_TwoStageSearch(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 768)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 768, 10_000)
	vs.SetTwoStageSearch(true)

	vectors, _ := generateClusteredVectors(99, 768, 1, 20)
	queryVec := vectors[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := vs.searchDualHNSW(queryVec, 10)
		if err != nil {
			b.Fatalf("searchDualHNSW: %v", err)
		}
		_ = results
	}
}

func BenchmarkPerf_Dim1536_TwoStageSearch(b *testing.B) {
	vs, gs, s, dataDir, cleanup := openDimVectorSearch(b, 1536)
	defer cleanup()
	buildDimHNSWIndexQuantized(b, vs, gs, s, dataDir, 1536, 10_000)
	vs.SetTwoStageSearch(true)

	vectors, _ := generateClusteredVectors(99, 1536, 1, 20)
	queryVec := vectors[0]

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
// Build Time Benchmarks per dimension
// ---------------------------------------------------------------------------

func BenchmarkPerf_Dim384_BuildDualModalHNSW(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vs, gs, s, _, cleanup := openDimVectorSearch(b, 384)
		vectors, _ := generateClusteredVectors(42, 384, 10_000, 20)
		repo := gs.Repo()
		var nodeIDs []string
		for j := 0; j < 10_000; j++ {
			nodeID := fmt.Sprintf("build384n%d_%d", j, i)
			nodeIDs = append(nodeIDs, nodeID)
			vecData := util.EncodeFloat32Vec(vectors[j])
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

		if err := vs.BuildSemanticIndex(); err != nil {
			b.Fatalf("BuildSemanticIndex: %v", err)
		}

		b.StopTimer()
		cleanup()
		b.StartTimer()
	}
}

func BenchmarkPerf_Dim768_BuildDualModalHNSW(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vs, gs, s, _, cleanup := openDimVectorSearch(b, 768)
		vectors, _ := generateClusteredVectors(42, 768, 10_000, 20)
		repo := gs.Repo()
		var nodeIDs []string
		for j := 0; j < 10_000; j++ {
			nodeID := fmt.Sprintf("build768n%d_%d", j, i)
			nodeIDs = append(nodeIDs, nodeID)
			vecData := util.EncodeFloat32Vec(vectors[j])
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

		if err := vs.BuildSemanticIndex(); err != nil {
			b.Fatalf("BuildSemanticIndex: %v", err)
		}

		b.StopTimer()
		cleanup()
		b.StartTimer()
	}
}

func BenchmarkPerf_Dim1536_BuildDualModalHNSW(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vs, gs, s, _, cleanup := openDimVectorSearch(b, 1536)
		vectors, _ := generateClusteredVectors(42, 1536, 10_000, 20)
		repo := gs.Repo()
		var nodeIDs []string
		for j := 0; j < 10_000; j++ {
			nodeID := fmt.Sprintf("build1536n%d_%d", j, i)
			nodeIDs = append(nodeIDs, nodeID)
			vecData := util.EncodeFloat32Vec(vectors[j])
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

		if err := vs.BuildSemanticIndex(); err != nil {
			b.Fatalf("BuildSemanticIndex: %v", err)
		}

		b.StopTimer()
		cleanup()
		b.StartTimer()
	}
}

// ---------------------------------------------------------------------------
// Memory Footprint Benchmarks per dimension
// ---------------------------------------------------------------------------

func BenchmarkPerf_Dim384_Memory(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 384)
	defer cleanup()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	buildDimHNSWIndex(b, vs, gs, s, 384, 10_000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / 10_000
	est1M := perNode * 1_000_000

	b.ReportMetric(perNode, "bytes/node")
	b.ReportMetric(est1M/1e6, "MB_est_1M")
}

func BenchmarkPerf_Dim768_Memory(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 768)
	defer cleanup()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	buildDimHNSWIndex(b, vs, gs, s, 768, 10_000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / 10_000
	est1M := perNode * 1_000_000

	b.ReportMetric(perNode, "bytes/node")
	b.ReportMetric(est1M/1e6, "MB_est_1M")
}

func BenchmarkPerf_Dim1536_Memory(b *testing.B) {
	vs, gs, s, _, cleanup := openDimVectorSearch(b, 1536)
	defer cleanup()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	buildDimHNSWIndex(b, vs, gs, s, 1536, 10_000)

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocDiff := m2.Alloc - m1.Alloc
	perNode := float64(allocDiff) / 10_000
	est1M := perNode * 1_000_000

	b.ReportMetric(perNode, "bytes/node")
	b.ReportMetric(est1M/1e6, "MB_est_1M")
}

// ---------------------------------------------------------------------------
// Recall@10 Tests per dimension (using direct HNSW API + brute force)
// ---------------------------------------------------------------------------

// recallDimResult holds recall test results for a single dimension.
type recallDimResult struct {
	dim           int
	float32Recall float64
	int8Recall    float64
	ratio         float64
}

// computeDimRecall computes float32 and int8 HNSW recall for a given dimension.
func computeDimRecall(t *testing.T, dim int) recallDimResult {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	n := 2000
	k := 10
	numClusters := 20

	// Generate clustered vectors
	centers := make([][]float32, numClusters)
	for c := range centers {
		center := make([]float32, dim)
		for j := range center {
			center[j] = rng.Float32()*2.0 - 1.0
		}
		centers[c] = center
	}

	spread := float32(0.3)
	vectors := make([][]float32, n)
	for i := range vectors {
		centerIdx := i % numClusters
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = centers[centerIdx][j] + (rng.Float32()*2.0-1.0)*spread
		}
		vectors[i] = vec
	}

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

	measureRecall := func(g *hnsw.Graph[int]) float64 {
		totalRecall := 0.0
		queries := 100
		queryRng := rand.New(rand.NewSource(99))
		for q := 0; q < queries; q++ {
			queryIdx := queryRng.Intn(n)
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
	float32Recall := measureRecall(gFloat)

	// Int8 quantized HNSW
	params := hnsw.TrainQuantParams(vectors)
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
	int8Recall := measureRecall(gInt8)

	ratio := int8Recall / float32Recall * 100

	return recallDimResult{
		dim:           dim,
		float32Recall: float32Recall,
		int8Recall:    int8Recall,
		ratio:         ratio,
	}
}

// TestPerf_DimRecall measures recall@10 across dimensions 384, 768, 1536.
func TestPerf_DimRecall(t *testing.T) {
	if testing.Short() || os.Getenv("CODETRIP_PERF_TESTS") == "" {
		t.Skip("set CODETRIP_PERF_TESTS=1 to run performance tests")
	}

	for _, dim := range []int{384, 768, 1536} {
		result := computeDimRecall(t, dim)
		t.Logf("dim=%d: float32 recall@10=%.4f, int8 recall@10=%.4f, ratio=%.1f%%",
			result.dim, result.float32Recall, result.int8Recall, result.ratio)

		if result.int8Recall < result.float32Recall*0.85 {
			t.Errorf("dim=%d: int8 recall %.4f is below 85%% of float32 recall %.4f",
				result.dim, result.int8Recall, result.float32Recall)
		}
	}
}

// ---------------------------------------------------------------------------
// Comprehensive dimension benchmark: single test that runs all dims
// and reports search latency, build time, memory, and recall.
// ---------------------------------------------------------------------------

// TestPerf_DimComprehensive runs a comprehensive per-dimension test
// that measures search latency (100 queries), build time, and memory.
func TestPerf_DimComprehensive(t *testing.T) {
	if testing.Short() || os.Getenv("CODETRIP_PERF_TESTS") == "" {
		t.Skip("set CODETRIP_PERF_TESTS=1 to run performance tests")
	}

	for _, dim := range []int{384, 768, 1536} {
		t.Run(fmt.Sprintf("dim%d", dim), func(t *testing.T) {
			// Setup
			dir, err := os.MkdirTemp("", fmt.Sprintf("dim%d-comprehensive-*", dim))
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			cfg := store.DefaultConfig(filepath.Join(dir, "db"))
			cfg.CacheSize = 64 << 20
			s, err := store.Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			gs := graph.NewGraphStore(s, fmt.Sprintf("dim%dcomp", dim))
			embedder := &dimEmbedder{dim: dim}
			vs := NewVectorSearchWithDir(embedder, s, gs, dir)
			defer vs.Close()

			// Generate vectors
			vectors, _ := generateClusteredVectors(42, dim, 10_000, 20)
			repo := gs.Repo()

			// Store dual-modal vectors
			var nodeIDs []string
			for i := 0; i < 10_000; i++ {
				nodeID := fmt.Sprintf("compnode%d", i)
				nodeIDs = append(nodeIDs, nodeID)
				vecData := util.EncodeFloat32Vec(vectors[i])
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

			// Measure build time
			buildStart := time.Now()
			if err := vs.BuildSemanticIndex(); err != nil {
				t.Fatalf("BuildSemanticIndex: %v", err)
			}
			buildTime := time.Since(buildStart)

			// Measure memory
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			memMB := float64(m.Alloc) / 1e6
			perNode := float64(m.Alloc) / 10_000
			est1M := perNode * 1_000_000 / 1e6

			// Measure float32 search latency (100 queries)
			queryVecs, _ := generateClusteredVectors(99, dim, 100, 20)
			searchStart := time.Now()
			for i := 0; i < 100; i++ {
				vs.searchDualHNSW(queryVecs[i], 10)
			}
			searchTime := time.Since(searchStart)
			avgSearchUs := searchTime.Microseconds() / 100

			// Build quantized index and measure int8 search
			vectors2, qp := generateClusteredVectors(42, dim, 10_000, 20)
			// Re-store dual-modal vectors with quantized prefix
			var qNodeIDs []string
			for i := 0; i < 10_000; i++ {
				nodeID := fmt.Sprintf("compqnode%d", i)
				qNodeIDs = append(qNodeIDs, nodeID)
				vecData := util.EncodeFloat32Vec(vectors2[i])
				descKey := graph.EmbDescKey(repo, nodeID)
				codeKey := graph.EmbCodeKey(repo, nodeID)
				s.Set([]byte(descKey), vecData)
				s.Set([]byte(codeKey), vecData)
			}
			qDescIdxKey := graph.EmbDescIdxKey(repo)
			qCodeIdxKey := graph.EmbCodeIdxKey(repo)
			qIdxData := util.EncodeStringList(qNodeIDs)
			s.Set([]byte(qDescIdxKey), qIdxData)
			s.Set([]byte(qCodeIdxKey), qIdxData)

			// Write vector file
			vfWriter := NewVectorFileWriter(dim, qp.Scale, qp.Offset)
			for i := 0; i < 10_000; i++ {
				qvec := hnsw.Quantize(vectors2[i], qp)
				vfWriter.AddNodeVector(qvec)
			}
			vecFilePath := VectorFilePath(dir, repo)
			if err := vfWriter.Write(vecFilePath); err != nil {
				t.Fatalf("write vector file: %v", err)
			}

			// Need a fresh VectorSearch for quantized (to avoid index conflict)
			vs2 := NewVectorSearchWithDir(embedder, s, gs, dir)
			defer vs2.Close()
			if err := vs2.LoadVectorFile(); err != nil {
				t.Fatalf("LoadVectorFile: %v", err)
			}
			qBuildStart := time.Now()
			if err := vs2.BuildSemanticIndex(); err != nil {
				t.Fatalf("BuildSemanticIndex quantized: %v", err)
			}
			qBuildTime := time.Since(qBuildStart)

			// Int8 search latency
			qQueryVecs, _ := generateClusteredVectors(99, dim, 100, 20)
			int8Start := time.Now()
			for i := 0; i < 100; i++ {
				vs2.searchDualHNSW(qQueryVecs[i], 10)
			}
			int8SearchTime := time.Since(int8Start)
			avgInt8Us := int8SearchTime.Microseconds() / 100

			// Two-stage search
			vs2.SetTwoStageSearch(true)
			tsStart := time.Now()
			for i := 0; i < 100; i++ {
				vs2.searchDualHNSW(qQueryVecs[i], 10)
			}
			tsSearchTime := time.Since(tsStart)
			avgTsUs := tsSearchTime.Microseconds() / 100

			t.Logf("=== dim=%d ===", dim)
			t.Logf("  Build time (float32): %v", buildTime)
			t.Logf("  Build time (int8):    %v", qBuildTime)
			t.Logf("  Search avg (float32): %d µs", avgSearchUs)
			t.Logf("  Search avg (int8):    %d µs", avgInt8Us)
			t.Logf("  Search avg (2-stage): %d µs", avgTsUs)
			t.Logf("  Memory (alloc):       %.1f MB", memMB)
			t.Logf("  Est. 1M memory:       %.1f MB", est1M)
			t.Logf("  Per node:             %.0f bytes", perNode)
		})
	}
}
