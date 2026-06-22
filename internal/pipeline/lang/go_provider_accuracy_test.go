package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ GoProvider InterpretDeclaration Accuracy Tests ============

func TestGoProvider_InterpretDeclaration_VarDef(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "var.def", Name: "var.name", Text: "err", StartRow: 5, EndRow: 5},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "err" {
		t.Errorf("expected name=err, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelVariable {
		t.Errorf("expected label=variable, got %s", result[0].Label)
	}
}

func TestGoProvider_InterpretDeclaration_ConstDef(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "const.def", Name: "const.name", Text: "MaxRetries", StartRow: 3, EndRow: 3},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Name != "MaxRetries" {
		t.Errorf("expected name=MaxRetries, got %s", result[0].Name)
	}
	if result[0].Label != graph.LabelConst {
		t.Errorf("expected label=const, got %s", result[0].Label)
	}
	// Go provider does not set Visibility for const.def (only fn/method/struct/iface set it)
	if result[0].Visibility != "" {
		t.Errorf("expected visibility= (not set for const), got %s", result[0].Visibility)
	}
}

func TestGoProvider_InterpretDeclaration_MultipleMatchesNoCrossContamination(t *testing.T) {
	p := NewGoProvider()
	// Two function declarations — ensure match 0's fn.name doesn't leak into match 1
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "fn.def", Name: "fn.name", Text: "foo", StartRow: 5, EndRow: 10},
		{MatchIndex: 1, NodeType: "fn.def", Name: "fn.name", Text: "bar", StartRow: 15, EndRow: 20},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(result))
	}
	names := map[string]bool{}
	for _, sym := range result {
		names[sym.Name] = true
	}
	if !names["foo"] {
		t.Error("expected foo in results")
	}
	if !names["bar"] {
		t.Error("expected bar in results")
	}
}

func TestGoProvider_InterpretDeclaration_MethodReceiverIsolation(t *testing.T) {
	p := NewGoProvider()
	// Two methods with different receivers — ensure match isolation
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "method.def", Name: "method.name", Text: "Start", StartRow: 5, EndRow: 10},
		{MatchIndex: 0, Name: "method.recv", Text: "s *Server"},
		{MatchIndex: 1, NodeType: "method.def", Name: "method.name", Text: "Process", StartRow: 15, EndRow: 20},
		{MatchIndex: 1, Name: "method.recv", Text: "h *Handler"},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(result))
	}

	recvMap := map[string]string{}
	for _, sym := range result {
		if sym.Props != nil {
			recvMap[sym.Name] = sym.Props["receiver"].(string)
		}
	}
	if recvMap["Start"] != "Server" {
		t.Errorf("expected Start receiver=Server, got %s", recvMap["Start"])
	}
	if recvMap["Process"] != "Handler" {
		t.Errorf("expected Process receiver=Handler, got %s", recvMap["Process"])
	}
}

// ============ GoProvider InterpretImport Accuracy Tests ============

func TestGoProvider_InterpretImport_DotImport(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.decl", Name: "import.path", Text: `"fmt"`, StartRow: 3},
		{MatchIndex: 0, Name: "import.alias", Text: "."},
	}
	result := p.InterpretImport(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if !result[0].IsWildcard {
		t.Error("dot import should be IsWildcard=true")
	}
	if result[0].Alias != "." {
		t.Errorf("expected alias=., got %s", result[0].Alias)
	}
}

func TestGoProvider_InterpretImport_MultipleImportsDifferentMatches(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.decl", Name: "import.path", Text: `"fmt"`, StartRow: 3},
		{MatchIndex: 1, NodeType: "import.decl", Name: "import.path", Text: `"os"`, StartRow: 4},
	}
	result := p.InterpretImport(captures, nil, "test.go")
	if len(result) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, imp := range result {
		paths[imp.Path] = true
	}
	if !paths["fmt"] {
		t.Error("expected fmt in imports")
	}
	if !paths["os"] {
		t.Error("expected os in imports")
	}
}

