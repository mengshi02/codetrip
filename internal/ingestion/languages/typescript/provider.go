// Package typescript implements the TypeScript language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for TypeScript.
// This file wires all TypeScript-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/typescript/index.ts (typescriptProvider factory).
package typescript

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TypeScriptProvider returns the TypeScript LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func TypeScriptProvider() core.LanguageProvider {
	return &tsProviderImpl{}
}

// tsProviderImpl implements core.LanguageProvider for TypeScript.
type tsProviderImpl struct{}

func (p *tsProviderImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageTypeScript
}

func (p *tsProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to emitTsScopeCaptures + interpretTsTypeBinding
	return nil, nil
}

func (p *tsProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to emitTsScopeCaptures + interpretTsImport
	return nil, nil
}

func (p *tsProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to resolveTsImportTarget
	return "", core.ImportKindDirect
}

func (p *tsProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return TypeScriptMergeBindings(append(existing, incoming...))
}

func (p *tsProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *tsProviderImpl) IsTestFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".test.ts") ||
		strings.HasSuffix(filePath, ".spec.ts") ||
		strings.HasSuffix(filePath, ".test.tsx") ||
		strings.HasSuffix(filePath, ".spec.tsx")
}

func (p *tsProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// TypeScript frameworks: Next.js, Express, NestJS, etc.
	// Path-based detection is handled by core.DetectFrameworkFromPath.
	return nil
}