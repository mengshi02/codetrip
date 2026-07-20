package graph

import (
	"testing"

	ingestgraph "github.com/mengshi02/codetrip/internal/model"
	store "github.com/mengshi02/codetrip/internal/storage"
)

func TestImportKnowledgeGraphPreservesNodesAndEdges(t *testing.T) {
	db, err := store.Open(store.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	source := ingestgraph.NewKnowledgeGraph()
	line := 7
	source.AddNode(&ingestgraph.GraphNode{
		ID: "Function:main.go:run", Label: ingestgraph.LabelFunction,
		Properties: ingestgraph.NodeProperties{Name: "run", FilePath: "main.go", Language: "go", StartLine: &line},
	})
	source.AddNode(&ingestgraph.GraphNode{
		ID: "Function:main.go:work", Label: ingestgraph.LabelFunction,
		Properties: ingestgraph.NodeProperties{Name: "work", FilePath: "main.go", Language: "go"},
	})
	source.AddRelationship(&ingestgraph.GraphRelationship{
		ID: "CALLS:run->work", SourceID: "Function:main.go:run", TargetID: "Function:main.go:work",
		Type: ingestgraph.RelCALLS, Confidence: 0.95, Reason: "resolved call",
	})

	graphStore := NewGraphStore(db, "fixture")
	if err := graphStore.ImportKnowledgeGraph(source); err != nil {
		t.Fatal(err)
	}
	node, err := graphStore.GetNode("Function:main.go:run")
	if err != nil {
		t.Fatal(err)
	}
	if node.FilePath != "main.go" || node.GetPropString("language") != "go" || node.GetPropInt("startLine") != 7 {
		t.Fatalf("unexpected node: %#v", node)
	}
	edges, err := graphStore.GetOutEdges(node.ID, "CALLS")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("unexpected edges: %#v", edges)
	}
	if edges[0].Target != "Function:main.go:work" {
		t.Fatalf("unexpected edge target: %q", edges[0].Target)
	}
	stored, err := graphStore.GetEdge("CALLS:run->work")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Confidence() < 0.949 || stored.Confidence() > 0.951 || stored.GetPropString("reason") != "resolved call" {
		t.Fatalf("unexpected stored edge: confidence=%v props=%#v", stored.Confidence(), stored.Props)
	}
}

func TestEdgeStorageIsIsolatedByRepository(t *testing.T) {
	db, err := store.Open(store.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := NewGraphStore(db, "first")
	second := NewGraphStore(db, "second")
	for _, item := range []struct {
		store *GraphStore
		note  string
	}{{first, "first edge"}, {second, "second edge"}} {
		edge := NewEdge(RelCalls, "same-source", "same-target").WithID("same-edge")
		edge.WithProp("reason", item.note)
		if err := item.store.AddEdge(edge); err != nil {
			t.Fatal(err)
		}
	}

	firstEdge, err := first.GetEdge("same-edge")
	if err != nil {
		t.Fatal(err)
	}
	secondEdge, err := second.GetEdge("same-edge")
	if err != nil {
		t.Fatal(err)
	}
	if firstEdge.Repo != "first" || firstEdge.GetPropString("reason") != "first edge" {
		t.Fatalf("unexpected first edge: %#v", firstEdge)
	}
	if secondEdge.Repo != "second" || secondEdge.GetPropString("reason") != "second edge" {
		t.Fatalf("unexpected second edge: %#v", secondEdge)
	}
}