// ============ GoProvider InterpretTypeBinding Accuracy Tests ============

func TestGoProvider_InterpretTypeBinding_Constructor(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.constructor", Name: "type-binding.var.name", Text: "srv", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.var.type", Text: "Server"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.go")
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

func TestGoProvider_InterpretTypeBinding_Return(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.return", Name: "type-binding.return.name", Text: "NewServer", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.return.type", Text: "*Server"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "return" {
		t.Errorf("expected kind=return, got %s", result[0].Kind)
	}
	if result[0].TypeName != "*Server" {
		t.Errorf("expected typeName=*Server, got %s", result[0].TypeName)
	}
}

func TestGoProvider_InterpretTypeBinding_Assignment(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.assignment", Name: "type-binding.var.name", Text: "timeout", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.var.type", Text: "time.Duration"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "assignment" {
		t.Errorf("expected kind=assignment, got %s", result[0].Kind)
	}
	if result[0].TypeName != "time.Duration" {
		t.Errorf("expected typeName=time.Duration, got %s", result[0].TypeName)
	}
}

func TestGoProvider_InterpretTypeBinding_MultipleBindingsNoCrossContamination(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Name: "type-binding.param.name", Text: "name", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.param.type", Text: "string"},
		{MatchIndex: 1, NodeType: "type-binding.parameter", Name: "type-binding.param.name", Text: "age", StartRow: 6},
		{MatchIndex: 1, Name: "type-binding.param.type", Text: "int"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.go")
	if len(result) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result))
	}
	typeMap := map[string]string{}
	for _, tb := range result {
		typeMap[tb.Kind] = tb.TypeName
	}
	// Both are parameters, so we check by TypeName
	typeNames := map[string]bool{}
	for _, tb := range result {
		typeNames[tb.TypeName] = true
	}
	if !typeNames["string"] {
		t.Error("expected string type in bindings")
	}
	if !typeNames["int"] {
		t.Error("expected int type in bindings")
	}
}

// ============ GoProvider InterpretReference Accuracy Tests ============

func TestGoProvider_InterpretReference_FieldWriteAccuracy(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.field_write", Name: "reference.field_write.receiver", Text: "cfg", StartRow: 12},
		{MatchIndex: 0, Name: "reference.field_write.name", Text: "Port"},
	}
	result := p.InterpretReference(captures, nil, "config.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "field_write" {
		t.Errorf("expected kind=field_write, got %s", ref.Kind)
	}
	if ref.Receiver != "cfg" {
		t.Errorf("expected receiver=cfg, got %s", ref.Receiver)
	}
	if ref.Name != "Port" {
		t.Errorf("expected name=Port, got %s", ref.Name)
	}
}

func TestGoProvider_InterpretReference_MultipleRefsIsolated(t *testing.T) {
	p := NewGoProvider()
	// Three different reference types across three matches
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.free_call", Name: "reference.free_call.name", Text: "log.Fatal", StartRow: 10},
		{MatchIndex: 1, NodeType: "reference.member_call", Name: "reference.member_call.receiver", Text: "db", StartRow: 15},
		{MatchIndex: 1, Name: "reference.member_call.method", Text: "Query"},
		{MatchIndex: 2, NodeType: "reference.constructor", Name: "reference.constructor.type", Text: "Config", StartRow: 20},
	}
	result := p.InterpretReference(captures, nil, "main.go")
	if len(result) != 3 {
		t.Fatalf("expected 3 references, got %d", len(result))
	}

	// Verify each reference has correct kind and data
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

// ============ GoProvider InterpretScope Accuracy Tests ============

