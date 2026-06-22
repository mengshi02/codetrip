package lang

import (
	"testing"
)

// ============ tsFirstLine Tests ============

func TestTsFirstLine_MultiLine(t *testing.T) {
	result := tsFirstLine("function foo()\n  return bar")
	if result != "function foo()" {
		t.Errorf("expected 'function foo()', got '%s'", result)
	}
}

func TestTsFirstLine_SingleLine(t *testing.T) {
	result := tsFirstLine("class Foo {")
	if result != "class Foo {" {
		t.Errorf("expected 'class Foo {', got '%s'", result)
	}
}

func TestTsFirstLine_Empty(t *testing.T) {
	result := tsFirstLine("")
	if result != "" {
		t.Errorf("expected empty, got '%s'", result)
	}
}

func TestTsFirstLine_TrailingNewline(t *testing.T) {
	result := tsFirstLine("interface Bar {\n")
	if result != "interface Bar {" {
		t.Errorf("expected 'interface Bar {', got '%s'", result)
	}
}

// ============ tsParseFuncName Tests ============

func TestTsParseFuncName_SimpleFunction(t *testing.T) {
	result := tsParseFuncName("function main() {")
	if result != "main" {
		t.Errorf("expected main, got %s", result)
	}
}

func TestTsParseFuncName_AsyncFunction(t *testing.T) {
	result := tsParseFuncName("async function fetchData() {")
	if result != "fetchData" {
		t.Errorf("expected fetchData, got %s", result)
	}
}

func TestTsParseFuncName_GeneratorFunction(t *testing.T) {
	result := tsParseFuncName("function* gen() {")
	if result != "gen" {
		t.Errorf("expected gen, got %s", result)
	}
}

func TestTsParseFuncName_ExportFunction(t *testing.T) {
	result := tsParseFuncName("export function helper() {")
	if result != "helper" {
		t.Errorf("expected helper, got %s", result)
	}
}

func TestTsParseFuncName_DeclareFunction(t *testing.T) {
	result := tsParseFuncName("declare function apiCall(): void;")
	if result != "apiCall" {
		t.Errorf("expected apiCall, got %s", result)
	}
}

func TestTsParseFuncName_MethodDefinition(t *testing.T) {
	result := tsParseFuncName("process(data: string) {")
	if result != "process" {
		t.Errorf("expected process, got %s", result)
	}
}

// ============ tsParseClassName Tests ============

