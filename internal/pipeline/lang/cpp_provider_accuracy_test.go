package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ CPPProvider Language() and ImportSemantics() Tests ============

func TestCppProvider_Language(t *testing.T) {
	p := NewCPPProvider()
	if p.Language() != graph.LabelCPPFile {
		t.Errorf("expected LabelCPPFile, got %s", p.Language())
	}
}

func TestCppProvider_ImportSemantics(t *testing.T) {
	p := NewCPPProvider()
	if p.ImportSemantics() != ImportSemanticsWildcardTransitive {
		t.Errorf("expected ImportSemanticsWildcardTransitive, got %s", p.ImportSemantics())
	}
}

// ============ CPPProvider InterpretDeclaration Accuracy Tests ============

func TestCppProvider_InterpretDeclaration_Namespace(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.namespace", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "MyNS", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "test.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MyNS" {
		t.Errorf("expected name=MyNS, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelNamespace {
		t.Errorf("expected label=Namespace, got %s", result[0].Label)
	}
	if result[0].FilePath != "test.cpp" {
		t.Errorf("expected filePath=test.cpp, got %s", result[0].FilePath)
	}
}

func TestCppProvider_InterpretDeclaration_Class(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 30},
		{MatchIndex: 0, Name: "declaration.name", Text: "Server", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "server.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=Class, got %s", result[0].Label)
	}
}

func TestCppProvider_InterpretDeclaration_Struct(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.struct", StartRow: 3, EndRow: 15},
		{MatchIndex: 0, Name: "declaration.name", Text: "Node", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "node.cpp")
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

func TestCppProvider_InterpretDeclaration_Enum(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.enum", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Color", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "color.cpp")
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

func TestCppProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", StartRow: 5, EndRow: 15},
		{MatchIndex: 0, Name: "declaration.name", Text: "process", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "process" {
		t.Errorf("expected name=process, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=Function, got %s", result[0].Label)
	}
}

func TestCppProvider_InterpretDeclaration_Method(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.method", StartRow: 10, EndRow: 25},
		{MatchIndex: 0, Name: "declaration.name", Text: "run", StartRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "engine.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "run" {
		t.Errorf("expected name=run, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelMethod {
		t.Errorf("expected label=Method, got %s", result[0].Label)
	}
}

func TestCppProvider_InterpretDeclaration_Field(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.property", StartRow: 7, EndRow: 7},
		{MatchIndex: 0, Name: "declaration.name", Text: "count_", StartRow: 7},
	}
	result := p.InterpretDeclaration(captures, nil, "obj.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "count_" {
		t.Errorf("expected name=count_, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelProperty {
		t.Errorf("expected label=Property, got %s", result[0].Label)
	}
}

func TestCppProvider_InterpretDeclaration_Variable(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", StartRow: 12, EndRow: 12},
		{MatchIndex: 0, Name: "declaration.name", Text: "total", StartRow: 12},
	}
	result := p.InterpretDeclaration(captures, nil, "main.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "total" {
		t.Errorf("expected name=total, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelVariable {
		t.Errorf("expected label=Variable, got %s", result[0].Label)
	}
}

func TestCppProvider_InterpretDeclaration_Macro(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.macro", StartRow: 1, EndRow: 1},
		{MatchIndex: 0, Name: "declaration.name", Text: "MAX_SIZE", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "config.cpp")
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

func TestCppProvider_InterpretDeclaration_Const(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.const", StartRow: 5, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "RED", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "colors.cpp")
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

func TestCppProvider_InterpretDeclaration_Typedef(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.typedef", StartRow: 8, EndRow: 8},
		{MatchIndex: 0, Name: "declaration.name", Text: "Handle", StartRow: 8},
	}
	result := p.InterpretDeclaration(captures, nil, "types.cpp")
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

func TestCppProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "Foo", StartRow: 1},
		{MatchIndex: 1, NodeType: "declaration.struct", StartRow: 25, EndRow: 50},
		{MatchIndex: 1, Name: "declaration.name", Text: "Bar", StartRow: 25},
	}
	result := p.InterpretDeclaration(captures, nil, "test.cpp")
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
	if !names["Bar"] {
		t.Error("expected Bar in results")
	}
}

func TestCppProvider_InterpretDeclaration_UnknownOuterTypeSkipped(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Something", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown outer type, got %d", len(result))
	}
}

func TestCppProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewCPPProvider()
	result := p.InterpretDeclaration(nil, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for nil captures, got %d", len(result))
	}
}

