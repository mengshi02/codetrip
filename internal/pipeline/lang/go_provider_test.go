package lang

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ GoProvider InterpretScope Tests ============

func TestGoProvider_InterpretScope_Empty(t *testing.T) {
	p := NewGoProvider()
	result := p.InterpretScope(nil, nil, "test.go")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestGoProvider_InterpretScope_FunctionScope(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.function", Name: "scope.function", Text: "func main() {", StartRow: 5, EndRow: 10},
	}
	result := p.InterpretScope(captures, nil, "main.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(result))
	}
	s := result[0]
	if s.Kind != "function" {
		t.Errorf("expected kind=function, got %s", s.Kind)
	}
	if s.Name != "main" {
		t.Errorf("expected name=main, got %s", s.Name)
	}
	if s.FilePath != "main.go" {
		t.Errorf("expected filePath=main.go, got %s", s.FilePath)
	}
}

func TestGoProvider_InterpretScope_NestedScopes(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "scope.class", Name: "scope.class", Text: "type Server struct {", StartRow: 3, EndRow: 50},
		{MatchIndex: 1, NodeType: "scope.method", Name: "scope.method", Text: "func (s *Server) Start() {", StartRow: 10, EndRow: 30},
		{MatchIndex: 2, NodeType: "scope.method", Name: "scope.method", Text: "func (s *Server) Stop() {", StartRow: 35, EndRow: 45},
	}
	result := p.InterpretScope(captures, nil, "server.go")
	if len(result) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(result))
	}

	// After buildScopeParentIDs, methods should have class as parent
	// Sort by name for stable testing
	scopeMap := make(map[string]*pipeline.ScopeInfo)
	for _, s := range result {
		scopeMap[s.Name] = s
	}

	if scopeMap["Server"].ParentID != "" {
		t.Errorf("Server should be root scope, got parentID=%s", scopeMap["Server"].ParentID)
	}
	if scopeMap["Start"].ParentID != scopeMap["Server"].ID {
		t.Errorf("Start should have Server as parent, got parentID=%s", scopeMap["Start"].ParentID)
	}
	if scopeMap["Stop"].ParentID != scopeMap["Server"].ID {
		t.Errorf("Stop should have Server as parent, got parentID=%s", scopeMap["Stop"].ParentID)
	}
}

// ============ GoProvider InterpretDeclaration Tests ============

func TestGoProvider_InterpretDeclaration_Empty(t *testing.T) {
	p := NewGoProvider()
	result := p.InterpretDeclaration(nil, nil, "test.go")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestGoProvider_InterpretDeclaration_Function(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "fn.def", Name: "fn.name", Text: "Hello", StartRow: 5, EndRow: 10},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	sym := result[0]
	if sym.Name != "Hello" {
		t.Errorf("expected name=Hello, got %s", sym.Name)
	}
	if sym.Label != graph.LabelFunction {
		t.Errorf("expected label=function, got %s", sym.Label)
	}
	if sym.Visibility != "public" {
		t.Errorf("expected visibility=public for Hello, got %s", sym.Visibility)
	}
}

func TestGoProvider_InterpretDeclaration_MethodWithReceiver(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "method.def", Name: "method.name", Text: "Process", StartRow: 15, EndRow: 25},
		{MatchIndex: 0, NodeType: "method.recv", Name: "method.recv", Text: "s *Server", StartRow: 15, EndRow: 15},
	}
	result := p.InterpretDeclaration(captures, nil, "server.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	sym := result[0]
	if sym.Name != "Process" {
		t.Errorf("expected name=Process, got %s", sym.Name)
	}
	if sym.Label != graph.LabelMethod {
		t.Errorf("expected label=method, got %s", sym.Label)
	}
	if sym.Props["receiver"] != "Server" {
		t.Errorf("expected receiver=Server, got %v", sym.Props["receiver"])
	}
}

func TestGoProvider_InterpretDeclaration_StructAndInterface(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "struct.def", Name: "type.name", Text: "Config", StartRow: 5, EndRow: 15},
		{MatchIndex: 1, NodeType: "iface.def", Name: "type.name", Text: "Reader", StartRow: 20, EndRow: 25},
	}
	result := p.InterpretDeclaration(captures, nil, "types.go")
	if len(result) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(result))
	}
	// InterpretDeclaration iterates a map (seen), so order is non-deterministic
	found := map[string]string{}
	for _, sym := range result {
		found[string(sym.Label)] = sym.Name
	}
	if found[string(graph.LabelStruct)] != "Config" {
		t.Errorf("expected struct name=Config, got %s", found[string(graph.LabelStruct)])
	}
	if found[string(graph.LabelInterface)] != "Reader" {
		t.Errorf("expected interface name=Reader, got %s", found[string(graph.LabelInterface)])
	}
}

func TestGoProvider_InterpretDeclaration_PrivateVisibility(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "fn.def", Name: "fn.name", Text: "helper", StartRow: 5, EndRow: 8},
	}
	result := p.InterpretDeclaration(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(result))
	}
	if result[0].Visibility != "private" {
		t.Errorf("expected visibility=private for lowercase, got %s", result[0].Visibility)
	}
}

// ============ GoProvider InterpretImport Tests ============

