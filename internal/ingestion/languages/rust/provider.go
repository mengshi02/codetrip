// Package rust implements the Rust language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for Rust.
// This file wires all Rust-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/rust.ts (rustProvider factory).
package rust

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustProvider returns the Rust LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func RustProvider() core.LanguageProvider {
	return &rustProviderImpl{}
}

// rustProviderImpl implements core.LanguageProvider for Rust.
type rustProviderImpl struct{}

func (p *rustProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageRust }

func (p *rustProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to emitRustScopeCaptures + interpretRustTypeBinding
	return nil, nil
}

func (p *rustProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to emitRustScopeCaptures + interpretRustImport
	return nil, nil
}

func (p *rustProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to resolveRustImportTarget
	return "", core.ImportKindDirect
}

func (p *rustProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return RustMergeBindings(existing, incoming, scopeID)
}

func (p *rustProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *rustProviderImpl) IsTestFile(filePath string) bool {
	// Rust test convention: files in tests/ directory or *_test.rs pattern
	return strings.Contains(filePath, "/tests/") || strings.HasSuffix(filePath, "_test.rs")
}

func (p *rustProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// Rust frameworks: Actix, Axum, Rocket patterns
	// TODO: implement framework detection hints
	return nil
}