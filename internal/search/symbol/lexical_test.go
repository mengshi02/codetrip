package symbol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	store "github.com/mengshi02/codetrip/internal/store"
)

func openTestLexical(t *testing.T) (*LexicalIndex, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "lexical-test-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := NewLexicalIndexWithDir(dir, "testrepo", s)
	if err != nil {
		s.Close()
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	cleanup := func() {
		idx.Close()
		s.Close()
		os.RemoveAll(dir)
	}
	return idx, cleanup
}

func TestLexicalIndexNode(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "handleRequest").WithID("node-1")
	node.FilePath = "pkg/handler.go"
	node.Props = graph.NodePropsFromMap(map[string]any{"startLine": 10, "endLine": 20})

	if err := idx.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
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

func TestLexicalBatchIndex(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	nodes := []*graph.Node{
		graph.NewNode("testrepo", graph.LabelFunction, "funcA").WithID("node-a"),
		graph.NewNode("testrepo", graph.LabelFunction, "funcB").WithID("node-b"),
		graph.NewNode("testrepo", graph.LabelVariable, "varX").WithID("node-x"),
	}
	if err := idx.BatchIndex(nodes); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
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

func TestLexicalSearch(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	node := graph.NewNode("testrepo", graph.LabelFunction, "handleRequest").WithID("search-node-1")
	node.FilePath = "pkg/handler.go"
	if err := idx.IndexNode(node); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
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

func TestLexicalSearch_MultiField(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	n1 := graph.NewNode("testrepo", graph.LabelFunction, "processOrder").WithID("multi-node-1")
	n1.FilePath = "pkg/order/service.go"
	n2 := graph.NewNode("testrepo", graph.LabelFunction, "processPayment").WithID("multi-node-2")
	n2.FilePath = "pkg/payment/service.go"

	if err := idx.BatchIndex([]*graph.Node{n1, n2}); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
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

func TestLexicalEmptySearch(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	if err := idx.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected empty results for empty query")
	}
}

func TestLexicalRepo(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	if idx.Repo() != "testrepo" {
		t.Errorf("repo = %q, want testrepo", idx.Repo())
	}
}

func TestLexicalSearch_DatabaseConnection(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	// Simulate the e2e test scenario: search for "DatabaseConnection"
	nodes := []*graph.Node{
		graph.NewNode("testrepo", graph.LabelStruct, "DatabaseConnection").WithID("db-conn-1"),
		graph.NewNode("testrepo", graph.LabelFunction, "NewDatabaseConnection").WithID("new-db-conn-1"),
		graph.NewNode("testrepo", graph.LabelMethod, "Connect").WithID("connect-1"),
		graph.NewNode("testrepo", graph.LabelFunction, "UserService").WithID("user-svc-1"),
	}
	nodes[0].FilePath = "pkg/main.go"
	nodes[1].FilePath = "pkg/main.go"
	nodes[2].FilePath = "pkg/main.go"
	nodes[3].FilePath = "pkg/service/user_service.go"

	if err := idx.BatchIndex(nodes); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	// Search for "DatabaseConnection"
	t.Logf("preparedQuery for 'DatabaseConnection': '%s'", prepareSearchText("DatabaseConnection"))
	t.Logf("preparedQuery for 'Connect': '%s'", prepareSearchText("Connect"))
	t.Logf("preparedQuery for 'UserService': '%s'", prepareSearchText("UserService"))

	results1, err := idx.Search("DatabaseConnection", 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'DatabaseConnection': %d results", len(results1))
	for _, r := range results1 {
		t.Logf("  name=%s label=%s file=%s score=%.4f", r.Name, r.Label, r.FilePath, r.Score)
	}
	if len(results1) == 0 {
		t.Error("expected results for DatabaseConnection")
	}

	// Search for "Connect"
	results2, err := idx.Search("Connect", 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'Connect': %d results", len(results2))
	for _, r := range results2 {
		t.Logf("  name=%s label=%s file=%s score=%.4f", r.Name, r.Label, r.FilePath, r.Score)
	}
	if len(results2) == 0 {
		t.Error("expected results for Connect")
	}

	// Search for "UserService"
	results3, err := idx.Search("UserService", 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'UserService': %d results", len(results3))
	for _, r := range results3 {
		t.Logf("  name=%s label=%s file=%s score=%.4f", r.Name, r.Label, r.FilePath, r.Score)
	}
	if len(results3) == 0 {
		t.Error("expected results for UserService")
	}
}

// TestLexicalSearch_AfterCloseReopen simulates the e2e flow: index → close → reopen → search
func TestLexicalSearch_AfterCloseReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "lexical-reopen-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	s, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Phase 1: Create index, batch index, finalize, close
	idx, err := NewLexicalIndexWithDir(dir, "testrepo", s)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []*graph.Node{
		graph.NewNode("testrepo", graph.LabelStruct, "DatabaseConnection").WithID("db-conn-1"),
		graph.NewNode("testrepo", graph.LabelFunction, "NewDatabaseConnection").WithID("new-db-conn-1"),
		graph.NewNode("testrepo", graph.LabelMethod, "Connect").WithID("connect-1"),
		graph.NewNode("testrepo", graph.LabelFunction, "UserService").WithID("user-svc-1"),
	}
	nodes[0].FilePath = "pkg/main.go"
	nodes[1].FilePath = "pkg/main.go"
	nodes[2].FilePath = "pkg/main.go"
	nodes[3].FilePath = "pkg/service/user_service.go"

	if err := idx.BatchIndexChunked(nodes, 1000); err != nil {
		t.Fatal(err)
	}

	if err := idx.FinalizeBuild(); err != nil {
		t.Fatal(err)
	}

	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}

	// Phase 2: Reopen and search (simulates getLexicalIndex)
	idx2, err := NewLexicalIndexWithDir(dir, "testrepo", s)
	if err != nil {
		t.Fatal(err)
	}
	defer idx2.Close()

	// Verify document count
	count, err := idx2.DocumentCount()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Document count after reopen: %d", count)
	if count != 4 {
		t.Fatalf("expected 4 documents after reopen, got %d", count)
	}

	// Search for "DatabaseConnection"
	results1, err := idx2.Search("DatabaseConnection", 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'DatabaseConnection' after reopen: %d results", len(results1))
	for _, r := range results1 {
		t.Logf("  name=%s label=%s file=%s score=%.4f", r.Name, r.Label, r.FilePath, r.Score)
	}
	if len(results1) == 0 {
		t.Error("expected results for DatabaseConnection after reopen")
	}

	// Search for "Connect"
	results2, err := idx2.Search("Connect", 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'Connect' after reopen: %d results", len(results2))
	for _, r := range results2 {
		t.Logf("  name=%s label=%s file=%s score=%.4f", r.Name, r.Label, r.FilePath, r.Score)
	}
	if len(results2) == 0 {
		t.Error("expected results for Connect after reopen")
	}
}

func TestLexicalBatchIndexEmpty(t *testing.T) {
	idx, cleanup := openTestLexical(t)
	defer cleanup()

	if err := idx.BatchIndex(nil); err != nil {
		t.Error("nil batch should not error")
	}
	if err := idx.BatchIndex([]*graph.Node{}); err != nil {
		t.Error("empty batch should not error")
	}
}