func TestGoProvider_InterpretScope_ModuleScope(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.module", Text: "package main", StartRow: 1, EndRow: 100},
	}
	result := p.InterpretScope(captures, nil, "cmd/app/main.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	if result[0].Kind != "module" {
		t.Errorf("expected kind=module, got %s", result[0].Kind)
	}
	// GoPackageName("cmd/app/main.go") returns "app" (base of "cmd/app" is "app", not "cmd" or "main")
	if result[0].Name != "app" {
		t.Errorf("expected name=app for cmd/app dir, got %s", result[0].Name)
	}
}

func TestGoProvider_InterpretScope_SkipsUnknownNodeType(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.unknown", Text: "unknown", StartRow: 1, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "test.go")
	if len(result) != 0 {
		t.Errorf("expected 0 scopes for unknown node type, got %d", len(result))
	}
}

// ============ GoProvider Language() and ImportSemantics() Tests ============

func TestGoProvider_Language(t *testing.T) {
	p := NewGoProvider()
	if p.Language() != graph.LabelGoFile {
		t.Errorf("expected LabelGoFile, got %s", p.Language())
	}
}

func TestGoProvider_ImportSemantics(t *testing.T) {
	p := NewGoProvider()
	if p.ImportSemantics() != ImportSemanticsWildcardLeaf {
		t.Errorf("expected ImportSemanticsWildcardLeaf, got %s", p.ImportSemantics())
	}
}

// ============ GoProvider QuerySet Tests ============

func TestGoProvider_QuerySetNotNil(t *testing.T) {
	p := NewGoProvider()
	qs := p.QuerySet()
	if qs == nil {
		t.Fatal("QuerySet should not be nil")
	}
	if qs.Scope == "" {
		t.Error("Scope query should not be empty")
	}
	if qs.Declaration == "" {
		t.Error("Declaration query should not be empty")
	}
	if qs.Import == "" {
		t.Error("Import query should not be empty")
	}
	if qs.TypeBinding == "" {
		t.Error("TypeBinding query should not be empty")
	}
	if qs.Reference == "" {
		t.Error("Reference query should not be empty")
	}
}

// ============ GoProvider Captures/CallExtract/ClassExtract/FieldExtract/ImportResolve Tests ============

func TestGoProvider_CapturesConfig(t *testing.T) {
	p := NewGoProvider()
	cc := p.Captures()
	if cc == nil {
		t.Fatal("Captures config should not be nil")
	}
	if cc.Query == "" {
		t.Error("Captures query should not be empty")
	}
	if len(cc.CaptureMap) == 0 {
		t.Error("CaptureMap should not be empty")
	}
}

func TestGoProvider_CallExtractConfig(t *testing.T) {
	p := NewGoProvider()
	cec := p.CallExtractConfig()
	if cec == nil {
		t.Fatal("CallExtractConfig should not be nil")
	}
	if cec.Query == "" {
		t.Error("CallExtract query should not be empty")
	}
}

func TestGoProvider_ClassExtractConfig(t *testing.T) {
	p := NewGoProvider()
	cec := p.ClassExtractConfig()
	if cec == nil {
		t.Fatal("ClassExtractConfig should not be nil")
	}
	if cec.Query == "" {
		t.Error("ClassExtract query should not be empty")
	}
}

func TestGoProvider_FieldExtractConfig(t *testing.T) {
	p := NewGoProvider()
	fec := p.FieldExtractConfig()
	if fec == nil {
		t.Fatal("FieldExtractConfig should not be nil")
	}
	if fec.Query == "" {
		t.Error("FieldExtract query should not be empty")
	}
}

func TestGoProvider_ImportResolveConfig(t *testing.T) {
	p := NewGoProvider()
	irc := p.ImportResolveConfig()
	if irc == nil {
		t.Fatal("ImportResolveConfig should not be nil")
	}
	if irc.Query == "" {
		t.Error("ImportResolve query should not be empty")
	}
	if !irc.IsDotImport {
		t.Error("Go should support dot imports")
	}
	if !irc.IsWildcard {
		t.Error("Go should support wildcard imports")
	}
}