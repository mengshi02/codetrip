package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ MarkdownProvider Language() and ImportSemantics() Tests ============

func TestMarkdownProvider_Language(t *testing.T) {
	p := NewMarkdownProvider()
	if p.Language() != graph.LabelMarkdownFile {
		t.Errorf("expected LabelMarkdownFile, got %s", p.Language())
	}
}

func TestMarkdownProvider_ImportSemantics(t *testing.T) {
	p := NewMarkdownProvider()
	if p.ImportSemantics() != "" {
		t.Errorf("expected empty ImportSemantics for Markdown, got %s", p.ImportSemantics())
	}
}

// ============ MarkdownProvider QuerySet Tests ============

func TestMarkdownProvider_QuerySetNotNil(t *testing.T) {
	p := NewMarkdownProvider()
	qs := p.QuerySet()
	if qs == nil {
		t.Fatal("QuerySet should not be nil")
	}
	if qs.Scope == "" {
		t.Error("Scope query should not be empty")
	}
	if qs.Declaration == "" {
		t.Error("Declaration query should not be empty")
	}
}

func TestMarkdownProvider_QuerySet_EmptyImportTypeBindingReference(t *testing.T) {
	p := NewMarkdownProvider()
	qs := p.QuerySet()
	if qs.Import != "" {
		t.Errorf("expected empty Import query for Markdown, got %s", qs.Import)
	}
	if qs.TypeBinding != "" {
		t.Errorf("expected empty TypeBinding query for Markdown, got %s", qs.TypeBinding)
	}
	if qs.Reference != "" {
		t.Errorf("expected empty Reference query for Markdown, got %s", qs.Reference)
	}
}

// ============ MarkdownProvider InterpretScope Tests ============

func TestMarkdownProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "document", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "docs/guide.md")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "guide" {
		t.Errorf("expected name=guide (from mdModuleName), got %s", result[0].Name)
	}
	if result[0].FilePath != "docs/guide.md" {
		t.Errorf("expected FilePath=docs/guide.md, got %s", result[0].FilePath)
	}
}

func TestMarkdownProvider_InterpretScope_SectionScope(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.section", Text: "## Introduction", StartRow: 5, EndRow: 20},
	}
	result := p.InterpretScope(captures, nil, "README.md")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "section" {
		t.Errorf("expected kind=section, got %s", result[0].Kind)
	}
	if result[0].Name != "Introduction" {
		t.Errorf("expected name=Introduction (from mdExtractHeadingText), got %s", result[0].Name)
	}
}

func TestMarkdownProvider_InterpretScope_MultipleScopes(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "document", StartRow: 1, EndRow: 200},
		{MatchIndex: 1, NodeType: "scope.section", Text: "# Getting Started", StartRow: 5, EndRow: 50},
		{MatchIndex: 2, NodeType: "scope.section", Text: "## Installation", StartRow: 55, EndRow: 80},
	}
	result := p.InterpretScope(captures, nil, "docs/guide.md")
	if len(result) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(result))
	}

	scopeMap := map[string]*pipeline.ScopeInfo{}
	for _, s := range result {
		scopeMap[s.Kind + ":" + s.Name] = s
	}

	if _, ok := scopeMap["module:guide"]; !ok {
		t.Error("expected module:guide scope")
	}
	if _, ok := scopeMap["section:Getting Started"]; !ok {
		t.Error("expected section:Getting Started scope")
	}
	if _, ok := scopeMap["section:Installation"]; !ok {
		t.Error("expected section:Installation scope")
	}
}

func TestMarkdownProvider_InterpretScope_SkipsUnknownNodeType(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.md")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

func TestMarkdownProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewMarkdownProvider()
	result := p.InterpretScope(nil, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestMarkdownProvider_InterpretScope_NestedParentIDs(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "document", StartRow: 1, EndRow: 200},
		{MatchIndex: 1, NodeType: "scope.section", Text: "# Title", StartRow: 3, EndRow: 100},
		{MatchIndex: 2, NodeType: "scope.section", Text: "## Subtitle", StartRow: 10, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "doc.md")
	if len(result) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(result))
	}

	// Module should have no parent
	if result[0].ParentID != "" {
		t.Errorf("module should be root, got ParentID=%s", result[0].ParentID)
	}

	// Section within module should have module as parent
	if result[1].ParentID == "" {
		t.Error("Title section should have a parent")
	}
}

