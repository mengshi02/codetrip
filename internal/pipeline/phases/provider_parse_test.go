package phases

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/mengshi02/codetrip/internal/pipeline/lang"
)

// ============ parseWithProvider End-to-End Tests ============

// TestParseWithProvider_GoFile verifies that parseWithProvider correctly
// extracts Scopes, Symbols, Imports, TypeBindings, and References from
// a real Go source file using tree-sitter queries.
func TestParseWithProvider_GoFile(t *testing.T) {
	goSource := []byte(`package main

import "fmt"

type Server struct {
	Port int
}

func NewServer(port int) *Server {
	return &Server{Port: port}
}

func (s *Server) Start() error {
	fmt.Println("starting")
	return nil
}

func main() {
	srv := NewServer(8080)
	srv.Start()
}
`)

	provider := lang.NewGoProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Go query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "main.go",
		Language: "go",
		Content:  goSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes to be extracted, got 0")
	}
	scopeKinds := make(map[string]int)
	for _, s := range f.Scopes {
		scopeKinds[s.Kind]++
	}
	if scopeKinds["function"] == 0 {
		t.Error("expected at least one function scope")
	}
	if scopeKinds["class"] == 0 {
		t.Error("expected at least one class scope (struct)")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols to be extracted, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["Server"] {
		t.Error("expected Server symbol")
	}
	if !symbolNames["Start"] {
		t.Error("expected Start symbol")
	}
	if !symbolNames["NewServer"] {
		t.Error("expected NewServer symbol")
	}
	if !symbolNames["main"] {
		t.Error("expected main symbol")
	}

	// Verify visibility
	for _, sym := range f.Symbols {
		if sym.Name == "main" && sym.Visibility != "private" {
			t.Errorf("main should be private (unexported in Go), got %s", sym.Visibility)
		}
		if sym.Name == "Server" && sym.Visibility != "public" {
			t.Errorf("Server should be public, got %s", sym.Visibility)
		}
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports to be extracted, got 0")
	}
	foundFmt := false
	for _, imp := range f.Imports {
		if imp.Path == "fmt" {
			foundFmt = true
		}
	}
	if !foundFmt {
		t.Error("expected 'fmt' import")
	}

	// Verify TypeBindings
	if len(f.TypeBindings) == 0 {
		t.Error("expected type bindings to be extracted, got 0")
	}
	foundReceiverBinding := false
	for _, tb := range f.TypeBindings {
		if tb.Kind == "receiver" && tb.TypeName == "Server" {
			foundReceiverBinding = true
		}
	}
	if !foundReceiverBinding {
		t.Error("expected receiver type binding for Server")
	}

	// Verify References
	if len(f.References) == 0 {
		t.Error("expected references to be extracted, got 0")
	}
	refKinds := make(map[string]int)
	for _, ref := range f.References {
		refKinds[ref.Kind]++
	}
	// Should have at least one free_call (fmt.Println)
	if refKinds["free_call"] == 0 {
		t.Error("expected at least one free_call reference")
	}
	// Should have at least one constructor (&Server{})
	if refKinds["constructor"] == 0 {
		t.Error("expected at least one constructor reference")
	}
	// Should have at least one member_call (srv.Start)
	if refKinds["member_call"] == 0 {
		t.Error("expected at least one member_call reference")
	}
}

// TestParseWithProvider_PythonFile verifies Python extraction via parseWithProvider.
func TestParseWithProvider_PythonFile(t *testing.T) {
	pySource := []byte(`import os
from typing import List

class DataProcessor:
    def process(self, data: List[str]) -> bool:
        result = self.transform(data)
        print(result)
        return True

    def transform(self, data):
        return data
`)

	provider := lang.NewPythonProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Python query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "processor.py",
		Language: "python",
		Content:  pySource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes to be extracted from Python, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols to be extracted from Python, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["DataProcessor"] {
		t.Error("expected DataProcessor symbol")
	}
	if !symbolNames["process"] {
		t.Error("expected process symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from Python, got 0")
	}

	// Verify TypeBindings
	if len(f.TypeBindings) == 0 {
		t.Error("expected type bindings from Python, got 0")
	}

	// Verify References
	if len(f.References) == 0 {
		t.Error("expected references from Python, got 0")
	}
}

// TestCompileQueries_GoProvider verifies that all Go queries compile successfully.
func TestCompileQueries_GoProvider(t *testing.T) {
	provider := lang.NewGoProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Go query compilation failed: %v", err)
	}
	if cq.scope == nil {
		t.Error("expected scope query to be compiled")
	}
	if cq.declaration == nil {
		t.Error("expected declaration query to be compiled")
	}
	if cq.importQ == nil {
		t.Error("expected import query to be compiled")
	}
	if cq.typeBinding == nil {
		t.Error("expected type-binding query to be compiled")
	}
	if cq.reference == nil {
		t.Error("expected reference query to be compiled")
	}
}

