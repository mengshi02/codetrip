package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ findCaptureTextInMatch Tests ============

func TestFindCaptureTextInMatch_Found(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "Hello"},
		{MatchIndex: 0, Name: "fn.def", Text: ""},
		{MatchIndex: 1, Name: "fn.name", Text: "World"},
	}
	result := findCaptureTextInMatch(captures, 0, "fn.name")
	if result != "Hello" {
		t.Errorf("expected Hello, got %s", result)
	}
}

func TestFindCaptureTextInMatch_DifferentMatch(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "Hello"},
		{MatchIndex: 1, Name: "fn.name", Text: "World"},
	}
	result := findCaptureTextInMatch(captures, 1, "fn.name")
	if result != "World" {
		t.Errorf("expected World, got %s", result)
	}
}

func TestFindCaptureTextInMatch_NotFound(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "Hello"},
	}
	result := findCaptureTextInMatch(captures, 0, "method.name")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestFindCaptureTextInMatch_WrongMatchIndex(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "Hello"},
	}
	result := findCaptureTextInMatch(captures, 99, "fn.name")
	if result != "" {
		t.Errorf("expected empty string for non-existent match, got %s", result)
	}
}

// ============ findCaptureText Tests ============

func TestFindCaptureText_Found(t *testing.T) {
	captures := []pipeline.LangCapture{
		{Name: "type.name", Text: "Config"},
		{Name: "fn.name", Text: "New"},
	}
	result := findCaptureText(captures, "fn.name")
	if result != "New" {
		t.Errorf("expected New, got %s", result)
	}
}

func TestFindCaptureText_NotFound(t *testing.T) {
	captures := []pipeline.LangCapture{
		{Name: "fn.name", Text: "New"},
	}
	result := findCaptureText(captures, "missing")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

// ============ capturesInMatch Tests ============

func TestCapturesInMatch(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "A"},
		{MatchIndex: 0, Name: "fn.def", Text: ""},
		{MatchIndex: 1, Name: "fn.name", Text: "B"},
		{MatchIndex: 1, Name: "fn.def", Text: ""},
		{MatchIndex: 2, Name: "fn.name", Text: "C"},
	}
	result := capturesInMatch(captures, 1)
	if len(result) != 2 {
		t.Fatalf("expected 2 captures, got %d", len(result))
	}
	if result[0].Text != "B" {
		t.Errorf("expected first capture text=B, got %s", result[0].Text)
	}
}

func TestCapturesInMatch_Empty(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "A"},
	}
	result := capturesInMatch(captures, 99)
	if len(result) != 0 {
		t.Errorf("expected 0 captures for non-existent match, got %d", len(result))
	}
}

// ============ captureByName Tests ============

func TestCaptureByName(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "A"},
		{MatchIndex: 1, Name: "fn.name", Text: "B"},
		{MatchIndex: 0, Name: "fn.def", Text: ""},
	}
	result := captureByName(captures, "fn.name")
	if len(result) != 2 {
		t.Fatalf("expected 2 captures, got %d", len(result))
	}
}

// ============ captureByNameInMatch Tests ============

func TestCaptureByNameInMatch(t *testing.T) {
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, Name: "fn.name", Text: "A"},
		{MatchIndex: 1, Name: "fn.name", Text: "B"},
	}
	result := captureByNameInMatch(captures, 1, "fn.name")
	if len(result) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(result))
	}
	if result[0].Text != "B" {
		t.Errorf("expected B, got %s", result[0].Text)
	}
}

// ============ buildScopeParentIDs Tests ============

func TestBuildScopeParentIDs_Empty(t *testing.T) {
	buildScopeParentIDs(nil) // should not panic
}

func TestBuildScopeParentIDs_SingleRoot(t *testing.T) {
	scopes := []*pipeline.ScopeInfo{
		{ID: "s1", Kind: "class", Name: "Foo", StartLine: 1, EndLine: 50},
	}
	buildScopeParentIDs(scopes)
	if scopes[0].ParentID != "" {
		t.Errorf("root scope should have empty ParentID, got %s", scopes[0].ParentID)
	}
}

