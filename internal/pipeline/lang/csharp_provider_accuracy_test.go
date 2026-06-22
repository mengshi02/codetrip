package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ CSharpProvider Language() and ImportSemantics() Tests ============

func TestCSharpProvider_Language(t *testing.T) {
	p := NewCSharpProvider()
	if p.Language() != graph.LabelCSharpFile {
		t.Errorf("expected LabelCSharpFile, got %s", p.Language())
	}
}

func TestCSharpProvider_ImportSemantics(t *testing.T) {
	p := NewCSharpProvider()
	if p.ImportSemantics() != ImportSemanticsNamed {
		t.Errorf("expected ImportSemanticsNamed, got %s", p.ImportSemantics())
	}
}

// ============ CSharpProvider InterpretDeclaration Accuracy Tests ============

func TestCSharpProvider_InterpretDeclaration_Class(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 30},
		{MatchIndex: 0, Name: "declaration.name", Text: "Server", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Server.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=Class, got %s", result[0].Label)
	}
	if result[0].FilePath != "Server.cs" {
		t.Errorf("expected filePath=Server.cs, got %s", result[0].FilePath)
	}
}

func TestCSharpProvider_InterpretDeclaration_Interface(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.interface", StartRow: 1, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "IRepository", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "IRepository.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "IRepository" {
		t.Errorf("expected name=IRepository, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelInterface {
		t.Errorf("expected label=Interface, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Struct(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.struct", StartRow: 3, EndRow: 15},
		{MatchIndex: 0, Name: "declaration.name", Text: "Point", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "Point.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Point" {
		t.Errorf("expected name=Point, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelStruct {
		t.Errorf("expected label=Struct, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Record(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.record", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Person", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Person.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Person" {
		t.Errorf("expected name=Person, got %s", result[0].Name)
	}
	// record maps to LabelClass
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=Class for record, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Enum(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.enum", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Color", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Color.cs")
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

func TestCSharpProvider_InterpretDeclaration_Method(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.method", StartRow: 5, EndRow: 15},
		{MatchIndex: 0, Name: "declaration.name", Text: "Process", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "Service.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Process" {
		t.Errorf("expected name=Process, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelMethod {
		t.Errorf("expected label=Method, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Constructor(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.constructor", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "MyClass", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "MyClass.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MyClass" {
		t.Errorf("expected name=MyClass, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConstructor {
		t.Errorf("expected label=Constructor, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.function", StartRow: 8, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "Helper", StartRow: 8},
	}
	result := p.InterpretDeclaration(captures, nil, "Utils.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Helper" {
		t.Errorf("expected name=Helper, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelFunction {
		t.Errorf("expected label=Function, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Property(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.property", StartRow: 3, EndRow: 3},
		{MatchIndex: 0, Name: "declaration.name", Text: "Name", StartRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "Model.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Name" {
		t.Errorf("expected name=Name, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelProperty {
		t.Errorf("expected label=Property, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_Variable(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", StartRow: 10, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "count", StartRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "App.cs")
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

func TestCSharpProvider_InterpretDeclaration_Namespace(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.namespace", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "MyApp.Services", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Service.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MyApp.Services" {
		t.Errorf("expected name=MyApp.Services, got %s", result[0].Name)
	}
	// namespace maps to LabelPackage
	if result[0].Label != graph.LabelPackage {
		t.Errorf("expected label=Package for namespace, got %s", result[0].Label)
	}
}

func TestCSharpProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewCSharpProvider()
	// Two class declarations — ensure match 0's name doesn't leak into match 1
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "Foo", StartRow: 1},
		{MatchIndex: 1, NodeType: "declaration.class", StartRow: 25, EndRow: 50},
		{MatchIndex: 1, Name: "declaration.name", Text: "Bar", StartRow: 25},
	}
	result := p.InterpretDeclaration(captures, nil, "Test.cs")
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

func TestCSharpProvider_InterpretDeclaration_UnknownOuterTypeSkipped(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Something", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown outer type, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewCSharpProvider()
	result := p.InterpretDeclaration(nil, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for nil captures, got %d", len(result))
	}
}

// ============ CSharpProvider InterpretImport Accuracy Tests ============

func TestCSharpProvider_InterpretImport_StandardUsing(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "using System;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "System" {
		t.Errorf("expected path=System, got %s", result[0].Path)
	}
	if result[0].SourceFile != "App.cs" {
		t.Errorf("expected sourceFile=App.cs, got %s", result[0].SourceFile)
	}
}

func TestCSharpProvider_InterpretImport_NonImportStatementFiltering(t *testing.T) {
	p := NewCSharpProvider()
	// Non-import.statement captures should be filtered out
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.path", Text: "System", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "App.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import.statement capture, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretImport_MultipleImportsDifferentMatches(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "using System;", StartRow: 3},
		{MatchIndex: 1, NodeType: "import.statement", Text: "using System.Collections.Generic;", StartRow: 4},
	}
	result := p.InterpretImport(captures, nil, "App.cs")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["System"] {
		t.Error("expected System in imports")
	}
	if !paths["System.Collections.Generic"] {
		t.Error("expected System.Collections.Generic in imports")
	}
}

func TestCSharpProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewCSharpProvider()
	result := p.InterpretImport(nil, nil, "App.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for nil captures, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretImport_StaticUsing(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "using static System.Math;", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "System.Math" {
		t.Errorf("expected path=System.Math, got %s", result[0].Path)
	}
}

func TestCSharpProvider_InterpretImport_AliasUsing(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "using M = System.Math;", StartRow: 1},
	}
	result := p.InterpretImport(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "System.Math" {
		t.Errorf("expected path=System.Math, got %s", result[0].Path)
	}
}

// ============ CSharpProvider InterpretScope Accuracy Tests ============

func TestCSharpProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "compilation_unit", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "/path/to/Models/App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "Models" {
		t.Errorf("expected name=Models (from csharpModuleName), got %s", result[0].Name)
	}
}

func TestCSharpProvider_InterpretScope_NamespaceScope(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.namespace", Text: "namespace MyApp.Services {", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "Service.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "namespace" {
		t.Errorf("expected kind=namespace, got %s", result[0].Kind)
	}
	if result[0].Name != "MyApp.Services" {
		t.Errorf("expected name=MyApp.Services, got %s", result[0].Name)
	}
}

func TestCSharpProvider_InterpretScope_ClassScope(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "class Server {\n  void Run()", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "Server.cs")
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

func TestCSharpProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "public void Process() {\n  return;", StartRow: 5, EndRow: 15},
	}
	result := p.InterpretScope(captures, nil, "Service.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "function" {
		t.Errorf("expected kind=function, got %s", result[0].Kind)
	}
	if result[0].Name != "Process" {
		t.Errorf("expected name=Process, got %s", result[0].Name)
	}
}

func TestCSharpProvider_InterpretScope_UnknownKindSkipped(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewCSharpProvider()
	result := p.InterpretScope(nil, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for nil captures, got %d", len(result))
	}
}

// ============ CSharpProvider InterpretTypeBinding Accuracy Tests ============

func TestCSharpProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "name"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "String"},
	}
	result := p.InterpretTypeBinding(captures, nil, "Service.cs")
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

func TestCSharpProvider_InterpretTypeBinding_Annotation(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.annotation", StartRow: 8},
		{MatchIndex: 0, Name: "type-binding.name", Text: "handler"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "IHandler"},
	}
	result := p.InterpretTypeBinding(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "annotation" {
		t.Errorf("expected kind=annotation, got %s", result[0].Kind)
	}
	if result[0].TypeName != "IHandler" {
		t.Errorf("expected typeName=IHandler, got %s", result[0].TypeName)
	}
}

func TestCSharpProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Server"},
	}
	result := p.InterpretTypeBinding(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Server" {
		t.Errorf("expected typeName=Server, got %s", result[0].TypeName)
	}
	// C# provider does NOT set BoundNode
	if result[0].BoundNode != "" {
		t.Errorf("expected empty boundNode for C# type-binding, got %s", result[0].BoundNode)
	}
}

func TestCSharpProvider_InterpretTypeBinding_Return(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "getConfig"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Config"},
	}
	result := p.InterpretTypeBinding(captures, nil, "Service.cs")
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

func TestCSharpProvider_InterpretTypeBinding_UnknownKindSkipped(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.unknown", StartRow: 5},
	}
	result := p.InterpretTypeBinding(captures, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown kind, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretTypeBinding_EmptyCaptures(t *testing.T) {
	p := NewCSharpProvider()
	result := p.InterpretTypeBinding(nil, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for nil captures, got %d", len(result))
	}
}

// ============ CSharpProvider InterpretReference Accuracy Tests ============

func TestCSharpProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "Console.WriteLine"},
	}
	result := p.InterpretReference(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "free_call" {
		t.Errorf("expected kind=free_call, got %s", ref.Kind)
	}
	if ref.Name != "Console.WriteLine" {
		t.Errorf("expected name=Console.WriteLine, got %s", ref.Name)
	}
	if ref.Receiver != "" {
		t.Errorf("expected empty receiver for free_call, got %s", ref.Receiver)
	}
	if ref.FilePath != "App.cs" {
		t.Errorf("expected filePath=App.cs, got %s", ref.FilePath)
	}
}

func TestCSharpProvider_InterpretReference_MemberCallWithReceiver(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 0, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 0, Name: "reference.name", Text: "Query"},
	}
	result := p.InterpretReference(captures, nil, "Service.cs")
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
	if ref.Name != "Query" {
		t.Errorf("expected name=Query, got %s", ref.Name)
	}
}

func TestCSharpProvider_InterpretReference_ConstructorCall(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.constructor", StartRow: 20},
		{MatchIndex: 0, Name: "reference.name", Text: "List"},
	}
	result := p.InterpretReference(captures, nil, "App.cs")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", ref.Kind)
	}
	if ref.Name != "List" {
		t.Errorf("expected name=List, got %s", ref.Name)
	}
}

func TestCSharpProvider_InterpretReference_FieldWrite(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.write.member", StartRow: 25},
		{MatchIndex: 0, Name: "reference.receiver", Text: "config"},
		{MatchIndex: 0, Name: "reference.name", Text: "Port"},
	}
	result := p.InterpretReference(captures, nil, "App.cs")
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
	if ref.Name != "Port" {
		t.Errorf("expected name=Port, got %s", ref.Name)
	}
}

func TestCSharpProvider_InterpretReference_UnknownKindSkipped(t *testing.T) {
	p := NewCSharpProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 references for unknown kind, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewCSharpProvider()
	result := p.InterpretReference(nil, nil, "Test.cs")
	if len(result) != 0 {
		t.Errorf("expected 0 references for nil captures, got %d", len(result))
	}
}

func TestCSharpProvider_InterpretReference_MultipleRefsIsolated(t *testing.T) {
	p := NewCSharpProvider()
	// Three different reference types across three matches
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "DoWork"},
		{MatchIndex: 1, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 1, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 1, Name: "reference.name", Text: "Query"},
		{MatchIndex: 2, NodeType: "reference.call.constructor", StartRow: 20},
		{MatchIndex: 2, Name: "reference.name", Text: "Dictionary"},
	}
	result := p.InterpretReference(captures, nil, "App.cs")
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
	if !foundKinds["constructor"] {
		t.Error("expected constructor reference")
	}
}