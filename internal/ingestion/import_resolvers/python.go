// Package importresolvers — Python import resolution helpers.
//
// Resolves Python imports using PEP 328 relative import rules,
// proximity-based bare import resolution, and ancestor directory walking.
// Mirrors TS import-resolvers/python.ts.
package importresolvers

import (
	"path/filepath"
	"strings"
)

// ResolvePythonImportInternal resolves a Python import path to a file path.
//
// Resolution strategies (in order):
//  1. PEP 328 relative imports (leading dots)
//  2. Proximity bare imports (single segment, look in same directory)
//  3. Ancestor directory walk (multi-segment absolute imports)
func ResolvePythonImportInternal(currentFile string, importPath string, allFiles map[string]bool) string {
	// 1. PEP 328 relative imports
	if strings.HasPrefix(importPath, ".") {
		return resolvePythonRelative(currentFile, importPath, allFiles)
	}

	// 2. Single-segment bare imports — proximity resolution
	segments := strings.Split(importPath, ".")
	if len(segments) == 1 {
		return resolvePythonBareImport(currentFile, importPath, allFiles)
	}

	// 3. Multi-segment absolute imports — ancestor walk
	return resolvePythonAbsolute(currentFile, importPath, allFiles)
}

// resolvePythonRelative resolves PEP 328 relative imports.
//
// Dot counting:
//   - "." → same package (current directory)
//   - ".." → parent package
//   - "..." → grandparent package
//   - ".module" → module in same package
//   - "..module" → module in parent package
func resolvePythonRelative(currentFile string, importPath string, allFiles map[string]bool) string {
	// Count leading dots
	dotCount := 0
	for _, ch := range importPath {
		if ch == '.' {
			dotCount++
		} else {
			break
		}
	}

	// The part after the dots is the module path
	modulePart := importPath[dotCount:]

	// Start from the current file's directory
	dir := filepath.Dir(currentFile)

	// Go up (dotCount - 1) levels:
	// 1 dot = same directory (no going up)
	// 2 dots = parent directory
	// 3 dots = grandparent directory
	levelsUp := dotCount - 1
	for i := 0; i < levelsUp; i++ {
		dir = filepath.Dir(dir)
	}

	if modulePart == "" {
		// Import the package itself: look for __init__.py
		pkgInit := filepath.ToSlash(filepath.Join(dir, "__init__.py"))
		if allFiles[pkgInit] {
			return pkgInit
		}
		return ""
	}

	// Convert dots in module part to directory separators
	modulePath := strings.ReplaceAll(modulePart, ".", "/")
	return tryPythonModulePath(dir, modulePath, allFiles)
}

// resolvePythonBareImport resolves a single-segment bare import using
// proximity-based resolution: look in the same directory as the current file.
//
// Resolution order:
//  1. __init__.py in a subdirectory matching the import name
//  2. <name>.py file in the same directory
func resolvePythonBareImport(currentFile string, importName string, allFiles map[string]bool) string {
	dir := filepath.Dir(currentFile)

	// Try as package: <dir>/<importName>/__init__.py
	pkgInit := filepath.ToSlash(filepath.Join(dir, importName, "__init__.py"))
	if allFiles[pkgInit] {
		return pkgInit
	}

	// Try as module: <dir>/<importName>.py
	moduleFile := filepath.ToSlash(filepath.Join(dir, importName+".py"))
	if allFiles[moduleFile] {
		return moduleFile
	}

	return ""
}

