package enrich

import (
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestCSharpOverrideModifierDetection(t *testing.T) {
	explicit := &graph.GraphNode{Properties: graph.NodeProperties{
		Name:      "OnGetObject",
		Modifiers: "protected override",
	}}
	if !hasCSharpOverrideModifier(explicit) {
		t.Fatal("explicit C# override was not detected")
	}
	hidden := &graph.GraphNode{Properties: graph.NodeProperties{
		Name:      "Verify",
		Modifiers: "public",
	}}
	if hasCSharpOverrideModifier(hidden) {
		t.Fatal("same-named C# method without override modifier was accepted")
	}
}
