// Import Target Adapter — resolves raw import targets to concrete file paths.
//
// Mirrors TS import-target-adapter.ts, simplified for codetrip:
//   - Per-language resolution logic deferred to LanguageProviders (Phase 6)
//   - Common resolution patterns (relative path, index file, re-export) are stubs
//   - ImportKind constants for classifying import resolution results
//
// This adapter is the bridge between raw import strings and the graph.
// When finalize_orchestrator resolves an import, it calls the adapter
// to determine the actual target file path.

package core

import (
	"path/filepath"
	"strings"
)

// ─── Import Kind ────────────────────────────────────────────

// ImportKind classifies how an import was resolved.
// Mirrors the importKind union in TS import-target-adapter.ts.
type ImportKind string

const (
	// ImportKindIndexFile indicates the target is an index/barrel file (e.g., index.ts).
	ImportKindIndexFile ImportKind = "indexFile"
	// ImportKindReexport indicates the target re-exports symbols from another module.
	ImportKindReexport ImportKind = "reexport"
	// ImportKindDirect indicates a direct import to a specific file.
	ImportKindDirect ImportKind = "direct"
	// ImportKindUnresolved indicates the import could not be resolved.
	ImportKindUnresolved ImportKind = "unresolved"
	// ImportKindPackage indicates the import is a package-level reference.
	ImportKindPackage ImportKind = "package"
	// ImportKindSideEffect indicates the import is for side effects only (no bindings).
	ImportKindSideEffect ImportKind = "sideEffect"
)

// ─── Import Target Adapter interface ──────────────────────────

// ImportTargetAdapter resolves raw import paths into concrete target file paths.
// This is the bridge between the finalize algorithm and per-language
// import resolution logic.
//
// Mirrors TS ImportTargetAdapter, simplified:
//   - resolveImportTarget: returns (targetFile, importKind) or ("", ImportKindUnresolved)
//   - Per-language logic deferred to LanguageProviders (Phase 6)
type ImportTargetAdapter interface {
	// ResolveImportTarget resolves a raw import path from sourceFile into
	// a concrete target file path and import kind classification.
	//
	// Parameters:
	//   - sourceFile: the file containing the import
	//   - importPath: the raw import string (e.g., "./utils", "react", "@org/pkg")
	//   - workspaceIndex: list of all file paths in the workspace
	//
	// Returns: (targetFilePath, importKind) or ("", ImportKindUnresolved) if
	// the import cannot be resolved.
	ResolveImportTarget(sourceFile string, importPath string, workspaceIndex []string) (string, ImportKind)
}

// ─── Common Adapter (baseline) ──────────────────────────────

// CommonImportTargetAdapter provides baseline resolution for relative imports
// using common path resolution rules. Per-language specifics are deferred
// to LanguageProviders (Phase 6).
type CommonImportTargetAdapter struct {
	// LanguageConfigs holds per-language configurations for path resolution.
	configs []LanguageConfig
}

// NewCommonImportTargetAdapter creates a baseline adapter with language configs.
func NewCommonImportTargetAdapter(configs []LanguageConfig) *CommonImportTargetAdapter {
	return &CommonImportTargetAdapter{configs: configs}
}

// ResolveImportTarget resolves imports using common rules:
//   1. Relative imports (starting with . or ..) → resolve against source file directory
//   2. Package imports → check workspaceIndex for matching paths
//   3. Everything else → ImportKindUnresolved
func (a *CommonImportTargetAdapter) ResolveImportTarget(sourceFile string, importPath string, workspaceIndex []string) (string, ImportKind) {
	// Relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		// Resolve relative path against source file directory
		dir := filepath.Dir(sourceFile)
		resolved := filepath.Join(dir, importPath)
		resolved = filepath.Clean(resolved)

		// Normalize
		resolved = strings.ReplaceAll(resolved, "\\", "/")

		// Try with common extensions
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".py", ".go", ".java", ".rs", ".cs", ".c", ".cpp"} {
			candidate := resolved + ext
			for _, f := range workspaceIndex {
				if f == candidate {
					return candidate, ImportKindDirect
				}
			}
		}

		// Try as directory (index file)
		for _, indexFile := range []string{"index.ts", "index.tsx", "index.js", "index.jsx", "__init__.py", "mod.rs"} {
			candidate := resolved + "/" + indexFile
			for _, f := range workspaceIndex {
				if f == candidate {
					return candidate, ImportKindIndexFile
				}
			}
		}

		return "", ImportKindUnresolved
	}

	// Package imports — check workspace for partial path matches
	// TODO(Phase 6): Add per-language resolution (Go module, Java package, C# namespace)
	for _, f := range workspaceIndex {
		if strings.Contains(f, importPath) {
			return f, ImportKindPackage
		}
	}

	return "", ImportKindUnresolved
}