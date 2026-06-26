// Package importresolvers — C# import resolution helpers.
//
// Resolves C# namespace-based imports using .csproj project configurations.
// Mirrors TS import-resolvers/csharp.ts.
package importresolvers

import (
	"path/filepath"
	"strings"
)

// ResolveCSharpImportInternal resolves a C# import path using namespace→project
// directory mappings from .csproj files.
//
// Resolution strategy:
//  1. Map namespace → projectDir via .csproj configs
//  2. Look for a single .cs file matching the last segment
//  3. Look for multiple .cs files in the corresponding directory
//  4. Suffix fallback (gated by namespace evidence to prevent BCL spurious matches)
func ResolveCSharpImportInternal(
	importPath string,
	csharpConfigs []CSharpProjectConfig,
	normalizedFileList, allFileList []string,
	index *SuffixIndex,
	evidence *CSharpNamespaceEvidence,
) []string {
	// Convert namespace path to directory path
	// e.g. "MyApp.Services" → "Services" under projectDir
	segments := strings.Split(importPath, ".")
	if len(segments) == 0 {
		return nil
	}

	// Try each project config
	for _, config := range csharpConfigs {
		// Check if the import path starts with the root namespace
		if config.RootNamespace != "" && !strings.HasPrefix(importPath, config.RootNamespace) {
			continue
		}

		// Compute the relative directory path from the namespace
		relativePath := importPath
		if config.RootNamespace != "" && strings.HasPrefix(importPath, config.RootNamespace+".") {
			relativePath = strings.TrimPrefix(importPath, config.RootNamespace+".")
		} else if importPath == config.RootNamespace {
			// Import is the root namespace itself — return all files in project dir
			return filesInDir(config.ProjectDir, normalizedFileList, allFileList, ".cs")
		}

		// Convert remaining segments to directory path
		dirPath := filepath.ToSlash(filepath.Join(config.ProjectDir, strings.ReplaceAll(relativePath, ".", "/")))

		// Try to find a single file matching the last segment
		lastSegment := segments[len(segments)-1]
		singleFile := dirPath + ".cs"
		if index != nil {
			if result, ok := index.Get(singleFile); ok {
				return []string{result}
			}
			if result, ok := index.GetInsensitive(singleFile); ok {
				return []string{result}
			}
		}

		// Try to find files in the directory
		dirFiles := filesInDir(dirPath, normalizedFileList, allFileList, ".cs")
		if len(dirFiles) > 0 {
			return dirFiles
		}

		// Try looking for the last segment as a file in the parent directory
		if len(segments) > 1 {
			parentDir := filepath.ToSlash(filepath.Join(config.ProjectDir,
				strings.ReplaceAll(strings.Join(segments[:len(segments)-1], "."), ".", "/")))
			candidate := parentDir + "/" + lastSegment + ".cs"
			if index != nil {
				if result, ok := index.Get(candidate); ok {
					return []string{result}
				}
				if result, ok := index.GetInsensitive(candidate); ok {
					return []string{result}
				}
			}
		}
	}

	// Suffix fallback — gated by namespace evidence
	if evidence != nil && !isCSharpSuffixFallbackAllowed(importPath, evidence) {
		return nil // gate blocks suffix fallback
	}

	// Try generic suffix resolution
	pathParts := strings.Split(strings.ReplaceAll(importPath, ".", "/"), "/")
	if result := SuffixResolve(pathParts, normalizedFileList, allFileList, index); result != "" {
		return []string{result}
	}

	return nil
}

// ResolveCSharpNamespaceDir computes the directory suffix for a C# namespace import.
// e.g. importPath="MyApp.Services", config with RootNamespace="MyApp", ProjectDir="src/App"
//      → "/src/App/Services/"
func ResolveCSharpNamespaceDir(importPath string, csharpConfigs []CSharpProjectConfig) string {
	for _, config := range csharpConfigs {
		if config.RootNamespace == "" {
			continue
		}
		if !strings.HasPrefix(importPath, config.RootNamespace) {
			continue
		}
		relativePath := importPath
		if strings.HasPrefix(importPath, config.RootNamespace+".") {
			relativePath = strings.TrimPrefix(importPath, config.RootNamespace+".")
		} else if importPath == config.RootNamespace {
			return "/" + config.ProjectDir + "/"
		}
		dirPath := filepath.ToSlash(filepath.Join(config.ProjectDir, strings.ReplaceAll(relativePath, ".", "/")))
		return "/" + dirPath + "/"
	}
	return ""
}

// CSharpNamespaceStrategy is the C# namespace-based resolution strategy.
func CSharpNamespaceStrategy(rawImportPath string, _ string, ctx *ResolveCtx) *ImportResult {
	csharpConfigs := ctx.Configs.CSharpConfigs
	evidence := ctx.Configs.CSharpNamespaces

	if len(csharpConfigs) == 0 {
		// No csproj configs — gate suffix fallback with namespace evidence
		if evidence != nil && !isCSharpSuffixFallbackAllowed(rawImportPath, evidence) {
			return &ImportResult{Kind: "files", Files: []string{}}
		}
		return nil // let next strategy try
	}

	resolvedFiles := ResolveCSharpImportInternal(
		rawImportPath,
		csharpConfigs,
		ctx.NormalizedFileList,
		ctx.AllFileList,
		ctx.Index,
		evidence,
	)

	if len(resolvedFiles) > 1 {
		dirSuffix := ResolveCSharpNamespaceDir(rawImportPath, csharpConfigs)
		if dirSuffix != "" {
			return &ImportResult{Kind: "package", Files: resolvedFiles, DirSuffix: dirSuffix}
		}
	}

	// Authoritative: return even empty result to stop chain
	if resolvedFiles == nil {
		resolvedFiles = []string{}
	}
	return &ImportResult{Kind: "files", Files: resolvedFiles}
}

// ---- Internal helpers ----

// filesInDir returns all files with the given extension in a directory.
func filesInDir(dirPath string, normalizedFileList, allFileList []string, extension string) []string {
	dirPrefix := dirPath + "/"
	var files []string
	for i, normalized := range normalizedFileList {
		if !strings.HasPrefix(normalized, dirPrefix) {
			continue
		}
		if !strings.HasSuffix(normalized, extension) {
			continue
		}
		afterDir := normalized[len(dirPrefix):]
		if strings.Contains(afterDir, "/") {
			continue // only direct children
		}
		files = append(files, allFileList[i])
	}
	return files
}

// isCSharpSuffixFallbackAllowed checks whether suffix fallback should be
// allowed for a given import path based on in-repo namespace evidence.
// This prevents BCL/system namespace imports from spuriously matching
// local files with the same suffix.
func isCSharpSuffixFallbackAllowed(importPath string, evidence *CSharpNamespaceEvidence) bool {
	if evidence == nil || len(evidence.Namespaces) == 0 {
		return true // no evidence → gate fails open
	}
	// Check if any declared namespace is a prefix of the import path
	for ns := range evidence.Namespaces {
		if strings.HasPrefix(importPath, ns) || strings.HasPrefix(ns, importPath) {
			return true
		}
	}
	return false
}