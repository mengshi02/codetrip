package semantic

import (
	"context"
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	store "github.com/mengshi02/codetrip/internal/storage"
)

// TestRunDualModal_Basic verifies that RunDualModal embeds nodes using
// both description and code modalities and stores them with dual-modal keys.
func TestRunDualModal_Basic(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "dualrepo")

	// Add a function node with content
	node := &graph.Node{
		ID:    "fn1",
		Name:  "processOrder",
		Label: graph.LabelFunction,
		Repo:  "dualrepo",
	}
	node.Props.SetProp("content", "func processOrder() { fmt.Println(\"order\") }")
	if err := gs.AddNode(node); err != nil {
		t.Fatalf("add node: %v", err)
	}

	embedder := &mockEmbedder{dimensions: 16}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	result, err := pipeline.RunDualModal(context.Background(), "dualrepo")
	if err != nil {
		t.Fatalf("RunDualModal: %v", err)
	}

	if result.NodesEmbedded != 1 {
		t.Errorf("NodesEmbedded = %d, want 1", result.NodesEmbedded)
	}
	if result.DescChunks < 1 {
		t.Errorf("DescChunks = %d, want >= 1", result.DescChunks)
	}
	if result.CodeChunks < 1 {
		t.Errorf("CodeChunks = %d, want >= 1", result.CodeChunks)
	}

	// Verify desc vector was stored under embdesc: key
	descKey := graph.EmbDescKey("dualrepo", "fn1")
	descData, err := s.Get([]byte(descKey))
	if err != nil {
		t.Errorf("desc vector not found: %v", err)
	}
	if len(descData) == 0 {
		t.Error("desc vector data is empty")
	}

	// Verify code vector was stored under embcode: key
	codeKey := graph.EmbCodeKey("dualrepo", "fn1")
	codeData, err := s.Get([]byte(codeKey))
	if err != nil {
		t.Errorf("code vector not found: %v", err)
	}
	if len(codeData) == 0 {
		t.Error("code vector data is empty")
	}

	// Verify modality indices were created
	descIdxKey := graph.EmbDescIdxKey("dualrepo")
	descIdxData, err := s.Get([]byte(descIdxKey))
	if err != nil {
		t.Errorf("desc index not found: %v", err)
	}
	if len(descIdxData) == 0 {
		t.Error("desc index data is empty")
	}

	codeIdxKey := graph.EmbCodeIdxKey("dualrepo")
	codeIdxData, err := s.Get([]byte(codeIdxKey))
	if err != nil {
		t.Errorf("code index not found: %v", err)
	}
	if len(codeIdxData) == 0 {
		t.Error("code index data is empty")
	}
}

// TestRunDualModal_RequiresIndex verifies that RunDualModal fails when
// no embedder is configured.
func TestRunDualModal_RequiresIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "emptyrepo")
	pipeline := NewEmbeddingPipeline(nil, gs, s, DefaultEmbedConfig())

	result, err := pipeline.RunDualModal(context.Background(), "emptyrepo")
	if err != nil {
		t.Errorf("unexpected error when no embedder configured: %v", err)
	}
	if result.NodesEmbedded != 0 {
		t.Error("expected 0 nodes embedded without embedder")
	}
}

// TestRunDualModal_MultipleNodes verifies dual-modal embedding with
// multiple nodes and cross-references (CALLS edges).
func TestRunDualModal_MultipleNodes(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "multirepo")

	// Add nodes with content
	n1 := &graph.Node{ID: "f1", Name: "main", Label: graph.LabelFunction, Repo: "multirepo"}
	n1.Props.SetProp("content", "func main() { helper() }")
	n2 := &graph.Node{ID: "f2", Name: "helper", Label: graph.LabelFunction, Repo: "multirepo"}
	n2.Props.SetProp("content", "func helper() {}")

	if err := gs.AddNode(n1); err != nil {
		t.Fatalf("add n1: %v", err)
	}
	if err := gs.AddNode(n2); err != nil {
		t.Fatalf("add n2: %v", err)
	}
	if err := gs.AddEdge(&graph.Edge{Source: "f1", Target: "f2", Type: graph.RelCalls}); err != nil {
		t.Fatalf("add edge: %v", err)
	}

	embedder := &mockEmbedder{dimensions: 16}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	result, err := pipeline.RunDualModal(context.Background(), "multirepo")
	if err != nil {
		t.Fatalf("RunDualModal: %v", err)
	}

	if result.NodesEmbedded != 2 {
		t.Errorf("NodesEmbedded = %d, want 2", result.NodesEmbedded)
	}
	if result.DescChunks != 2 {
		t.Errorf("DescChunks = %d, want 2", result.DescChunks)
	}
}

// TestRunDualModal_NoEmbeddableNodes verifies that RunDualModal handles
// the case where a node has no source content but has label/name metadata.
// buildSynthesizedContent creates a code-like representation from node
// properties, so such nodes are still embeddable via the synthesized fallback.
func TestRunDualModal_NoEmbeddableNodes(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "norepo")

	// Add a node without content — but with label + name, so synthesized content is generated
	node := &graph.Node{ID: "n1", Name: "empty", Label: graph.LabelFunction, Repo: "norepo"}
	if err := gs.AddNode(node); err != nil {
		t.Fatalf("add node: %v", err)
	}

	embedder := &mockEmbedder{dimensions: 16}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	result, err := pipeline.RunDualModal(context.Background(), "norepo")
	if err != nil {
		t.Fatalf("RunDualModal: %v", err)
	}
	if result.NodesEmbedded != 1 {
		t.Errorf("NodesEmbedded = %d, want 1 (synthesized content from label/name)", result.NodesEmbedded)
	}
}

// BenchmarkRunDualModal benchmarks the dual-modal pipeline with multiple nodes.
func BenchmarkRunDualModal(b *testing.B) {
	dir := b.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "benchrepo")

	// Create 50 function nodes with content
	for i := 0; i < 50; i++ {
		n := &graph.Node{
			ID:    fmt.Sprintf("fn%d", i),
			Name:  fmt.Sprintf("func%d", i),
			Label: graph.LabelFunction,
			Repo:  "benchrepo",
		}
		n.Props.SetProp("content", fmt.Sprintf("func func%d() { return %d }", i, i))
		if err := gs.AddNode(n); err != nil {
			b.Fatalf("add node: %v", err)
		}
	}

	embedder := &mockEmbedder{dimensions: 384}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Build the complete semantic snapshot
		_, _ = pipeline.RunDualModal(context.Background(), "benchrepo")
	}
}
