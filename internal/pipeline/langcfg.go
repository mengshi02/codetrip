package pipeline

import "github.com/mengshi02/codetrip/internal/graph"

// ============ Language Provider Configuration Types ============
// These types are defined in the pipeline package to avoid circular dependencies
// (the codetrip top-level package imports pipeline, while lang providers import codetrip).
// The lang package uses type aliases to refer to these types.

// LangQuerySet contains all tree-sitter S-expression queries for a language provider.
// Each query targets a specific extraction dimension. The parse phase executes each
// query against the same AST tree and passes the captures to the corresponding
// Interpret method for synthesis.
type LangQuerySet struct {
	// Scope query: builds the nested scope tree (module/class/function/block/loop/closure)
	Scope string
	// Declaration query: extracts symbol declarations (functions, methods, classes, vars, consts)
	Declaration string
	// Import query: extracts import statements
	Import string
	// TypeBinding query: extracts type annotations, return types, receiver types, aliases
	TypeBinding string
	// Reference query: extracts classified references (calls, field access, constructors, macros)
	Reference string
}

// LangCapturesConfig defines tree-sitter capture rules for a language provider.
type LangCapturesConfig struct {
	// Tree-sitter query string
	Query string
	// Capture name -> Label mapping
	CaptureMap map[string]graph.Label
}

// LangCallExtractConfig defines call extraction configuration for tree-sitter parsing.
type LangCallExtractConfig struct {
	Query       string
	CaptureMap  map[string]graph.Label
	ReceiverKey string // receiver capture name
	MethodKey   string // method name capture name
	ArgsKey     string // arguments capture name
}

// LangClassExtractConfig defines class extraction configuration.
type LangClassExtractConfig struct {
	Query      string
	CaptureMap map[string]graph.Label
	NameKey    string
	BaseKey    string // base class capture name
}

// LangFieldExtractConfig defines field extraction configuration.
type LangFieldExtractConfig struct {
	Query      string
	CaptureMap map[string]graph.Label
	NameKey    string
	TypeKey    string
}

// LangImportResolveConfig defines import resolution configuration.
type LangImportResolveConfig struct {
	Query       string
	CaptureMap  map[string]graph.Label
	PathKey     string // import path capture name
	AliasKey    string // alias capture name
	ItemsKey    string // imported items capture name
	IsDotImport bool   // whether dot import is supported
	IsWildcard  bool   // whether wildcard import
}

// LangCaptureResult contains tree-sitter capture results for a file.
type LangCaptureResult struct {
	Filepath string
	Captures []LangCapture
}

// LangCapture represents a single tree-sitter capture.
type LangCapture struct {
	MatchIndex int    // Index of the query match this capture belongs to
	NodeType   string
	Name       string
	Text       string
	StartRow   int
	StartCol   int
	EndRow     int
	EndCol     int
	Children   []LangCapture
}

// LangInterpretResult contains the interpreted symbols from a captured file.
type LangInterpretResult struct {
	Symbols   []SymbolInfo
	Imports   []LangImportInfo
	CallSites []LangCallSiteInfo
	Classes   []LangClassInfo
	Fields    []LangFieldInfo
}

// LangImportInfo contains detailed import information from tree-sitter parsing.
// This is more detailed than the simplified pipeline.ImportInfo.
type LangImportInfo struct {
	Path      string   // import path "fmt", "github.com/..."
	Alias     string   // alias
	Items     []string // explicitly imported items
	IsDot     bool     // dot import
	FilePath  string   // source file
	StartLine int
}

// LangCallSiteInfo contains detailed call site information from tree-sitter parsing.
// This is more detailed than the simplified pipeline.CallSite.
type LangCallSiteInfo struct {
	Receiver    string // receiver expression
	MethodName  string // method name
	Args        []string
	FilePath    string
	StartLine   int
	EndLine     int
	EnclosingFn string // enclosing function
}

// LangClassInfo contains detailed class information from tree-sitter parsing.
// This is more detailed than the simplified pipeline.ClassInfo.
type LangClassInfo struct {
	Name      string
	BaseTypes []string
	Methods   []string
	FilePath  string
	StartLine int
	EndLine   int
}

// LangFieldInfo contains detailed field information from tree-sitter parsing.
// This is more detailed than the simplified pipeline.FieldInfo.
type LangFieldInfo struct {
	Name       string
	TypeName   string
	FilePath   string
	StartLine  int
	IsExported bool
}