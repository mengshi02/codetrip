package semantic

import (
	"context"
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	store "github.com/mengshi02/codetrip/internal/storage"
	"github.com/mengshi02/codetrip/internal/util"
)

// setupDualModalData creates a store with dual-modal embedding data
// (desc + code vectors and indices) for testing.
func setupDualModalData(t *testing.T) (*store.Store, *graph.GraphStore, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	repo := "dualtest"
	gs := graph.NewGraphStore(s, repo)

	// Add function nodes
	n1 := &graph.Node{ID: "f1", Name: "processOrder", Label: graph.LabelFunction, Repo: repo}
	n1.Props.SetProp("content", "func processOrder() {}")
	n2 := &graph.Node{ID: "f2", Name: "validateInput", Label: graph.LabelFunction, Repo: repo}
	n2.Props.SetProp("content", "func validateInput() {}")

	if err := gs.AddNode(n1); err != nil {
		t.Fatalf("add n1: %v", err)
	}
	if err := gs.AddNode(n2); err != nil {
		t.Fatalf("add n2: %v", err)
	}
	if err := gs.AddEdge(&graph.Edge{Source: "f1", Target: "f2", Type: graph.RelCalls}); err != nil {
		t.Fatalf("add edge: %v", err)
	}

	// Store desc vectors (embdesc:{repo}:{nodeID})
	dim := 8
	vec1 := make([]float32, dim)
	vec2 := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vec1[i] = 0.5
		vec2[i] = 0.3
	}

	// Write desc vectors
	descKey1 := graph.EmbDescKey(repo, "f1")
	descKey2 := graph.EmbDescKey(repo, "f2")
	if err := s.Set([]byte(descKey1), util.EncodeFloat32Vec(vec1)); err != nil {
		t.Fatalf("set desc vec1: %v", err)
	}
	if err := s.Set([]byte(descKey2), util.EncodeFloat32Vec(vec2)); err != nil {
		t.Fatalf("set desc vec2: %v", err)
	}

	// Write code vectors
	codeKey1 := graph.EmbCodeKey(repo, "f1")
	codeKey2 := graph.EmbCodeKey(repo, "f2")
	if err := s.Set([]byte(codeKey1), util.EncodeFloat32Vec(vec1)); err != nil {
		t.Fatalf("set code vec1: %v", err)
	}
	if err := s.Set([]byte(codeKey2), util.EncodeFloat32Vec(vec2)); err != nil {
		t.Fatalf("set code vec2: %v", err)
	}

	// Write modality indices
	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	if err := s.Set([]byte(descIdxKey), util.EncodeStringList([]string{"f1", "f2"})); err != nil {
		t.Fatalf("set desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), util.EncodeStringList([]string{"f1", "f2"})); err != nil {
		t.Fatalf("set code index: %v", err)
	}

	return s, gs, repo
}

// TestBuildSemanticIndex verifies that dual-modal HNSW indices
// can be built from stored embedding data.
func TestBuildSemanticIndex(t *testing.T) {
	s, gs, _ := setupDualModalData(t)
	defer s.Close()

	vs := NewVectorSearch(&vectorMockEmbedder{dim: 8}, s, gs)

	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatalf("BuildSemanticIndex: %v", err)
	}

	// Verify dual HNSW indices are built
	vs.hnswMu.RLock()
	built := vs.dualHnswBuilt
	descIdx := vs.descHnswIdx
	codeIdx := vs.codeHnswIdx
	vs.hnswMu.RUnlock()

	if !built {
		t.Error("dualHnswBuilt should be true after BuildSemanticIndex")
	}
	if descIdx == nil {
		t.Error("descHnswIdx should not be nil after build")
	}
	if codeIdx == nil {
		t.Error("codeHnswIdx should not be nil after build")
	}
}

// TestSearchDualModal_WithHNSW verifies that SearchDualModal returns
// results when dual-modal HNSW indices are available.
func TestSearchDualModal_WithHNSW(t *testing.T) {
	s, gs, _ := setupDualModalData(t)
	defer s.Close()

	vs := NewVectorSearch(&vectorMockEmbedder{dim: 8}, s, gs)

	// Build dual-modal HNSW index
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatalf("BuildSemanticIndex: %v", err)
	}

	results, err := vs.SearchDualModal(context.Background(), "process order", 10)
	if err != nil {
		t.Fatalf("SearchDualModal: %v", err)
	}

	// Should return results from dual-modal search
	if len(results) == 0 {
		t.Error("expected non-empty results from dual-modal search with HNSW")
	}
}

