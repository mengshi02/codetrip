package c

import (
	"path/filepath"
	"strings"
)

// ResolveCImportTarget resolves a C #include path to a file in the workspace.
//
// Strategy:
// 1. Check for a same-directory sibling relative to the including file
//    (matches C compiler `#include "…"` relative-lookup semantics).
// 2. Check for an exact match (path as-is in the workspace).
// 3. Fall back to suffix matching against all workspace file paths.
//    Tie-breaking: prefer the match with the fewest path components
//    (closest to root). On equal depth, break ties lexicographically.
//
// Ported from GitNexus c/import-target.ts.
func ResolveCImportTarget(targetRaw, fromFile string, allFilePaths map[string]bool) string {
	if targetRaw == "" {
		return ""
	}

	normalizedTarget := strings.ReplaceAll(targetRaw, "\\", "/")

	// Same-directory sibling first: mirrors the C compiler's #include "…"
	// relative-lookup semantics.
	if fromFile != "" {
		siblingRaw := filepath.Join(filepath.Dir(fromFile), targetRaw)
		sibling := strings.ReplaceAll(siblingRaw, "\\", "/")
		if allFilePaths[sibling] {
			return sibling
		}
		if targetRaw != normalizedTarget {
			siblingAlt := filepath.Join(filepath.Dir(fromFile), normalizedTarget)
			siblingAltNorm := strings.ReplaceAll(siblingAlt, "\\", "/")
			if allFilePaths[siblingAltNorm] {
				return siblingAltNorm
			}
		}
	}

	// Exact match (path as-is in the workspace)
	if allFilePaths[normalizedTarget] {
		return normalizedTarget
	}

	// Suffix match: find files ending with /targetRaw or equal to targetRaw.
	suffix := "/" + normalizedTarget
	targetBasename := normalizedTarget
	if idx := strings.LastIndex(normalizedTarget, "/"); idx >= 0 {
		targetBasename = normalizedTarget[idx+1:]
	}

	var bestMatch string
	bestDepth := int(^uint(0) >> 1) // max int
	bestNormalized := ""

	for path := range allFilePaths {
		normalized := strings.ReplaceAll(path, "\\", "/")
		basename := normalized
		if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
			basename = normalized[idx+1:]
		}

		if basename != targetBasename {
			continue
		}

		if normalized == normalizedTarget || strings.HasSuffix(normalized, suffix) {
			depth := strings.Count(normalized, "/") + 1
			if depth < bestDepth || (depth == bestDepth && normalized < bestNormalized) {
				bestDepth = depth
				bestMatch = path
				bestNormalized = normalized
			}
		}
	}

	return bestMatch
}