// ============ MarkdownProvider InterpretDeclaration Tests ============

func TestMarkdownProvider_InterpretDeclaration_Section(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.section", Text: "## Introduction", StartRow: 5, EndRow: 20},
	}
	result := p.InterpretDeclaration(captures, nil, "README.md")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Introduction" {
		t.Errorf("expected name=Introduction, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelSection {
		t.Errorf("expected label=LabelSection, got %s", result[0].Label)
	}
	if result[0].FilePath != "README.md" {
		t.Errorf("expected FilePath=README.md, got %s", result[0].FilePath)
	}
	if result[0].StartLine != 5 {
		t.Errorf("expected StartLine=5, got %d", result[0].StartLine)
	}
	if result[0].EndLine != 20 {
		t.Errorf("expected EndLine=20, got %d", result[0].EndLine)
	}
}

func TestMarkdownProvider_InterpretDeclaration_MultipleSections(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.section", Text: "# Title", StartRow: 1, EndRow: 50},
		{MatchIndex: 1, NodeType: "declaration.section", Text: "## Subtitle", StartRow: 5, EndRow: 30},
		{MatchIndex: 2, NodeType: "declaration.section", Text: "### Details", StartRow: 10, EndRow: 20},
	}
	result := p.InterpretDeclaration(captures, nil, "doc.md")
	if len(result) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(result))
	}

	names := make([]string, len(result))
	for i, sym := range result {
		names[i] = sym.Name
		if sym.Label != graph.LabelSection {
			t.Errorf("expected LabelSection for %s, got %s", sym.Name, sym.Label)
		}
	}
	if names[0] != "Title" {
		t.Errorf("expected first name=Title, got %s", names[0])
	}
	if names[1] != "Subtitle" {
		t.Errorf("expected second name=Subtitle, got %s", names[1])
	}
	if names[2] != "Details" {
		t.Errorf("expected third name=Details, got %s", names[2])
	}
}

func TestMarkdownProvider_InterpretDeclaration_NonSectionCaptureSkipped(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.other", Text: "something", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "test.md")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for non-section declaration, got %d", len(result))
	}
}

func TestMarkdownProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewMarkdownProvider()
	result := p.InterpretDeclaration(nil, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestMarkdownProvider_InterpretDeclaration_MixedCaptureTypes(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.section", Text: "## Valid Section", StartRow: 5, EndRow: 20},
		{MatchIndex: 1, NodeType: "declaration.unknown", Text: "invalid", StartRow: 25, EndRow: 30},
		{MatchIndex: 2, NodeType: "declaration.section", Text: "### Another", StartRow: 35, EndRow: 45},
	}
	result := p.InterpretDeclaration(captures, nil, "test.md")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols (only declaration.section), got %d", len(result))
	}
	names := map[string]bool{}
	for _, sym := range result {
		names[sym.Name] = true
	}
	if !names["Valid Section"] {
		t.Error("expected Valid Section in results")
	}
	if !names["Another"] {
		t.Error("expected Another in results")
	}
}

// ============ MarkdownProvider InterpretImport Tests ============

func TestMarkdownProvider_InterpretImport_AlwaysNil(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "something", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for Markdown InterpretImport, got %v", result)
	}
}

func TestMarkdownProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewMarkdownProvider()
	result := p.InterpretImport(nil, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ MarkdownProvider InterpretTypeBinding Tests ============

func TestMarkdownProvider_InterpretTypeBinding_AlwaysNil(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Text: "x", StartRow: 1},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for Markdown InterpretTypeBinding, got %v", result)
	}
}

// ============ MarkdownProvider InterpretReference Tests ============

func TestMarkdownProvider_InterpretReference_AlwaysNil(t *testing.T) {
	p := NewMarkdownProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call", Text: "func", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for Markdown InterpretReference, got %v", result)
	}
}

func TestMarkdownProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewMarkdownProvider()
	result := p.InterpretReference(nil, nil, "test.md")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}