package search

import (
	"testing"

	"os"
	"path/filepath"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

func openTestHybrid(t *testing.T) (*HybridSearch, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "hybridtest-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bm25, err := NewBM25IndexWithDir(dir, "testrepo", s)
	if err != nil {
		s.Close()
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	hs := NewHybridSearch(bm25, nil) // no vector search for unit test
	cleanup := func() {
		bm25.Close()
		s.Close()
		os.RemoveAll(dir)
	}
	return hs, cleanup
}

func TestHybridSearch_BM25Only(t *testing.T) {
	hs, cleanup := openTestHybrid(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "searchUser").WithID("hybrid-node-1")
	node.FilePath = "pkg/user/service.go"
	if err := hs.bm25.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	if err := hs.bm25.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	result, err := hs.Search("searchUser", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 {
		t.Error("expected hybrid search results")
	}
}

func TestHybridSearch_RRFScore(t *testing.T) {
	hs, cleanup := openTestHybrid(t)
	defer cleanup()

	n1 := graph.NewNode("testrepo", graph.LabelFunction, "createOrder").WithID("rrf-node-1")
	n1.FilePath = "pkg/order/service.go"
	n2 := graph.NewNode("testrepo", graph.LabelFunction, "createPayment").WithID("rrf-node-2")
	n2.FilePath = "pkg/payment/service.go"
	if err := hs.bm25.BatchIndex([]*graph.Node{n1, n2}); err != nil {
		t.Fatal(err)
	}

	if err := hs.bm25.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	result, err := hs.Search("createOrder", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 {
		t.Error("expected results")
	}
	// Results should be sorted by RRF score descending
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i].Score > result.Results[i-1].Score {
			t.Errorf("results not sorted by RRF score at index %d", i)
		}
	}
}

func TestHybridSearch_Limit(t *testing.T) {
	hs, cleanup := openTestHybrid(t)
	defer cleanup()

	nodes := make([]*graph.Node, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = graph.NewNode("testrepo", graph.LabelFunction, "func"+string(rune('A'+i)))
	}
	if err := hs.bm25.BatchIndex(nodes); err != nil {
		t.Fatal(err)
	}

	if err := hs.bm25.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	result, err := hs.Search("func", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) > 3 {
		t.Errorf("got %d results, want at most 3", len(result.Results))
	}
}