// resolvePythonAbsolute resolves multi-segment absolute imports by
// walking up ancestor directories looking for the package/module.
//
// e.g. "accounts.models" → look for "accounts/models.py" or
// "accounts/models/__init__.py" starting from the current directory
// and walking up.
func resolvePythonAbsolute(currentFile string, importPath string, allFiles map[string]bool) string {
	modulePath := strings.ReplaceAll(importPath, ".", "/")
	segments := strings.Split(importPath, ".")

	dir := filepath.Dir(currentFile)
	for {
		// Try as module file
		moduleFile := filepath.ToSlash(filepath.Join(dir, modulePath+".py"))
		if allFiles[moduleFile] {
			return moduleFile
		}

		// Try as package __init__.py
		pkgInit := filepath.ToSlash(filepath.Join(dir, modulePath, "__init__.py"))
		if allFiles[pkgInit] {
			return pkgInit
		}

		// Try last segment as a file in the package directory
		if len(segments) > 1 {
			parentPath := strings.Join(segments[:len(segments)-1], "/")
			lastSegment := segments[len(segments)-1]
			// <parent>/<last>.py
			lastFile := filepath.ToSlash(filepath.Join(dir, parentPath, lastSegment+".py"))
			if allFiles[lastFile] {
				return lastFile
			}
			// <parent>/__init__.py (the package itself)
			parentInit := filepath.ToSlash(filepath.Join(dir, parentPath, "__init__.py"))
			if allFiles[parentInit] {
				return parentInit
			}
		}

		// Walk up
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break // reached root
		}
		dir = parentDir
	}

	return ""
}

// tryPythonModulePath tries to resolve a Python module path to a file.
// Tries both <path>.py and <path>/__init__.py.
func tryPythonModulePath(baseDir string, modulePath string, allFiles map[string]bool) string {
	// Try as module file: <baseDir>/<modulePath>.py
	moduleFile := filepath.ToSlash(filepath.Join(baseDir, modulePath+".py"))
	if allFiles[moduleFile] {
		return moduleFile
	}

	// Try as package: <baseDir>/<modulePath>/__init__.py
	pkgInit := filepath.ToSlash(filepath.Join(baseDir, modulePath, "__init__.py"))
	if allFiles[pkgInit] {
		return pkgInit
	}

	// Try last segment as file in parent package
	parts := strings.Split(modulePath, "/")
	if len(parts) > 1 {
		lastSegment := parts[len(parts)-1]
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		// <baseDir>/<parentPath>/<lastSegment>.py
		lastFile := filepath.ToSlash(filepath.Join(baseDir, parentPath, lastSegment+".py"))
		if allFiles[lastFile] {
			return lastFile
		}
	}

	return ""
}

// PythonImportStrategy is the Python-specific import resolution strategy.
// Returns null to continue chain for non-relative imports.
// Absorbs unresolved relative imports (returns empty result to stop the chain).
func PythonImportStrategy(rawImportPath string, filePath string, ctx *ResolveCtx) *ImportResult {
	resolved := ResolvePythonImportInternal(filePath, rawImportPath, ctx.AllFilePaths)
	if resolved != "" {
		return &ImportResult{Kind: "files", Files: []string{resolved}}
	}

	// PEP 328: unresolved relative imports should not fall through
	if strings.HasPrefix(rawImportPath, ".") {
		return &ImportResult{Kind: "files", Files: []string{}}
	}

	// External dotted imports: gate suffix fallback
	// Only allow suffix fallback when the leading segment appears in-repo
	pathLike := strings.ReplaceAll(rawImportPath, ".", "/")
	if strings.Contains(pathLike, "/") {
		leadingSegment := strings.Split(pathLike, "/")[0]
		if leadingSegment != "" {
			hasRepoCandidate := false
			// Check if leading segment has a .py file or __init__.py or files in dir
			if ctx.Index != nil {
				if _, ok := ctx.Index.Get(leadingSegment+".py"); ok {
					hasRepoCandidate = true
				}
				if !hasRepoCandidate {
					if _, ok := ctx.Index.Get(leadingSegment+"/__init__.py"); ok {
						hasRepoCandidate = true
					}
				}
				if !hasRepoCandidate {
					if len(ctx.Index.GetFilesInDir(leadingSegment, ".py")) > 0 {
						hasRepoCandidate = true
					}
				}
			}
			if !hasRepoCandidate {
				return &ImportResult{Kind: "files", Files: []string{}}
			}
		}
	}

	return nil // let next strategy try
}