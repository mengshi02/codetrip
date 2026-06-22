package lang

import (
	"testing"
)

// ============ cppFirstLine Tests ============

func TestCppFirstLine_Normal(t *testing.T) {
	result := cppFirstLine("void foo() {\n  return;\n}")
	if result != "void foo() {" {
		t.Errorf("expected 'void foo() {', got %q", result)
	}
}

func TestCppFirstLine_NoNewline(t *testing.T) {
	result := cppFirstLine("void foo();")
	if result != "void foo();" {
		t.Errorf("expected 'void foo();', got %q", result)
	}
}

func TestCppFirstLine_Empty(t *testing.T) {
	result := cppFirstLine("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ============ cppParseFuncName Tests ============

func TestCppParseFuncName_Simple(t *testing.T) {
	result := cppParseFuncName("void foo(")
	if result != "foo" {
		t.Errorf("expected foo, got %q", result)
	}
}

func TestCppParseFuncName_Qualified(t *testing.T) {
	result := cppParseFuncName("int MyClass::bar(")
	if result != "bar" {
		t.Errorf("expected bar, got %q", result)
	}
}

func TestCppParseFuncName_NoParen(t *testing.T) {
	result := cppParseFuncName("void foo")
	if result != "" {
		t.Errorf("expected empty for no paren, got %q", result)
	}
}

func TestCppParseFuncName_PointerReturn(t *testing.T) {
	// LastIndex of "(" finds the final "(", beforeParen = "void (*callback)"
	// Walking back from beforeParen skips spaces, finds ')' which is not an ident char
	// So no identifier is found before the last paren
	result := cppParseFuncName("void (*callback)(")
	if result != "" {
		t.Errorf("expected empty (complex pointer syntax not handled), got %q", result)
	}
}

func TestCppParseFuncName_NoIdentifier(t *testing.T) {
	result := cppParseFuncName("  (")
	if result != "" {
		t.Errorf("expected empty for no identifier before paren, got %q", result)
	}
}

func TestCppParseFuncName_ComplexQualified(t *testing.T) {
	result := cppParseFuncName("std::vector<int> NS::Container::push_back(")
	if result != "push_back" {
		t.Errorf("expected push_back, got %q", result)
	}
}

// ============ cppParseTypeName Tests ============

func TestCppParseTypeName_Class(t *testing.T) {
	result := cppParseTypeName("class Foo {")
	if result != "Foo" {
		t.Errorf("expected Foo, got %q", result)
	}
}

func TestCppParseTypeName_Struct(t *testing.T) {
	result := cppParseTypeName("struct Bar : public Base {")
	if result != "Bar" {
		t.Errorf("expected Bar, got %q", result)
	}
}

func TestCppParseTypeName_Enum(t *testing.T) {
	result := cppParseTypeName("enum Color {")
	if result != "Color" {
		t.Errorf("expected Color, got %q", result)
	}
}

func TestCppParseTypeName_EnumClass(t *testing.T) {
	result := cppParseTypeName("enum class Direction {")
	if result != "Direction" {
		t.Errorf("expected Direction, got %q", result)
	}
}

func TestCppParseTypeName_Namespace(t *testing.T) {
	result := cppParseTypeName("namespace MyNS {")
	if result != "MyNS" {
		t.Errorf("expected MyNS, got %q", result)
	}
}

func TestCppParseTypeName_NamespaceNested(t *testing.T) {
	// cppExtractIdentifier treats ':' as ident char, so "MyNS::Nested" is extracted as whole
	result := cppParseTypeName("namespace MyNS::Nested {")
	if result != "MyNS::Nested" {
		t.Errorf("expected MyNS::Nested (colon is ident char), got %q", result)
	}
}

func TestCppParseTypeName_NoKeyword(t *testing.T) {
	result := cppParseTypeName("void foo() {}")
	if result != "" {
		t.Errorf("expected empty for no keyword, got %q", result)
	}
}

// ============ cppExtractScopeName Tests ============

func TestCppExtractScopeName_Function(t *testing.T) {
	result := cppExtractScopeName("void process(\n  pass", "function")
	if result != "process" {
		t.Errorf("expected process, got %q", result)
	}
}

func TestCppExtractScopeName_Class(t *testing.T) {
	result := cppExtractScopeName("class Server {\n  int x;", "class")
	if result != "Server" {
		t.Errorf("expected Server, got %q", result)
	}
}

func TestCppExtractScopeName_Namespace(t *testing.T) {
	result := cppExtractScopeName("namespace MyNS {\n  void foo();", "namespace")
	if result != "MyNS" {
		t.Errorf("expected MyNS, got %q", result)
	}
}

func TestCppExtractScopeName_Empty(t *testing.T) {
	result := cppExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %q", result)
	}
}

func TestCppExtractScopeName_UnknownKind(t *testing.T) {
	result := cppExtractScopeName("something", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %q", result)
	}
}

// ============ cppModuleName Tests ============

func TestCppModuleName_NormalPath(t *testing.T) {
	result := cppModuleName("src/core/engine.cpp")
	if result != "core" {
		t.Errorf("expected core, got %q", result)
	}
}

func TestCppModuleName_RootPath(t *testing.T) {
	result := cppModuleName("main.cpp")
	if result != "main" {
		t.Errorf("expected main (file stem as fallback), got %q", result)
	}
}

func TestCppModuleName_NestedPath(t *testing.T) {
	result := cppModuleName("project/src/module/utils.cpp")
	if result != "module" {
		t.Errorf("expected module, got %q", result)
	}
}

// ============ cppExtractIncludePath Tests ============

func TestCppExtractIncludePath_AngleBrackets(t *testing.T) {
	result := cppExtractIncludePath("#include <stdio.h>")
	if result != "stdio.h" {
		t.Errorf("expected stdio.h, got %q", result)
	}
}

func TestCppExtractIncludePath_DoubleQuotes(t *testing.T) {
	result := cppExtractIncludePath("#include \"foo.h\"")
	if result != "foo.h" {
		t.Errorf("expected foo.h, got %q", result)
	}
}

func TestCppExtractIncludePath_Fallback(t *testing.T) {
	result := cppExtractIncludePath("  something.h  ")
	if result != "something.h" {
		t.Errorf("expected 'something.h' (TrimSpace fallback), got %q", result)
	}
}

// ============ cppExtractIdentifier Tests ============

func TestCppExtractIdentifier_Normal(t *testing.T) {
	result := cppExtractIdentifier("FooBar {")
	if result != "FooBar" {
		t.Errorf("expected FooBar, got %q", result)
	}
}

func TestCppExtractIdentifier_WithColon(t *testing.T) {
	result := cppExtractIdentifier("MyNS::Sub {")
	if result != "MyNS::Sub" {
		t.Errorf("expected 'MyNS::Sub' (colon is ident char), got %q", result)
	}
}

func TestCppExtractIdentifier_Empty(t *testing.T) {
	result := cppExtractIdentifier("  ")
	if result != "" {
		t.Errorf("expected empty for whitespace-only, got %q", result)
	}
}

func TestCppExtractIdentifier_StopsAtNonIdent(t *testing.T) {
	result := cppExtractIdentifier("Name<T>")
	if result != "Name" {
		t.Errorf("expected Name (stops at <), got %q", result)
	}
}

// ============ isCppIdentChar Tests ============

func TestIsCppIdentChar_Letter(t *testing.T) {
	if !isCppIdentChar('a') || !isCppIdentChar('Z') {
		t.Error("letters should be ident chars")
	}
}

func TestIsCppIdentChar_Digit(t *testing.T) {
	if !isCppIdentChar('5') {
		t.Error("digits should be ident chars")
	}
}

func TestIsCppIdentChar_Underscore(t *testing.T) {
	if !isCppIdentChar('_') {
		t.Error("underscore should be an ident char")
	}
}

func TestIsCppIdentChar_Colon(t *testing.T) {
	if !isCppIdentChar(':') {
		t.Error("colon should be an ident char")
	}
}

func TestIsCppIdentChar_NonIdentChar(t *testing.T) {
	if isCppIdentChar(' ') || isCppIdentChar('(') || isCppIdentChar(';') {
		t.Error("space, paren, semi should not be ident chars")
	}
}