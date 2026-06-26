package core

import "github.com/odvcencio/gotreesitter"

// VariableVisibility reuses FieldVisibility — variable and field modifiers share the same set.
type VariableVisibility = FieldVisibility

// VariableScope represents the scoping level of a variable declaration.
//   - module: top-level module/namespace scope (e.g. Python module variable)
//   - file: file-level scope (e.g. Go file-level var)
//   - block: local block/function scope (e.g. let/const inside a function)
type VariableScope string

const (
	ScopeModule VariableScope = "module"
	ScopeFile   VariableScope = "file"
	ScopeBlock  VariableScope = "block"
)

// VariableInfo represents a variable declaration extracted from source code.
type VariableInfo struct {
	Name       string
	Type       *string            // null = nil (untyped)
	Visibility VariableVisibility
	IsConst    bool               // const/val/final
	IsStatic   bool               // static field in class
	IsMutable  bool               // mutable (var, let) vs immutable (const, val)
	Scope      VariableScope
	SourceFile string
	Line       int
}

// VariableExtractorContext holds the AST node, name, and file path for a variable extraction.
type VariableExtractorContext struct {
	DefinitionNode *gotreesitter.Node // nil = null
	NameNode       *gotreesitter.Node // nil = undefined
	FilePath       string             // source file path
	Language       SupportedLanguage  // language ID
}

// VariableExtractor extracts variable declarations from AST nodes.
type VariableExtractor interface {
	Language() SupportedLanguage
	Extract(node *gotreesitter.Node, ctx *VariableExtractorContext, source []byte, lang *gotreesitter.Language) []VariableInfo
	IsVariableDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool
}

// VariableExtractionConfig defines per-language variable extraction configuration.
// Each field mirrors a TS property; nil func fields mean "not set".
type VariableExtractionConfig struct {
	Language          SupportedLanguage
	ConstNodeTypes    []string // AST node types for const declarations (e.g., 'const_item', 'lexical_declaration')
	StaticNodeTypes   []string // AST node types for static declarations (e.g., 'static_item')
	VariableNodeTypes []string // AST node types for variable declarations (e.g., 'var_declaration')

	ExtractName             func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string  // optional
	ExtractNames            func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string // optional — multi-name declarations
	ExtractType             func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string            // optional
	ExtractTypeForName      func(node *gotreesitter.Node, name string, source []byte, lang *gotreesitter.Language) *string // optional — per-name type in multi-name decl
	ExtractVisibility       func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) VariableVisibility // optional
	ExtractVisibilityForName func(node *gotreesitter.Node, name string, source []byte, lang *gotreesitter.Language) VariableVisibility // optional
	IsConst                 func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool // optional
	IsStatic                func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool // optional
	IsMutable               func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool // optional
	ExtractScope            func(node *gotreesitter.Node, lang *gotreesitter.Language) VariableScope // optional
	ExtractVariableFromNode func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *VariableInfo // optional
}