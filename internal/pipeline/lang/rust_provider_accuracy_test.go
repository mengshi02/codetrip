package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ RustProvider Language() and ImportSemantics() Tests ============

func TestRustProvider_Language(t *testing.T) {
	p := NewRustProvider()
	if p.Language() != graph.LabelRustFile {
		t.Errorf("expected LabelRustFile, got %s", p.Language())
	}
}

func TestRustProvider_ImportSemantics(t *testing.T) {
	p := NewRustProvider()
	if p.ImportSemantics() != ImportSemanticsNamed {
		t.Errorf("expected ImportSemanticsNamed, got %s", p.ImportSemantics())
	}
}

// ============ RustProvider InterpretDeclaration Accuracy Tests ============

func TestRustProvider_InterpretDeclaration_StructDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.struct", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Server", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "server.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelStruct {
		t.Errorf("expected label=Struct, got %s", result[0].Label)
	}
	if result[0].FilePath != "server.rs" {
		t.Errorf("expected filePath=server.rs, got %s", result[0].FilePath)
	}
}

func TestRustProvider_InterpretDeclaration_TraitDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.trait", StartRow: 1, EndRow: 10},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Drawable", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "drawable.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Drawable" {
		t.Errorf("expected name=Drawable, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelTrait {
		t.Errorf("expected label=Trait, got %s", result[0].Label)
	}
}

func TestRustProvider_InterpretDeclaration_EnumDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.enum", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Color", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "color.rs")
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

func TestRustProvider_InterpretDeclaration_FunctionDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", StartRow: 5, EndRow: 15},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "process", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "main.rs")
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

func TestRustProvider_InterpretDeclaration_FieldDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.property", StartRow: 3, EndRow: 3},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "name", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "user.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "name" {
		t.Errorf("expected name=name, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelProperty {
		t.Errorf("expected label=Property, got %s", result[0].Label)
	}
}

func TestRustProvider_InterpretDeclaration_VariableDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", StartRow: 10, EndRow: 10},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "count", StartRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "app.rs")
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

func TestRustProvider_InterpretDeclaration_ConstDef(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.const", StartRow: 1, EndRow: 1},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "MAX_SIZE", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "config.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MAX_SIZE" {
		t.Errorf("expected name=MAX_SIZE, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConst {
		t.Errorf("expected label=Const, got %s", result[0].Label)
	}
}

func TestRustProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.struct", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Foo", StartRow: 1},
		{MatchIndex: 1, NodeType: "declaration.struct", StartRow: 25, EndRow: 50},
		{MatchIndex: 1, NodeType: "declaration.name", Name: "declaration.name", Text: "Bar", StartRow: 25},
	}
	result := p.InterpretDeclaration(captures, nil, "test.rs")
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

func TestRustProvider_InterpretDeclaration_UnknownOuterTypeSkipped(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Something", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown outer type, got %d", len(result))
	}
}

func TestRustProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewRustProvider()
	result := p.InterpretDeclaration(nil, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for nil captures, got %d", len(result))
	}
}

// ============ RustProvider InterpretImport Accuracy Tests ============

func TestRustProvider_InterpretImport_StandardUse(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "use std::collections::HashMap;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "main.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "std::collections::HashMap" {
		t.Errorf("expected path=std::collections::HashMap, got %s", result[0].Path)
	}
	if result[0].IsWildcard {
		t.Error("expected IsWildcard=false for standard import")
	}
	if result[0].SourceFile != "main.rs" {
		t.Errorf("expected sourceFile=main.rs, got %s", result[0].SourceFile)
	}
}

func TestRustProvider_InterpretImport_WildcardUse(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "use std::io::*;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "main.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	// rustExtractUsePath already strips ::*, so IsWildcard check on the
	// returned path (which no longer ends with ::*) is false.
	if result[0].Path != "std::io" {
		t.Errorf("expected path=std::io (::* stripped), got %s", result[0].Path)
	}
	if result[0].IsWildcard {
		t.Error("expected IsWildcard=false (rustExtractUsePath already strips ::*)")
	}
}

func TestRustProvider_InterpretImport_NonImportStatementSkipped(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.path", Text: "std::collections::HashMap", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "main.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import.statement capture, got %d", len(result))
	}
}

func TestRustProvider_InterpretImport_MultipleImports(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "use std::collections::HashMap;", StartRow: 3},
		{MatchIndex: 1, NodeType: "import.statement", Text: "use std::io::Read;", StartRow: 4},
	}
	result := p.InterpretImport(captures, nil, "main.rs")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["std::collections::HashMap"] {
		t.Error("expected std::collections::HashMap in imports")
	}
	if !paths["std::io::Read"] {
		t.Error("expected std::io::Read in imports")
	}
}

func TestRustProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewRustProvider()
	result := p.InterpretImport(nil, nil, "main.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for nil captures, got %d", len(result))
	}
}

// ============ RustProvider InterpretScope Accuracy Tests ============

func TestRustProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "src/lib.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "lib" {
		t.Errorf("expected name=lib (from rustModuleName), got %s", result[0].Name)
	}
}

func TestRustProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "fn process() {\n    let x = 1;", StartRow: 5, EndRow: 15},
	}
	result := p.InterpretScope(captures, nil, "main.rs")
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

func TestRustProvider_InterpretScope_ClassScope(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "struct Server {\n    name: String,", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "server.rs")
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

func TestRustProvider_InterpretScope_BlockScope(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.block", Text: "{\n    let x = 1;", StartRow: 5, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "main.rs")
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

func TestRustProvider_InterpretScope_UnknownKindSkipped(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

func TestRustProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewRustProvider()
	result := p.InterpretScope(nil, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for nil captures, got %d", len(result))
	}
}

// ============ RustProvider InterpretTypeBinding Accuracy Tests ============

func TestRustProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.param.name", Text: "name", NodeType: "type-binding.param.name"},
		{MatchIndex: 0, Name: "type-binding.param.type", Text: "String", NodeType: "type-binding.param.type"},
	}
	result := p.InterpretTypeBinding(captures, nil, "service.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", result[0].Kind)
	}
	if result[0].TypeName != "String" {
		t.Errorf("expected typeName=String, got %s", result[0].TypeName)
	}
}

func TestRustProvider_InterpretTypeBinding_Variable(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.variable", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.var.name", Text: "count", NodeType: "type-binding.var.name"},
		{MatchIndex: 0, Name: "type-binding.var.type", Text: "i32", NodeType: "type-binding.var.type"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "variable" {
		t.Errorf("expected kind=variable, got %s", result[0].Kind)
	}
	if result[0].TypeName != "i32" {
		t.Errorf("expected typeName=i32, got %s", result[0].TypeName)
	}
}

func TestRustProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", StartRow: 8},
		{MatchIndex: 0, Name: "type-binding.var.name", Text: "server", NodeType: "type-binding.var.name"},
		{MatchIndex: 0, Name: "type-binding.var.type", Text: "Server", NodeType: "type-binding.var.type"},
	}
	result := p.InterpretTypeBinding(captures, nil, "main.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Server" {
		t.Errorf("expected typeName=Server, got %s", result[0].TypeName)
	}
}

func TestRustProvider_InterpretTypeBinding_CallReturn(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.call-return", StartRow: 12},
		{MatchIndex: 0, Name: "type-binding.var.name", Text: "result", NodeType: "type-binding.var.name"},
		{MatchIndex: 0, Name: "type-binding.var.type", Text: "parse", NodeType: "type-binding.var.type"},
	}
	result := p.InterpretTypeBinding(captures, nil, "parser.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "call-return" {
		t.Errorf("expected kind=call-return, got %s", result[0].Kind)
	}
	if result[0].TypeName != "parse" {
		t.Errorf("expected typeName=parse, got %s", result[0].TypeName)
	}
}

func TestRustProvider_InterpretTypeBinding_Alias(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.alias", StartRow: 7},
		{MatchIndex: 0, Name: "type-binding.alias.name", Text: "handler", NodeType: "type-binding.alias.name"},
		{MatchIndex: 0, Name: "type-binding.alias.target", Text: "callback", NodeType: "type-binding.alias.target"},
	}
	result := p.InterpretTypeBinding(captures, nil, "app.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "alias" {
		t.Errorf("expected kind=alias, got %s", result[0].Kind)
	}
	if result[0].TypeName != "callback" {
		t.Errorf("expected typeName=callback, got %s", result[0].TypeName)
	}
}

func TestRustProvider_InterpretTypeBinding_Return(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.return.type", Text: "Config", NodeType: "type-binding.return.type"},
	}
	result := p.InterpretTypeBinding(captures, nil, "config.rs")
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

func TestRustProvider_InterpretTypeBinding_UnknownKindSkipped(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.unknown", StartRow: 5},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown kind, got %d", len(result))
	}
}

func TestRustProvider_InterpretTypeBinding_EmptyCaptures(t *testing.T) {
	p := NewRustProvider()
	result := p.InterpretTypeBinding(nil, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for nil captures, got %d", len(result))
	}
}

