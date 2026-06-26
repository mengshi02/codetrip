// Package cpp implements the C++ language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for C++.
// This file wires all C++-specific extraction hooks into the core.LanguageProvider interface.
//
// C++ specifics:
//   - #include and using declarations as imports
//   - Overload resolution via arity + constraint compatibility
//   - ADL (argument-dependent lookup) for free function resolution
//   - Two-phase name lookup for template instantiation
//
// Ported from TS languages/cpp.ts (cppProvider factory).
package cpp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CppProvider returns the C++ LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func CppProvider() core.LanguageProvider {
	return &cppProviderImpl{}
}

// cppProviderImpl implements core.LanguageProvider for C++.
type cppProviderImpl struct{}

func (p *cppProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageCpp }

func (p *cppProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to emitCppScopeCaptures + interpretCppTypeBinding
	return nil, nil
}

func (p *cppProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to emitCppScopeCaptures + interpretCppImport
	return nil, nil
}

func (p *cppProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to resolveCppImportTarget
	return "", core.ImportKindDirect
}

func (p *cppProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return CppMergeBindings(existing, incoming, scopeID)
}

func (p *cppProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *cppProviderImpl) IsTestFile(filePath string) bool {
	// C++ test conventions: files in test/ directory or matching *test*.cpp/*test*.cc
	lower := strings.ToLower(filePath)
	return strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "\\test\\") ||
		strings.Contains(lower, "test_") ||
		strings.Contains(lower, "_test.") ||
		strings.Contains(lower, "tests/") ||
		strings.Contains(lower, "\\tests\\")
}

func (p *cppProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// C++ frameworks: Google Test, Catch2, Boost.Test patterns
	// TODO: implement framework detection hints
	return nil
}