func TestTsParseClassName_SimpleClass(t *testing.T) {
	result := tsParseClassName("class Server {")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestTsParseClassName_AbstractClass(t *testing.T) {
	result := tsParseClassName("abstract class BaseService {")
	if result != "BaseService" {
		t.Errorf("expected BaseService, got %s", result)
	}
}

func TestTsParseClassName_Interface(t *testing.T) {
	result := tsParseClassName("interface Reader {")
	if result != "Reader" {
		t.Errorf("expected Reader, got %s", result)
	}
}

func TestTsParseClassName_ExportClass(t *testing.T) {
	result := tsParseClassName("export class Config {")
	if result != "Config" {
		t.Errorf("expected Config, got %s", result)
	}
}

func TestTsParseClassName_Enum(t *testing.T) {
	result := tsParseClassName("enum Direction {")
	if result != "Direction" {
		t.Errorf("expected Direction, got %s", result)
	}
}

func TestTsParseClassName_NoKeyword(t *testing.T) {
	result := tsParseClassName("function foo() {}")
	// Should fall back to tsParseIdentifierBefore(line, ' ')
	// which would return "function" — this tests the fallback path
	if result == "" {
		t.Error("expected non-empty result from fallback path")
	}
}

// ============ tsParseInterfaceName Tests ============

func TestTsParseInterfaceName_Simple(t *testing.T) {
	result := tsParseInterfaceName("interface IService {")
	if result != "IService" {
		t.Errorf("expected IService, got %s", result)
	}
}

func TestTsParseInterfaceName_Export(t *testing.T) {
	result := tsParseInterfaceName("export interface IConfig {")
	if result != "IConfig" {
		t.Errorf("expected IConfig, got %s", result)
	}
}

func TestTsParseInterfaceName_Declare(t *testing.T) {
	result := tsParseInterfaceName("declare interface GlobalAPI {")
	if result != "GlobalAPI" {
		t.Errorf("expected GlobalAPI, got %s", result)
	}
}

func TestTsParseInterfaceName_NoKeyword(t *testing.T) {
	result := tsParseInterfaceName("class Foo {")
	if result != "" {
		t.Errorf("expected empty for no interface keyword, got %s", result)
	}
}

// ============ tsParseNamespaceName Tests ============

func TestTsParseNamespaceName_Namespace(t *testing.T) {
	result := tsParseNamespaceName("namespace Utils {")
	if result != "Utils" {
		t.Errorf("expected Utils, got %s", result)
	}
}

func TestTsParseNamespaceName_Module(t *testing.T) {
	result := tsParseNamespaceName("module MyModule {")
	if result != "MyModule" {
		t.Errorf("expected MyModule, got %s", result)
	}
}

func TestTsParseNamespaceName_Export(t *testing.T) {
	result := tsParseNamespaceName("export namespace Helpers {")
	if result != "Helpers" {
		t.Errorf("expected Helpers, got %s", result)
	}
}

func TestTsParseNamespaceName_NoKeyword(t *testing.T) {
	result := tsParseNamespaceName("function foo() {}")
	if result != "" {
		t.Errorf("expected empty for no namespace/module keyword, got %s", result)
	}
}

// ============ tsExtractScopeName Tests ============

func TestTsExtractScopeName_Function(t *testing.T) {
	result := tsExtractScopeName("function process() {\n  pass", "function")
	if result != "process" {
		t.Errorf("expected process, got %s", result)
	}
}

func TestTsExtractScopeName_Class(t *testing.T) {
	result := tsExtractScopeName("class Server {\n", "class")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestTsExtractScopeName_Namespace(t *testing.T) {
	result := tsExtractScopeName("namespace Utils {\n", "namespace")
	if result != "Utils" {
		t.Errorf("expected Utils, got %s", result)
	}
}

func TestTsExtractScopeName_Empty(t *testing.T) {
	result := tsExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %s", result)
	}
}

// ============ tsModuleName Tests ============

func TestTsModuleName_SimplePath(t *testing.T) {
	result := tsModuleName("src/utils/helper.ts")
	if result != "helper" {
		t.Errorf("expected helper, got %s", result)
	}
}

func TestTsModuleName_IndexFile(t *testing.T) {
	result := tsModuleName("src/utils/index.ts")
	if result != "utils" {
		t.Errorf("expected utils for index file, got %s", result)
	}
}

func TestTsModuleName_NoPath(t *testing.T) {
	result := tsModuleName("app.ts")
	if result != "app" {
		t.Errorf("expected app, got %s", result)
	}
}

func TestTsModuleName_BackslashPath(t *testing.T) {
	result := tsModuleName("src\\utils\\helper.ts")
	if result != "helper" {
		t.Errorf("expected helper, got %s", result)
	}
}

// ============ tsExtractImportPath Tests ============

func TestTsExtractImportPath_DefaultImport(t *testing.T) {
	path, isDefault, isDynamic := tsExtractImportPath("import React from 'react'")
	if path != "react" {
		t.Errorf("expected path=react, got %s", path)
	}
	if !isDefault {
		t.Error("expected isDefault=true")
	}
	if isDynamic {
		t.Error("expected isDynamic=false")
	}
}

func TestTsExtractImportPath_NamedImport(t *testing.T) {
	path, isDefault, _ := tsExtractImportPath("import { useState } from 'react'")
	if path != "react" {
		t.Errorf("expected path=react, got %s", path)
	}
	if isDefault {
		t.Error("expected isDefault=false for named import")
	}
}

func TestTsExtractImportPath_DynamicImport(t *testing.T) {
	path, isDefault, isDynamic := tsExtractImportPath("import('lodash')")
	if path != "lodash" {
		t.Errorf("expected path=lodash, got %s", path)
	}
	if isDefault {
		t.Error("expected isDefault=false for dynamic import")
	}
	if !isDynamic {
		t.Error("expected isDynamic=true for dynamic import")
	}
}

func TestTsExtractImportPath_SideEffectImport(t *testing.T) {
	path, _, _ := tsExtractImportPath("import './styles.css'")
	if path != "./styles.css" {
		t.Errorf("expected path=./styles.css, got %s", path)
	}
}

func TestTsExtractImportPath_NamespaceImport(t *testing.T) {
	path, isDefault, _ := tsExtractImportPath("import * as fs from 'fs'")
	if path != "fs" {
		t.Errorf("expected path=fs, got %s", path)
	}
	if isDefault {
		t.Error("expected isDefault=false for namespace import (starts with *)")
	}
}

// ============ tsExtractImportSymbols Tests ============

func TestTsExtractImportSymbols_SimpleNamed(t *testing.T) {
	symbols := tsExtractImportSymbols("import { useState, useEffect } from 'react'")
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0] != "useState" {
		t.Errorf("expected useState, got %s", symbols[0])
	}
	if symbols[1] != "useEffect" {
		t.Errorf("expected useEffect, got %s", symbols[1])
	}
}

