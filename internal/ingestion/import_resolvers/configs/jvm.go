// Package configs — JVM (Java/Kotlin) import resolution configurations.
//
// Java/Kotlin use JVM wildcard/member strategy, then standard fallback.
// Mirrors TS import-resolvers/configs/jvm.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// javaJvmStrategy resolves Java wildcard and member imports.
func javaJvmStrategy(rawImportPath string, _ string, ctx *importresolvers.ResolveCtx) *importresolvers.ImportResult {
	if len(rawImportPath) >= 2 && rawImportPath[len(rawImportPath)-2:] == ".*" {
		matchedFiles := importresolvers.ResolveJvmWildcard(
			rawImportPath,
			ctx.NormalizedFileList,
			ctx.AllFileList,
			[]string{".java"},
			ctx.Index,
		)
		if len(matchedFiles) > 0 {
			return &importresolvers.ImportResult{Kind: "files", Files: matchedFiles}
		}
	} else {
		memberResolved := importresolvers.ResolveJvmMemberImport(
			rawImportPath,
			ctx.NormalizedFileList,
			ctx.AllFileList,
			[]string{".java"},
			ctx.Index,
		)
		if memberResolved != "" {
			return &importresolvers.ImportResult{Kind: "files", Files: []string{memberResolved}}
		}
	}
	return nil
}

// kotlinJvmStrategy resolves Kotlin wildcard and member imports,
// with Java interop fallback and top-level function import support.
func kotlinJvmStrategy(rawImportPath string, _ string, ctx *importresolvers.ResolveCtx) *importresolvers.ImportResult {
	if len(rawImportPath) >= 2 && rawImportPath[len(rawImportPath)-2:] == ".*" {
		matchedFiles := importresolvers.ResolveJvmWildcard(
			rawImportPath,
			ctx.NormalizedFileList,
			ctx.AllFileList,
			importresolvers.KotlinExtensions,
			ctx.Index,
		)
		if len(matchedFiles) == 0 {
			javaMatches := importresolvers.ResolveJvmWildcard(
				rawImportPath,
				ctx.NormalizedFileList,
				ctx.AllFileList,
				[]string{".java"},
				ctx.Index,
			)
			if len(javaMatches) > 0 {
				return &importresolvers.ImportResult{Kind: "files", Files: javaMatches}
			}
		}
		if len(matchedFiles) > 0 {
			return &importresolvers.ImportResult{Kind: "files", Files: matchedFiles}
		}
	} else {
		memberResolved := importresolvers.ResolveJvmMemberImport(
			rawImportPath,
			ctx.NormalizedFileList,
			ctx.AllFileList,
			importresolvers.KotlinExtensions,
			ctx.Index,
		)
		if memberResolved == "" {
			memberResolved = importresolvers.ResolveJvmMemberImport(
				rawImportPath,
				ctx.NormalizedFileList,
				ctx.AllFileList,
				[]string{".java"},
				ctx.Index,
			)
		}
		if memberResolved != "" {
			return &importresolvers.ImportResult{Kind: "files", Files: []string{memberResolved}}
		}

		// Kotlin: top-level function imports (e.g. import models.getUser)
		// resolveJvmMemberImport skips these (requires >=3 segments).
		// Fall back to package-directory scan for lowercase last segments.
		segments := splitDotPath(rawImportPath)
		lastSeg := segments[len(segments)-1]
		if len(segments) >= 2 && len(lastSeg) > 0 && isLower(lastSeg[0]) {
			pkgWildcard := joinSegments(segments[:len(segments)-1]) + ".*"
			var dirFiles []string
			dirFiles = importresolvers.ResolveJvmWildcard(
				pkgWildcard,
				ctx.NormalizedFileList,
				ctx.AllFileList,
				importresolvers.KotlinExtensions,
				ctx.Index,
			)
			if len(dirFiles) == 0 {
				dirFiles = importresolvers.ResolveJvmWildcard(
					pkgWildcard,
					ctx.NormalizedFileList,
					ctx.AllFileList,
					[]string{".java"},
					ctx.Index,
				)
			}
			if len(dirFiles) > 0 {
				return &importresolvers.ImportResult{Kind: "files", Files: dirFiles}
			}
		}
	}
	return nil
}

// JavaImportConfig is the import resolution configuration for Java.
var JavaImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangJava,
	Strategies: []importresolvers.ImportResolverStrategy{
		javaJvmStrategy,
		importresolvers.CreateStandardStrategy(core.LangJava),
	},
}

// KotlinImportConfig is the import resolution configuration for Kotlin.
// Note: Kotlin is not a core language in codetrip; this config is provided
// for completeness but may not be used in the core pipeline.
var KotlinImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangJava, // Kotlin shares Java's language code for now
	Strategies: []importresolvers.ImportResolverStrategy{
		kotlinJvmStrategy,
		importresolvers.CreateStandardStrategy(core.LangJava),
	},
}

// ---- helpers ----

func splitDotPath(s string) []string {
	// Split on dot, filter empty
	var result []string
	for _, part := range splitOnDot(s) {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitOnDot(s string) []string {
	parts := make([]string, 0)
	current := ""
	for _, ch := range s {
		if ch == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	parts = append(parts, current)
	return parts
}

func joinSegments(segments []string) string {
	result := ""
	for i, s := range segments {
		if i > 0 {
			result += "."
		}
		result += s
	}
	return result
}

func isLower(b byte) bool {
	return b >= 'a' && b <= 'z'
}