func TestBuildScopeParentIDs_FlatSiblings(t *testing.T) {
	scopes := []*pipeline.ScopeInfo{
		{ID: "s1", Kind: "function", Name: "foo", StartLine: 1, EndLine: 10},
		{ID: "s2", Kind: "function", Name: "bar", StartLine: 15, EndLine: 25},
		{ID: "s3", Kind: "function", Name: "baz", StartLine: 30, EndLine: 40},
	}
	buildScopeParentIDs(scopes)
	for _, s := range scopes {
		if s.ParentID != "" {
			t.Errorf("flat sibling %s should have no parent, got %s", s.Name, s.ParentID)
		}
	}
}

func TestBuildScopeParentIDs_NestedTwoLevels(t *testing.T) {
	scopes := []*pipeline.ScopeInfo{
		{ID: "class1", Kind: "class", Name: "Server", StartLine: 1, EndLine: 100},
		{ID: "method1", Kind: "method", Name: "Start", StartLine: 10, EndLine: 30},
		{ID: "method2", Kind: "method", Name: "Stop", StartLine: 35, EndLine: 50},
	}
	buildScopeParentIDs(scopes)

	scopeMap := make(map[string]*pipeline.ScopeInfo)
	for _, s := range scopes {
		scopeMap[s.Name] = s
	}

	if scopeMap["Server"].ParentID != "" {
		t.Errorf("Server should be root, got parentID=%s", scopeMap["Server"].ParentID)
	}
	if scopeMap["Start"].ParentID != "class1" {
		t.Errorf("Start should have parent=class1, got %s", scopeMap["Start"].ParentID)
	}
	if scopeMap["Stop"].ParentID != "class1" {
		t.Errorf("Stop should have parent=class1, got %s", scopeMap["Stop"].ParentID)
	}
}

func TestBuildScopeParentIDs_DeepNesting(t *testing.T) {
	scopes := []*pipeline.ScopeInfo{
		{ID: "module", Kind: "module", Name: "main", StartLine: 1, EndLine: 200},
		{ID: "class", Kind: "class", Name: "App", StartLine: 5, EndLine: 150},
		{ID: "method", Kind: "method", Name: "Run", StartLine: 20, EndLine: 80},
		{ID: "block", Kind: "block", Name: "", StartLine: 30, EndLine: 60},
	}
	buildScopeParentIDs(scopes)

	scopeMap := make(map[string]*pipeline.ScopeInfo)
	for _, s := range scopes {
		scopeMap[s.ID] = s
	}

	if scopeMap["module"].ParentID != "" {
		t.Errorf("module should be root")
	}
	if scopeMap["class"].ParentID != "module" {
		t.Errorf("class parent should be module, got %s", scopeMap["class"].ParentID)
	}
	if scopeMap["method"].ParentID != "class" {
		t.Errorf("method parent should be class, got %s", scopeMap["method"].ParentID)
	}
	if scopeMap["block"].ParentID != "method" {
		t.Errorf("block parent should be method, got %s", scopeMap["block"].ParentID)
	}
}

func TestBuildScopeParentIDs_OverlappingSiblings(t *testing.T) {
	// Two classes at the same level, each with one method
	scopes := []*pipeline.ScopeInfo{
		{ID: "c1", Kind: "class", Name: "Foo", StartLine: 1, EndLine: 30},
		{ID: "m1", Kind: "method", Name: "A", StartLine: 5, EndLine: 15},
		{ID: "c2", Kind: "class", Name: "Bar", StartLine: 35, EndLine: 60},
		{ID: "m2", Kind: "method", Name: "B", StartLine: 40, EndLine: 50},
	}
	buildScopeParentIDs(scopes)

	scopeMap := make(map[string]*pipeline.ScopeInfo)
	for _, s := range scopes {
		scopeMap[s.ID] = s
	}

	if scopeMap["c1"].ParentID != "" {
		t.Errorf("Foo should be root")
	}
	if scopeMap["m1"].ParentID != "c1" {
		t.Errorf("A should have parent c1, got %s", scopeMap["m1"].ParentID)
	}
	if scopeMap["c2"].ParentID != "" {
		t.Errorf("Bar should be root")
	}
	if scopeMap["m2"].ParentID != "c2" {
		t.Errorf("B should have parent c2, got %s", scopeMap["m2"].ParentID)
	}
}