func TestTsExtractImportSymbols_WithAlias(t *testing.T) {
	symbols := tsExtractImportSymbols("import { Foo as Bar } from 'x'")
	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}
	// Should take the alias "Bar", not "Foo"
	if symbols[0] != "Bar" {
		t.Errorf("expected Bar (alias), got %s", symbols[0])
	}
}

func TestTsExtractImportSymbols_NoBraces(t *testing.T) {
	symbols := tsExtractImportSymbols("import React from 'react'")
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols for default import, got %d", len(symbols))
	}
}

// ============ tsExtractStringLiteral Tests ============

func TestTsExtractStringLiteral_SingleQuote(t *testing.T) {
	result := tsExtractStringLiteral("'hello'")
	if result != "hello" {
		t.Errorf("expected hello, got %s", result)
	}
}

func TestTsExtractStringLiteral_DoubleQuote(t *testing.T) {
	result := tsExtractStringLiteral("\"world\"")
	if result != "world" {
		t.Errorf("expected world, got %s", result)
	}
}

func TestTsExtractStringLiteral_TemplateLiteral(t *testing.T) {
	result := tsExtractStringLiteral("`path`")
	if result != "path" {
		t.Errorf("expected path, got %s", result)
	}
}

func TestTsExtractStringLiteral_TooShort(t *testing.T) {
	result := tsExtractStringLiteral("'")
	if result != "'" {
		t.Errorf("expected single quote returned as-is, got %s", result)
	}
}

// ============ tsParseIdentifierBefore Tests ============

func TestTsParseIdentifierBefore_StopAtParen(t *testing.T) {
	result := tsParseIdentifierBefore("main()", '(')
	if result != "main" {
		t.Errorf("expected main, got %s", result)
	}
}

func TestTsParseIdentifierBefore_StopAtSpace(t *testing.T) {
	result := tsParseIdentifierBefore("Server extends", ' ')
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestTsParseIdentifierBefore_StopAtBrace(t *testing.T) {
	result := tsParseIdentifierBefore("Config{", ' ')
	if result != "Config" {
		t.Errorf("expected Config, got %s", result)
	}
}

func TestTsParseIdentifierBefore_Empty(t *testing.T) {
	result := tsParseIdentifierBefore("", '(')
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

// ============ tsVisibility Tests ============

func TestTsVisibility_AlwaysPrivate(t *testing.T) {
	// tsVisibility always returns "private" since export context
	// is determined by export.def capture, not name casing
	result := tsVisibility("MyClass")
	if result != "private" {
		t.Errorf("expected private, got %s", result)
	}
}

func TestTsVisibility_Lowercase(t *testing.T) {
	result := tsVisibility("myFunc")
	if result != "private" {
		t.Errorf("expected private, got %s", result)
	}
}