// TestSearchDualModal_NoHNSW verifies that SearchDualModal
// returns empty results when no HNSW indices are built (no brute-force fallback).
func TestSearchDualModal_NoHNSW(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	repo := "fallbackrepo"
	gs := graph.NewGraphStore(s, repo)

	// Add node and store dual-modal vector data
	n1 := &graph.Node{ID: "fn1", Name: "helper", Label: graph.LabelFunction, Repo: repo}
	n1.Props.SetProp("content", "func helper() {}")
	if err := gs.AddNode(n1); err != nil {
		t.Fatalf("add node: %v", err)
	}

	// Store dual-modal vectors and indices
	dim := 8
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vec[i] = 0.5
	}
	descKey := graph.EmbDescKey(repo, "fn1")
	codeKey := graph.EmbCodeKey(repo, "fn1")
	if err := s.Set([]byte(descKey), util.EncodeFloat32Vec(vec)); err != nil {
		t.Fatalf("set desc vec: %v", err)
	}
	if err := s.Set([]byte(codeKey), util.EncodeFloat32Vec(vec)); err != nil {
		t.Fatalf("set code vec: %v", err)
	}
	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	if err := s.Set([]byte(descIdxKey), util.EncodeStringList([]string{"fn1"})); err != nil {
		t.Fatalf("set desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), util.EncodeStringList([]string{"fn1"})); err != nil {
		t.Fatalf("set code index: %v", err)
	}

	vs := NewVectorSearch(&vectorMockEmbedder{dim: 8}, s, gs)

	// Do NOT build any HNSW index — should return empty results (no fallback)
	results, err := vs.SearchDualModal(context.Background(), "helper function", 10)
	if err != nil {
		t.Fatalf("SearchDualModal: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected empty results when no HNSW index built")
	}

	// Build HNSW and search again — now should return results
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatalf("BuildSemanticIndex: %v", err)
	}
	results, err = vs.SearchDualModal(context.Background(), "helper function", 10)
	if err != nil {
		t.Fatalf("SearchDualModal after HNSW build: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results after building HNSW index")
	}
}

// TestSearchDualModal_NoData verifies that SearchDualModal returns
// empty results when no vector data exists at all.
func TestSearchDualModal_NoData(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "nodatarepo")
	vs := NewVectorSearch(&vectorMockEmbedder{dim: 8}, s, gs)

	results, err := vs.SearchDualModal(context.Background(), "test query", 10)
	if err != nil {
		t.Fatalf("SearchDualModal no data: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with no data, got %d", len(results))
	}
}

// TestBuildSemanticIndex_NoData verifies that BuildSemanticIndex
// handles the case where no dual-modal embedding data exists.
func TestBuildSemanticIndex_NoData(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "emptyrepo")
	vs := NewVectorSearch(&vectorMockEmbedder{dim: 8}, s, gs)

	// No vectors stored — should return nil without error
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Errorf("expected nil error with no data, got: %v", err)
	}
}

// TestRestoreSemanticIndex verifies that dual-modal HNSW indices
// can be persisted and restored.
func TestRestoreSemanticIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	repo := "persistrepo"
	gs := graph.NewGraphStore(s, repo)

	// Add nodes and dual-modal vectors
	n1 := &graph.Node{ID: "f1", Name: "testFunc", Label: graph.LabelFunction, Repo: repo}
	n1.Props.SetProp("content", "func testFunc() {}")
	if err := gs.AddNode(n1); err != nil {
		t.Fatalf("add node: %v", err)
	}

	dim := 8
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vec[i] = 0.5
	}

	descKey := graph.EmbDescKey(repo, "f1")
	codeKey := graph.EmbCodeKey(repo, "f1")
	if err := s.Set([]byte(descKey), util.EncodeFloat32Vec(vec)); err != nil {
		t.Fatalf("set desc vec: %v", err)
	}
	if err := s.Set([]byte(codeKey), util.EncodeFloat32Vec(vec)); err != nil {
		t.Fatalf("set code vec: %v", err)
	}

	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	if err := s.Set([]byte(descIdxKey), util.EncodeStringList([]string{"f1"})); err != nil {
		t.Fatalf("set desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), util.EncodeStringList([]string{"f1"})); err != nil {
		t.Fatalf("set code index: %v", err)
	}

	// Build with data dir for persistence
	vs := NewVectorSearchWithDir(&vectorMockEmbedder{dim: 8}, s, gs, dir)

	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatalf("BuildSemanticIndex: %v", err)
	}

	// Verify built
	vs.hnswMu.RLock()
	built := vs.dualHnswBuilt
	vs.hnswMu.RUnlock()
	if !built {
		t.Fatal("dual-modal HNSW should be built")
	}

	// Create new VectorSearch and restore
	vs2 := NewVectorSearchWithDir(&vectorMockEmbedder{dim: 8}, s, gs, dir)
	restored := vs2.RestoreSemanticIndex()
	if !restored {
		t.Error("RestoreSemanticIndex should return true when persisted indices exist")
	}

	vs2.hnswMu.RLock()
	built2 := vs2.dualHnswBuilt
	descIdx2 := vs2.descHnswIdx
	codeIdx2 := vs2.codeHnswIdx
	vs2.hnswMu.RUnlock()

	if !built2 {
		t.Error("dualHnswBuilt should be true after restore")
	}
	if descIdx2 == nil {
		t.Error("descHnswIdx should not be nil after restore")
	}
	if codeIdx2 == nil {
		t.Error("codeHnswIdx should not be nil after restore")
	}

	s.Close()
}

