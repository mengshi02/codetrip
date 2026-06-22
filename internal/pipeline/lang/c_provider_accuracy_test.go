package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ CProvider InterpretDeclaration Accuracy Tests ============

func TestCProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "myFunc", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "myFunc" {
		t.Errorf("expected name=myFunc, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=Function, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Struct(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.struct", StartRow: 3, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "Node", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Node" {
		t.Errorf("expected name=Node, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelStruct {
		t.Errorf("expected label=Struct, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Union(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.union", StartRow: 2, EndRow: 8},
		{MatchIndex: 0, Name: "declaration.name", Text: "Data", StartRow: 2},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Data" {
		t.Errorf("expected name=Data, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelStruct {
		t.Errorf("expected label=Struct for union, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Enum(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.enum", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Color", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Color" {
		t.Errorf("expected name=Color, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelEnum {
		t.Errorf("expected label=Enum, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Typedef(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.typedef", StartRow: 7, EndRow: 7},
		{MatchIndex: 0, Name: "declaration.name", Text: "Handle", StartRow: 7},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Handle" {
		t.Errorf("expected name=Handle, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelTypedef {
		t.Errorf("expected label=Typedef, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Field(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.field", StartRow: 4, EndRow: 4},
		{MatchIndex: 0, Name: "declaration.name", Text: "value", StartRow: 4},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "value" {
		t.Errorf("expected name=value, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelField {
		t.Errorf("expected label=Field, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Variable(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", StartRow: 10, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "count", StartRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "count" {
		t.Errorf("expected name=count, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelVariable {
		t.Errorf("expected label=Variable, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Macro(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.macro", StartRow: 1, EndRow: 1},
		{MatchIndex: 0, Name: "declaration.name", Text: "MAX_SIZE", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MAX_SIZE" {
		t.Errorf("expected name=MAX_SIZE, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelMacro {
		t.Errorf("expected label=Macro, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_Const(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.const", StartRow: 3, EndRow: 3},
		{MatchIndex: 0, Name: "declaration.name", Text: "RED", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "RED" {
		t.Errorf("expected name=RED, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConst {
		t.Errorf("expected label=Const, got %s", result[0].Label)
	}
}

func TestCProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "foo", StartRow: 5},
		{MatchIndex: 1, NodeType: "declaration.function", StartRow: 15, EndRow: 20},
		{MatchIndex: 1, Name: "declaration.name", Text: "bar", StartRow: 15},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(result))
	}
	names := map[string]bool{}
	for _, sym := range result {
		names[sym.Name] = true
	}
	if !names["foo"] {
		t.Error("expected foo in results")
	}
	if !names["bar"] {
		t.Error("expected bar in results")
	}
}

func TestCProvider_InterpretDeclaration_UnknownOuterSkipped(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "x", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "test.c")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown outer type, got %d", len(result))
	}
}

func TestCProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewCProvider()
	result := p.InterpretDeclaration(nil, nil, "test.c")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ CProvider InterpretImport Accuracy Tests ============

func TestCProvider_InterpretImport_QuotedPath(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: `#include "myheader.h"`, StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "myheader.h" {
		t.Errorf("expected path=myheader.h, got %s", result[0].Path)
	}
	if !result[0].IsWildcard {
		t.Error("expected IsWildcard=true for C import")
	}
}

func TestCProvider_InterpretImport_AngleBracketPath(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "#include <stdio.h>", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "stdio.h" {
		t.Errorf("expected path=stdio.h, got %s", result[0].Path)
	}
}

func TestCProvider_InterpretImport_NonImportFiltered(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", Text: "void foo(", StartRow: 5},
	}
	result := p.InterpretImport(captures, nil, "test.c")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import captures, got %d", len(result))
	}
}

func TestCProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewCProvider()
	result := p.InterpretImport(nil, nil, "test.c")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ CProvider InterpretScope Accuracy Tests ============

func TestCProvider_InterpretScope_Module(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "translation unit", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "src/main.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "main" {
		t.Errorf("expected name=main, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretScope_Class(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "struct Node {\n    int val;", StartRow: 5, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "class" {
		t.Errorf("expected kind=class, got %s", result[0].Kind)
	}
	if result[0].Name != "Node" {
		t.Errorf("expected name=Node, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretScope_Function(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "int main() {\n    return 0;", StartRow: 1, EndRow: 5},
	}
	result := p.InterpretScope(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "function" {
		t.Errorf("expected kind=function, got %s", result[0].Kind)
	}
	if result[0].Name != "main" {
		t.Errorf("expected name=main, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretScope_Block(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.block", Text: "{ ... }", StartRow: 3, EndRow: 8},
	}
	result := p.InterpretScope(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "block" {
		t.Errorf("expected kind=block, got %s", result[0].Kind)
	}
	if result[0].Name != "" {
		t.Errorf("expected empty name for block, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewCProvider()
	result := p.InterpretScope(nil, nil, "test.c")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ CProvider InterpretTypeBinding Accuracy Tests ============

func TestCProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "name"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "const char *"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", result[0].Kind)
	}
	if result[0].TypeName != "const char *" {
		t.Errorf("expected typeName='const char *', got %s", result[0].TypeName)
	}
}

func TestCProvider_InterpretTypeBinding_Assignment(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.assignment", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.name", Text: "count"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "assignment" {
		t.Errorf("expected kind=assignment, got %s", result[0].Kind)
	}
	if result[0].TypeName != "int" {
		t.Errorf("expected typeName=int, got %s", result[0].TypeName)
	}
}

func TestCProvider_InterpretTypeBinding_BoundNodeNotSet(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "x"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	// BoundNode should NOT be set in C provider (name is captured but discarded with _ =)
	if result[0].BoundNode != "" {
		t.Errorf("expected BoundNode to be empty (not set), got %s", result[0].BoundNode)
	}
}

func TestCProvider_InterpretTypeBinding_UnknownKindSkipped(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.unknown", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "x"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.c")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown kind, got %d", len(result))
	}
}

func TestCProvider_InterpretTypeBinding_EmptyCaptures(t *testing.T) {
	p := NewCProvider()
	result := p.InterpretTypeBinding(nil, nil, "test.c")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ CProvider InterpretReference Accuracy Tests ============

func TestCProvider_InterpretReference_CallFree(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 5},
		{MatchIndex: 0, Name: "reference.name", Text: "printf"},
	}
	result := p.InterpretReference(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(result))
	}
	if result[0].Kind != "call.free" {
		t.Errorf("expected kind=call.free, got %s", result[0].Kind)
	}
	if result[0].Name != "printf" {
		t.Errorf("expected name=printf, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretReference_CallMember(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 8},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, Name: "reference.name", Text: "method"},
	}
	result := p.InterpretReference(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(result))
	}
	if result[0].Kind != "call.member" {
		t.Errorf("expected kind=call.member, got %s", result[0].Kind)
	}
	if result[0].Receiver != "obj" {
		t.Errorf("expected receiver=obj, got %s", result[0].Receiver)
	}
	if result[0].Name != "method" {
		t.Errorf("expected name=method, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretReference_Read(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read", StartRow: 12},
		{MatchIndex: 0, Name: "reference.receiver", Text: "ptr"},
		{MatchIndex: 0, Name: "reference.name", Text: "field"},
	}
	result := p.InterpretReference(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(result))
	}
	if result[0].Kind != "read" {
		t.Errorf("expected kind=read, got %s", result[0].Kind)
	}
	if result[0].Receiver != "ptr" {
		t.Errorf("expected receiver=ptr, got %s", result[0].Receiver)
	}
	if result[0].Name != "field" {
		t.Errorf("expected name=field, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretReference_Write(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write", StartRow: 15},
		{MatchIndex: 0, Name: "reference.receiver", Text: "cfg"},
		{MatchIndex: 0, Name: "reference.name", Text: "port"},
	}
	result := p.InterpretReference(captures, nil, "test.c")
	if len(result) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(result))
	}
	if result[0].Kind != "write" {
		t.Errorf("expected kind=write, got %s", result[0].Kind)
	}
	if result[0].Receiver != "cfg" {
		t.Errorf("expected receiver=cfg, got %s", result[0].Receiver)
	}
	if result[0].Name != "port" {
		t.Errorf("expected name=port, got %s", result[0].Name)
	}
}

func TestCProvider_InterpretReference_UnknownKindSkipped(t *testing.T) {
	p := NewCProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", StartRow: 5},
		{MatchIndex: 0, Name: "reference.name", Text: "x"},
	}
	result := p.InterpretReference(captures, nil, "test.c")
	if len(result) != 0 {
		t.Errorf("expected 0 refs for unknown kind, got %d", len(result))
	}
}

func TestCProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewCProvider()
	result := p.InterpretReference(nil, nil, "test.c")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

// ============ CProvider Language() and ImportSemantics() Tests ============

func TestCProvider_Language(t *testing.T) {
	p := NewCProvider()
	if p.Language() != graph.LabelCFile {
		t.Errorf("expected LabelCFile, got %s", p.Language())
	}
}

func TestCProvider_ImportSemantics(t *testing.T) {
	p := NewCProvider()
	if p.ImportSemantics() != ImportSemanticsWildcardTransitive {
		t.Errorf("expected ImportSemanticsWildcardTransitive, got %s", p.ImportSemantics())
	}
}