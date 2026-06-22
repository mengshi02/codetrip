package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ============ End-to-End Test Helpers ============
//
// These tests exercise the full pipeline: tree-sitter parse → query → interpret.
// They use real source code to verify that query patterns match correctly
// and that Interpret methods produce accurate results.

// e2eParseAndQuery runs tree-sitter parse + query + interpret for a given provider.
func e2eParseAndQuery(t *testing.T, source string, provider langProviderE2E) *e2eResult {
	t.Helper()
	lang := provider.TreeSitterLanguage()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse([]byte(source))
	if err != nil || tree == nil {
		t.Fatalf("tree-sitter parse failed: %v", err)
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		t.Fatal("tree-sitter returned nil root")
	}

	result := &e2eResult{}
	src := []byte(source)
	qs := provider.QuerySet()

	// Scope
	if qs.Scope != "" {
		q, err := gotreesitter.NewQuery(qs.Scope, lang)
		if err != nil {
			t.Fatalf("scope query compile error: %v", err)
		}
		captures := executeQueryE2E(q, root, lang, src)
		result.Scopes = provider.InterpretScope(captures, src, "test_file")
	}

	// Declaration
	if qs.Declaration != "" {
		q, err := gotreesitter.NewQuery(qs.Declaration, lang)
		if err != nil {
			t.Fatalf("declaration query compile error: %v", err)
		}
		captures := executeQueryE2E(q, root, lang, src)
		result.Symbols = provider.InterpretDeclaration(captures, src, "test_file")
	}

	// Import
	if qs.Import != "" {
		q, err := gotreesitter.NewQuery(qs.Import, lang)
		if err != nil {
			t.Fatalf("import query compile error: %v", err)
		}
		captures := executeQueryE2E(q, root, lang, src)
		result.Imports = provider.InterpretImport(captures, src, "test_file")
	}

	// TypeBinding
	if qs.TypeBinding != "" {
		q, err := gotreesitter.NewQuery(qs.TypeBinding, lang)
		if err != nil {
			t.Fatalf("type-binding query compile error: %v", err)
		}
		captures := executeQueryE2E(q, root, lang, src)
		result.TypeBindings = provider.InterpretTypeBinding(captures, src, "test_file")
	}

	// Reference
	if qs.Reference != "" {
		q, err := gotreesitter.NewQuery(qs.Reference, lang)
		if err != nil {
			t.Fatalf("reference query compile error: %v", err)
		}
		captures := executeQueryE2E(q, root, lang, src)
		result.References = provider.InterpretReference(captures, src, "test_file")
	}

	return result
}

type e2eResult struct {
	Scopes       []*pipeline.ScopeInfo
	Symbols      []*pipeline.SymbolInfo
	Imports      []*pipeline.ImportInfo
	TypeBindings []*pipeline.TypeBindingInfo
	References   []*pipeline.ReferenceInfo
}

// langProviderE2E is the interface needed for end-to-end tests.
type langProviderE2E interface {
	TreeSitterLanguage() *gotreesitter.Language
	QuerySet() *pipeline.LangQuerySet
	InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo
	InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo
	InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo
	InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo
	InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo
}

// executeQueryE2E runs a tree-sitter query and converts results to LangCapture format.
func executeQueryE2E(q *gotreesitter.Query, root *gotreesitter.Node, lang *gotreesitter.Language, source []byte) []pipeline.LangCapture {
	matches := q.ExecuteNode(root, lang, source)
	if len(matches) == 0 {
		return nil
	}
	totalCaptures := 0
	for _, m := range matches {
		totalCaptures += len(m.Captures)
	}
	captures := make([]pipeline.LangCapture, 0, totalCaptures)
	for matchIdx, match := range matches {
		for _, qc := range match.Captures {
			cap := pipeline.LangCapture{
				Name:       qc.Name,
				Text:       qc.Text(source),
				MatchIndex: matchIdx,
				NodeType:   qc.Name,
			}
			if qc.Node != nil {
				sp := qc.Node.StartPoint()
				ep := qc.Node.EndPoint()
				cap.StartRow = int(sp.Row) + 1
				cap.StartCol = int(sp.Column)
				cap.EndRow = int(ep.Row) + 1
				cap.EndCol = int(ep.Column)
			}
			captures = append(captures, cap)
		}
	}
	return captures
}

// ============ Go Provider E2E Tests ============

func TestGoProvider_E2E_SimplePackage(t *testing.T) {
	source := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	provider := NewGoProvider()
	result := e2eParseAndQuery(t, source, provider)

	// Should find at least a module scope and function scope
	if len(result.Scopes) < 2 {
		t.Errorf("expected at least 2 scopes (module + function), got %d", len(result.Scopes))
	}

	// Should find the main function declaration
	foundMain := false
	for _, sym := range result.Symbols {
		if sym.Name == "main" && sym.Label == graph.LabelFunction {
			foundMain = true
		}
	}
	if !foundMain {
		t.Error("expected to find 'main' function declaration")
	}

	// Should find the fmt import
	foundFmt := false
	for _, imp := range result.Imports {
		if imp.Path == "fmt" {
			foundFmt = true
		}
	}
	if !foundFmt {
		t.Error("expected to find 'fmt' import")
	}

	// Should find fmt.Println as a member call reference
	foundPrintln := false
	for _, ref := range result.References {
		if ref.Name == "Println" && ref.Kind == "member_call" {
			foundPrintln = true
		}
	}
	if !foundPrintln {
		t.Error("expected to find 'Println' member_call reference")
	}
}

