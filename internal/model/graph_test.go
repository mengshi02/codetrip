package model

import "testing"

func TestRemoveDanglingRelationships(t *testing.T) {
	graph := NewKnowledgeGraph()
	graph.AddNode(&GraphNode{ID: "a"})
	graph.AddNode(&GraphNode{ID: "b"})
	graph.AddRelationship(&GraphRelationship{ID: "valid", SourceID: "a", TargetID: "b"})
	graph.AddRelationship(&GraphRelationship{ID: "missing-source", SourceID: "x", TargetID: "b"})
	graph.AddRelationship(&GraphRelationship{ID: "missing-target", SourceID: "a", TargetID: "y"})

	if removed := graph.RemoveDanglingRelationships(); removed != 2 {
		t.Fatalf("removed %d dangling relationships, want 2", removed)
	}
	if graph.RelationshipCount() != 1 || graph.Relationships()[0].ID != "valid" {
		t.Fatalf("remaining relationships = %#v", graph.Relationships())
	}
}
