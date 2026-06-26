// Package importresolvers — Rust import resolution helpers.
//
// Resolves Rust module paths using crate::/super::/self:: conventions.
// Mirrors TS import-resolvers/rust.ts.
package importresolvers

import (
	"path/filepath"
	"strings"
)

// ResolveRustImportInternal resolves a Rust import path to a file path.
//
// Handles:
//   - crate:: → src/ root
//   - super:: → parent directory
//   - self:: → current directory
//   - bare identifier → suffix resolution
func ResolveRustImportInternal(currentFile string, importPath string, allFiles map[string]bool) string {
	// crate:: → resolve from src/ root
	if strings.HasPrefix(importPath, "crate::") {
		rest := strings.TrimPrefix(importPath, "crate::")
		modulePath := strings.ReplaceAll(rest, "::", "/")
		return tryRustModulePath("src/"+modulePath, allFiles)
	}

	// super:: → resolve from parent directory
	if strings.HasPrefix(importPath, "super::") {
		dir := filepath.Dir(currentFile)
		rest := importPath
		for strings.HasPrefix(rest, "super::") {
			rest = strings.TrimPrefix(rest, "super::")
			dir = filepath.Dir(dir)
		}
		modulePath := strings.ReplaceAll(rest, "::", "/")
		resolved := filepath.ToSlash(filepath.Join(dir, modulePath))
		return tryRustModulePath(resolved, allFiles)
	}

	// self:: → resolve from current directory
	if strings.HasPrefix(importPath, "self::") {
		dir := filepath.Dir(currentFile)
		rest := strings.TrimPrefix(importPath, "self::")
		modulePath := strings.ReplaceAll(rest, "::", "/")
		resolved := filepath.ToSlash(filepath.Join(dir, modulePath))
		return tryRustModulePath(resolved, allFiles)
	}

	// Bare identifier (e.g. "models" from `use models::User`)
	// Try as a module path relative to current directory
	dir := filepath.Dir(currentFile)
	modulePath := strings.ReplaceAll(importPath, "::", "/")
	resolved := filepath.ToSlash(filepath.Join(dir, modulePath))
	if result := tryRustModulePath(resolved, allFiles); result != "" {
		return result
	}

	// Also try from src/
	return tryRustModulePath("src/"+modulePath, allFiles)
}

// tryRustModulePath tries to resolve a Rust module path to an actual file.
//
// Resolution order for a module path like "src/models/user":
//  1. "src/models/user.rs"
//  2. "src/models/user/mod.rs"
//  3. "src/models/user/lib.rs"
//  4. Strip last segment: try "src/models.rs" (the parent module file)
func tryRustModulePath(modulePath string, allFiles map[string]bool) string {
	// Try direct .rs file
	rsPath := modulePath + ".rs"
	if allFiles[rsPath] {
		return rsPath
	}

	// Try mod.rs in directory
	modPath := modulePath + "/mod.rs"
	if allFiles[modPath] {
		return modPath
	}

	// Try lib.rs in directory
	libPath := modulePath + "/lib.rs"
	if allFiles[libPath] {
		return libPath
	}

	// Strip last segment: parent module might be a file
	// e.g. "src/models/user" → "src/models.rs"
	lastSlash := strings.LastIndex(modulePath, "/")
	if lastSlash >= 0 {
		parentPath := modulePath[:lastSlash] + ".rs"
		if allFiles[parentPath] {
			return parentPath
		}
	}

	return ""
}

// RustModuleStrategy is the Rust-specific import resolution strategy.
// Handles grouped imports like `{crate::a, crate::b}` and
// `crate::models::{User, Repo}`.
func RustModuleStrategy(rawImportPath string, filePath string, ctx *ResolveCtx) *ImportResult {
	// Top-level grouped: {crate::a, crate::b}
	if strings.HasPrefix(rawImportPath, "{") && strings.HasSuffix(rawImportPath, "}") {
		inner := rawImportPath[1 : len(rawImportPath)-1]
		parts := splitGroupedImports(inner)
		var resolved []string
		for _, part := range parts {
			if r := ResolveRustImportInternal(filePath, part, ctx.AllFilePaths); r != "" {
				resolved = append(resolved, r)
			}
		}
		if len(resolved) > 0 {
			return &ImportResult{Kind: "files", Files: resolved}
		}
		return nil
	}

	// Scoped grouped: crate::models::{User, Repo}
	braceIdx := strings.Index(rawImportPath, "::{")
	if braceIdx >= 0 && strings.HasSuffix(rawImportPath, "}") {
		pathPrefix := rawImportPath[:braceIdx]
		braceContent := rawImportPath[braceIdx+3 : len(rawImportPath)-1]
		items := splitGroupedImports(braceContent)
		var resolved []string
		for _, item := range items {
			// Handle `use crate::models::{User, Repo as R}` — strip alias
			itemName := stripRustAlias(item)
			fullPath := pathPrefix + "::" + itemName
			if r := ResolveRustImportInternal(filePath, fullPath, ctx.AllFilePaths); r != "" {
				resolved = append(resolved, r)
			}
		}
		if len(resolved) > 0 {
			return &ImportResult{Kind: "files", Files: resolved}
		}
		// Fallback: resolve the prefix path itself
		if prefixResult := ResolveRustImportInternal(filePath, pathPrefix, ctx.AllFilePaths); prefixResult != "" {
			return &ImportResult{Kind: "files", Files: []string{prefixResult}}
		}
	}

	return nil
}

// splitGroupedImports splits a comma-separated list of import items,
// trimming whitespace and filtering empty strings.
func splitGroupedImports(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// stripRustAlias strips the `as` alias from a Rust import item.
// e.g. "Repo as R" → "Repo"
func stripRustAlias(item string) string {
	if idx := strings.Index(item, " as "); idx >= 0 {
		return strings.TrimSpace(item[:idx])
	}
	return strings.TrimSpace(item)
}