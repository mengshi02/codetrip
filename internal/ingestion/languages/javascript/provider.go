// Package javascript implements the JavaScript language provider for the codetrip ingestion pipeline.
// JavaScript shares the TypeScript tree-sitter grammar and many hooks (arity, merge-bindings,
// interpret, simple-hooks) — JS-specific differences are documented per-module.
//
// Ported from TS languages/javascript/index.ts.
package javascript

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

func JavaScriptProvider() core.LanguageProvider { return &jsProviderImpl{} }
type jsProviderImpl struct{}

func (p *jsProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageJavaScript }
func (p *jsProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	return nil, nil // TODO: full implementation
}
func (p *jsProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	return nil, nil // TODO: full implementation
}
func (p *jsProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	return "", core.ImportKindDirect // TODO: full implementation
}
func (p *jsProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return JsMergeBindings(append(existing, incoming...))
}
func (p *jsProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	return nil, nil // TODO: full implementation
}
func (p *jsProviderImpl) IsTestFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".test.js") || strings.HasSuffix(filePath, ".spec.js") || strings.HasSuffix(filePath, ".test.jsx") || strings.HasSuffix(filePath, ".spec.jsx")
}
func (p *jsProviderImpl) FrameworkHints() []core.FrameworkHintExt { return nil }