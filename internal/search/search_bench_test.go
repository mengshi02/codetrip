package search

import (
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func BenchmarkBM25IndexNode(b *testing.B) {
	idx, cleanup := openTestBM25B(b)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "benchFunc")
	node.FilePath = "pkg/bench.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.Name = fmt.Sprintf("benchFunc%d", i)
		idx.IndexNode(node)
	}
}

func BenchmarkBM25BatchIndex(b *testing.B) {
	idx, cleanup := openTestBM25B(b)
	defer cleanup()

	nodes := make([]*graph.Node, 100)
	for i := range nodes {
		nodes[i] = graph.NewNode("testrepo", graph.LabelFunction, fmt.Sprintf("batchFunc%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.BatchIndex(nodes)
	}
}

func BenchmarkBM25Search(b *testing.B) {
	idx, cleanup := openTestBM25B(b)
	defer cleanup()

	for i := 0; i < 100; i++ {
		n := graph.NewNode("testrepo", graph.LabelFunction, fmt.Sprintf("searchFunc%d", i))
		n.FilePath = fmt.Sprintf("pkg/file%d.go", i)
		idx.IndexNode(n)
	}
	if err := idx.FinalizeBuild(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search("searchFunc", 10)
	}
}

func BenchmarkTokenize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tokenize("getUserByIDHTTPRequest")
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float32, 384)
	v := make([]float32, 384)
	for i := range a {
		a[i] = float32(i) / 384.0
		v[i] = float32(i+1) / 384.0
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, v)
	}
}

func openTestBM25B(b *testing.B) (*BM25Index, func()) {
	b.Helper()
	dir, err := mkTmpDir("bench-bm25-")
	if err != nil {
		b.Fatal(err)
	}
	cfg := defaultCfg(dir)
	s, err := openStore(cfg)
	if err != nil {
		b.Fatal(err)
	}
	idx, err := NewBM25IndexWithDir(dir, "testrepo", s)
	if err != nil {
		s.Close()
		rmDir(dir)
		b.Fatal(err)
	}
	return idx, func() {
		idx.Close()
		s.Close()
		rmDir(dir)
	}
}
