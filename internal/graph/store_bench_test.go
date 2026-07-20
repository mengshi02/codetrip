package graph

import (
	"context"
	"fmt"
	"os"
	"testing"

	store "github.com/mengshi02/codetrip/internal/storage"
)

func benchGS(b *testing.B) *GraphStore {
	dir, _ := os.MkdirTemp("", "bench-*")
	b.Cleanup(func() { os.RemoveAll(dir) })
	cfg := store.DefaultConfig(dir)
	cfg.CacheSize = 64 << 20
	store, _ := store.Open(cfg)
	b.Cleanup(func() { store.Close() })
	return NewGraphStore(store, "benchrepo")
}

func BenchmarkAddNode(b *testing.B) {
	gs := benchGS(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.AddNode(NewNode("benchrepo", LabelFunction, fmt.Sprintf("n%d", i)))
	}
}

func BenchmarkAddEdge(b *testing.B) {
	gs := benchGS(b)
	nodes := make([]string, b.N+1)
	for i := range nodes {
		n := NewNode("benchrepo", LabelFunction, fmt.Sprintf("n%d", i))
		gs.AddNode(n)
		nodes[i] = n.ID
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.AddEdge(NewEdge(RelCalls, nodes[i], nodes[i+1]))
	}
}

func BenchmarkBatchFlush(b *testing.B) {
	gs := benchGS(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.Batch(func(bat *Batch) error {
			for j := 0; j < 1000; j++ {
				bat.AddNode(NewNode("benchrepo", LabelFunction, fmt.Sprintf("bn%d_%d", i, j)))
			}
			return nil
		})
	}
}

func BenchmarkGetAllOutEdges(b *testing.B) {
	gs := benchGS(b)
	src := NewNode("benchrepo", LabelFunction, "src")
	gs.AddNode(src)
	for i := 0; i < 100; i++ {
		tgt := NewNode("benchrepo", LabelFunction, fmt.Sprintf("tgt%d", i))
		gs.AddNode(tgt)
		gs.AddEdge(NewEdge(RelCalls, src.ID, tgt.ID))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.GetAllOutEdges(src.ID)
	}
}

func BenchmarkBFS(b *testing.B) {
	gs := benchGS(b)
	prev := NewNode("benchrepo", LabelFunction, "n0")
	gs.AddNode(prev)
	for i := 1; i < 100; i++ {
		cur := NewNode("benchrepo", LabelFunction, fmt.Sprintf("n%d", i))
		gs.AddNode(cur)
		gs.AddEdge(NewEdge(RelCalls, prev.ID, cur.ID))
		prev = cur
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.BFS(context.Background(), NewNode("benchrepo", LabelFunction, "n0").ID, TraverseOut, 10, nil)
	}
}

func BenchmarkShortestPath(b *testing.B) {
	gs := benchGS(b)
	nodes := make([]string, 50)
	for i := range nodes {
		n := NewNode("benchrepo", LabelFunction, fmt.Sprintf("sp%d", i))
		gs.AddNode(n)
		nodes[i] = n.ID
	}
	for i := 0; i < len(nodes)-1; i++ {
		gs.AddEdge(NewEdge(RelCalls, nodes[i], nodes[i+1]))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.ShortestPath(context.Background(), nodes[0], nodes[len(nodes)-1])
	}
}