// BenchmarkBuildSemanticIndex benchmarks building dual-modal HNSW indices.
func BenchmarkBuildSemanticIndex(b *testing.B) {
	dir := b.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer s.Close()

	repo := "benchdual"
	gs := graph.NewGraphStore(s, repo)

	dim := 384
	// Create 100 nodes with dual-modal vectors
	for i := 0; i < 100; i++ {
		nodeID := fmt.Sprintf("fn%d", i)
		n := &graph.Node{ID: nodeID, Name: fmt.Sprintf("func%d", i), Label: graph.LabelFunction, Repo: repo}
		n.Props.SetProp("content", fmt.Sprintf("func func%d() {}", i))
		if err := gs.AddNode(n); err != nil {
			b.Fatalf("add node: %v", err)
		}

		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			vec[j] = float32(i+j) / float32(dim)
		}

		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), util.EncodeFloat32Vec(vec)); err != nil {
			b.Fatalf("set desc vec: %v", err)
		}
		if err := s.Set([]byte(codeKey), util.EncodeFloat32Vec(vec)); err != nil {
			b.Fatalf("set code vec: %v", err)
		}
	}

	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)

	nodeIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		nodeIDs[i] = fmt.Sprintf("fn%d", i)
	}
	if err := s.Set([]byte(descIdxKey), util.EncodeStringList(nodeIDs)); err != nil {
		b.Fatalf("set desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), util.EncodeStringList(nodeIDs)); err != nil {
		b.Fatalf("set code index: %v", err)
	}

	embedder := &vectorMockEmbedder{dim: dim}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vs := NewVectorSearch(embedder, s, gs)
		if err := vs.BuildSemanticIndex(); err != nil {
			b.Fatalf("build: %v", err)
		}
	}
}

// BenchmarkSearchDualModal benchmarks dual-modal search performance.
func BenchmarkSearchDualModal(b *testing.B) {
	dir := b.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer s.Close()

	repo := "benchsearch"
	gs := graph.NewGraphStore(s, repo)

	dim := 384
	for i := 0; i < 100; i++ {
		nodeID := fmt.Sprintf("fn%d", i)
		n := &graph.Node{ID: nodeID, Name: fmt.Sprintf("func%d", i), Label: graph.LabelFunction, Repo: repo}
		n.Props.SetProp("content", fmt.Sprintf("func func%d() {}", i))
		if err := gs.AddNode(n); err != nil {
			b.Fatalf("add node: %v", err)
		}

		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			vec[j] = float32(i+j) / float32(dim)
		}

		descKey := graph.EmbDescKey(repo, nodeID)
		codeKey := graph.EmbCodeKey(repo, nodeID)
		if err := s.Set([]byte(descKey), util.EncodeFloat32Vec(vec)); err != nil {
			b.Fatalf("set desc vec: %v", err)
		}
		if err := s.Set([]byte(codeKey), util.EncodeFloat32Vec(vec)); err != nil {
			b.Fatalf("set code vec: %v", err)
		}
	}

	descIdxKey := graph.EmbDescIdxKey(repo)
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	nodeIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		nodeIDs[i] = fmt.Sprintf("fn%d", i)
	}
	if err := s.Set([]byte(descIdxKey), util.EncodeStringList(nodeIDs)); err != nil {
		b.Fatalf("set desc index: %v", err)
	}
	if err := s.Set([]byte(codeIdxKey), util.EncodeStringList(nodeIDs)); err != nil {
		b.Fatalf("set code index: %v", err)
	}

	vs := NewVectorSearch(&vectorMockEmbedder{dim: dim}, s, gs)
	if err := vs.BuildSemanticIndex(); err != nil {
		b.Fatalf("build index: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vs.SearchDualModal(context.Background(), "test query", 10)
	}
}