func TestGoProvider_InterpretImport_Empty(t *testing.T) {
	p := NewGoProvider()
	result := p.InterpretImport(nil, nil, "test.go")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestGoProvider_InterpretImport_SingleImport(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.decl", Name: "import.path", Text: "\"fmt\"", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	imp := result[0]
	if imp.Path != "fmt" {
		t.Errorf("expected path=fmt, got %s", imp.Path)
	}
	if imp.SourceFile != "test.go" {
		t.Errorf("expected sourceFile=test.go, got %s", imp.SourceFile)
	}
}

func TestGoProvider_InterpretImport_WithAlias(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "import.decl", Name: "import.path", Text: "\"encoding/json\"", StartRow: 3},
		{MatchIndex: 0, NodeType: "import.decl", Name: "import.alias", Text: "json", StartRow: 3},
	}
	result := p.InterpretImport(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 import, got %d", len(result))
	}
	if result[0].Alias != "json" {
		t.Errorf("expected alias=json, got %s", result[0].Alias)
	}
}

// ============ GoProvider InterpretTypeBinding Tests ============

func TestGoProvider_InterpretTypeBinding_Empty(t *testing.T) {
	p := NewGoProvider()
	result := p.InterpretTypeBinding(nil, nil, "test.go")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestGoProvider_InterpretTypeBinding_Parameter(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.parameter", Name: "type-binding.param.name", Text: "name", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.param.type", Text: "string"},
	}
	result := p.InterpretTypeBinding(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	tb := result[0]
	if tb.Kind != "parameter" {
		t.Errorf("expected kind=parameter, got %s", tb.Kind)
	}
	if tb.TypeName != "string" {
		t.Errorf("expected typeName=string, got %s", tb.TypeName)
	}
}

func TestGoProvider_InterpretTypeBinding_Receiver(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.receiver", Name: "type-binding.receiver.type", Text: "Server", StartRow: 10},
		{MatchIndex: 0, Name: "type-binding.receiver.name", Text: "s"},
	}
	result := p.InterpretTypeBinding(captures, nil, "server.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "receiver" {
		t.Errorf("expected kind=receiver, got %s", result[0].Kind)
	}
	if result[0].TypeName != "Server" {
		t.Errorf("expected typeName=Server, got %s", result[0].TypeName)
	}
}

func TestGoProvider_InterpretTypeBinding_Alias(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "type-binding.alias", Name: "type-binding.alias.name", Text: "Handler", StartRow: 5},
		{MatchIndex: 0, Name: "type-binding.alias.target", Text: "http.Handler"},
	}
	result := p.InterpretTypeBinding(captures, nil, "types.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result))
	}
	if result[0].Kind != "alias" {
		t.Errorf("expected kind=alias, got %s", result[0].Kind)
	}
	if result[0].TypeName != "http.Handler" {
		t.Errorf("expected typeName=http.Handler, got %s", result[0].TypeName)
	}
}

// ============ GoProvider InterpretReference Tests ============

func TestGoProvider_InterpretReference_Empty(t *testing.T) {
	p := NewGoProvider()
	result := p.InterpretReference(nil, nil, "test.go")
	if result != nil {
		t.Errorf("expected nil for empty captures, got %v", result)
	}
}

func TestGoProvider_InterpretReference_FreeCall(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.free_call", Name: "reference.free_call.name", Text: "fmt.Println", StartRow: 10},
	}
	result := p.InterpretReference(captures, nil, "test.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "free_call" {
		t.Errorf("expected kind=free_call, got %s", ref.Kind)
	}
	if ref.Name != "fmt.Println" {
		t.Errorf("expected name=fmt.Println, got %s", ref.Name)
	}
	if ref.Receiver != "" {
		t.Errorf("expected empty receiver for free_call, got %s", ref.Receiver)
	}
}

func TestGoProvider_InterpretReference_MemberCall(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.member_call", Name: "reference.member_call.receiver", Text: "srv", StartRow: 15},
		{MatchIndex: 0, Name: "reference.member_call.method", Text: "Start"},
	}
	result := p.InterpretReference(captures, nil, "server.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	ref := result[0]
	if ref.Kind != "member_call" {
		t.Errorf("expected kind=member_call, got %s", ref.Kind)
	}
	if ref.Receiver != "srv" {
		t.Errorf("expected receiver=srv, got %s", ref.Receiver)
	}
	if ref.Name != "Start" {
		t.Errorf("expected name=Start, got %s", ref.Name)
	}
}

func TestGoProvider_InterpretReference_Constructor(t *testing.T) {
	p := NewGoProvider()
	captures := []pipeline.LangCapture{
		{MatchIndex: 0, NodeType: "reference.constructor", Name: "reference.constructor.type", Text: "Server", StartRow: 20},
	}
	result := p.InterpretReference(captures, nil, "server.go")
	if len(result) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(result))
	}
	if result[0].Kind != "constructor" {
		t.Errorf("expected kind=constructor, got %s", result[0].Kind)
	}
	if result[0].Name != "Server" {
		t.Errorf("expected name=Server, got %s", result[0].Name)
	}
}

func TestGoProvider_InterpretReference_FieldWrite(t *testing.T) {
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

// ============ GoProvider Mixed Reference Types ============

func TestGoProvider_InterpretReference_MultipleTypes(t *testing.T) {
	p := NewGoProvider()
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

	kinds := make(map[string]bool)
	for _, ref := range result {
		kinds[ref.Kind] = true
	}
	if !kinds["free_call"] {
		t.Error("expected free_call reference")
	}
	if !kinds["member_call"] {
		t.Error("expected member_call reference")
	}
	if !kinds["constructor"] {
		t.Error("expected constructor reference")
	}
}
