package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ JavaProvider Language() and ImportSemantics() Tests ============

func TestJavaProvider_Language(t *testing.T) {
	p := NewJavaProvider()
	if p.Language() != graph.LabelJavaFile {
		t.Errorf("expected LabelJavaFile, got %s", p.Language())
	}
}

func TestJavaProvider_ImportSemantics(t *testing.T) {
	p := NewJavaProvider()
	if p.ImportSemantics() != ImportSemanticsNamed {
		t.Errorf("expected ImportSemanticsNamed, got %s", p.ImportSemantics())
	}
}

// ============ JavaProvider InterpretDeclaration Accuracy Tests ============

func TestJavaProvider_InterpretDeclaration_ClassDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", Name: "declaration.class", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, NodeType: "declaration.name", Name: "declaration.name", Text: "Server", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Server.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=class, got %s", result[0].Label)
	}
	if result[0].FilePath != "Server.java" {
		t.Errorf("expected filePath=Server.java, got %s", result[0].FilePath)
	}
}

func TestJavaProvider_InterpretDeclaration_InterfaceDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.interface", StartRow: 1, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "Runnable", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Runnable.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Runnable" {
		t.Errorf("expected name=Runnable, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelInterface {
		t.Errorf("expected label=interface, got %s", result[0].Label)
	}
}

func TestJavaProvider_InterpretDeclaration_EnumDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.enum", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Color", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Color.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Color" {
		t.Errorf("expected name=Color, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelEnum {
		t.Errorf("expected label=enum, got %s", result[0].Label)
	}
}

func TestJavaProvider_InterpretDeclaration_MethodDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.method", StartRow: 5, EndRow: 15},
		{MatchIndex: 0, Name: "declaration.name", Text: "process", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "Service.java")
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

func TestJavaProvider_InterpretDeclaration_VariableDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.variable", StartRow: 10, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "count", StartRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "App.java")
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

func TestJavaProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewJavaProvider()
	// Two class declarations — ensure match 0's name doesn't leak into match 1
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.class", StartRow: 1, EndRow: 20},
		{MatchIndex: 0, Name: "declaration.name", Text: "Foo", StartRow: 1},
		{MatchIndex: 1, NodeType: "declaration.class", StartRow: 25, EndRow: 50},
		{MatchIndex: 1, Name: "declaration.name", Text: "Bar", StartRow: 25},
	}
	result := p.InterpretDeclaration(captures, nil, "Test.java")
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

func TestJavaProvider_InterpretDeclaration_UnknownOuterTypeSkipped(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.unknown", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Something", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for unknown outer type, got %d", len(result))
	}
}

func TestJavaProvider_InterpretDeclaration_ConstructorDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.constructor", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "declaration.name", Text: "MyClass", StartRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "MyClass.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MyClass" {
		t.Errorf("expected name=MyClass, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConstructor {
		t.Errorf("expected label=constructor, got %s", result[0].Label)
	}
}

func TestJavaProvider_InterpretDeclaration_RecordDef(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "declaration.record", StartRow: 1, EndRow: 5},
		{MatchIndex: 0, Name: "declaration.name", Text: "Point", StartRow: 1},
	}
	result := p.InterpretDeclaration(captures, nil, "Point.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "Point" {
		t.Errorf("expected name=Point, got %s", result[0].Name)
	}
	// record maps to LabelClass
	if result[0].Label != graph.LabelClass {
		t.Errorf("expected label=class for record, got %s", result[0].Label)
	}
}

func TestJavaProvider_InterpretDeclaration_EmptyCaptures(t *testing.T) {
	p := NewJavaProvider()
	result := p.InterpretDeclaration(nil, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 symbols for nil captures, got %d", len(result))
	}
}

// ============ JavaProvider InterpretImport Accuracy Tests ============

