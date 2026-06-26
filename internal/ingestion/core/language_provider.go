// Language Provider — defines the contract for per-language parsing/analysis plugins.
//
// Mirrors TS language-provider.ts, skeleton for codetrip.
// Each supported language (TypeScript, Python, Go, Rust, Java, etc.)
// implements the LanguageProvider interface which provides:
//   - Tree-sitter queries for symbol extraction
//   - Import resolution rules
//   - Scope extraction callbacks
//   - Binding merge rules
// Deferred to Phase 6 when language-specific providers are implemented.

package core

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// LanguageProvider is the contract that each language plugin must implement.
type LanguageProvider interface {
	// Language returns the SupportedLanguage this provider handles.
	Language() shared.SupportedLanguage

	// ExtractSymbols runs tree-sitter queries on source and returns SymbolDefinitions.
	ExtractSymbols(source []byte, filePath string, config *LanguageConfig) ([]shared.SymbolDefinition, error)

	// ExtractImports finds import statements and returns ImportEdges.
	ExtractImports(source []byte, filePath string, config *LanguageConfig) ([]ImportEdge, error)

	// ResolveImportTarget resolves a raw import path to a target file.
	ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, ImportKind)

	// MergeBindings combines binding sets from different scopes.
	MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef

	// ExtractScopes extracts scope boundaries (function, class, module) from parsed source.
	ExtractScopes(source []byte, filePath string) ([]ScopeInfo, error)

	// IsTestFile returns true if the file path looks like a test file for this language.
	IsTestFile(filePath string) bool

	// FrameworkHints returns framework detection patterns for this language.
	FrameworkHints() []FrameworkHintExt
}

// ScopeInfo describes a scope boundary found in source code.
type ScopeInfo struct {
	ScopeID   string
	Name      string
	Kind      string // "function", "class", "module", "block", "namespace"
	ParentID  string // empty for top-level scopes
	StartRow  int
	EndRow    int
	IsExported bool
}

// LanguageProviderRegistry holds all registered language providers.
type LanguageProviderRegistry struct {
	providers map[shared.SupportedLanguage]LanguageProvider
}

// NewLanguageProviderRegistry creates an empty registry.
func NewLanguageProviderRegistry() *LanguageProviderRegistry {
	return &LanguageProviderRegistry{
		providers: map[shared.SupportedLanguage]LanguageProvider{},
	}
}

// Register adds a language provider to the registry.
func (r *LanguageProviderRegistry) Register(p LanguageProvider) {
	r.providers[p.Language()] = p
}

// Get retrieves a provider for the given language.
func (r *LanguageProviderRegistry) Get(lang shared.SupportedLanguage) (LanguageProvider, bool) {
	p, ok := r.providers[lang]
	return p, ok
}

// All returns all registered providers.
func (r *LanguageProviderRegistry) All() []LanguageProvider {
	var result []LanguageProvider
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}