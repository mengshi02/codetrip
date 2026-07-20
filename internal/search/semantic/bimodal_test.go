package semantic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	store "github.com/mengshi02/codetrip/internal/storage"
)

func TestBuildDescriptionText_Function(t *testing.T) {
	node := &graph.Node{
		ID:    "func1",
		Name:  "processOrder",
		Label: graph.LabelFunction,
	}
	node.Props.SetProp("params", "order Order")
	node.Props.SetProp("returnType", "error")

	ec := &embedContext{
		nameIndex: map[string]string{
			"func2": "validateOrder",
			"func3": "calculateTotal",
			"func4": "handleHTTPRequest",
		},
		callOut: map[string][]string{
			"func1": {"func2", "func3"},
		},
		callIn: map[string][]string{
			"func1": {"func4"},
		},
	}

	text := buildDescriptionText(node, ec)

	if !strings.Contains(text, "processOrder") {
		t.Errorf("description should contain function name, got: %s", text)
	}
	if !strings.Contains(text, "Calls: validateOrder, calculateTotal") {
		t.Errorf("description should contain Calls, got: %s", text)
	}
	if !strings.Contains(text, "Called by: handleHTTPRequest") {
		t.Errorf("description should contain Called by, got: %s", text)
	}
	if !strings.Contains(text, "Function") {
		t.Errorf("description should contain Type, got: %s", text)
	}
}

func TestBuildDescriptionText_MethodWithReceiver(t *testing.T) {
	node := &graph.Node{
		ID:    "method1",
		Name:  "ServeHTTP",
		Label: graph.LabelMethod,
	}
	node.Props.SetProp("receiver", "*Server")
	node.Props.SetProp("visibility", "public")

	ec := &embedContext{
		nameIndex: map[string]string{},
		callOut:   map[string][]string{},
		callIn:    map[string][]string{},
	}

	text := buildDescriptionText(node, ec)

	if !strings.Contains(text, "*Server.ServeHTTP") {
		t.Errorf("method description should contain receiver.name, got: %s", text)
	}
	if !strings.Contains(text, "Visibility: public") {
		t.Errorf("method description should contain visibility, got: %s", text)
	}
}

func TestBuildDescriptionText_Class(t *testing.T) {
	node := &graph.Node{
		ID:    "class1",
		Name:  "OrderService",
		Label: graph.LabelClass,
	}

	ec := &embedContext{
		nameIndex: map[string]string{},
		callOut:   map[string][]string{},
		callIn:    map[string][]string{},
	}

	text := buildDescriptionText(node, ec)

	if !strings.Contains(text, "Class OrderService") {
		t.Errorf("class description should contain 'Class Name', got: %s", text)
	}
}

func TestBuildDescriptionText_NoEdges(t *testing.T) {
	node := &graph.Node{
		ID:    "func1",
		Name:  "helper",
		Label: graph.LabelFunction,
	}

	ec := &embedContext{
		nameIndex: map[string]string{},
		callOut:   map[string][]string{},
		callIn:    map[string][]string{},
	}

	text := buildDescriptionText(node, ec)

	if strings.Contains(text, "Calls:") {
		t.Errorf("description should not contain Calls when no out-edges, got: %s", text)
	}
	if strings.Contains(text, "Called by:") {
		t.Errorf("description should not contain Called by when no in-edges, got: %s", text)
	}
}

