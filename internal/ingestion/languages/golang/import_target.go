// Package golang — Go import target resolution.
// Resolves a Go import path (e.g. "github.com/user/repo/pkg") to a
// repo-relative file path, using go.mod module path and workspace file set.
// Ported from TS languages/go/import-target.ts.
package golang

import (
	"path/filepath"
	"strings"
)

// GoModuleConfig holds the parsed go.mod information needed for import resolution.
type GoModuleConfig struct {
	ModulePath string   // e.g. "github.com/user/repo"
	Dir        string   // module root directory (relative to workspace)
	Packages   []string // known subdirectory packages
}

// ResolveGoImportTarget resolves a Go import path to one or more
// repo-relative file paths. Returns nil for unresolvable/external modules,
// a single entry for resolved, or multiple entries for ambiguous targets.
//
// Strategy:
//  1. Stdlib packages (no dots in first segment) → nil (external, unresolvable)
//  2. Strip module prefix from import path
//  3. Match remaining path against workspace files
//  4. For directory packages, look for directory/*.go files
//
// Mirrors TS resolveGoImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig).
func ResolveGoImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	if targetRaw == "" {
		return nil
	}

	// Stdlib packages have no dots in the first path segment
	// e.g. "fmt", "strings", "encoding/json"
	if isGoStdlibPackage(targetRaw) {
		return nil
	}

	// Extract module prefix from resolution config
	var modulePrefix string
	if cfg, ok := resolutionConfig.(*GoModuleConfig); ok && cfg != nil {
		modulePrefix = cfg.ModulePath
	}

	// Strip module prefix from import path to get the relative path
	relPath := targetRaw
	if modulePrefix != "" {
		if strings.HasPrefix(targetRaw, modulePrefix) {
			relPath = strings.TrimPrefix(targetRaw, modulePrefix)
			relPath = strings.TrimPrefix(relPath, "/")
		}
	}

	if relPath == "" {
		// Import of the module root itself
		relPath = "."
	}

	normalizedRel := strings.ReplaceAll(relPath, "\\", "/")

	// Strategy 1: exact file match (for .go files in the import path)
	var results []string

	// For Go packages, we need to find .go files in the directory
	// e.g. import "github.com/user/repo/pkg" → look for pkg/*.go
	for fp := range allFilePaths {
		normalized := strings.ReplaceAll(fp, "\\", "/")

		// Direct directory match: the file is in the target package directory
		dir := filepath.Dir(normalized)
		if dir == normalizedRel {
			results = append(results, fp)
		}

		// Also check if the import path matches as a suffix
		if strings.HasSuffix(normalized, "/"+normalizedRel) {
			dirOfDir := filepath.Dir(normalized)
			if strings.HasSuffix(dirOfDir, normalizedRel) || dirOfDir == normalizedRel {
				// Avoid double-counting
				found := false
				for _, r := range results {
					if r == fp {
						found = true
						break
					}
				}
				if !found {
					results = append(results, fp)
				}
			}
		}
	}

	if len(results) > 0 {
		return results
	}

	// Strategy 2: suffix match against all file paths
	suffix := "/" + normalizedRel
	for fp := range allFilePaths {
		normalized := strings.ReplaceAll(fp, "\\", "/")
		if strings.HasSuffix(normalized, suffix) {
			results = append(results, fp)
		}
	}

	if len(results) > 0 {
		return results
	}

	// Strategy 3: basename match (e.g. for single-file packages)
	basename := normalizedRel
	if idx := strings.LastIndex(normalizedRel, "/"); idx >= 0 {
		basename = normalizedRel[idx+1:]
	}
	for fp := range allFilePaths {
		normalized := strings.ReplaceAll(fp, "\\", "/")
		dir := filepath.Dir(normalized)
		dirBase := filepath.Base(dir)
		if dirBase == basename && strings.HasSuffix(normalized, ".go") {
			results = append(results, fp)
		}
	}

	if len(results) > 0 {
		return results
	}

	return nil
}

// isGoStdlibPackage returns true if the import path looks like a Go standard
// library package. Stdlib packages have no dots in their first segment.
func isGoStdlibPackage(importPath string) bool {
	firstSlash := strings.Index(importPath, "/")
	firstSegment := importPath
	if firstSlash > 0 {
		firstSegment = importPath[:firstSlash]
	}
	// Stdlib packages have no dots (e.g. "fmt", "encoding")
	// External packages always have dots (e.g. "github.com")
	return !strings.Contains(firstSegment, ".")
}