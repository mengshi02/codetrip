package lang

import (
	"testing"
)

// ============ pyParseImportText Tests ============

func TestPyParseImportText_SimpleImport(t *testing.T) {
	path, symbols := pyParseImportText("import os")
	if path != "os" {
		t.Errorf("expected path=os, got %s", path)
	}
	if len(symbols) != 1 || symbols[0] != "os" {
		t.Errorf("expected symbols=[os], got %v", symbols)
	}
}

func TestPyParseImportText_MultipleImports(t *testing.T) {
	path, symbols := pyParseImportText("import os, sys")
	if path != "os" {
		t.Errorf("expected path=os, got %s", path)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0] != "os" {
		t.Errorf("expected first symbol=os, got %s", symbols[0])
	}
	if symbols[1] != "sys" {
		t.Errorf("expected second symbol=sys, got %s", symbols[1])
	}
}

func TestPyParseImportText_FromImport(t *testing.T) {
	path, symbols := pyParseImportText("from typing import List, Dict")
	if path != "typing" {
		t.Errorf("expected path=typing, got %s", path)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0] != "List" {
		t.Errorf("expected first symbol=List, got %s", symbols[0])
	}
	if symbols[1] != "Dict" {
		t.Errorf("expected second symbol=Dict, got %s", symbols[1])
	}
}

func TestPyParseImportText_FromImportSingle(t *testing.T) {
	path, symbols := pyParseImportText("from os import path")
	if path != "os" {
		t.Errorf("expected path=os, got %s", path)
	}
	if len(symbols) != 1 || symbols[0] != "path" {
		t.Errorf("expected symbols=[path], got %v", symbols)
	}
}

func TestPyParseImportText_FromImportWithAs(t *testing.T) {
	path, symbols := pyParseImportText("from collections import defaultdict as dd")
	if path != "collections" {
		t.Errorf("expected path=collections, got %s", path)
	}
	if len(symbols) != 1 || symbols[0] != "defaultdict" {
		t.Errorf("expected symbols=[defaultdict], got %v", symbols)
	}
}

func TestPyParseImportText_ImportWithAs(t *testing.T) {
	path, symbols := pyParseImportText("import numpy as np")
	if path != "numpy" {
		t.Errorf("expected path=numpy, got %s", path)
	}
	if len(symbols) != 1 || symbols[0] != "numpy" {
		t.Errorf("expected symbols=[numpy], got %v", symbols)
	}
}

func TestPyParseImportText_MalformedFromImport(t *testing.T) {
	// "from xxx" without "import" — should return rest as path
	path, symbols := pyParseImportText("from something")
	if path != "something" {
		t.Errorf("expected path=something, got %s", path)
	}
	if len(symbols) != 0 {
		t.Errorf("expected no symbols for malformed import, got %v", symbols)
	}
}

func TestPyParseImportText_UnknownFormat(t *testing.T) {
	path, _ := pyParseImportText("some random text")
	if path != "some random text" {
		t.Errorf("expected path=some random text for unknown format, got %s", path)
	}
}

func TestPyParseImportText_Whitespace(t *testing.T) {
	path, symbols := pyParseImportText("  import os  ")
	if path != "os" {
		t.Errorf("expected path=os with whitespace, got %s", path)
	}
	if len(symbols) != 1 || symbols[0] != "os" {
		t.Errorf("expected symbols=[os], got %v", symbols)
	}
}

// ============ pythonModuleName Tests ============

func TestPythonModuleName_RegularFile(t *testing.T) {
	result := pythonModuleName("pkg/module.py")
	if result != "module" {
		t.Errorf("expected module, got %s", result)
	}
}

func TestPythonModuleName_InitFile(t *testing.T) {
	result := pythonModuleName("pkg/__init__.py")
	if result != "pkg" {
		t.Errorf("expected pkg for __init__.py, got %s", result)
	}
}

func TestPythonModuleName_NestedFile(t *testing.T) {
	result := pythonModuleName("a/b/c/utils.py")
	if result != "utils" {
		t.Errorf("expected utils, got %s", result)
	}
}

func TestPythonModuleName_NoPyExtension(t *testing.T) {
	result := pythonModuleName("scripts/run")
	if result != "run" {
		t.Errorf("expected run for non-.py file, got %s", result)
	}
}

// ============ pythonParseFuncName Tests ============

func TestPythonParseFuncName_Simple(t *testing.T) {
	result := pythonParseFuncName("def process():")
	if result != "process" {
		t.Errorf("expected process, got %s", result)
	}
}

func TestPythonParseFuncName_Async(t *testing.T) {
	result := pythonParseFuncName("async def fetch():")
	if result != "fetch" {
		t.Errorf("expected fetch for async def, got %s", result)
	}
}

func TestPythonParseFuncName_WithParams(t *testing.T) {
	result := pythonParseFuncName("def calculate(x, y):")
	if result != "calculate" {
		t.Errorf("expected calculate, got %s", result)
	}
}

func TestPythonParseFuncName_NoDef(t *testing.T) {
	result := pythonParseFuncName("x = 1")
	if result != "" {
		t.Errorf("expected empty for no def keyword, got %s", result)
	}
}

// ============ pythonParseClassName Tests ============

func TestPythonParseClassName_Simple(t *testing.T) {
	result := pythonParseClassName("class Foo:")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestPythonParseClassName_WithInheritance(t *testing.T) {
	result := pythonParseClassName("class Bar(Base):")
	if result != "Bar" {
		t.Errorf("expected Bar, got %s", result)
	}
}

func TestPythonParseClassName_NoClassKeyword(t *testing.T) {
	result := pythonParseClassName("def foo():")
	if result != "" {
		t.Errorf("expected empty for no class keyword, got %s", result)
	}
}

// ============ pythonExtractScopeName Tests ============

func TestPythonExtractScopeName_Function(t *testing.T) {
	result := pythonExtractScopeName("def process():\n    pass", "function")
	if result != "process" {
		t.Errorf("expected process, got %s", result)
	}
}

func TestPythonExtractScopeName_Class(t *testing.T) {
	result := pythonExtractScopeName("class Server:\n    pass", "class")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestPythonExtractScopeName_Empty(t *testing.T) {
	result := pythonExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %s", result)
	}
}

func TestPythonExtractScopeName_UnknownKind(t *testing.T) {
	result := pythonExtractScopeName("something", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %s", result)
	}
}