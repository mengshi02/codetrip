package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ PythonProvider InterpretDeclaration Accuracy Tests ============

func TestPythonProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", Name: "declaration.name", Text: "foo", StartRow: 5, EndRow: 10},
		{MatchIndex: 1, NodeType: "declaration.function", Name: "declaration.name", Text: "bar", StartRow: 15, EndRow: 20},
		{MatchIndex: 2, NodeType: "declaration.class", Name: "declaration.name", Text: "MyClass", StartRow: 25, EndRow: 50},
	}
	result := p.InterpretDeclaration(captures, nil, "test.py")
	if len(result) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(result))
	}

	symMap := map[string]graph.Label{}
	for _, sym := range result {
		symMap[sym.Name] = sym.Label
	}
	if symMap["foo"] != graph.LabelFunction {
		t.Errorf("expected foo to be Function, got %s", symMap["foo"])
	}
	if symMap["bar"] != graph.LabelFunction {
		t.Errorf("expected bar to be Function, got %s", symMap["bar"])
	}
	if symMap["MyClass"] != graph.LabelClass {
		t.Errorf("expected MyClass to be Class, got %s", symMap["MyClass"])
	}
}

func TestPythonProvider_InterpretDeclaration_UnknownNodeTypeSkipped(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.decorator", Name: "declaration.name", Text: "property", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "test.py")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown node type, got %d", len(result))
	}
}

// ============ PythonProvider InterpretImport Accuracy Tests ============

func TestPythonProvider_InterpretImport_FromImportMultipleSymbols(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "from typing import List, Dict, Optional", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "typing" {
		t.Errorf("expected path=typing, got %s", result[0].Path)
	}
	if len(result[0].Symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(result[0].Symbols))
	}
	expectedSyms := []string{"List", "Dict", "Optional"}
	for i, s := range expectedSyms {
		if result[0].Symbols[i] != s {
			t.Errorf("expected symbol[%d]=%s, got %s", i, s, result[0].Symbols[i])
		}
	}
}

func TestPythonProvider_InterpretImport_ImportWithAs(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import numpy as np", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "numpy" {
		t.Errorf("expected path=numpy, got %s", result[0].Path)
	}
}

func TestPythonProvider_InterpretImport_MultipleMatches(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import os", StartRow: 1},
		{MatchIndex: 1, NodeType: "import.statement", Text: "from sys import argv", StartRow: 2},
	}
	result := p.InterpretImport(captures, nil, "test.py")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["os"] {
		t.Error("expected os in imports")
	}
	if !paths["sys"] {
		t.Error("expected sys in imports")
	}
}

// ============ PythonProvider InterpretScope Accuracy Tests ============

func TestPythonProvider_InterpretScope_DeepNesting(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "module code", StartRow: 1, EndRow: 100},
		{MatchIndex: 1, NodeType: "scope.class", Text: "class App:\n    pass", StartRow: 5, EndRow: 90},
		{MatchIndex: 2, NodeType: "scope.function", Text: "def __init__(self):\n    pass", StartRow: 10, EndRow: 30},
		{MatchIndex: 3, NodeType: "scope.function", Text: "def process(self):\n    pass", StartRow: 35, EndRow: 60},
	}
	result := p.InterpretScope(captures, nil, "app.py")
	if len(result) != 4 {
		t.Fatalf("expected 4 scopes, got %d", len(result))
	}

	scopeMap := map[string]*pipeline.ScopeInfo{}
	for _, s := range result {
		scopeMap[s.Name] = s
	}

	// App should be child of module, __init__ and process should be children of App
	if scopeMap["App"].ParentID == "" {
		t.Error("App should have a parent (module)")
	}
	if scopeMap["__init__"].ParentID != scopeMap["App"].ID {
		t.Errorf("__init__ should have App as parent, got %s", scopeMap["__init__"].ParentID)
	}
	if scopeMap["process"].ParentID != scopeMap["App"].ID {
		t.Errorf("process should have App as parent, got %s", scopeMap["process"].ParentID)
	}
}

func TestPythonProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "module code", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "pkg/utils.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "utils" {
		t.Errorf("expected name=utils for pkg/utils.py, got %s", result[0].Name)
	}
}

func TestPythonProvider_InterpretScope_SkipsUnknownNodeType(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.py")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

// ============ PythonProvider InterpretTypeBinding Accuracy Tests ============

func TestPythonProvider_InterpretTypeBinding_MultipleBindingsNoCrossContamination(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Name: "type-binding.name", Text: "name", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.type", Text: "str"},
		{MatchIndex: 1, NodeType: "type-binding.parameter", Name: "type-binding.name", Text: "age", StartRow: 6},
		{MatchIndex: 1, Name: "type-binding.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.py")
	if len(result) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result))
	}

	typeNames := map[string]bool{}
	for _, tb := range result {
		typeNames[tb.TypeName] = true
	}
	if !typeNames["str"] {
		t.Error("expected str type in bindings")
	}
	if !typeNames["int"] {
		t.Error("expected int type in bindings")
	}
}

// ============ PythonProvider InterpretReference Accuracy Tests ============

func TestPythonProvider_InterpretReference_MultipleRefTypes(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", Name: "reference.name", Text: "print", StartRow: 10},
		{MatchIndex: 1, NodeType: "reference.call.member", Name: "reference.receiver", Text: "self", StartRow: 15},
		{MatchIndex: 1, Name: "reference.name", Text: "process"},
		{MatchIndex: 2, NodeType: "reference.write.member", Name: "reference.receiver", Text: "config", StartRow: 20},
		{MatchIndex: 2, Name: "reference.name", Text: "timeout"},
	}
	result := p.InterpretReference(captures, nil, "test.py")
	if len(result) != 3 {
		t.Fatalf("expected 3 references, got %d", len(result))
	}

	foundKinds := map[string]bool{}
	for _, ref := range result {
		foundKinds[ref.Kind] = true
	}
	if !foundKinds["call.free"] {
		t.Error("expected call.free reference")
	}
	if !foundKinds["call.member"] {
		t.Error("expected call.member reference")
	}
	if !foundKinds["write.member"] {
		t.Error("expected write.member reference")
	}
}

func TestPythonProvider_InterpretReference_UnknownNodeTypeSkipped(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", Name: "reference.name", Text: "foo", StartRow: 10},
	}
	result := p.InterpretReference(captures, nil, "test.py")
	if len(result) != 0 {
		t.Errorf("expected 0 references for unknown node type, got %d", len(result))
	}
}

// ============ PythonProvider Language() and ImportSemantics() Tests ============

func TestPythonProvider_Language(t *testing.T) {
	p := NewPythonProvider()
	if p.Language() != graph.LabelPythonFile {
		t.Errorf("expected LabelPythonFile, got %s", p.Language())
	}
}

func TestPythonProvider_ImportSemantics(t *testing.T) {
	p := NewPythonProvider()
	if p.ImportSemantics() != ImportSemanticsNamespace {
		t.Errorf("expected ImportSemanticsNamespace, got %s", p.ImportSemantics())
	}
}

// ============ PythonProvider QuerySet Tests ============

func TestPythonProvider_QuerySetNotNil(t *testing.T) {
	p := NewPythonProvider()
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
	if qs.Import == "" {
		t.Error("Import query should not be empty")
	}
	if qs.TypeBinding == "" {
		t.Error("TypeBinding query should not be empty")
	}
	if qs.Reference == "" {
		t.Error("Reference query should not be empty")
	}
}