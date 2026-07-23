package enrich

import (
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestApplyCommunitiesSkipsMembershipsWithMissingEndpoints(t *testing.T) {
	kg := graph.NewKnowledgeGraph()
	kg.AddNode(&graph.GraphNode{
		ID:    "Function:main.go:run",
		Label: graph.LabelFunction,
		Properties: graph.NodeProperties{
			Name:     "run",
			FilePath: "main.go",
		},
	})

	ApplyCommunitiesToGraph(kg, CommunityDetectionResult{
		Communities: []CommunityNode{{
			ID:             "comm_0",
			HeuristicLabel: "main",
			SymbolCount:    2,
		}},
		Memberships: []CommunityMembership{
			{NodeID: "Function:main.go:run", CommunityID: "comm_0"},
			{NodeID: "Function:main.go:run", CommunityID: "comm_singleton"},
			{NodeID: "Function:missing.go:missing", CommunityID: "comm_0"},
		},
	})

	var relationships []*graph.GraphRelationship
	kg.ForEachRelationship(func(relationship *graph.GraphRelationship) {
		relationships = append(relationships, relationship)
	})

	if len(relationships) != 1 {
		t.Fatalf("expected one valid membership relationship, got %d", len(relationships))
	}
	if relationships[0].SourceID != "Function:main.go:run" ||
		relationships[0].TargetID != "comm_0" {
		t.Fatalf("unexpected membership relationship: %#v", relationships[0])
	}
}
