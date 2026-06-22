package graph

import (
	"testing"
)

func TestNewNode(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo")
	if n.Repo != "repo1" {
		t.Errorf("Repo = %s, want repo1", n.Repo)
	}
	if n.Label != LabelFunction {
		t.Errorf("Label = %s, want Function", n.Label)
	}
	if n.Name != "foo" {
		t.Errorf("Name = %s, want foo", n.Name)
	}
	if n.ID != "" {
		t.Errorf("ID should be empty, got %s", n.ID)
	}
}

func TestNode_WithID(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithID("id123")
	if n.ID != "id123" {
		t.Errorf("ID = %s, want id123", n.ID)
	}
}

func TestNode_WithFile(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithFile("main.go")
	if n.FilePath != "main.go" {
		t.Errorf("FilePath = %s, want main.go", n.FilePath)
	}
}

func TestNode_WithProp(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithProp("key", "val")
	if n.GetProp("key", "") != "val" {
		t.Error("WithProp/GetProp mismatch")
	}
}

func TestNode_GetPropString(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithProp("name", "bar")
	if n.GetPropString("name") != "bar" {
		t.Error("GetPropString mismatch")
	}
	if n.GetPropString("missing") != "" {
		t.Error("GetPropString should return empty for missing")
	}
}

func TestNode_GetPropInt(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithProp("line", 42)
	if n.GetPropInt("line") != 42 {
		t.Errorf("GetPropInt = %d, want 42", n.GetPropInt("line"))
	}
}

func TestNode_GetPropBool(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithProp("exported", true)
	if !n.GetPropBool("exported") {
		t.Error("GetPropBool should be true")
	}
	if n.GetPropBool("missing") {
		t.Error("GetPropBool should be false for missing")
	}
}

func TestNode_GetPropFloat64(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithProp("score", 0.95)
	if n.GetPropFloat64("score") != 0.95 {
		t.Errorf("GetPropFloat64 = %f, want 0.95", n.GetPropFloat64("score"))
	}
}

func TestNode_UID(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithFile("main.go")
	uid := n.UID()
	expected := "repo1:main.go:Function:foo"
	if uid != expected {
		t.Errorf("UID = %s, want %s", uid, expected)
	}
}

func TestNode_Key(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithID("id123")
	key := n.Key()
	expected := "n:repo1:id123"
	if key != expected {
		t.Errorf("Key = %s, want %s", key, expected)
	}
}

func TestNode_SymbolDescription(t *testing.T) {
	n := NewNode("repo1", LabelFunction, "foo").WithFile("main.go")
	desc := n.SymbolDescription()
	if desc != "Function foo main.go" {
		t.Errorf("SymbolDescription = %s, want 'Function foo main.go'", desc)
	}
}

func TestLabel_IsSymbol(t *testing.T) {
	symbolLabels := []Label{LabelFunction, LabelClass, LabelInterface, LabelMethod, LabelStruct}
	for _, l := range symbolLabels {
		if !l.IsSymbol() {
			t.Errorf("%s should be a symbol", l)
		}
	}
	nonSymbolLabels := []Label{LabelFile, LabelProject, LabelCommunity}
	for _, l := range nonSymbolLabels {
		if l.IsSymbol() {
			t.Errorf("%s should not be a symbol", l)
		}
	}
}