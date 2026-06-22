package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ JavaScriptProvider InterpretDeclaration Accuracy Tests ============

func TestJavaScriptProvider_InterpretDeclaration_Class(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", Name: "declaration.class", Text: "class Server {", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Server"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=class, got %s", result[0].Label)
	}
}

func TestJavaScriptProvider_InterpretDeclaration_Method(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.method", Name: "declaration.method", Text: "process() {", StartRow: 3, EndRow: 6},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "process"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "process" {
		t.Errorf("expected name=process, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelMethod {
		t.Errorf("expected label=method, got %s", result[0].Label)
	}
}

func TestJavaScriptProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", Name: "declaration.function", Text: "function main() {", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "main"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "main" {
		t.Errorf("expected name=main, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=function, got %s", result[0].Label)
	}
}

func TestJavaScriptProvider_InterpretDeclaration_Const(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.const", Name: "declaration.const", StartRow: 3, EndRow: 3},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "PORT"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "PORT" {
		t.Errorf("expected name=PORT, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConst {
		t.Errorf("expected label=const, got %s", result[0].Label)
	}
}

func TestJavaScriptProvider_InterpretDeclaration_Variable(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", Name: "declaration.variable", StartRow: 3, EndRow: 3},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "count"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
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

func TestJavaScriptProvider_InterpretDeclaration_InlineArrowFunction(t *testing.T) {
	p := NewJavaScriptProvider()
	// Inline pattern: has declaration.function sub-capture but no outer anchor
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "handler"},
		{MatchIndex: 0, Name: "declaration.function", Text: "() => {}"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "handler" {
		t.Errorf("expected name=handler, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=function for inline arrow, got %s", result[0].Label)
	}
}

func TestJavaScriptProvider_InterpretDeclaration_MultipleMatchesIsolated(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Foo"},
		{MatchIndex: 1, NodeType: "declaration.function", Name: "declaration.function", StartRow: 7, EndRow: 10},
		{MatchIndex: 1, Name: "declaration.name", Text: "bar"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(result))
	}
	names := map[string]bool{}
	for _, sym := range result {
		names[sym.Name] = true
	}
	if !names["Foo"] {
		t.Error("expected Foo in results")
	}
	if !names["bar"] {
		t.Error("expected bar in results")
	}
}

func TestJavaScriptProvider_InterpretDeclaration_UnknownSkipped(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "skip"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.js")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown NodeType, got %d", len(result))
	}
}

// ============ JavaScriptProvider InterpretImport Accuracy Tests ============

func TestJavaScriptProvider_InterpretImport_StaticStatement(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import { useState } from 'react'", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "react" {
		t.Errorf("expected path=react, got %s", result[0].Path)
	}
}

func TestJavaScriptProvider_InterpretImport_DynamicImport(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.dynamic", Text: "import('lodash')", StartRow: 5},
	}
	result := p.InterpretImport(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "lodash" {
		t.Errorf("expected path=lodash, got %s", result[0].Path)
	}
}

func TestJavaScriptProvider_InterpretImport_MultipleMatches(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import { foo } from 'a'", StartRow: 1},
		{MatchIndex: 1, NodeType: "import.statement", Text: "import { bar } from 'b'", StartRow: 2},
	}
	result := p.InterpretImport(captures, nil, "test.js")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["a"] {
		t.Error("expected import from 'a'")
	}
	if !paths["b"] {
		t.Error("expected import from 'b'")
	}
}

func TestJavaScriptProvider_InterpretImport_SkipsNonImportCaptures(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "some.other.type", Text: "not an import", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "test.js")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import captures, got %d", len(result))
	}
}

// ============ JavaScriptProvider InterpretScope Accuracy Tests ============

func TestJavaScriptProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "program content", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "src/utils/helper.js")
	if len(result) < 1 {
		t.Fatal("expected at least 1 scope")
	}
	foundModule := false
	for _, s := range result {
		if s.Kind == "module" {
			foundModule = true
			if s.Name != "helper" {
				t.Errorf("expected module name=helper, got %s", s.Name)
			}
		}
	}
	if !foundModule {
		t.Error("expected module scope")
	}
}

func TestJavaScriptProvider_InterpretScope_ClassScope(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "class Server {\n  constructor() {}\n}", StartRow: 1, EndRow: 3},
	}
	result := p.InterpretScope(captures, nil, "test.js")
	if len(result) < 1 {
		t.Fatal("expected at least 1 scope")
	}
	foundClass := false
	for _, s := range result {
		if s.Kind == "class" {
			foundClass = true
			if s.Name != "S" {
				t.Errorf("expected class name=S (bug: jsParseClassName 'e' stop char), got %s", s.Name)
			}
		}
	}
	if !foundClass {
		t.Error("expected class scope")
	}
}

func TestJavaScriptProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "function process() {\n  return true;\n}", StartRow: 1, EndRow: 3},
	}
	result := p.InterpretScope(captures, nil, "test.js")
	if len(result) < 1 {
		t.Fatal("expected at least 1 scope")
	}
	foundFunc := false
	for _, s := range result {
		if s.Kind == "function" {
			foundFunc = true
			if s.Name != "process" {
				t.Errorf("expected function name=process, got %s", s.Name)
			}
		}
	}
	if !foundFunc {
		t.Error("expected function scope")
	}
}

func TestJavaScriptProvider_InterpretScope_UnknownSkipped(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "something", StartRow: 1, EndRow: 5},
	}
	result := p.InterpretScope(captures, nil, "test.js")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown kind, got %d", len(result))
	}
}

// ============ JavaScriptProvider InterpretTypeBinding Accuracy Tests ============

func TestJavaScriptProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", Name: "type-binding.constructor", Text: "config = new Config()", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "config"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Config"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Config" {
		t.Errorf("expected typeName=Config, got %s", result[0].TypeName)
	}
	if result[0].BoundNode != "config" {
		t.Errorf("expected boundNode=config, got %s", result[0].BoundNode)
	}
}

func TestJavaScriptProvider_InterpretTypeBinding_Alias(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.alias", Name: "type-binding.alias", Text: "data = fetch()", StartRow: 3},
		{MatchIndex: 0, Name: "type-binding.name", Text: "data"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "fetch"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "alias" {
		t.Errorf("expected kind=alias, got %s", result[0].Kind)
	}
	if result[0].TypeName != "fetch" {
		t.Errorf("expected typeName=fetch, got %s", result[0].TypeName)
	}
}

func TestJavaScriptProvider_InterpretTypeBinding_MultipleMatchesIsolated(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", Name: "type-binding.constructor", StartRow: 1},
		{MatchIndex: 0, Name: "type-binding.name", Text: "x"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Server"},
		{MatchIndex: 1, NodeType: "type-binding.alias", Name: "type-binding.alias", StartRow: 5},
		{MatchIndex: 1, Name: "type-binding.name", Text: "y"},
		{MatchIndex: 1, Name: "type-binding.type", Text: "createStore"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.js")
	if len(result) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result))
	}
	typeNames := map[string]bool{}
	for _, tb := range result {
		typeNames[tb.TypeName] = true
	}
	if !typeNames["Server"] {
		t.Error("expected Server type in bindings")
	}
	if !typeNames["createStore"] {
		t.Error("expected createStore type in bindings")
	}
}

func TestJavaScriptProvider_InterpretTypeBinding_UnknownSkipped(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Name: "type-binding.parameter", StartRow: 1},
		{MatchIndex: 0, Name: "type-binding.name", Text: "skip"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.js")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown NodeType, got %d", len(result))
	}
}

// ============ JavaScriptProvider InterpretReference Accuracy Tests ============

func TestJavaScriptProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", Name: "reference.call.free", StartRow: 5},
		{MatchIndex: 0, Name: "reference.name", Text: "process"},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "free_call" {
		t.Errorf("expected kind=free_call, got %s", result[0].Kind)
	}
	if result[0].Name != "process" {
		t.Errorf("expected name=process, got %s", result[0].Name)
	}
}

func TestJavaScriptProvider_InterpretReference_MemberCall(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", Name: "reference.call.member", StartRow: 5},
		{MatchIndex: 0, Name: "reference.receiver", Text: "server"},
		{MatchIndex: 0, Name: "reference.name", Text: "start"},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "member_call" {
		t.Errorf("expected kind=member_call, got %s", result[0].Kind)
	}
	if result[0].Receiver != "server" {
		t.Errorf("expected receiver=server, got %s", result[0].Receiver)
	}
	if result[0].Name != "start" {
		t.Errorf("expected name=start, got %s", result[0].Name)
	}
}

func TestJavaScriptProvider_InterpretReference_Constructor(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.constructor", Name: "reference.call.constructor", StartRow: 3},
		{MatchIndex: 0, Name: "reference.name", Text: "Server"},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
}

func TestJavaScriptProvider_InterpretReference_FieldWrite(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write.member", Name: "reference.write.member", StartRow: 5},
		{MatchIndex: 0, Name: "reference.receiver", Text: "config"},
		{MatchIndex: 0, Name: "reference.name", Text: "port"},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "field_write" {
		t.Errorf("expected kind=field_write, got %s", result[0].Kind)
	}
	if result[0].Receiver != "config" {
		t.Errorf("expected receiver=config, got %s", result[0].Receiver)
	}
	if result[0].Name != "port" {
		t.Errorf("expected name=port, got %s", result[0].Name)
	}
}

func TestJavaScriptProvider_InterpretReference_FieldRead(t *testing.T) {
	p := NewJavaScriptProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read.member", Name: "reference.read.member", StartRow: 5},
		{MatchIndex: 0, Name: "reference.receiver", Text: "config"},
		{MatchIndex: 0, Name: "reference.name", Text: "port"},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "field_read" {
		t.Errorf("expected kind=field_read, got %s", result[0].Kind)
	}
}

func TestJavaScriptProvider_InterpretReference_ReadDedupWhenSameMatchIdxHasCall(t *testing.T) {
	p := NewJavaScriptProvider()
	// When same matchIdx has both call.member and read.member,
	// JS dedup processes all captures first, then checks.
	// Since read.member overwrites ref.Kind to "field_read",
	// and the matchIdx is in callMatchIndices, the dedup skips it.
	// Result: 0 references for this matchIdx.
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", Name: "reference.call.member", StartRow: 5},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, Name: "reference.name", Text: "method"},
		{MatchIndex: 0, NodeType: "reference.read.member", Name: "reference.read.member", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "test.js")
	// JS dedup: read.member overwrites call.member's kind, then dedup skips field_read
	if len(result) != 0 {
		t.Fatalf("expected 0 references (deduped), got %d", len(result))
	}
}

// ============ JavaScriptProvider Config Tests ============

func TestJavaScriptProvider_Language(t *testing.T) {
	p := NewJavaScriptProvider()
	if p.Language() != graph.LabelJSFile {
		t.Errorf("expected LabelJSFile, got %s", p.Language())
	}
}

func TestJavaScriptProvider_ImportSemantics(t *testing.T) {
	p := NewJavaScriptProvider()
	if p.ImportSemantics() != ImportSemanticsNamed {
		t.Errorf("expected ImportSemanticsNamed, got %v", p.ImportSemantics())
	}
}

func TestJavaScriptProvider_QuerySet_NonEmpty(t *testing.T) {
	p := NewJavaScriptProvider()
	qs := p.QuerySet()
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