func TestJavaProvider_InterpretImport_StandardImport(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import java.util.List;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "java.util.List" {
		t.Errorf("expected path=java.util.List, got %s", result[0].Path)
	}
	if result[0].IsWildcard {
		t.Error("expected IsWildcard=false for standard import")
	}
	if result[0].SourceFile != "App.java" {
		t.Errorf("expected sourceFile=App.java, got %s", result[0].SourceFile)
	}
}

func TestJavaProvider_InterpretImport_WildcardImport(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import java.util.*;", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Path != "java.util" {
		t.Errorf("expected path=java.util (.* stripped), got %s", result[0].Path)
	}
	if !result[0].IsWildcard {
		t.Error("expected IsWildcard=true for wildcard import")
	}
}

func TestJavaProvider_InterpretImport_NonImportCapturesSkipped(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.path", Text: "java.util.List", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "App.java")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for non-import.statement capture, got %d", len(result))
	}
}

func TestJavaProvider_InterpretImport_MultipleImportsDifferentMatches(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.statement", Text: "import java.util.List;", StartRow: 3},
		{MatchIndex: 1, NodeType: "import.statement", Text: "import java.io.IOException;", StartRow: 4},
	}
	result := p.InterpretImport(captures, nil, "App.java")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["java.util.List"] {
		t.Error("expected java.util.List in imports")
	}
	if !paths["java.io.IOException"] {
		t.Error("expected java.io.IOException in imports")
	}
}

func TestJavaProvider_InterpretImport_EmptyCaptures(t *testing.T) {
	p := NewJavaProvider()
	result := p.InterpretImport(nil, nil, "App.java")
	if len(result) != 0 {
		t.Errorf("expected 0 imports for nil captures, got %d", len(result))
	}
}

// ============ JavaProvider InterpretScope Accuracy Tests ============

func TestJavaProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "public class App {", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "/path/to/App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	if result[0].Name != "App" {
		t.Errorf("expected name=App (from javaModuleName), got %s", result[0].Name)
	}
}

func TestJavaProvider_InterpretScope_ClassScope(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "class Server {\n  void run()", StartRow: 1, EndRow: 50},
	}
	result := p.InterpretScope(captures, nil, "Server.java")
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

func TestJavaProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Text: "public void process() {\n  return;", StartRow: 5, EndRow: 15},
	}
	result := p.InterpretScope(captures, nil, "Service.java")
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

func TestJavaProvider_InterpretScope_UnknownKindSkipped(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

func TestJavaProvider_InterpretScope_InterfaceScope(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Text: "interface Runnable {\n  void run();", StartRow: 1, EndRow: 5},
	}
	result := p.InterpretScope(captures, nil, "Runnable.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "class" {
		t.Errorf("expected kind=class, got %s", result[0].Kind)
	}
	if result[0].Name != "Runnable" {
		t.Errorf("expected name=Runnable, got %s", result[0].Name)
	}
}

func TestJavaProvider_InterpretScope_EmptyCaptures(t *testing.T) {
	p := NewJavaProvider()
	result := p.InterpretScope(nil, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for nil captures, got %d", len(result))
	}
}

// ============ JavaProvider InterpretTypeBinding Accuracy Tests ============

func TestJavaProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "name"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "String"},
	}
	result := p.InterpretTypeBinding(captures, nil, "Service.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", result[0].Kind)
	}
	if result[0].TypeName != "String" {
		t.Errorf("expected typeName=String, got %s", result[0].TypeName)
	}
	if result[0].BoundNode != "name" {
		t.Errorf("expected boundNode=name, got %s", result[0].BoundNode)
	}
}

