// Package java implements the Java language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for Java.
// This file wires all Java-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/java.ts (javaProvider factory).
package java

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JavaProvider returns the Java LanguageProvider instance.
// TODO: full implementation — currently returns a skeleton.
func JavaProvider() core.LanguageProvider {
	return &javaProviderImpl{}
}

// javaProviderImpl implements core.LanguageProvider for Java.
type javaProviderImpl struct{}

func (p *javaProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageJava }

func (p *javaProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	// TODO: wire to EmitJavaScopeCaptures + InterpretJavaTypeBinding
	return nil, nil
}

func (p *javaProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	// TODO: wire to EmitJavaScopeCaptures + InterpretJavaImport
	return nil, nil
}

func (p *javaProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	// TODO: wire to ResolveJavaImportTarget
	return "", core.ImportKindDirect
}

func (p *javaProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return JavaMergeBindings(existing, incoming, scopeID)
}

func (p *javaProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	// TODO: wire to scope extraction
	return nil, nil
}

func (p *javaProviderImpl) IsTestFile(filePath string) bool {
	// Java test files: Maven convention src/test/java
	return strings.Contains(filePath, "/test/") || strings.Contains(filePath, "/Test/")
}

func (p *javaProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// Java frameworks: Spring, JUnit, Mockito patterns
	return nil
}