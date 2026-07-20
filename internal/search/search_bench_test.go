package search

import (
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/search/symbol"
)

func BenchmarkLexicalIndexNode(b *testing.B) {
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

func openTestBM25B(b *testing.B) (*symbol.LexicalIndex, func()) {
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
	idx, err := symbol.NewLexicalIndexWithDir(dir, "testrepo", s)
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
