package lang

import (
	"testing"
)

// ============ rustFirstLine Tests ============

func TestRustFirstLine_Normal(t *testing.T) {
	result := rustFirstLine("fn main() {\n    println!(\"hello\");\n}")
	if result != "fn main() {" {
		t.Errorf("expected 'fn main() {', got %q", result)
	}
}

func TestRustFirstLine_NoNewline(t *testing.T) {
	result := rustFirstLine("struct Foo")
	if result != "struct Foo" {
		t.Errorf("expected 'struct Foo', got %q", result)
	}
}

func TestRustFirstLine_Empty(t *testing.T) {
	result := rustFirstLine("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// ============ rustParseFuncName Tests ============

func TestRustParseFuncName_SimpleFn(t *testing.T) {
	result := rustParseFuncName("fn foo(")
	if result != "foo" {
		t.Errorf("expected foo, got %q", result)
	}
}

func TestRustParseFuncName_PubFn(t *testing.T) {
	result := rustParseFuncName("pub fn bar(")
	if result != "bar" {
		t.Errorf("expected bar, got %q", result)
	}
}

func TestRustParseFuncName_GenericFn(t *testing.T) {
	result := rustParseFuncName("fn name<T>(")
	if result != "name" {
		t.Errorf("expected name, got %q", result)
	}
}

func TestRustParseFuncName_AsyncFn(t *testing.T) {
	result := rustParseFuncName("async fn process(")
	if result != "process" {
		t.Errorf("expected process, got %q", result)
	}
}

func TestRustParseFuncName_NoFnKeyword(t *testing.T) {
	result := rustParseFuncName("struct Foo {")
	if result != "" {
		t.Errorf("expected empty for no 'fn ' keyword, got %q", result)
	}
}

func TestRustParseFuncName_FnWithNoNameBeforeParen(t *testing.T) {
	result := rustParseFuncName("fn (")
	if result != "" {
		t.Errorf("expected empty for fn with no name before (, got %q", result)
	}
}

func TestRustParseFuncName_FnWithSpaceBeforeBrace(t *testing.T) {
	result := rustParseFuncName("fn run {")
	if result != "run" {
		t.Errorf("expected run, got %q", result)
	}
}

// ============ rustParseTypeName Tests ============

func TestRustParseTypeName_Struct(t *testing.T) {
	result := rustParseTypeName("struct Foo")
	if result != "Foo" {
		t.Errorf("expected Foo, got %q", result)
	}
}

func TestRustParseTypeName_Enum(t *testing.T) {
	result := rustParseTypeName("enum Bar")
	if result != "Bar" {
		t.Errorf("expected Bar, got %q", result)
	}
}

func TestRustParseTypeName_Trait(t *testing.T) {
	result := rustParseTypeName("trait Baz")
	if result != "Baz" {
		t.Errorf("expected Baz, got %q", result)
	}
}

func TestRustParseTypeName_Impl(t *testing.T) {
	result := rustParseTypeName("impl Qux")
	if result != "Qux" {
		t.Errorf("expected Qux, got %q", result)
	}
}

func TestRustParseTypeName_ImplTraitForType(t *testing.T) {
	result := rustParseTypeName("impl Trait for Type")
	if result != "Type" {
		t.Errorf("expected Type (after 'for'), got %q", result)
	}
}

func TestRustParseTypeName_StructGeneric(t *testing.T) {
	result := rustParseTypeName("struct Name<T>")
	if result != "Name" {
		t.Errorf("expected Name, got %q", result)
	}
}

func TestRustParseTypeName_NoKeyword(t *testing.T) {
	result := rustParseTypeName("fn main() {")
	if result != "" {
		t.Errorf("expected empty for no type keyword, got %q", result)
	}
}

func TestRustParseTypeName_StructWithBrace(t *testing.T) {
	result := rustParseTypeName("struct Point {")
	if result != "Point" {
		t.Errorf("expected Point, got %q", result)
	}
}

func TestRustParseTypeName_ImplTraitForTypeGeneric(t *testing.T) {
	result := rustParseTypeName("impl Trait for Type<T>")
	if result != "Type" {
		t.Errorf("expected Type, got %q", result)
	}
}

// ============ rustExtractScopeName Tests ============

func TestRustExtractScopeName_FunctionKind(t *testing.T) {
	result := rustExtractScopeName("fn process(", "function")
	if result != "process" {
		t.Errorf("expected process, got %q", result)
	}
}

func TestRustExtractScopeName_ClassKind(t *testing.T) {
	result := rustExtractScopeName("struct Server {", "class")
	if result != "Server" {
		t.Errorf("expected Server, got %q", result)
	}
}

func TestRustExtractScopeName_EmptyText(t *testing.T) {
	result := rustExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %q", result)
	}
}

func TestRustExtractScopeName_UnknownKind(t *testing.T) {
	result := rustExtractScopeName("fn main(", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %q", result)
	}
}

// ============ rustModuleName Tests ============

func TestRustModuleName_SimpleRsFile(t *testing.T) {
	result := rustModuleName("src/lib.rs")
	if result != "lib" {
		t.Errorf("expected lib, got %q", result)
	}
}

func TestRustModuleName_ModRs(t *testing.T) {
	result := rustModuleName("src/mod.rs")
	if result != "src" {
		t.Errorf("expected src (parent dir), got %q", result)
	}
}

func TestRustModuleName_BareModRs(t *testing.T) {
	result := rustModuleName("mod.rs")
	if result != "root" {
		t.Errorf("expected root, got %q", result)
	}
}

func TestRustModuleName_NestedPath(t *testing.T) {
	result := rustModuleName("src/collections/mod.rs")
	if result != "collections" {
		t.Errorf("expected collections (parent dir), got %q", result)
	}
}

func TestRustModuleName_NestedRsFile(t *testing.T) {
	result := rustModuleName("src/network/http.rs")
	if result != "http" {
		t.Errorf("expected http, got %q", result)
	}
}

// ============ rustExtractUsePath Tests ============

func TestRustExtractUsePath_StandardImport(t *testing.T) {
	result := rustExtractUsePath("use std::collections::HashMap;")
	if result != "std::collections::HashMap" {
		t.Errorf("expected std::collections::HashMap, got %q", result)
	}
}

func TestRustExtractUsePath_WildcardImport(t *testing.T) {
	result := rustExtractUsePath("use std::io::*;")
	if result != "std::io" {
		t.Errorf("expected std::io (::* removed), got %q", result)
	}
}

func TestRustExtractUsePath_CrateImport(t *testing.T) {
	result := rustExtractUsePath("use crate::module::Item;")
	if result != "crate::module::Item" {
		t.Errorf("expected crate::module::Item, got %q", result)
	}
}

func TestRustExtractUsePath_CurlyBraceGroup(t *testing.T) {
	result := rustExtractUsePath("use std::collections::{HashMap, BTreeMap};")
	if result != "std::collections" {
		t.Errorf("expected std::collections (curly group removed), got %q", result)
	}
}

func TestRustExtractUsePath_NoUsePrefix(t *testing.T) {
	result := rustExtractUsePath("std::collections::HashMap")
	if result != "std::collections::HashMap" {
		t.Errorf("expected std::collections::HashMap (no prefix passthrough), got %q", result)
	}
}

func TestRustExtractUsePath_WithSemicolon(t *testing.T) {
	result := rustExtractUsePath("use std::io;")
	if result != "std::io" {
		t.Errorf("expected std::io, got %q", result)
	}
}