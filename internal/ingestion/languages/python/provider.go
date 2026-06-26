// Package python implements the Python language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for Python.
// This file wires all Python-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/python.ts (pythonProvider factory).
package python

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PythonProvider returns the Python LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func PythonProvider() core.LanguageProvider {
	return &pythonProviderImpl{}
}

// pythonProviderImpl implements core.LanguageProvider for Python.
type pythonProviderImpl struct{}

func (p *pythonProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguagePython }

func (p *pythonProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to emitPythonScopeCaptures + interpretPythonTypeBinding
	return nil, nil
}

func (p *pythonProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to emitPythonScopeCaptures + interpretPythonImport
	return nil, nil
}

func (p *pythonProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to resolvePythonImportTarget
	return "", core.ImportKindDirect
}

func (p *pythonProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	// Python LEGB: local > import/namespace/reexport > wildcard
	return PythonMergeBindings(append(existing, incoming...))
}

func (p *pythonProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *pythonProviderImpl) IsTestFile(filePath string) bool {
	// Python test files: test_*.py or *_test.py (unittest/pytest convention)
	base := strings.ToLower(filePath)
	return strings.Contains(base, "/test_") || strings.HasSuffix(base, "_test.py")
}

func (p *pythonProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// Python frameworks: Django, Flask, FastAPI patterns
	// TODO: implement framework detection hints
	return nil
}