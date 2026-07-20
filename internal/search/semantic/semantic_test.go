package semantic

import (
	"context"
	"testing"

	"os"
	"path/filepath"

	"github.com/mengshi02/codetrip/internal/graph"
	store "github.com/mengshi02/codetrip/internal/store"
)

// vectorMockEmbedder 模拟嵌入模型
type vectorMockEmbedder struct {
	dim int
}

func (m *vectorMockEmbedder) Dimensions() int { return m.dim }
func (m *vectorMockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, m.dim)
		for j := range vec {
			vec[j] = float32(i+j+1) / float32(m.dim)
		}
		result[i] = vec
	}
	return result, nil
}

func openTestVector(t *testing.T) (*VectorSearch, *graph.GraphStore, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "vectest-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	store, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	gs := graph.NewGraphStore(store, "testrepo")
	embedder := &vectorMockEmbedder{dim: 8}
	vs := NewVectorSearch(embedder, store, gs)
	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}
	return vs, gs, cleanup
}

func TestVectorSearch_NoEmbedder(t *testing.T) {
	dir, _ := os.MkdirTemp("", "vectest-*")
	defer os.RemoveAll(dir)
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	store, _ := store.Open(cfg)
	defer store.Close()
	gs := graph.NewGraphStore(store, "testrepo")
	vs := NewVectorSearch(nil, store, gs)

	results, err := vs.Search(context.Background(), "test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected empty results with nil embedder")
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if sim := cosineSimilarity(a, b); sim < 0.99 {
		t.Errorf("identical vectors: got %f, want 1.0", sim)
	}

	c := []float32{0, 1, 0}
	if sim := cosineSimilarity(a, c); sim > 0.01 {
		t.Errorf("orthogonal vectors: got %f, want 0.0", sim)
	}

	if sim := cosineSimilarity(nil, b); sim != 0 {
		t.Errorf("nil vector: got %f, want 0", sim)
	}
}

func TestCosineSimilarity_Unaligned(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5}
	b := []float32{2, 3, 4, 5, 6}
	sim := cosineSimilarity(a, b)
	if sim <= 0 || sim > 1 {
		t.Errorf("cosine similarity = %f, want (0, 1]", sim)
	}
}

func TestVectorSearch_BuildSemanticIndex_NoData(t *testing.T) {
	vs, _, cleanup := openTestVector(t)
	defer cleanup()

	// No vectors stored, should return nil
	if err := vs.BuildSemanticIndex(); err != nil {
		t.Error("expected nil error with no data")
	}
}