func TestBuildEmbedContext(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "testrepo")

	gs.AddNode(&graph.Node{ID: "n1", Name: "processOrder", Label: graph.LabelFunction, Repo: "testrepo"})
	gs.AddNode(&graph.Node{ID: "n2", Name: "validateOrder", Label: graph.LabelFunction, Repo: "testrepo"})
	gs.AddNode(&graph.Node{ID: "n3", Name: "calculateTotal", Label: graph.LabelFunction, Repo: "testrepo"})
	gs.AddEdge(&graph.Edge{Source: "n1", Target: "n2", Type: graph.RelCalls})
	gs.AddEdge(&graph.Edge{Source: "n1", Target: "n3", Type: graph.RelCalls})

	ec, err := buildEmbedContext(gs)
	if err != nil {
		t.Fatalf("buildEmbedContext failed: %v", err)
	}

	if ec.nameIndex["n1"] != "processOrder" {
		t.Errorf("nameIndex[n1] = %q, want processOrder", ec.nameIndex["n1"])
	}
	if len(ec.callOut["n1"]) != 2 {
		t.Errorf("callOut[n1] has %d entries, want 2", len(ec.callOut["n1"]))
	}
	if len(ec.callIn["n2"]) != 1 || ec.callIn["n2"][0] != "n1" {
		t.Errorf("callIn[n2] = %v, want [n1]", ec.callIn["n2"])
	}
}

func TestBuildNodeSignature(t *testing.T) {
	tests := []struct {
		name     string
		node     *graph.Node
		contains string
	}{
		{
			name: "function with signature",
			node: func() *graph.Node {
				n := &graph.Node{Name: "doWork", Label: graph.LabelFunction}
				n.Props.SetProp("signature", "(x int) error")
				return n
			}(),
			contains: "doWork(x int) error",
		},
		{
			name: "method with receiver",
			node: func() *graph.Node {
				n := &graph.Node{Name: "Run", Label: graph.LabelMethod}
				n.Props.SetProp("receiver", "*Worker")
				return n
			}(),
			contains: "*Worker.Run",
		},
		{
			name:     "class",
			node:     &graph.Node{Name: "Service", Label: graph.LabelClass},
			contains: "Class Service",
		},
		{
			name:     "interface",
			node:     &graph.Node{Name: "Handler", Label: graph.LabelInterface},
			contains: "Interface Handler",
		},
		{
			name:     "struct",
			node:     &graph.Node{Name: "Config", Label: graph.LabelStruct},
			contains: "Struct Config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := buildNodeSignature(tt.node)
			if !strings.Contains(sig, tt.contains) {
				t.Errorf("signature %q should contain %q", sig, tt.contains)
			}
		})
	}
}

func BenchmarkBuildDescriptionText(b *testing.B) {
	node := &graph.Node{
		ID:    "func1",
		Name:  "processOrder",
		Label: graph.LabelFunction,
	}
	node.Props.SetProp("params", "order Order")
	node.Props.SetProp("returnType", "error")

	ec := &embedContext{
		nameIndex: map[string]string{},
		callOut:   map[string][]string{"func1": {}},
		callIn:    map[string][]string{"func1": {}},
	}
	for i := 0; i < 10; i++ {
		targetID := "target_" + string(rune('A'+i))
		ec.nameIndex[targetID] = "call_" + string(rune('A'+i))
		ec.callOut["func1"] = append(ec.callOut["func1"], targetID)
	}
	for i := 0; i < 5; i++ {
		srcID := "source_" + string(rune('A'+i))
		ec.nameIndex[srcID] = "caller_" + string(rune('A'+i))
		ec.callIn["func1"] = append(ec.callIn["func1"], srcID)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildDescriptionText(node, ec)
	}
}

func BenchmarkBuildEmbedContext(b *testing.B) {
	dir := b.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		b.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	gs := graph.NewGraphStore(s, "benchrepo")
	for i := 0; i < 1000; i++ {
		nodeID := fmt.Sprintf("n_%d", i)
		gs.AddNode(&graph.Node{ID: nodeID, Name: fmt.Sprintf("func%d", i), Label: graph.LabelFunction, Repo: "benchrepo"})
		if i > 0 {
			prevID := fmt.Sprintf("n_%d", i-1)
			gs.AddEdge(&graph.Edge{Source: nodeID, Target: prevID, Type: graph.RelCalls})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buildEmbedContext(gs)
	}
}