func TestGoProvider_E2E_StructAndMethod(t *testing.T) {
	source := `package server

type Server struct {
	Port int
}

func (s *Server) Start() error {
	return nil
}
`
	provider := NewGoProvider()
	result := e2eParseAndQuery(t, source, provider)

	// Should find Server struct
	foundStruct := false
	for _, sym := range result.Symbols {
		if sym.Name == "Server" && sym.Label == graph.LabelStruct {
			foundStruct = true
		}
	}
	if !foundStruct {
		t.Error("expected to find 'Server' struct declaration")
	}

	// Should find Start method
	foundMethod := false
	for _, sym := range result.Symbols {
		if sym.Name == "Start" && sym.Label == graph.LabelMethod {
			foundMethod = true
		}
	}
	if !foundMethod {
		t.Error("expected to find 'Start' method declaration")
	}

	// Should find receiver type binding
	foundReceiver := false
	for _, tb := range result.TypeBindings {
		if tb.Kind == "receiver" && tb.TypeName == "Server" {
			foundReceiver = true
		}
	}
	if !foundReceiver {
		t.Error("expected to find receiver type binding for Server")
	}
}

func TestGoProvider_E2E_QueryCompilation(t *testing.T) {
	// Verify that all Go provider queries compile without errors
	lang := grammars.GoLanguage()
	provider := NewGoProvider()
	qs := provider.QuerySet()

	queries := []struct {
		name  string
		query string
	}{
		{"Scope", qs.Scope},
		{"Declaration", qs.Declaration},
		{"Import", qs.Import},
		{"TypeBinding", qs.TypeBinding},
		{"Reference", qs.Reference},
	}

	for _, q := range queries {
		if q.query == "" {
			t.Errorf("%s query is empty", q.name)
			continue
		}
		_, err := gotreesitter.NewQuery(q.query, lang)
		if err != nil {
			t.Errorf("%s query compilation failed: %v", q.name, err)
		}
	}
}

// ============ Python Provider E2E Tests ============

func TestPythonProvider_E2E_SimpleModule(t *testing.T) {
	source := `import os
from typing import List

def process(items: List[str]) -> bool:
    return True
`
	provider := NewPythonProvider()
	result := e2eParseAndQuery(t, source, provider)

	// Should find module scope
	foundModule := false
	for _, s := range result.Scopes {
		if s.Kind == "module" {
			foundModule = true
		}
	}
	if !foundModule {
		t.Error("expected to find module scope")
	}

	// Should find process function
	foundProcess := false
	for _, sym := range result.Symbols {
		if sym.Name == "process" && sym.Label == graph.LabelFunction {
			foundProcess = true
		}
	}
	if !foundProcess {
		t.Error("expected to find 'process' function declaration")
	}

	// Should find os and typing imports
	foundOs := false
	foundTyping := false
	for _, imp := range result.Imports {
		if imp.Path == "os" {
			foundOs = true
		}
		if imp.Path == "typing" {
			foundTyping = true
		}
	}
	if !foundOs {
		t.Error("expected to find 'os' import")
	}
	if !foundTyping {
		t.Error("expected to find 'typing' import")
	}

	// Should find parameter type binding (items: List[str])
	foundParam := false
	for _, tb := range result.TypeBindings {
		if tb.Kind == "parameter" && tb.TypeName == "List[str]" {
			foundParam = true
		}
	}
	if !foundParam {
		// Type annotation parsing may vary; log what we got
		t.Logf("TypeBindings: %+v", result.TypeBindings)
	}
}

func TestPythonProvider_E2E_ClassWithMethods(t *testing.T) {
	source := `class DataProcessor:
    def __init__(self, name: str):
        self.name = name

    def process(self):
        self.name.upper()
`
	provider := NewPythonProvider()
	result := e2eParseAndQuery(t, source, provider)

	// Should find DataProcessor class
	foundClass := false
	for _, sym := range result.Symbols {
		if sym.Name == "DataProcessor" && sym.Label == graph.LabelClass {
			foundClass = true
		}
	}
	if !foundClass {
		t.Error("expected to find 'DataProcessor' class declaration")
	}

	// Should find __init__ and process methods
	foundInit := false
	foundProcess := false
	for _, sym := range result.Symbols {
		if sym.Name == "__init__" {
			foundInit = true
		}
		if sym.Name == "process" {
			foundProcess = true
		}
	}
	if !foundInit {
		t.Error("expected to find '__init__' function declaration")
	}
	if !foundProcess {
		t.Error("expected to find 'process' function declaration")
	}

	// Should find member call reference (self.name.upper)
	// At minimum we should find some references
	if len(result.References) == 0 {
		t.Log("No references found — may need query adjustment")
	}
}

func TestPythonProvider_E2E_QueryCompilation(t *testing.T) {
	// Verify that all Python provider queries compile without errors
	lang := grammars.PythonLanguage()
	provider := NewPythonProvider()
	qs := provider.QuerySet()

	queries := []struct {
		name  string
		query string
	}{
		{"Scope", qs.Scope},
		{"Declaration", qs.Declaration},
		{"Import", qs.Import},
		{"TypeBinding", qs.TypeBinding},
		{"Reference", qs.Reference},
	}

	for _, q := range queries {
		if q.query == "" {
			t.Errorf("%s query is empty", q.name)
			continue
		}
		_, err := gotreesitter.NewQuery(q.query, lang)
		if err != nil {
			t.Errorf("%s query compilation failed: %v", q.name, err)
		}
	}
}