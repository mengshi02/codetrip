package lang

import (
	"testing"
)

// ============ csharpModuleName Tests ============

func TestCSharpModuleName_NormalPath(t *testing.T) {
	result := csharpModuleName("project/Models/App.cs")
	if result != "Models" {
		t.Errorf("expected Models, got %s", result)
	}
}

func TestCSharpModuleName_EmptyDir(t *testing.T) {
	result := csharpModuleName("App.cs")
	if result != "global" {
		t.Errorf("expected global for empty dir, got %s", result)
	}
}

func TestCSharpModuleName_DotDir(t *testing.T) {
	result := csharpModuleName("./App.cs")
	if result != "global" {
		t.Errorf("expected global for dot dir, got %s", result)
	}
}

func TestCSharpModuleName_NestedPath(t *testing.T) {
	result := csharpModuleName("src/project/services/Handler.cs")
	if result != "services" {
		t.Errorf("expected services, got %s", result)
	}
}

// ============ firstLineOfCaptureText Tests ============

func TestFirstLineOfCaptureText_Normal(t *testing.T) {
	result := firstLineOfCaptureText("namespace Foo {\n  class Bar")
	if result != "namespace Foo {" {
		t.Errorf("expected 'namespace Foo {', got %s", result)
	}
}

func TestFirstLineOfCaptureText_NoNewline(t *testing.T) {
	result := firstLineOfCaptureText("class Foo")
	if result != "class Foo" {
		t.Errorf("expected 'class Foo', got %s", result)
	}
}

