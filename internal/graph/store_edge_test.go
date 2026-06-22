package graph

import (
	"fmt"
	"testing"
)

func TestAddGetEdge(t *testing.T) {
	gs := openTestGS(t)
	n1 := NewNode("testrepo", LabelFunction, "a")
	n2 := NewNode("testrepo", LabelFunction, "b")
	addN(gs, n1)
	addN(gs, n2)
	e := NewEdge(RelCalls, n1.ID, n2.ID).WithProp("confidence", 0.9)
	addE(gs, e)
	got, err := gs.GetEdge(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != RelCalls || got.Source != n1.ID || got.Target != n2.ID {
		t.Errorf("got = %+v", got)
	}
}

func TestGetAllOutEdges(t *testing.T) {
	gs := openTestGS(t)
	n1 := NewNode("testrepo", LabelFunction, "a")
	n2 := NewNode("testrepo", LabelFunction, "b")
	n3 := NewNode("testrepo", LabelFunction, "c")
	addN(gs, n1)
	addN(gs, n2)
	addN(gs, n3)
	addE(gs, NewEdge(RelCalls, n1.ID, n2.ID).WithProp("w", 0.8))
	addE(gs, NewEdge(RelCalls, n1.ID, n3.ID).WithProp("w", 0.5))
	edges, err := gs.GetAllOutEdges(n1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d, want 2", len(edges))
	}
	for _, e := range edges {
		if e.Props.IsEmpty() {
			t.Error("Props should not be nil (方案A)")
		}
	}
}

func TestGetAllInEdges(t *testing.T) {
	gs := openTestGS(t)
	n1 := NewNode("testrepo", LabelFunction, "a")
	n2 := NewNode("testrepo", LabelFunction, "b")
	addN(gs, n1)
	addN(gs, n2)
	addE(gs, NewEdge(RelCalls, n1.ID, n2.ID))
	edges, _ := gs.GetAllInEdges(n2.ID)
	if len(edges) != 1 {
		t.Fatalf("got %d, want 1", len(edges))
	}
}

func TestDeleteEdge(t *testing.T) {
	gs := openTestGS(t)
	n1 := NewNode("testrepo", LabelFunction, "a")
	n2 := NewNode("testrepo", LabelFunction, "b")
	addN(gs, n1)
	addN(gs, n2)
	e := NewEdge(RelCalls, n1.ID, n2.ID)
	addE(gs, e)
	gs.DeleteEdge(e.ID)
	flushGS(gs)
	if _, err := gs.GetEdge(e.ID); err == nil {
		t.Error("edge should be deleted")
	}
	edges, _ := gs.GetAllOutEdges(n1.ID)
	if len(edges) != 0 {
		t.Errorf("adj index not cleaned: %d edges", len(edges))
	}
}

func TestBatch_WithEdges(t *testing.T) {
	gs := openTestGS(t)
	err := gs.Batch(func(b *Batch) error {
		n1 := NewNode("testrepo", LabelFunction, "ba")
		n2 := NewNode("testrepo", LabelFunction, "bb")
		b.AddNode(n1)
		b.AddNode(n2)
		b.AddEdge(NewEdge(RelCalls, n1.ID, n2.ID))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	flushGS(gs)
	nodes := gs.GetAllNodes("testrepo", 100)
	if len(nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(nodes))
	}
}

func TestBatchFlush1K(t *testing.T) {
	gs := openTestGS(t)
	gs.Batch(func(b *Batch) error {
		for i := 0; i < 1000; i++ {
			b.AddNode(NewNode("testrepo", LabelFunction, fmt.Sprintf("k%d", i)))
		}
		return nil
	})
	flushGS(gs)
	nodes := gs.GetAllNodes("testrepo", 0)
	if len(nodes) != 1000 {
		t.Errorf("got %d, want 1000", len(nodes))
	}
}