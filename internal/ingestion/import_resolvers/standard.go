// Package importresolvers — standard import resolver.
//
// Provides the generic fallback resolution strategy used by most languages.
// Mirrors TS import-resolvers/standard.ts.
package importresolvers

import (
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ResolveCacheCap is the maximum number of entries in the resolve cache.
// When exceeded, the oldest 20% of entries are evicted.
const ResolveCacheCap = 100_000

// resolveCacheEvictFraction is the fraction of entries to evict when the cache overflows.
const resolveCacheEvictFraction = 0.2

// ResolveImportPath performs the core import path resolution logic.
//
// Resolution order:
//  1. Cache lookup
//  2. TS path alias rewrite (if applicable)
//  3. Rust crate::/super::/self:: prefix conversion (if applicable)
//  4. Relative path resolution (. and ..)
//  5. ESM extension stripping (JS/TS)
//  6. Java wildcard skip
//  7. C/C++ dot-preservation
//  8. Suffix-based resolution
//
// Returns the resolved file path or empty string if not found.
func ResolveImportPath(rawImportPath string, filePath string, ctx *ResolveCtx, language core.SupportedLanguage) string {
	cacheKey := filePath + "::" + rawImportPath

	// 1. Cache lookup
	if resolved, ok := ctx.ResolveCache[cacheKey]; ok {
		if resolved != nil {
			return *resolved
		}
		return ""
	}

	resolved := resolveImportPathInternal(rawImportPath, filePath, ctx, language)

	// Store in cache (with eviction)
	if len(ctx.ResolveCache) >= ResolveCacheCap {
		evictResolveCache(ctx.ResolveCache)
	}
	if resolved != "" {
		ctx.ResolveCache[cacheKey] = &resolved
	} else {
		ctx.ResolveCache[cacheKey] = nil
	}

	return resolved
}

func resolveImportPathInternal(rawImportPath string, filePath string, ctx *ResolveCtx, language core.SupportedLanguage) string {
	importPath := rawImportPath

	// 2. TS path alias rewrite
	if language == core.LangTypeScript || language == core.LangJavaScript {
		if rewritten := rewriteTsPathAlias(importPath, ctx); rewritten != "" {
			return rewritten
		}
	}

	// 3. Rust prefix conversion
	if language == core.LangRust {
		importPath = convertRustPrefixes(importPath, filePath, ctx)
		if importPath == "" {
			return ""
		}
	}

	// 4. Relative path resolution
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		dir := filepath.Dir(filePath)
		resolved := filepath.Join(dir, importPath)
		resolved = filepath.ToSlash(resolved)
		if matched := TryResolveWithExtensions(resolved, ctx.AllFilePaths); matched != "" {
			return matched
		}
	}

	// 5. ESM extension stripping (JS/TS)
	if language == core.LangTypeScript || language == core.LangJavaScript {
		stripped := stripJsExtension(importPath)
		if stripped != importPath {
			pathParts := strings.Split(stripped, "/")
			if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
				return result
			}
		}
	}

	// 6. Java wildcard skip
	if language == core.LangJava && strings.HasSuffix(importPath, ".*") {
		return ""
	}

	// 7. C/C++ — preserve dots in path (e.g. "sys/types.h")
	if language == core.LangC || language == core.LangCpp {
		if matched := TryResolveWithExtensions(importPath, ctx.AllFilePaths); matched != "" {
			return matched
		}
		// Also try suffix resolve
		pathParts := strings.Split(importPath, "/")
		if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
			return result
		}
		return ""
	}

	// 8. Suffix-based resolution
	pathParts := strings.Split(importPath, "/")
	if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
		return result
	}

	return ""
}

// ResolveStandard wraps ResolveImportPath into an ImportResult.
// Returns nil if no resolution found (to let the next strategy try).
func ResolveStandard(rawImportPath string, filePath string, ctx *ResolveCtx, language core.SupportedLanguage) *ImportResult {
	resolved := ResolveImportPath(rawImportPath, filePath, ctx, language)
	if resolved != "" {
		return &ImportResult{Kind: "files", Files: []string{resolved}}
	}
	return nil
}