func TestFirstLineOfCaptureText_EmptyString(t *testing.T) {
	result := firstLineOfCaptureText("")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

// ============ csharpParseNamespaceName Tests ============

func TestCSharpParseNamespaceName_WithBrace(t *testing.T) {
	result := csharpParseNamespaceName("namespace Foo.Bar {")
	if result != "Foo.Bar" {
		t.Errorf("expected Foo.Bar, got %s", result)
	}
}

func TestCSharpParseNamespaceName_NoBrace(t *testing.T) {
	result := csharpParseNamespaceName("namespace Foo")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestCSharpParseNamespaceName_NoKeyword(t *testing.T) {
	result := csharpParseNamespaceName("class Foo")
	if result != "" {
		t.Errorf("expected empty for no namespace keyword, got %s", result)
	}
}

func TestCSharpParseNamespaceName_TrailingBraceWithSpaces(t *testing.T) {
	result := csharpParseNamespaceName("namespace MyApp.Services {   ")
	if result != "MyApp.Services" {
		t.Errorf("expected MyApp.Services, got %s", result)
	}
}

func TestCSharpParseNamespaceName_SimpleNamespace(t *testing.T) {
	result := csharpParseNamespaceName("namespace App")
	if result != "App" {
		t.Errorf("expected App, got %s", result)
	}
}

// ============ csharpParseClassName Tests ============

func TestCSharpParseClassName_Class(t *testing.T) {
	result := csharpParseClassName("class Foo")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestCSharpParseClassName_InterfaceGeneric(t *testing.T) {
	result := csharpParseClassName("interface Bar<T>")
	if result != "Bar" {
		t.Errorf("expected Bar, got %s", result)
	}
}

func TestCSharpParseClassName_StructWithBase(t *testing.T) {
	result := csharpParseClassName("struct Baz : Base")
	if result != "Baz" {
		t.Errorf("expected Baz, got %s", result)
	}
}

func TestCSharpParseClassName_Record(t *testing.T) {
	result := csharpParseClassName("record Rec")
	if result != "Rec" {
		t.Errorf("expected Rec, got %s", result)
	}
}

func TestCSharpParseClassName_Enum(t *testing.T) {
	result := csharpParseClassName("enum Color")
	if result != "Color" {
		t.Errorf("expected Color, got %s", result)
	}
}

func TestCSharpParseClassName_NoKeyword(t *testing.T) {
	result := csharpParseClassName("void Foo()")
	if result != "" {
		t.Errorf("expected empty for no class keyword, got %s", result)
	}
}

func TestCSharpParseClassName_ClassWithBrace(t *testing.T) {
	result := csharpParseClassName("class Server {")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestCSharpParseClassName_ClassWithColon(t *testing.T) {
	result := csharpParseClassName("class Derived : Base")
	if result != "Derived" {
		t.Errorf("expected Derived, got %s", result)
	}
}

// ============ csharpParseMethodName Tests ============

func TestCSharpParseMethodName_VoidMethod(t *testing.T) {
	result := csharpParseMethodName("void Foo(")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestCSharpParseMethodName_PublicInt(t *testing.T) {
	result := csharpParseMethodName("public int Bar(")
	if result != "Bar" {
		t.Errorf("expected Bar, got %s", result)
	}
}

func TestCSharpParseMethodName_StaticVoid(t *testing.T) {
	result := csharpParseMethodName("static void Main(")
	if result != "Main" {
		t.Errorf("expected Main, got %s", result)
	}
}

func TestCSharpParseMethodName_KeywordFiltered(t *testing.T) {
	// "void" is a keyword, so csharpParseMethodName recurses to find actual name
	result := csharpParseMethodName("void Calculate(")
	if result != "Calculate" {
		t.Errorf("expected Calculate, got %s", result)
	}
}

func TestCSharpParseMethodName_NoParenthesis(t *testing.T) {
	result := csharpParseMethodName("void Foo")
	if result != "" {
		t.Errorf("expected empty for no parenthesis, got %s", result)
	}
}

func TestCSharpParseMethodName_JustIdentifierBeforeParen(t *testing.T) {
	result := csharpParseMethodName("Foo(")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestCSharpParseMethodName_StringReturn(t *testing.T) {
	result := csharpParseMethodName("string GetName(")
	if result != "GetName" {
		t.Errorf("expected GetName, got %s", result)
	}
}

func TestCSharpParseMethodName_BoolReturn(t *testing.T) {
	result := csharpParseMethodName("bool IsValid(")
	if result != "IsValid" {
		t.Errorf("expected IsValid, got %s", result)
	}
}

func TestCSharpParseMethodName_AsyncVoid(t *testing.T) {
	result := csharpParseMethodName("async void ProcessAsync(")
	if result != "ProcessAsync" {
		t.Errorf("expected ProcessAsync, got %s", result)
	}
}

// ============ csharpExtractIdentifier Tests ============

func TestCSharpExtractIdentifier_Normal(t *testing.T) {
	result := csharpExtractIdentifier("FooBar")
	if result != "FooBar" {
		t.Errorf("expected FooBar, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithGeneric(t *testing.T) {
	result := csharpExtractIdentifier("List<T>")
	if result != "List" {
		t.Errorf("expected List, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithColon(t *testing.T) {
	result := csharpExtractIdentifier("Base : IFoo")
	if result != "Base" {
		t.Errorf("expected Base, got %s", result)
	}
}

func TestCSharpExtractIdentifier_Empty(t *testing.T) {
	result := csharpExtractIdentifier("")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithBrace(t *testing.T) {
	result := csharpExtractIdentifier("Server {")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithComma(t *testing.T) {
	result := csharpExtractIdentifier("First, Second")
	if result != "First" {
		t.Errorf("expected First, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithSemicolon(t *testing.T) {
	result := csharpExtractIdentifier("Foo;")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestCSharpExtractIdentifier_WithParen(t *testing.T) {
	result := csharpExtractIdentifier("Method(")
	if result != "Method" {
		t.Errorf("expected Method, got %s", result)
	}
}

func TestCSharpExtractIdentifier_StartsWithStopChar(t *testing.T) {
	result := csharpExtractIdentifier("<T>")
	if result != "" {
		t.Errorf("expected empty for stop char start, got %s", result)
	}
}

// ============ csharpExtractScopeName Tests ============

func TestCSharpExtractScopeName_NamespaceKind(t *testing.T) {
	result := csharpExtractScopeName("namespace MyApp {", "namespace")
	if result != "MyApp" {
		t.Errorf("expected MyApp, got %s", result)
	}
}

func TestCSharpExtractScopeName_ClassKind(t *testing.T) {
	result := csharpExtractScopeName("class Server {", "class")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestCSharpExtractScopeName_FunctionKind(t *testing.T) {
	result := csharpExtractScopeName("void Process(", "function")
	if result != "Process" {
		t.Errorf("expected Process, got %s", result)
	}
}

func TestCSharpExtractScopeName_EmptyText(t *testing.T) {
	result := csharpExtractScopeName("", "class")
	if result != "" {
		t.Errorf("expected empty for empty text, got %s", result)
	}
}

func TestCSharpExtractScopeName_UnknownKind(t *testing.T) {
	result := csharpExtractScopeName("something", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %s", result)
	}
}

// ============ csharpExtractImportPath Tests ============

func TestCSharpExtractImportPath_StandardUsing(t *testing.T) {
	result := csharpExtractImportPath("using System;")
	if result != "System" {
		t.Errorf("expected System, got %s", result)
	}
}

func TestCSharpExtractImportPath_StaticUsing(t *testing.T) {
	result := csharpExtractImportPath("using static System.Math;")
	if result != "System.Math" {
		t.Errorf("expected System.Math, got %s", result)
	}
}

func TestCSharpExtractImportPath_AliasUsing(t *testing.T) {
	result := csharpExtractImportPath("using Foo = Bar.Baz;")
	if result != "Bar.Baz" {
		t.Errorf("expected Bar.Baz, got %s", result)
	}
}

func TestCSharpExtractImportPath_NoUsingPrefix(t *testing.T) {
	result := csharpExtractImportPath("import something;")
	if result != "" {
		t.Errorf("expected empty for no using prefix, got %s", result)
	}
}

func TestCSharpExtractImportPath_WithSemicolon(t *testing.T) {
	result := csharpExtractImportPath("using System.Collections.Generic;")
	if result != "System.Collections.Generic" {
		t.Errorf("expected System.Collections.Generic, got %s", result)
	}
}

func TestCSharpExtractImportPath_NoSemicolon(t *testing.T) {
	result := csharpExtractImportPath("using System.Collections")
	if result != "System.Collections" {
		t.Errorf("expected System.Collections, got %s", result)
	}
}

func TestCSharpExtractImportPath_EmptyString(t *testing.T) {
	result := csharpExtractImportPath("")
	if result != "" {
		t.Errorf("expected empty for empty string, got %s", result)
	}
}

func TestCSharpExtractImportPath_MultilineText(t *testing.T) {
	result := csharpExtractImportPath("using System.IO;\nusing System.Net;")
	if result != "System.IO" {
		t.Errorf("expected System.IO (first line only), got %s", result)
	}
}

func TestCSharpExtractImportPath_StaticWithAlias(t *testing.T) {
	result := csharpExtractImportPath("using M = System.Math;")
	if result != "System.Math" {
		t.Errorf("expected System.Math, got %s", result)
	}
}