// ============ CPPProvider InterpretImport Accuracy Tests ============

func TestCppProvider_InterpretImport_IncludeStatement(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "#include <stdio.h>", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "stdio.h" {
		t.Errorf("expected path=stdio.h, got %s", result[0].Path)
	}
	if !result[0].IsWildcard {
		t.Error("expected IsWildcard=true for #include statement")
	}
	if result[0].SourceFile != "app.cpp" {
		t.Errorf("expected sourceFile=app.cpp, got %s", result[0].SourceFile)
	}
}

func TestCppProvider_InterpretImport_UsingDecl(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.using-decl", Text: "using std::vector;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].IsWildcard {
		t.Error("expected IsWildcard=false for using-decl")
	}
}

func TestCppProvider_InterpretImport_NonImportCapturesSkipped(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.path", Text: "stdio.h", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "app.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import capture, got %d", len(result))
	}
}

func TestCppProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewCPPProvider()
	result := p.InterpretImport(nil, nil, "app.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for nil captures, got %d", len(result))
	}
}

// ============ CPPProvider InterpretScope Accuracy Tests ============

func TestCppProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "void main() {}", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "/path/to/main.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	// cppModuleName uses parent dir name: filepath.Base(filepath.Dir("/path/to/main.cpp")) = "to"
	if result[0].Name != "to" {
		t.Errorf("expected name=to (from cppModuleName), got %s", result[0].Name)
	}
}

func TestCppProvider_InterpretScope_NamespaceScope(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.namespace", Text: "namespace MyNS {\n  void foo();", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "test.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "namespace" {
		t.Errorf("expected kind=namespace, got %s", result[0].Kind)
	}
	if result[0].Name != "MyNS" {
		t.Errorf("expected name=MyNS, got %s", result[0].Name)
	}
}

func TestCppProvider_InterpretScope_ClassScope(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "class Server {\n  void run();", StartRow: 5, EndRow: 30},
	}
	result := p.InterpretScope(captures, nil, "server.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "class" {
		t.Errorf("expected kind=class, got %s", result[0].Kind)
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
}

func TestCppProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "void process() {\n  return;", StartRow: 10, EndRow: 25},
	}
	result := p.InterpretScope(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "function" {
		t.Errorf("expected kind=function, got %s", result[0].Kind)
	}
	if result[0].Name != "process" {
		t.Errorf("expected name=process, got %s", result[0].Name)
	}
}

func TestCppProvider_InterpretScope_BlockScope(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.block", Text: "{\n  int x = 1;\n}", StartRow: 15, EndRow: 17},
	}
	result := p.InterpretScope(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "block" {
		t.Errorf("expected kind=block, got %s", result[0].Kind)
	}
	if result[0].Name != "" {
		t.Errorf("expected empty name for block scope, got %s", result[0].Name)
	}
}

func TestCppProvider_InterpretScope_UnknownKindSkipped(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

func TestCppProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewCPPProvider()
	result := p.InterpretScope(nil, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for nil captures, got %d", len(result))
	}
}

// ============ CPPProvider InterpretTypeBinding Accuracy Tests ============

func TestCppProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "name"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "string"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", result[0].Kind)
	}
	if result[0].TypeName != "string" {
		t.Errorf("expected typeName=string, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_Assignment(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.assignment", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
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

func TestCppProvider_InterpretTypeBinding_Annotation(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.annotation", StartRow: 8},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Server"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "annotation" {
		t.Errorf("expected kind=annotation, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Server" {
		t.Errorf("expected typeName=Server, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", StartRow: 12},
		{MatchIndex: 0, Name: "type-binding.type", Text: "vector"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].TypeName != "vector" {
		t.Errorf("expected typeName=vector, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_Alias(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.alias", StartRow: 7},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Container"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "alias" {
		t.Errorf("expected kind=alias, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Container" {
		t.Errorf("expected typeName=Container, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_Return(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", StartRow: 15},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Config"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "return" {
		t.Errorf("expected kind=return, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Config" {
		t.Errorf("expected typeName=Config, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_Field(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.field", StartRow: 20},
		{MatchIndex: 0, Name: "type-binding.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "field" {
		t.Errorf("expected kind=field, got %s", result[0].Kind)
	}
	if result[0].TypeName != "int" {
		t.Errorf("expected typeName=int, got %s", result[0].TypeName)
	}
}

func TestCppProvider_InterpretTypeBinding_UnknownKindSkipped(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.unknown", StartRow: 5},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown kind, got %d", len(result))
	}
}

func TestCppProvider_InterpretTypeBinding_EmptyCaptures(t *testing.T) {
	p := NewCPPProvider()
	result := p.InterpretTypeBinding(nil, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for nil captures, got %d", len(result))
	}
}

// ============ CPPProvider InterpretReference Accuracy Tests ============

func TestCppProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "printf"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "free_call" {
		t.Errorf("expected kind=free_call, got %s", ref.Kind)
	}
	if ref.Name != "printf" {
		t.Errorf("expected name=printf, got %s", ref.Name)
	}
	if ref.Receiver != "" {
		t.Errorf("expected empty receiver for free_call, got %s", ref.Receiver)
	}
	if ref.FilePath != "app.cpp" {
		t.Errorf("expected filePath=app.cpp, got %s", ref.FilePath)
	}
}

func TestCppProvider_InterpretReference_MemberCall(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, Name: "reference.name", Text: "method"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "member_call" {
		t.Errorf("expected kind=member_call, got %s", ref.Kind)
	}
	if ref.Receiver != "obj" {
		t.Errorf("expected receiver=obj, got %s", ref.Receiver)
	}
	if ref.Name != "method" {
		t.Errorf("expected name=method, got %s", ref.Name)
	}
}

func TestCppProvider_InterpretReference_QualifiedCall(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.qualified", StartRow: 20},
		{MatchIndex: 0, Name: "reference.receiver", Text: "std"},
		{MatchIndex: 0, Name: "reference.name", Text: "sort"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "qualified_call" {
		t.Errorf("expected kind=qualified_call, got %s", ref.Kind)
	}
	if ref.Receiver != "std" {
		t.Errorf("expected receiver=std, got %s", ref.Receiver)
	}
	if ref.Name != "sort" {
		t.Errorf("expected name=sort, got %s", ref.Name)
	}
}

func TestCppProvider_InterpretReference_Constructor(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.constructor", StartRow: 25},
		{MatchIndex: 0, Name: "reference.name", Text: "vector"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", ref.Kind)
	}
	if ref.Name != "vector" {
		t.Errorf("expected name=vector, got %s", ref.Name)
	}
}

func TestCppProvider_InterpretReference_FieldRead(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read", StartRow: 30},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, Name: "reference.name", Text: "x"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "field_read" {
		t.Errorf("expected kind=field_read, got %s", ref.Kind)
	}
	if ref.Name != "x" {
		t.Errorf("expected name=x, got %s", ref.Name)
	}
}

func TestCppProvider_InterpretReference_FieldWrite(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write", StartRow: 35},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, Name: "reference.name", Text: "y"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "field_write" {
		t.Errorf("expected kind=field_write, got %s", ref.Kind)
	}
	if ref.Name != "y" {
		t.Errorf("expected name=y, got %s", ref.Name)
	}
}

func TestCppProvider_InterpretReference_FieldReadDedupWhenMemberCall(t *testing.T) {
	p := NewCPPProvider()
	// Same matchIdx has both reference.read and reference.call.member.
	// When both are present, the code iterates captures in order and the last
	// matching NodeType wins for ref.Kind. Place reference.call.member last
	// so that ref.Kind = "member_call" (not "field_read"), and the dedup
	// check (which only skips field_read) does not filter it.
	// This simulates tree-sitter behavior where a field_expression inside
	// a call_expression triggers both patterns.
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "method"},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "method"},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference (dedup), got %d", len(result))
	}
	if result[0].Kind != "member_call" {
		t.Errorf("expected kind=member_call (read should be deduped), got %s", result[0].Kind)
	}
}

func TestCppProvider_InterpretReference_FieldReadNoDedupWhenNoMemberCall(t *testing.T) {
	p := NewCPPProvider()
	// reference.read without reference.call.member on same matchIdx → kept
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "field"},
		{MatchIndex: 0, Name: "reference.receiver", Text: "obj"},
	}
	result := p.InterpretReference(captures, nil, "app.cpp")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "field_read" {
		t.Errorf("expected kind=field_read, got %s", result[0].Kind)
	}
}

func TestCppProvider_InterpretReference_UnknownKindSkipped(t *testing.T) {
	p := NewCPPProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 references for unknown kind, got %d", len(result))
	}
}

func TestCppProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewCPPProvider()
	result := p.InterpretReference(nil, nil, "test.cpp")
	if len(result) != 0 {
		t.Errorf("expected 0 references for nil captures, got %d", len(result))
	}
}