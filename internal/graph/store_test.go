package graph

import (
	"fmt"
	"os"
	"testing"

	store "github.com/mengshi02/codetrip/internal/store"
)

func openTestGS(t *testing.T) *GraphStore {
	t.Helper()
	dir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg := store.DefaultConfig(dir)
	cfg.CacheSize = 8 << 20
	store, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewGraphStore(store, "testrepo")
}

// addN 添加节点并同步刷盘
func addN(gs *GraphStore, n *Node) error {
	if err := gs.AddNode(n); err != nil {
		return err
	}
	return gs.store.Flush()
}

// addE 添加边并同步刷盘
func addE(gs *GraphStore, e *Edge) error {
	if err := gs.AddEdge(e); err != nil {
		return err
	}
	return gs.store.Flush()
}

// flushGS 同步刷盘
func flushGS(gs *GraphStore) error {
	return gs.store.Flush()
}

func TestAddGetNode(t *testing.T) {
	gs := openTestGS(t)
	n := NewNode("testrepo", LabelFunction, "foo").WithFile("main.go").WithProp("line", 10)
	if err := addN(gs, n); err != nil {
		t.Fatal(err)
	}
	got, err := gs.GetNode(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "foo" || got.Label != LabelFunction || got.FilePath != "main.go" {
		t.Errorf("got = %+v", got)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	gs := openTestGS(t)
	if _, err := gs.GetNode("nope"); err == nil {
		t.Error("expected error")
	}
}

func TestGetNodesByLabel(t *testing.T) {
	gs := openTestGS(t)
	for i := 0; i < 5; i++ {
		addN(gs, NewNode("testrepo", LabelFunction, fmt.Sprintf("f%d", i)))
	}
	for i := 0; i < 3; i++ {
		addN(gs, NewNode("testrepo", LabelClass, fmt.Sprintf("c%d", i)))
	}
	funcs, _ := gs.GetNodesByLabel("testrepo", "Function")
	if len(funcs) != 5 {
		t.Errorf("got %d, want 5", len(funcs))
	}
}

func TestGetNodesByName(t *testing.T) {
	gs := openTestGS(t)
	addN(gs, NewNode("testrepo", LabelFunction, "foo"))
	addN(gs, NewNode("testrepo", LabelClass, "foo"))
	nodes, _ := gs.GetNodesByName("testrepo", "foo")
	if len(nodes) != 2 {
		t.Errorf("got %d, want 2", len(nodes))
	}
}

func TestGetNodesByFile(t *testing.T) {
	gs := openTestGS(t)
	addN(gs, NewNode("testrepo", LabelFunction, "a").WithFile("main.go"))
	addN(gs, NewNode("testrepo", LabelFunction, "b").WithFile("main.go"))
	addN(gs, NewNode("testrepo", LabelFunction, "c").WithFile("util.go"))
	nodes, _ := gs.GetNodesByFile("testrepo", "main.go")
	if len(nodes) != 2 {
		t.Errorf("got %d, want 2", len(nodes))
	}
}

func TestDeleteNode(t *testing.T) {
	gs := openTestGS(t)
	n := NewNode("testrepo", LabelFunction, "foo")
	addN(gs, n)
	if err := gs.DeleteNode(n.ID); err != nil {
		t.Fatal(err)
	}
	flushGS(gs)
	if _, err := gs.GetNode(n.ID); err == nil {
		t.Error("should be deleted")
	}
}

func TestGetAllNodes(t *testing.T) {
	gs := openTestGS(t)
	for i := 0; i < 20; i++ {
		addN(gs, NewNode("testrepo", LabelFunction, fmt.Sprintf("gn%d", i)))
	}
	if n := len(gs.GetAllNodes("testrepo", 10)); n != 10 {
		t.Errorf("limit=10 got %d", n)
	}
	if n := len(gs.GetAllNodes("testrepo", 0)); n != 20 {
		t.Errorf("limit=0 got %d", n)
	}
}

func TestIterNodes(t *testing.T) {
	gs := openTestGS(t)
	for i := 0; i < 5; i++ {
		addN(gs, NewNode("testrepo", LabelFunction, fmt.Sprintf("it%d", i)))
	}
	iter := gs.IterNodes("testrepo")
	defer iter.Close()
	count := 0
	for iter.Next() {
		count++
	}
	if count != 5 {
		t.Errorf("got %d, want 5", count)
	}
}

func TestGetIndexStats(t *testing.T) {
	gs := openTestGS(t)
	addN(gs, NewNode("testrepo", LabelFunction, "a").WithFile("f.go"))
	addN(gs, NewNode("testrepo", LabelClass, "b"))
	stats, err := gs.GetIndexStats("testrepo")
	if err != nil {
		t.Fatal(err)
	}
	if stats.LabelCount < 2 {
		t.Errorf("LabelCount = %d, want >= 2", stats.LabelCount)
	}
}

func TestRepo(t *testing.T) {
	gs := openTestGS(t)
	if gs.Repo() != "testrepo" {
		t.Errorf("Repo = %s, want testrepo", gs.Repo())
	}
}