// ============ RustProvider InterpretReference Accuracy Tests ============

func TestRustProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "println"},
	}
	result := p.InterpretReference(captures, nil, "main.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "free_call" {
		t.Errorf("expected kind=free_call, got %s", ref.Kind)
	}
	if ref.Name != "println" {
		t.Errorf("expected name=println, got %s", ref.Name)
	}
	if ref.Receiver != "" {
		t.Errorf("expected empty receiver for free_call, got %s", ref.Receiver)
	}
	if ref.FilePath != "main.rs" {
		t.Errorf("expected filePath=main.rs, got %s", ref.FilePath)
	}
}

func TestRustProvider_InterpretReference_MemberCallWithReceiver(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 0, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 0, Name: "reference.name", Text: "query"},
	}
	result := p.InterpretReference(captures, nil, "service.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "member_call" {
		t.Errorf("expected kind=member_call, got %s", ref.Kind)
	}
	if ref.Receiver != "db" {
		t.Errorf("expected receiver=db, got %s", ref.Receiver)
	}
	if ref.Name != "query" {
		t.Errorf("expected name=query, got %s", ref.Name)
	}
}

func TestRustProvider_InterpretReference_Constructor(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.constructor", StartRow: 20},
		{MatchIndex: 0, Name: "reference.name", Text: "HashMap"},
	}
	result := p.InterpretReference(captures, nil, "app.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", ref.Kind)
	}
	if ref.Name != "HashMap" {
		t.Errorf("expected name=HashMap, got %s", ref.Name)
	}
}

func TestRustProvider_InterpretReference_FieldReadWithReceiver(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.read", StartRow: 12},
		{MatchIndex: 0, Name: "reference.receiver", Text: "user"},
		{MatchIndex: 0, Name: "reference.name", Text: "name"},
	}
	result := p.InterpretReference(captures, nil, "model.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "field_read" {
		t.Errorf("expected kind=field_read, got %s", ref.Kind)
	}
	if ref.Receiver != "user" {
		t.Errorf("expected receiver=user, got %s", ref.Receiver)
	}
	if ref.Name != "name" {
		t.Errorf("expected name=name, got %s", ref.Name)
	}
}

func TestRustProvider_InterpretReference_FieldWriteWithReceiver(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write", StartRow: 18},
		{MatchIndex: 0, Name: "reference.receiver", Text: "config"},
		{MatchIndex: 0, Name: "reference.name", Text: "timeout"},
	}
	result := p.InterpretReference(captures, nil, "config.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "field_write" {
		t.Errorf("expected kind=field_write, got %s", ref.Kind)
	}
	if ref.Receiver != "config" {
		t.Errorf("expected receiver=config, got %s", ref.Receiver)
	}
	if ref.Name != "timeout" {
		t.Errorf("expected name=timeout, got %s", ref.Name)
	}
}

func TestRustProvider_InterpretReference_Macro(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.macro", StartRow: 5},
		{MatchIndex: 0, Name: "reference.name", Text: "vec"},
	}
	result := p.InterpretReference(captures, nil, "main.rs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "macro" {
		t.Errorf("expected kind=macro, got %s", ref.Kind)
	}
	if ref.Name != "vec" {
		t.Errorf("expected name=vec, got %s", ref.Name)
	}
}

func TestRustProvider_InterpretReference_MultipleRefsIsolated(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "println"},
		{MatchIndex: 1, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 1, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 1, Name: "reference.name", Text: "query"},
		{MatchIndex: 2, NodeType: "reference.macro", StartRow: 20},
		{MatchIndex: 2, Name: "reference.name", Text: "vec"},
	}
	result := p.InterpretReference(captures, nil, "app.rs")
	if len(result) != 3 {
		t.Fatalf("expected 3 references, got %d", len(result))
	}
	foundKinds := map[string]bool{}
	for _, ref := range result {
		foundKinds[ref.Kind] = true
	}
	if !foundKinds["free_call"] {
		t.Error("expected free_call reference")
	}
	if !foundKinds["member_call"] {
		t.Error("expected member_call reference")
	}
	if !foundKinds["macro"] {
		t.Error("expected macro reference")
	}
}

func TestRustProvider_InterpretReference_UnknownKindSkipped(t *testing.T) {
	p := NewRustProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 references for unknown kind, got %d", len(result))
	}
}

func TestRustProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewRustProvider()
	result := p.InterpretReference(nil, nil, "test.rs")
	if len(result) != 0 {
		t.Errorf("expected 0 references for nil captures, got %d", len(result))
	}
}