// TestCompileQueries_PythonProvider verifies that all Python queries compile successfully.
func TestCompileQueries_PythonProvider(t *testing.T) {
	provider := lang.NewPythonProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("failed to compile Python queries: %v", err)
	}
	if cq.scope == nil {
		t.Error("expected scope query to be compiled")
	}
	if cq.declaration == nil {
		t.Error("expected declaration query to be compiled")
	}
	if cq.importQ == nil {
		t.Error("expected import query to be compiled")
	}
	if cq.typeBinding == nil {
		t.Error("expected type-binding query to be compiled")
	}
	if cq.reference == nil {
		t.Error("expected reference query to be compiled")
	}
}

// TestParseWithProvider_EmptyFile verifies that parsing an empty file
// does not panic and returns empty results.
func TestParseWithProvider_EmptyFile(t *testing.T) {
	provider := lang.NewGoProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Go query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "empty.go",
		Language: "go",
		Content:  []byte(""),
	}

	parseWithProvider(f, provider, cq)

	if len(f.Scopes) != 0 {
		t.Errorf("expected 0 scopes for empty file, got %d", len(f.Scopes))
	}
	if len(f.Symbols) != 0 {
		t.Errorf("expected 0 symbols for empty file, got %d", len(f.Symbols))
	}
	if len(f.Imports) != 0 {
		t.Errorf("expected 0 imports for empty file, got %d", len(f.Imports))
	}
	if len(f.TypeBindings) != 0 {
		t.Errorf("expected 0 type bindings for empty file, got %d", len(f.TypeBindings))
	}
	if len(f.References) != 0 {
		t.Errorf("expected 0 references for empty file, got %d", len(f.References))
	}
}

// ============ createSymbolNode Tests ============

func TestCreateSymbolNode_StructuredFields(t *testing.T) {
	sym := &pipeline.SymbolInfo{
		Name:          "Process",
		Label:         graph.LabelMethod,
		FilePath:      "handler.go",
		StartLine:     10,
		EndLine:       25,
		Visibility:    "public",
		IsStatic:      false,
		IsAbstract:    true,
		IsAsync:       true,
		ReturnType:    "error",
		Annotations:   []string{"@override"},
		QualifiedName: "pkg.Handler.Process",
		Props:         map[string]any{"receiver": "Handler"},
	}

	node := createSymbolNode("test-repo", sym)

	if node.Name != "Process" {
		t.Errorf("expected name=Process, got %s", node.Name)
	}
	if node.GetPropString("visibility") != "public" {
		t.Errorf("expected visibility=public, got %v", node.GetProp("visibility", nil))
	}
	if !node.GetPropBool("isAbstract") {
		t.Errorf("expected isAbstract=true, got %v", node.GetProp("isAbstract", false))
	}
	if !node.GetPropBool("isAsync") {
		t.Errorf("expected isAsync=true, got %v", node.GetProp("isAsync", false))
	}
	if node.GetPropString("returnType") != "error" {
		t.Errorf("expected returnType=error, got %v", node.GetProp("returnType", nil))
	}
	if node.GetPropString("qualifiedName") != "pkg.Handler.Process" {
		t.Errorf("expected qualifiedName, got %v", node.GetProp("qualifiedName", nil))
	}
	// Props map should also be preserved
	if node.GetPropString("receiver") != "Handler" {
		t.Errorf("expected receiver=Handler from Props map, got %v", node.GetProp("receiver", nil))
	}
}

func TestCreateSymbolNode_MinimalFields(t *testing.T) {
	sym := &pipeline.SymbolInfo{
		Name:      "helper",
		Label:     graph.LabelFunction,
		FilePath:  "util.go",
		StartLine: 5,
		EndLine:   10,
	}

	node := createSymbolNode("test-repo", sym)

	if node.Name != "helper" {
		t.Errorf("expected name=helper, got %s", node.Name)
	}
	// Should not have optional props when fields are zero/default
	if v, ok := node.Props.GetProp("isStatic"); ok && v != false {
		t.Error("should not have isStatic prop when false")
	}
	if v, ok := node.Props.GetProp("visibility"); ok && v != "" {
		t.Errorf("should not have visibility prop when empty, got %v", v)
	}
}

// ============ Multi-Language E2E Tests ============

