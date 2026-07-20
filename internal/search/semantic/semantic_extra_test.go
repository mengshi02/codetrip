package semantic

import (
	"context"
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

func TestVectorSearch_BruteForce(t *testing.T) {
	vs, gs, cleanup := openTestVector(t)
	defer cleanup()

	// Add nodes
	n1 := graph.NewNode("testrepo", graph.LabelFunction, "funcA")
	n2 := graph.NewNode("testrepo", graph.LabelFunction, "funcB")
	gs.AddNode(n1)
	gs.AddNode(n2)
	gs.Flush()

	// Store dual-modal vector data using util encoding
	vec1 := make([]float32, 8)
	vec2 := make([]float32, 8)
	for i := range vec1 {
		vec1[i] = float32(i) / 8.0
		vec2[i] = float32(i+2) / 8.0
	}
	// Write desc vectors
	vs.store.Set([]byte(graph.EmbDescKey("testrepo", n1.ID)), util.EncodeFloat32Vec(vec1))
	vs.store.Set([]byte(graph.EmbDescKey("testrepo", n2.ID)), util.EncodeFloat32Vec(vec2))
	// Write code vectors
	vs.store.Set([]byte(graph.EmbCodeKey("testrepo", n1.ID)), util.EncodeFloat32Vec(vec1))
	vs.store.Set([]byte(graph.EmbCodeKey("testrepo", n2.ID)), util.EncodeFloat32Vec(vec2))

	// Store modality indices using util encoding
	vs.store.Set([]byte(graph.EmbDescIdxKey("testrepo")), util.EncodeStringList([]string{n1.ID, n2.ID}))
	vs.store.Set([]byte(graph.EmbCodeIdxKey("testrepo")), util.EncodeStringList([]string{n1.ID, n2.ID}))
	vs.store.Flush()

	// Without HNSW, SearchDualModal returns nil (no fallback)
	results, err := vs.Search(context.Background(), "test query", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected empty results when no HNSW index built")
	}

	// Build HNSW and search again — now should return results
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatal(err)
	}
	results, err = vs.Search(context.Background(), "test query", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected search results after building HNSW")
	}
}

func TestVectorSearch_WithHNSW(t *testing.T) {
	vs, gs, cleanup := openTestVector(t)
	defer cleanup()

	// Add nodes and dual-modal vectors
	for i := 0; i < 5; i++ {
		n := graph.NewNode("testrepo", graph.LabelFunction, fmt.Sprintf("hnswFunc%d", i))
		gs.AddNode(n)
		vec := make([]float32, 8)
		for j := range vec {
			vec[j] = float32(i*8+j) / 64.0
		}
		// Write desc and code vectors using util encoding
		vs.store.Set([]byte(graph.EmbDescKey("testrepo", n.ID)), util.EncodeFloat32Vec(vec))
		vs.store.Set([]byte(graph.EmbCodeKey("testrepo", n.ID)), util.EncodeFloat32Vec(vec))
	}
	gs.Flush()

	// Store modality indices using util encoding
	nodeIDs := make([]string, 5)
	iter := gs.IterNodes("testrepo")
	i := 0
	for iter.Next() {
		nodeIDs[i] = iter.Node().ID
		i++
	}
	iter.Close()
	vs.store.Set([]byte(graph.EmbDescIdxKey("testrepo")), util.EncodeStringList(nodeIDs))
	vs.store.Set([]byte(graph.EmbCodeIdxKey("testrepo")), util.EncodeStringList(nodeIDs))
	vs.store.Flush()

	// Build dual-modal HNSW index
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Fatal(err)
	}

	results, err := vs.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected HNSW search results")
	}
}

func TestVectorSearch_CancelledContext(t *testing.T) {
	vs, gs, cleanup := openTestVector(t)
	defer cleanup()

	n := graph.NewNode("testrepo", graph.LabelFunction, "cancelFunc")
	gs.AddNode(n)
	gs.Flush()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should still work or return nil (embedder may fail)
	results, err := vs.Search(ctx, "test", 10)
	_ = results
	_ = err
}

func TestNewVectorSearch_NilEmbedder(t *testing.T) {
	dir, _ := mkTmpDir("vectest-")
	defer rmDir(dir)
	cfg := defaultCfg(dir)
	store, _ := openStore(cfg)
	defer store.Close()
	gs := graph.NewGraphStore(store, "testrepo")
	vs := NewVectorSearch(nil, store, gs)
	if vs.embedder != nil {
		t.Error("expected nil embedder")
	}
}

func TestVectorSearch_EmbedDimensionsZero(t *testing.T) {
	dir, _ := mkTmpDir("vectest-")
	defer rmDir(dir)
	cfg := defaultCfg(dir)
	store, _ := openStore(cfg)
	defer store.Close()
	gs := graph.NewGraphStore(store, "testrepo")
	embedder := &vectorMockEmbedder{dim: 0}
	vs := NewVectorSearch(embedder, store, gs)

	results, err := vs.Search(context.Background(), "test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected empty results with 0-dim embedder")
	}
}
