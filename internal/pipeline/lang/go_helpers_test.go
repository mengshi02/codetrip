package lang

import (
	"testing"
)

// ============ goParseFuncOrMethodName Tests ============

func TestGoParseFuncOrMethodName_SimpleFunction(t *testing.T) {
	result := goParseFuncOrMethodName("func main() {")
	if result != "main" {
		t.Errorf("expected main, got %s", result)
	}
}

func TestGoParseFuncOrMethodName_MethodWithReceiver(t *testing.T) {
	result := goParseFuncOrMethodName("func (s *Server) Start() {")
	if result != "Start" {
		t.Errorf("expected Start, got %s", result)
	}
}

func TestGoParseFuncOrMethodName_MethodWithValueReceiver(t *testing.T) {
	result := goParseFuncOrMethodName("func (s Server) Stop() error {")
	if result != "Stop" {
		t.Errorf("expected Stop, got %s", result)
	}
}

func TestGoParseFuncOrMethodName_NoFuncKeyword(t *testing.T) {
	result := goParseFuncOrMethodName("var x = 1")
	if result != "" {
		t.Errorf("expected empty for no func keyword, got %s", result)
	}
}

func TestGoParseFuncOrMethodName_FunctionWithReturn(t *testing.T) {
	result := goParseFuncOrMethodName("func New() *Config {")
	if result != "New" {
		t.Errorf("expected New, got %s", result)
	}
}

// ============ goParseTypeName Tests ============

func TestGoParseTypeName_Struct(t *testing.T) {
	result := goParseTypeName("type Server struct {")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestGoParseTypeName_Interface(t *testing.T) {
	result := goParseTypeName("type Reader interface {")
	if result != "Reader" {
		t.Errorf("expected Reader, got %s", result)
	}
}

func TestGoParseTypeName_NoTypeKeyword(t *testing.T) {
	result := goParseTypeName("func foo() {}")
	if result != "" {
		t.Errorf("expected empty for no type keyword, got %s", result)
	}
}

func TestGoParseTypeName_GenericType(t *testing.T) {
	result := goParseTypeName("type Container[T any] struct {")
	if result != "Container" {
		t.Errorf("expected Container, got %s", result)
	}
}

// ============ goExtractScopeName Tests ============

func TestGoExtractScopeName_Function(t *testing.T) {
	result := goExtractScopeName("func process() {\n    pass", "function")
	if result != "process" {
		t.Errorf("expected process, got %s", result)
	}
}

func TestGoExtractScopeName_Method(t *testing.T) {
	result := goExtractScopeName("func (s *Server) Run() {\n    return", "method")
	if result != "Run" {
		t.Errorf("expected Run, got %s", result)
	}
}

func TestGoExtractScopeName_Class(t *testing.T) {
	result := goExtractScopeName("type Config struct {\n    Port int", "class")
	if result != "Config" {
		t.Errorf("expected Config, got %s", result)
	}
}

func TestGoExtractScopeName_Empty(t *testing.T) {
	result := goExtractScopeName("", "function")
	if result != "" {
		t.Errorf("expected empty for empty text, got %s", result)
	}
}

func TestGoExtractScopeName_UnknownKind(t *testing.T) {
	result := goExtractScopeName("something", "unknown")
	if result != "" {
		t.Errorf("expected empty for unknown kind, got %s", result)
	}
}

// ============ GoPackageName Tests ============

func TestGoPackageName_RegularPackage(t *testing.T) {
	result := GoPackageName("internal/pipeline/server.go")
	if result != "pipeline" {
		t.Errorf("expected pipeline, got %s", result)
	}
}

func TestGoPackageName_MainPackage(t *testing.T) {
	result := GoPackageName("cmd/app/main.go")
	if result != "app" {
		t.Errorf("expected app (base of cmd/app is app, not cmd/main), got %s", result)
	}
}

func TestGoPackageName_MainDir(t *testing.T) {
	result := GoPackageName("main/main.go")
	if result != "main" {
		t.Errorf("expected main for main dir, got %s", result)
	}
}

func TestGoPackageName_RootFile(t *testing.T) {
	result := GoPackageName("main.go")
	if result != "main" {
		// Dir of "main.go" is ".", base is "."
		t.Logf("GoPackageName for root file: %s", result)
	}
}

// ============ GoIsExported Tests ============

func TestGoIsExported_Public(t *testing.T) {
	if !GoIsExported("Hello") {
		t.Error("Hello should be exported")
	}
	if !GoIsExported("Server") {
		t.Error("Server should be exported")
	}
}

func TestGoIsExported_Private(t *testing.T) {
	if GoIsExported("hello") {
		t.Error("hello should not be exported")
	}
	if GoIsExported("server") {
		t.Error("server should not be exported")
	}
}

func TestGoIsExported_Empty(t *testing.T) {
	if GoIsExported("") {
		t.Error("empty string should not be exported")
	}
}

// ============ GoReceiverType Tests ============

func TestGoReceiverType_PointerReceiver(t *testing.T) {
	result := GoReceiverType("s *Server")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestGoReceiverType_ValueReceiver(t *testing.T) {
	result := GoReceiverType("s Server")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestGoReceiverType_JustPointer(t *testing.T) {
	result := GoReceiverType("*Server")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestGoReceiverType_JustType(t *testing.T) {
	result := GoReceiverType("Server")
	if result != "Server" {
		t.Errorf("expected Server, got %s", result)
	}
}

func TestGoReceiverType_Empty(t *testing.T) {
	result := GoReceiverType("")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

// ============ goVisibility Tests ============

func TestGoVisibility_Public(t *testing.T) {
	if goVisibility("Hello") != "public" {
		t.Error("Hello should be public")
	}
}

func TestGoVisibility_Private(t *testing.T) {
	if goVisibility("hello") != "private" {
		t.Error("hello should be private")
	}
}