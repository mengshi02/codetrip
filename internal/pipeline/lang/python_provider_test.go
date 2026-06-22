package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ PythonProvider InterpretScope Tests ============

func TestPythonProvider_InterpretScope_Empty(t *testing.T) {
	p := NewPythonProvider()
	result := p.InterpretScope(nil, nil, "test.py")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestPythonProvider_InterpretScope_FunctionAndClass(t *testing.T) {
	p := NewPythonProvider()
	// Scope captures no longer have name sub-captures.
	// Name is extracted from the captured node's Text field.
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "def process():\n    pass", StartRow: 5, EndRow: 20},
		{MatchIndex: 1, NodeType: "scope.class", Text: "class DataProcessor:\n    pass", StartRow: 25, EndRow: 60},
		{MatchIndex: 2, NodeType: "scope.function", Text: "def run(self):\n    pass", StartRow: 30, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "processor.py")
	if len(result) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(result))
	}

	scopeMap := make(map[string]*pipeline.ScopeInfo)
	for _, s := range result {
		scopeMap[s.Name] = s
	}

	if scopeMap["DataProcessor"].ParentID != "" {
		t.Errorf("DataProcessor should be root, got parentID=%s", scopeMap["DataProcessor"].ParentID)
	}
	if scopeMap["run"].ParentID != scopeMap["DataProcessor"].ID {
		t.Errorf("run should have DataProcessor as parent, got parentID=%s", scopeMap["run"].ParentID)
	}
	if scopeMap["process"].ParentID != "" {
		t.Errorf("process (outside class) should be root, got parentID=%s", scopeMap["process"].ParentID)
	}
}

// ============ PythonProvider InterpretDeclaration Tests ============

func TestPythonProvider_InterpretDeclaration_Empty(t *testing.T) {
	p := NewPythonProvider()
	result := p.InterpretDeclaration(nil, nil, "test.py")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestPythonProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", Name: "declaration.name", Text: "process_data", StartRow: 5, EndRow: 15},
	}
	result := p.InterpretDeclaration(captures, nil, "data.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "process_data" {
		t.Errorf("expected name=process_data, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=function, got %s", result[0].Label)
	}
}

func TestPythonProvider_InterpretDeclaration_Class(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", Name: "declaration.name", Text: "MyError", StartRow: 3, EndRow: 20},
	}
	result := p.InterpretDeclaration(captures, nil, "errors.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	sym := result[0]
	if sym.Name != "MyError" {
		t.Errorf("expected name=MyError, got %s", sym.Name)
	}
	if sym.Label != graph.LabelClass {
		t.Errorf("expected label=class, got %s", sym.Label)
	}
}

func TestPythonProvider_InterpretDeclaration_Variable(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", Name: "declaration.name", Text: "count", StartRow: 5, EndRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "count" {
		t.Errorf("expected name=count, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelVariable {
		t.Errorf("expected label=variable, got %s", result[0].Label)
	}
}

// ============ PythonProvider InterpretImport Tests ============

func TestPythonProvider_InterpretImport_Empty(t *testing.T) {
	p := NewPythonProvider()
	result := p.InterpretImport(nil, nil, "test.py")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestPythonProvider_InterpretImport_FromImport(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "from os import path, environ", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	imp := result[0]
	if imp.Path != "os" {
		t.Errorf("expected path=os, got %s", imp.Path)
	}
	if len(imp.Symbols) != 2 {
		t.Fatalf("expected 2 imported items, got %d", len(imp.Symbols))
	}
	if imp.Symbols[0] != "path" {
		t.Errorf("expected first item=path, got %s", imp.Symbols[0])
	}
	if imp.Symbols[1] != "environ" {
		t.Errorf("expected second item=environ, got %s", imp.Symbols[1])
	}
}

func TestPythonProvider_InterpretImport_SimpleImport(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import sys", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "sys" {
		t.Errorf("expected path=sys, got %s", result[0].Path)
	}
}

// ============ PythonProvider InterpretTypeBinding Tests ============

func TestPythonProvider_InterpretTypeBinding_Empty(t *testing.T) {
	p := NewPythonProvider()
	result := p.InterpretTypeBinding(nil, nil, "test.py")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestPythonProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Name: "type-binding.name", Text: "name", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.type", Text: "str"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	tb := result[0]
	if tb.Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", tb.Kind)
	}
	if tb.TypeName != "str" {
		t.Errorf("expected typeName=str, got %s", tb.TypeName)
	}
	if tb.BoundNode != "name" {
		t.Errorf("expected boundNode=name, got %s", tb.BoundNode)
	}
}

func TestPythonProvider_InterpretTypeBinding_ReturnType(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", Name: "type-binding.type", Text: "bool", StartRow: 5},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "return" {
		t.Errorf("expected kind=return, got %s", result[0].Kind)
	}
	if result[0].TypeName != "bool" {
		t.Errorf("expected typeName=bool, got %s", result[0].TypeName)
	}
}

func TestPythonProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", Name: "type-binding.name", Text: "db", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Database"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	tb := result[0]
	if tb.Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", tb.Kind)
	}
	if tb.TypeName != "Database" {
		t.Errorf("expected typeName=Database, got %s", tb.TypeName)
	}
	if tb.BoundNode != "db" {
		t.Errorf("expected boundNode=db, got %s", tb.BoundNode)
	}
}

// ============ PythonProvider InterpretReference Tests ============

func TestPythonProvider_InterpretReference_Empty(t *testing.T) {
	p := NewPythonProvider()
	result := p.InterpretReference(nil, nil, "test.py")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestPythonProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", Name: "reference.name", Text: "print", StartRow: 10},
	}
	result := p.InterpretReference(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "call.free" {
		t.Errorf("expected kind=call.free, got %s", ref.Kind)
	}
	if ref.Name != "print" {
		t.Errorf("expected name=print, got %s", ref.Name)
	}
}

func TestPythonProvider_InterpretReference_MemberCall(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", Name: "reference.receiver", Text: "self", StartRow: 15},
		{MatchIndex: 0, Name: "reference.name", Text: "process"},
	}
	result := p.InterpretReference(captures, nil, "test.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "call.member" {
		t.Errorf("expected kind=call.member, got %s", ref.Kind)
	}
	if ref.Receiver != "self" {
		t.Errorf("expected receiver=self, got %s", ref.Receiver)
	}
	if ref.Name != "process" {
		t.Errorf("expected name=process, got %s", ref.Name)
	}
}

func TestPythonProvider_InterpretReference_WriteMember(t *testing.T) {
	p := NewPythonProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write.member", Name: "reference.receiver", Text: "config", StartRow: 12},
		{MatchIndex: 0, Name: "reference.name", Text: "timeout"},
	}
	result := p.InterpretReference(captures, nil, "app.py")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "write.member" {
		t.Errorf("expected kind=write.member, got %s", ref.Kind)
	}
	if ref.Receiver != "config" {
		t.Errorf("expected receiver=config, got %s", ref.Receiver)
	}
	if ref.Name != "timeout" {
		t.Errorf("expected name=timeout, got %s", ref.Name)
	}
}