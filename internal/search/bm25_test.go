package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

func openTestBM25(t *testing.T) (*BM25Index, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "bm25test-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	store, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	idx := NewBM25Index(store, "testrepo")
	cleanup := func() {
		idx.Close()
		store.Close()
		os.RemoveAll(dir)
	}
	return idx, cleanup
}

func TestBM25IndexNode(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "handleRequest").WithID("node-1")
	node.FilePath = "pkg/handler.go"
	node.Props = graph.NodePropsFromMap(map[string]any{"startLine": 10, "endLine": 20})

	if err := idx.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocumentCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("document count = %d, want 1", count)
	}
}

func TestBM25BatchIndex(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	nodes := []*graph.Node{
		graph.NewNode("testrepo", graph.LabelFunction, "funcA").WithID("node-a"),
		graph.NewNode("testrepo", graph.LabelFunction, "funcB").WithID("node-b"),
		graph.NewNode("testrepo", graph.LabelVariable, "varX").WithID("node-x"),
	}
	if err := idx.BatchIndex(nodes); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocumentCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("document count = %d, want 3", count)
	}
}

func TestBM25Search(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "handleRequest").WithID("search-node-1")
	node.FilePath = "pkg/handler.go"
	if err := idx.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search("handleRequest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected search results")
	}
}

func TestBM25Search_MultiField(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	n1 := graph.NewNode("testrepo", graph.LabelFunction, "processOrder").WithID("multi-node-1")
	n1.FilePath = "pkg/order/service.go"
	n2 := graph.NewNode("testrepo", graph.LabelFunction, "processPayment").WithID("multi-node-2")
	n2.FilePath = "pkg/payment/service.go"

	if err := idx.BatchIndex([]*graph.Node{n1, n2}); err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search("processOrder", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected results for processOrder")
	}
}

func TestBM25DeleteNode(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "tempFunc").WithID("delete-node-1")
	if err := idx.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	if err := idx.DeleteNode(node.ID); err != nil {
		t.Fatal(err)
	}

	count, _ := idx.DocumentCount()
	if count != 0 {
		t.Errorf("document count after delete = %d, want 0", count)
	}
}

// TestBM25DeleteDocuments verifies batch deletion of documents by node IDs.
// This is the API used during incremental re-indexing to remove stale BM25 documents.
func TestBM25DeleteDocuments(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	// Index 3 documents with searchable names
	nodes := []*graph.Node{
		graph.NewNode("testrepo", graph.LabelFunction, "handleRequest").WithID("batch-del-a"),
		graph.NewNode("testrepo", graph.LabelFunction, "processOrder").WithID("batch-del-b"),
		graph.NewNode("testrepo", graph.LabelVariable, "connectionTimeout").WithID("batch-del-c"),
	}
	nodes[0].FilePath = "pkg/handler.go"
	nodes[1].FilePath = "pkg/order.go"
	nodes[2].FilePath = "pkg/timeout.go"

	if err := idx.BatchIndex(nodes); err != nil {
		t.Fatal(err)
	}

	// Verify all indexed
	count, _ := idx.DocumentCount()
	if count != 3 {
		t.Fatalf("document count = %d, want 3", count)
	}

	// Verify search finds handleRequest
	results, err := idx.Search("handleRequest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for handleRequest before deletion")
	}

	// Batch delete 2 of 3 documents
	if err := idx.DeleteDocuments([]string{"batch-del-a", "batch-del-b"}); err != nil {
		t.Fatal(err)
	}

	// Verify only 1 document remains
	countAfter, _ := idx.DocumentCount()
	if countAfter != 1 {
		t.Errorf("document count after batch delete = %d, want 1", countAfter)
	}

	// Verify search no longer finds handleRequest
	resultsAfter, err := idx.Search("handleRequest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resultsAfter) != 0 {
		t.Errorf("handleRequest should not be found after deletion, got %d results", len(resultsAfter))
	}

	// Verify connectionTimeout still searchable
	resultsC, err := idx.Search("connectionTimeout", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(resultsC) == 0 {
		t.Error("connectionTimeout should still be found after deletion of other docs")
	}
}

func TestBM25EmptySearch(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	results, err := idx.Search("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected empty results for empty query")
	}
}

func TestBM25Repo(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	if idx.Repo() != "testrepo" {
		t.Errorf("repo = %q, want testrepo", idx.Repo())
	}
}

func TestBM25BatchIndexEmpty(t *testing.T) {
	idx, cleanup := openTestBM25(t)
	defer cleanup()

	if err := idx.BatchIndex(nil); err != nil {
		t.Error("nil batch should not error")
	}
	if err := idx.BatchIndex([]*graph.Node{}); err != nil {
		t.Error("empty batch should not error")
	}
}
