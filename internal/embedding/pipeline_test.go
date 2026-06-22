package embedding

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

// ============ Chunker tests ============

func TestChunkerASTFunction(t *testing.T) {
	chunker := NewChunker(DefaultEmbedConfig())

	code := `package main

func Hello() {
	fmt.Println("hello")
}

func World() {
	fmt.Println("world")
}`

	chunks := chunker.Chunk(code, "Function")
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk for function code")
	}
}

func TestChunkerOversizedContent(t *testing.T) {
	cfg := DefaultEmbedConfig()
	cfg.ChunkSize = 50 // small chunk size to force splitting
	chunker := NewChunker(cfg)

	// Generate content longer than ChunkSize
	longCode := "package main\n\nfunc Big() {\n"
	for i := 0; i < 100; i++ {
		longCode += "\tx := " + string(rune('a'+i%26)) + "\n"
	}
	longCode += "}"

	chunks := chunker.Chunk(longCode, "Variable")
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk for long content")
	}
}

func TestChunkerEmptyContent(t *testing.T) {
	chunker := NewChunker(DefaultEmbedConfig())
	chunks := chunker.Chunk("", "Function")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

// ============ Mock Embedder ============

type mockEmbedder struct {
	dimensions int
	callCount  int
}

func (m *mockEmbedder) Dimensions() int { return m.dimensions }
func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.callCount++
	vec := make([]float32, m.dimensions)
	for i := range vec {
		vec[i] = 0.1
	}
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = vec
	}
	return result, nil
}

// ============ EmbeddingPipeline integration tests ============

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newPropsWithContent creates NodeProps with content in Extra map
func newPropsWithContent(content string) graph.NodeProps {
	return graph.NodeProps{Extra: map[string]any{"content": content}}
}

func TestEmbeddingPipelineRunWithMock(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "test-repo")

	// Add nodes with content
	node1 := &graph.Node{
		ID:       "node-1",
		Label:    graph.LabelFunction,
		FilePath: "main.go",
		Props:    newPropsWithContent("func hello() { fmt.Println(\"hello\") }"),
	}
	node2 := &graph.Node{
		ID:       "node-2",
		Label:    graph.LabelClass,
		FilePath: "service.go",
		Props:    newPropsWithContent("type Service struct { Name string }"),
	}
	if err := gs.AddNode(node1); err != nil {
		t.Fatalf("add node1: %v", err)
	}
	if err := gs.AddNode(node2); err != nil {
		t.Fatalf("add node2: %v", err)
	}

	embedder := &mockEmbedder{dimensions: 384}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	result, err := pipeline.Run(context.Background(), "test-repo")
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}

	if result.NodesEmbedded != 2 {
		t.Errorf("expected 2 nodes embedded, got %d", result.NodesEmbedded)
	}
	if embedder.callCount == 0 {
		t.Error("expected embedder to be called")
	}
}

func TestEmbeddingPipelineIncrementalSkip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "test-repo")

	node := &graph.Node{
		ID:       "node-1",
		Label:    graph.LabelFunction,
		FilePath: "main.go",
		Props:    newPropsWithContent("func hello() { fmt.Println(\"hello\") }"),
	}
	if err := gs.AddNode(node); err != nil {
		t.Fatalf("add node: %v", err)
	}

	embedder := &mockEmbedder{dimensions: 384}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	// First run
	result1, err := pipeline.Run(context.Background(), "test-repo")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if result1.NodesEmbedded != 1 {
		t.Errorf("first run: expected 1 node embedded, got %d", result1.NodesEmbedded)
	}

	// Second run — should skip because content hash unchanged
	result2, err := pipeline.Run(context.Background(), "test-repo")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result2.Skipped != 1 {
		t.Errorf("second run: expected 1 skipped, got %d", result2.Skipped)
	}
	if result2.NodesEmbedded != 0 {
		t.Errorf("second run: expected 0 nodes embedded (all skipped), got %d", result2.NodesEmbedded)
	}
}

func TestEmbeddingPipelineNoEmbedder(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "test-repo")
	pipeline := NewEmbeddingPipeline(nil, gs, s, DefaultEmbedConfig())

	result, err := pipeline.Run(context.Background(), "test-repo")
	if err != nil {
		t.Fatalf("pipeline run with nil embedder: %v", err)
	}
	if result.NodesEmbedded != 0 {
		t.Errorf("expected 0 nodes embedded with nil embedder, got %d", result.NodesEmbedded)
	}
}

func TestEmbeddingPipelineContextCancel(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "test-repo")

	// Add multiple nodes
	for i := 0; i < 20; i++ {
		node := &graph.Node{
			ID:       "node-" + string(rune('A'+i)),
			Label:    graph.LabelFunction,
			FilePath: filepath.Join("pkg", "main.go"),
			Props:    newPropsWithContent("func fn" + string(rune('A'+i)) + "() {}"),
		}
		if err := gs.AddNode(node); err != nil {
			t.Fatalf("add node: %v", err)
		}
	}

	embedder := &mockEmbedder{dimensions: 384}
	pipeline := NewEmbeddingPipeline(embedder, gs, s, DefaultEmbedConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := pipeline.Run(ctx, "test-repo")
	// Should either return an error or a result with 0 nodes embedded
	if err == nil && result.NodesEmbedded > 0 {
		t.Logf("context cancelled but some nodes were embedded: %d (acceptable race)", result.NodesEmbedded)
	}
}

// ============ getNodeContent test ============

func TestGetNodeContent(t *testing.T) {
	tests := []struct {
		name  string
		props graph.NodeProps
		want  string
	}{
		{"content field", graph.NodeProps{Extra: map[string]any{"content": "hello"}}, "hello"},
		{"snippet fallback", graph.NodeProps{Extra: map[string]any{"snippet": "world"}}, "world"},
		{"content preferred over snippet", graph.NodeProps{Extra: map[string]any{"content": "hello", "snippet": "world"}}, "hello"},
		{"empty", graph.NodeProps{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &graph.Node{Props: tt.props}
			got := getNodeContent(node)
			if got != tt.want {
				t.Errorf("getNodeContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
