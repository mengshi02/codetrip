// Package csharp implements the C# language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for C#.
// This file wires all C#-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/csharp.ts (csharpProvider factory).
package csharp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CSharpProvider returns the C# LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func CSharpProvider() core.LanguageProvider {
	return &csharpProviderImpl{}
}

// csharpProviderImpl implements core.LanguageProvider for C#.
type csharpProviderImpl struct{}

func (p *csharpProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageCSharp }

func (p *csharpProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to EmitCsharpScopeCaptures + InterpretCsharpTypeBinding
	return nil, nil
}

func (p *csharpProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to EmitCsharpScopeCaptures + InterpretCsharpImport
	return nil, nil
}

func (p *csharpProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to ResolveCsharpImportTarget
	return "", core.ImportKindDirect
}

func (p *csharpProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return CSharpMergeBindings(existing, incoming, scopeID)
}

func (p *csharpProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *csharpProviderImpl) IsTestFile(filePath string) bool {
	// C# test files: ending with "Test.cs" or containing "/Tests/" or "/Test/"
	normalized := strings.ReplaceAll(filePath, `\`, "/")
	return strings.HasSuffix(normalized, "Test.cs") ||
		strings.Contains(normalized, "/Tests/") ||
		strings.Contains(normalized, "/Test/")
}

func (p *csharpProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// C# frameworks: .NET, NUnit, xUnit, MSTest patterns
	return nil
}