func TestJavaProvider_InterpretTypeBinding_Return(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.name", Text: "getConfig"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Config"},
	}
	result := p.InterpretTypeBinding(captures, nil, "Service.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "return" {
		t.Errorf("expected kind=return, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Config" {
		t.Errorf("expected typeName=Config, got %s", result[0].TypeName)
	}
	if result[0].BoundNode != "getConfig" {
		t.Errorf("expected boundNode=getConfig, got %s", result[0].BoundNode)
	}
}

func TestJavaProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Server"},
	}
	result := p.InterpretTypeBinding(captures, nil, "App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Server" {
		t.Errorf("expected typeName=Server, got %s", result[0].TypeName)
	}
	// constructor binding has no BoundNode (no type-binding.name)
	if result[0].BoundNode != "" {
		t.Errorf("expected empty boundNode for constructor, got %s", result[0].BoundNode)
	}
}

func TestJavaProvider_InterpretTypeBinding_Annotation(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.annotation", StartRow: 8},
		{MatchIndex: 0, Name: "type-binding.name", Text: "handler"},
		{MatchIndex: 0, Name: "type-binding.type", Text: "Runnable"},
	}
	result := p.InterpretTypeBinding(captures, nil, "App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "annotation" {
		t.Errorf("expected kind=annotation, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Runnable" {
		t.Errorf("expected typeName=Runnable, got %s", result[0].TypeName)
	}
	if result[0].BoundNode != "handler" {
		t.Errorf("expected boundNode=handler, got %s", result[0].BoundNode)
	}
}

func TestJavaProvider_InterpretTypeBinding_UnknownKindSkipped(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.unknown", StartRow: 5},
	}
	result := p.InterpretTypeBinding(captures, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for unknown kind, got %d", len(result))
	}
}

func TestJavaProvider_InterpretTypeBinding_EmptyCaptures(t *testing.T) {
	p := NewJavaProvider()
	result := p.InterpretTypeBinding(nil, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 bindings for nil captures, got %d", len(result))
	}
}

// ============ JavaProvider InterpretReference Accuracy Tests ============

func TestJavaProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "println"},
	}
	result := p.InterpretReference(captures, nil, "App.java")
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
	if ref.FilePath != "App.java" {
		t.Errorf("expected filePath=App.java, got %s", ref.FilePath)
	}
}

func TestJavaProvider_InterpretReference_MemberCallWithReceiver(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 0, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 0, Name: "reference.name", Text: "query"},
	}
	result := p.InterpretReference(captures, nil, "Service.java")
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

func TestJavaProvider_InterpretReference_ConstructorCall(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.constructor", StartRow: 20},
		{MatchIndex: 0, Name: "reference.name", Text: "ArrayList"},
	}
	result := p.InterpretReference(captures, nil, "App.java")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", ref.Kind)
	}
	if ref.Name != "ArrayList" {
		t.Errorf("expected name=ArrayList, got %s", ref.Name)
	}
}

func TestJavaProvider_InterpretReference_MultipleRefsIsolated(t *testing.T) {
	p := NewJavaProvider()
	// Three different reference types across three matches
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.call.free", StartRow: 10},
		{MatchIndex: 0, Name: "reference.name", Text: "println"},
		{MatchIndex: 1, NodeType: "reference.call.member", StartRow: 15},
		{MatchIndex: 1, Name: "reference.receiver", Text: "db"},
		{MatchIndex: 1, Name: "reference.name", Text: "query"},
		{MatchIndex: 2, NodeType: "reference.call.constructor", StartRow: 20},
		{MatchIndex: 2, Name: "reference.name", Text: "HashMap"},
	}
	result := p.InterpretReference(captures, nil, "App.java")
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

func TestJavaProvider_InterpretReference_UnknownKindSkipped(t *testing.T) {
	p := NewJavaProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.unknown", StartRow: 5},
	}
	result := p.InterpretReference(captures, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 references for unknown kind, got %d", len(result))
	}
}

func TestJavaProvider_InterpretReference_EmptyCaptures(t *testing.T) {
	p := NewJavaProvider()
	result := p.InterpretReference(nil, nil, "Test.java")
	if len(result) != 0 {
		t.Errorf("expected 0 references for nil captures, got %d", len(result))
	}
}