// TestCompileQueries_AllProviders verifies that all language queries compile successfully.
func TestCompileQueries_AllProviders(t *testing.T) {
	type providerInit struct {
		name     string
		provider langProvider
	}
	providers := []providerInit{
		{"C", lang.NewCProvider()},
		{"CPP", lang.NewCPPProvider()},
		{"CSharp", lang.NewCSharpProvider()},
		{"JavaScript", lang.NewJavaScriptProvider()},
		{"TypeScript", lang.NewTypeScriptProvider()},
		{"Java", lang.NewJavaProvider()},
		{"Rust", lang.NewRustProvider()},
		{"Markdown", lang.NewMarkdownProvider()},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			tsLang := p.provider.TreeSitterLanguage()
			cq, err := compileQueries(p.provider, tsLang)
			if err != nil {
				t.Fatalf("%s query compilation failed: %v", p.name, err)
			}
			if cq.scope == nil {
				t.Errorf("%s: expected scope query", p.name)
			}
			if cq.declaration == nil {
				t.Errorf("%s: expected declaration query", p.name)
			}
			// Import/TypeBinding/Reference may be empty for some languages (e.g. Markdown)
		})
	}
}

// TestParseWithProvider_CFile verifies C extraction via parseWithProvider.
func TestParseWithProvider_CFile(t *testing.T) {
	cSource := []byte(`#include <stdio.h>
#include <stdlib.h>

typedef struct Node {
    int value;
    struct Node* next;
} Node;

Node* create_node(int val) {
    Node* n = (Node*)malloc(sizeof(Node));
    n->value = val;
    n->next = NULL;
    return n;
}

int main() {
    Node* head = create_node(42);
    printf("value: %d\n", head->value);
    free(head);
    return 0;
}
`)

	provider := lang.NewCProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("C query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "test.c",
		Language: "c",
		Content:  cSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from C, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from C, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["create_node"] {
		t.Error("expected create_node symbol")
	}
	if !symbolNames["main"] {
		t.Error("expected main symbol")
	}

	// Verify Imports (includes)
	if len(f.Imports) == 0 {
		t.Error("expected imports (includes) from C, got 0")
	}

	// Verify TypeBindings
	if len(f.TypeBindings) == 0 {
		t.Error("expected type bindings from C, got 0")
	}

	// Verify References
	if len(f.References) == 0 {
		t.Error("expected references from C, got 0")
	}
}

// TestParseWithProvider_CPPFile verifies C++ extraction via parseWithProvider.
func TestParseWithProvider_CPPFile(t *testing.T) {
	cppSource := []byte(`#include <iostream>
#include <vector>

class Calculator {
public:
    int add(int a, int b) {
        return a + b;
    }
};

int main() {
    Calculator calc;
    std::cout << calc.add(1, 2) << std::endl;
    return 0;
}
`)

	provider := lang.NewCPPProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("CPP query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "test.cpp",
		Language: "cpp",
		Content:  cppSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from CPP, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from CPP, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["Calculator"] {
		t.Error("expected Calculator symbol")
	}
	if !symbolNames["add"] {
		t.Error("expected add symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from CPP, got 0")
	}
}

// TestParseWithProvider_CSharpFile verifies C# extraction via parseWithProvider.
func TestParseWithProvider_CSharpFile(t *testing.T) {
	csSource := []byte(`using System;
using System.Collections.Generic;

namespace MyApp
{
    public class Processor
    {
        private List<string> items;

        public Processor(List<string> data)
        {
            items = data;
        }

        public int Process()
        {
            return items.Count;
        }
    }
}
`)

	provider := lang.NewCSharpProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("CSharp query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "Processor.cs",
		Language: "csharp",
		Content:  csSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from CSharp, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from CSharp, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["Processor"] {
		t.Error("expected Processor symbol")
	}
	if !symbolNames["Process"] {
		t.Error("expected Process symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from CSharp, got 0")
	}
}

// TestParseWithProvider_JSFile verifies JavaScript extraction via parseWithProvider.
func TestParseWithProvider_JSFile(t *testing.T) {
	jsSource := []byte(`import React from 'react';
import { useState, useEffect } from 'react';

class Counter extends React.Component {
    constructor(props) {
        super(props);
        this.state = { count: 0 };
    }

    increment() {
        this.setState({ count: this.state.count + 1 });
    }

    render() {
        return React.createElement('div', null, this.state.count);
    }
}

function App() {
    const [count, setCount] = useState(0);
    useEffect(() => {
        console.log('mounted');
    }, []);
    return count;
}

export default App;
`)

	provider := lang.NewJavaScriptProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("JS query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "App.js",
		Language: "javascript",
		Content:  jsSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from JS, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from JS, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["Counter"] {
		t.Error("expected Counter symbol")
	}
	if !symbolNames["App"] {
		t.Error("expected App symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from JS, got 0")
	}

	// Verify References
	if len(f.References) == 0 {
		t.Error("expected references from JS, got 0")
	}
}

// TestParseWithProvider_TSFile verifies TypeScript extraction via parseWithProvider.
func TestParseWithProvider_TSFile(t *testing.T) {
	tsSource := []byte(`import { Observable } from 'rxjs';

interface User {
    name: string;
    age: number;
}

class UserService {
    private users: User[] = [];

    addUser(user: User): void {
        this.users.push(user);
    }

    getUser(name: string): User | undefined {
        return this.users.find(u => u.name === name);
    }
}

function createService(): UserService {
    return new UserService();
}

export { UserService, createService };
`)

	provider := lang.NewTypeScriptProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("TS query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "service.ts",
		Language: "typescript",
		Content:  tsSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from TS, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from TS, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["UserService"] {
		t.Error("expected UserService symbol")
	}
	if !symbolNames["addUser"] {
		t.Error("expected addUser symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from TS, got 0")
	}

	// Verify TypeBindings
	if len(f.TypeBindings) == 0 {
		t.Error("expected type bindings from TS, got 0")
	}
}

// TestParseWithProvider_JavaFile verifies Java extraction via parseWithProvider.
func TestParseWithProvider_JavaFile(t *testing.T) {
	javaSource := []byte(`package com.example;

import java.util.List;
import java.util.ArrayList;

public class UserService {
    private List<String> users;

    public UserService() {
        this.users = new ArrayList<>();
    }

    public void addUser(String name) {
        users.add(name);
    }

    public List<String> getUsers() {
        return users;
    }
}
`)

	provider := lang.NewJavaProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Java query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "UserService.java",
		Language: "java",
		Content:  javaSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from Java, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from Java, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["UserService"] {
		t.Error("expected UserService symbol")
	}
	if !symbolNames["addUser"] {
		t.Error("expected addUser symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from Java, got 0")
	}

	// Verify TypeBindings
	if len(f.TypeBindings) == 0 {
		t.Error("expected type bindings from Java, got 0")
	}
}

// TestParseWithProvider_RustFile verifies Rust extraction via parseWithProvider.
func TestParseWithProvider_RustFile(t *testing.T) {
	rustSource := []byte(`use std::collections::HashMap;
use std::io::Result;

struct Cache {
    data: HashMap<String, String>,
}

impl Cache {
    fn new() -> Self {
        Cache {
            data: HashMap::new(),
        }
    }

    fn get(&self, key: &str) -> Option<&String> {
        self.data.get(key)
    }

    fn set(&mut self, key: String, value: String) {
        self.data.insert(key, value);
    }
}

fn main() -> Result<()> {
    let mut cache = Cache::new();
    cache.set("hello".to_string(), "world".to_string());
    Ok(())
}
`)

	provider := lang.NewRustProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Rust query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "main.rs",
		Language: "rust",
		Content:  rustSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from Rust, got 0")
	}

	// Verify Symbols
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from Rust, got 0")
	}
	symbolNames := make(map[string]bool)
	for _, sym := range f.Symbols {
		symbolNames[sym.Name] = true
	}
	if !symbolNames["Cache"] {
		t.Error("expected Cache symbol")
	}

	// Verify Imports
	if len(f.Imports) == 0 {
		t.Error("expected imports from Rust, got 0")
	}
}

// TestParseWithProvider_MarkdownFile verifies Markdown extraction via parseWithProvider.
func TestParseWithProvider_MarkdownFile(t *testing.T) {
	mdSource := []byte(`# Project Title

## Introduction

This is the introduction section.

## API Reference

### GET /users

Returns a list of users.

### POST /users

Creates a new user.
`)

	provider := lang.NewMarkdownProvider()
	tsLang := provider.TreeSitterLanguage()
	cq, err := compileQueries(provider, tsLang)
	if err != nil {
		t.Fatalf("Markdown query compilation failed: %v", err)
	}

	f := &pipeline.ParsedFile{
		Path:     "README.md",
		Language: "markdown",
		Content:  mdSource,
	}

	parseWithProvider(f, provider, cq)

	// Verify Scopes
	if len(f.Scopes) == 0 {
		t.Error("expected scopes from Markdown, got 0")
	}

	// Verify Symbols (headings become declarations)
	if len(f.Symbols) == 0 {
		t.Error("expected symbols from Markdown, got 0")
	}

	// Markdown has no imports
	if len(f.Imports) != 0 {
		t.Errorf("expected 0 imports from Markdown, got %d", len(f.Imports))
	}
}