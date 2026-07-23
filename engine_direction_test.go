package codetrip

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func TestInternalDirectionAliases(t *testing.T) {
	tests := []struct {
		input TraverseDirection
		want  graph.TraverseDir
	}{
		{"", graph.TraverseOut},
		{"out", graph.TraverseOut},
		{"FORWARD", graph.TraverseOut},
		{"down", graph.TraverseOut},
		{"downstream", graph.TraverseOut},
		{"call", graph.TraverseOut},
		{"in", graph.TraverseIn},
		{"backward", graph.TraverseIn},
		{"upstream", graph.TraverseIn},
		{"both", graph.TraverseBoth},
		{"any", graph.TraverseBoth},
		{"bidirectional", graph.TraverseBoth},
	}
	for _, test := range tests {
		t.Run(string(test.input), func(t *testing.T) {
			got, err := internalDirection(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("direction %q = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestInternalDirectionRejectsUnknownValue(t *testing.T) {
	if _, err := internalDirection("sideways"); err == nil {
		t.Fatal("expected unsupported direction error")
	}
}
