package lang

import (
	"testing"
)

// ============ cFirstLine Tests ============

func TestCFirstLine_Normal(t *testing.T) {
	result := cFirstLine("int main() {\n    return 0;\n}")
	if result != "int main() {" {
		t.Errorf("expected 'int main() {', got %q", result)
	}
}

func TestCFirstLine_NoNewline(t *testing.T) {
	result := cFirstLine("int x;")
	if result != "int x;" {
		t.Errorf("expected 'int x;', got %q", result)
	}
}

func TestCFirstLine_Empty(t *testing.T) {
	result := cFirstLine("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCFirstLine_OnlyNewline(t *testing.T) {
	result := cFirstLine("\nrest")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// ============ cParseFuncName Tests ============

func TestCParseFuncName_Simple(t *testing.T) {
	result := cParseFuncName("int main(")
	if result != "main" {
		t.Errorf("expected main, got %q", result)
	}
}

func TestCParseFuncName_VoidFunc(t *testing.T) {
	result := cParseFuncName("void process(")
	if result != "process" {
		t.Errorf("expected process, got %q", result)
	}
}

func TestCParseFuncName_PointerReturn(t *testing.T) {
	result := cParseFuncName("void *alloc(")
	if result != "alloc" {
		t.Errorf("expected alloc, got %q", result)
	}
}

func TestCParseFuncName_NoParen(t *testing.T) {
	result := cParseFuncName("int x")
	if result != "" {
		t.Errorf("expected empty for no paren, got %q", result)
	}
}

func TestCParseFuncName_NoIdentifierBeforeParen(t *testing.T) {
	result := cParseFuncName("(")
	if result != "" {
		t.Errorf("expected empty for no identifier before paren, got %q", result)
	}
}

func TestCParseFuncName_ComplexType(t *testing.T) {
	result := cParseFuncName("struct node *create_node(")
	if result != "create_node" {
		t.Errorf("expected create_node, got %q", result)
	}
}

// ============ cParseTypeName Tests ============

func TestCParseTypeName_Struct(t *testing.T) {
	result := cParseTypeName("struct Node {")
	if result != "Node" {
		t.Errorf("expected Node, got %q", result)
	}
}

func TestCParseTypeName_Union(t *testing.T) {
	result := cParseTypeName("union Data {")
	if result != "Data" {
		t.Errorf("expected Data, got %q", result)
	}
}

func TestCParseTypeName_Enum(t *testing.T) {
	result := cParseTypeName("enum Color {")
	if result != "Color" {
		t.Errorf("expected Color, got %q", result)
	}
}

func TestCParseTypeName_AnonymousStruct(t *testing.T) {
	result := cParseTypeName("struct {")
	if result != "" {
		t.Errorf("expected empty for anonymous struct, got %q", result)
	}
}

func TestCParseTypeName_NoPrefix(t *testing.T) {
	result := cParseTypeName("int x;")
	if result != "" {
		t.Errorf("expected empty for no struct/union/enum prefix, got %q", result)
	}
}

func TestCParseTypeName_StructWithSemicolon(t *testing.T) {
	result := cParseTypeName("struct Point;")
	if result != "Point" {
		t.Errorf("expected Point, got %q", result)
	}
}

// ============ cExtractScopeName Tests ============

func TestCExtractScopeName_Function(t *testing.T) {
	result := cExtractScopeName("int main() {\n    return 0;", "function")
	if result != "main" {
		t.Errorf("expected main, got %q", result)
	}
}

func TestCExtractScopeName_Class(t *testing.T) {
	result := cExtractScopeName("struct Node {\n    int val;", "class")
	if result != "Node" {
		t.Errorf("expected Node, got %q", result)
	}
}

func TestCExtractScopeName_EmptyText(t *testing.T) {
	result := cExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %q", result)
	}
}

func TestCExtractScopeName_UnknownKind(t *testing.T) {
	result := cExtractScopeName("something", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %q", result)
	}
}

// ============ cExtractModuleName Tests ============

func TestCExtractModuleName_NormalPath(t *testing.T) {
	result := cExtractModuleName("src/main.c")
	if result != "main" {
		t.Errorf("expected main, got %q", result)
	}
}

func TestCExtractModuleName_NoExtension(t *testing.T) {
	result := cExtractModuleName("src/Makefile")
	if result != "Makefile" {
		t.Errorf("expected Makefile, got %q", result)
	}
}

func TestCExtractModuleName_NestedPath(t *testing.T) {
	result := cExtractModuleName("/usr/include/stdio.h")
	if result != "stdio" {
		t.Errorf("expected stdio, got %q", result)
	}
}

// ============ cExtractImportPath Tests ============

func TestCExtractImportPath_DoubleQuoted(t *testing.T) {
	result := cExtractImportPath(`#include "myheader.h"`)
	if result != "myheader.h" {
		t.Errorf("expected myheader.h, got %q", result)
	}
}

func TestCExtractImportPath_AngleBracket(t *testing.T) {
	result := cExtractImportPath("#include <stdio.h>")
	if result != "stdio.h" {
		t.Errorf("expected stdio.h, got %q", result)
	}
}

func TestCExtractImportPath_NoQuotes(t *testing.T) {
	result := cExtractImportPath("#include stdio.h")
	if result != "#include stdio.h" {
		t.Errorf("expected TrimSpace fallback, got %q", result)
	}
}

func TestCExtractImportPath_JustText(t *testing.T) {
	result := cExtractImportPath("  myheader.h  ")
	if result != "myheader.h" {
		t.Errorf("expected TrimSpace fallback 'myheader.h', got %q", result)
	}
}