// CreateStandardStrategy creates a standard ImportResolverStrategy for the given language.
func CreateStandardStrategy(language core.SupportedLanguage) ImportResolverStrategy {
	return func(rawImportPath string, filePath string, ctx *ResolveCtx) *ImportResult {
		return ResolveStandard(rawImportPath, filePath, ctx, language)
	}
}

// ---- Internal helpers ----

// rewriteTsPathAlias rewrites import path using tsconfig path aliases.
func rewriteTsPathAlias(importPath string, ctx *ResolveCtx) string {
	if ctx.Configs.TsconfigPaths == nil {
		return ""
	}
	aliases := ctx.Configs.TsconfigPaths.Aliases
	if len(aliases) == 0 {
		return ""
	}
	for _, alias := range aliases {
		aliasPrefix := alias[0]
		targetPrefix := alias[1]
		if strings.HasPrefix(importPath, aliasPrefix) {
			rest := importPath[len(aliasPrefix):]
			rewritten := targetPrefix + rest
			pathParts := strings.Split(rewritten, "/")
			if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
				return result
			}
		}
	}
	return ""
}

// convertRustPrefixes converts Rust crate::/super::/self:: prefixes to file system paths.
// Returns empty string if the import should be skipped.
func convertRustPrefixes(importPath string, filePath string, ctx *ResolveCtx) string {
	// crate:: → src/ + root
	if strings.HasPrefix(importPath, "crate::") {
		rest := strings.TrimPrefix(importPath, "crate::")
		rest = strings.ReplaceAll(rest, "::", "/")
		pathParts := strings.Split("src/"+rest, "/")
		if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
			return result
		}
		// Also try without src/
		pathParts2 := strings.Split(rest, "/")
		if result := SuffixResolve(pathParts2, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
			return result
		}
		return ""
	}

	// super:: → parent directory
	if strings.HasPrefix(importPath, "super::") {
		dir := filepath.Dir(filePath)
		rest := strings.TrimPrefix(importPath, "super::")
		// Count additional super:: prefixes
		for strings.HasPrefix(rest, "super::") {
			rest = strings.TrimPrefix(rest, "super::")
			dir = filepath.Dir(dir)
		}
		rest = strings.ReplaceAll(rest, "::", "/")
		resolved := filepath.ToSlash(filepath.Join(dir, rest))
		if matched := TryResolveWithExtensions(resolved, ctx.AllFilePaths); matched != "" {
			return matched
		}
		// Also try Rust-specific extensions
		pathParts := strings.Split(rest, "/")
		if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
			return result
		}
		return ""
	}

	// self:: → current directory
	if strings.HasPrefix(importPath, "self::") {
		dir := filepath.Dir(filePath)
		rest := strings.TrimPrefix(importPath, "self::")
		rest = strings.ReplaceAll(rest, "::", "/")
		resolved := filepath.ToSlash(filepath.Join(dir, rest))
		if matched := TryResolveWithExtensions(resolved, ctx.AllFilePaths); matched != "" {
			return matched
		}
		pathParts := strings.Split(rest, "/")
		if result := SuffixResolve(pathParts, ctx.NormalizedFileList, ctx.AllFileList, ctx.Index); result != "" {
			return result
		}
		return ""
	}

	return importPath
}

// stripJsExtension removes JS/TS extensions for ESM→TS resolution.
// ESM imports use .js extensions but the source files are .ts.
func stripJsExtension(path string) string {
	for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}
	return path
}

// evictResolveCache evicts the oldest entries from the resolve cache.
func evictResolveCache(cache map[string]*string) {
	evictCount := int(float64(len(cache)) * resolveCacheEvictFraction)
	if evictCount < 1 {
		evictCount = 1
	}
	i := 0
	for key := range cache {
		delete(cache, key)
		i++
		if i >= evictCount {
			break
		}
	}
}