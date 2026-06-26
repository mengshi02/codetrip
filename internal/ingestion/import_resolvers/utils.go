package importresolvers

import (
	"path/filepath"
	"strings"
)

// Extensions is the list of all file extensions to try during resolution.
var Extensions = []string{
	"",
	// TypeScript/JavaScript
	".tsx", ".ts", ".mts", ".cts",
	".jsx", ".js", ".mjs", ".cjs",
	"/index.tsx", "/index.ts", "/index.jsx", "/index.js",
	// Python
	".py", "/__init__.py",
	// Java
	".java",
	// C/C++
	".c", ".h", ".cpp", ".hpp", ".cc", ".cxx", ".hxx", ".hh",
	// C#
	".cs",
	// Go
	".go",
	// Rust
	".rs", "/mod.rs",
}

// SuffixIndex provides O(1) endsWith lookups for import resolution.
// Maps every possible path suffix to its original file path.
type SuffixIndex struct {
	exactMap   map[string]string // exact suffix -> original file path
	lowerMap   map[string]string // lowercase suffix -> original file path
	dirMap     map[string][]string // directory suffix:extension -> file paths
}

// Get returns the original file path for an exact suffix match.
func (si *SuffixIndex) Get(suffix string) (string, bool) {
	v, ok := si.exactMap[suffix]
	return v, ok
}

// GetInsensitive returns the original file path for a case-insensitive suffix match.
func (si *SuffixIndex) GetInsensitive(suffix string) (string, bool) {
	v, ok := si.lowerMap[strings.ToLower(suffix)]
	return v, ok
}

// GetFilesInDir returns all files in a directory suffix with the given extension.
func (si *SuffixIndex) GetFilesInDir(dirSuffix, extension string) []string {
	key := dirSuffix + ":" + extension
	if v, ok := si.dirMap[key]; ok {
		return v
	}
	return nil
}

// BuildSuffixIndex builds a suffix index for O(1) endsWith lookups.
// Maps every possible path suffix to its original file path.
// e.g. for "src/com/example/Foo.java":
//
//	"Foo.java" -> "src/com/example/Foo.java"
//	"example/Foo.java" -> "src/com/example/Foo.java"
//	"com/example/Foo.java" -> "src/com/example/Foo.java"
func BuildSuffixIndex(normalizedFileList, allFileList []string) *SuffixIndex {
	exactMap := make(map[string]string)
	lowerMap := make(map[string]string)
	dirMap := make(map[string][]string)

	for i, normalized := range normalizedFileList {
		original := allFileList[i]
		parts := strings.Split(normalized, "/")

		// Index all suffixes: "a/b/c.java" -> ["c.java", "b/c.java", "a/b/c.java"]
		for j := len(parts) - 1; j >= 0; j-- {
			suffix := strings.Join(parts[j:], "/")
			// Only store first match (longest path wins for ambiguous suffixes)
			if _, ok := exactMap[suffix]; !ok {
				exactMap[suffix] = original
			}
			lower := strings.ToLower(suffix)
			if _, ok := lowerMap[lower]; !ok {
				lowerMap[lower] = original
			}
		}

		// Index directory membership
		lastSlash := strings.LastIndex(normalized, "/")
		if lastSlash >= 0 {
			dirParts := parts[:len(parts)-1]
			fileName := parts[len(parts)-1]
			ext := filepath.Ext(fileName)

			for j := len(dirParts) - 1; j >= 0; j-- {
				dirSuffix := strings.Join(dirParts[j:], "/")
				key := dirSuffix + ":" + ext
				dirMap[key] = append(dirMap[key], original)
			}
		}
	}

	return &SuffixIndex{
		exactMap: exactMap,
		lowerMap: lowerMap,
		dirMap:   dirMap,
	}
}

// TryResolveWithExtensions tries to match a path (with extensions) against the known file set.
// Returns the matched file path or empty string.
func TryResolveWithExtensions(basePath string, allFiles map[string]bool) string {
	for _, ext := range Extensions {
		candidate := basePath + ext
		if allFiles[candidate] {
			return candidate
		}
	}
	return ""
}

// SuffixResolve performs suffix-based resolution using index. O(1) per lookup instead of O(files).
func SuffixResolve(pathParts []string, normalizedFileList, allFileList []string, index *SuffixIndex) string {
	if index != nil {
		for i := 0; i < len(pathParts); i++ {
			suffix := strings.Join(pathParts[i:], "/")
			for _, ext := range Extensions {
				suffixWithExt := suffix + ext
				if result, ok := index.Get(suffixWithExt); ok {
					return result
				}
				if result, ok := index.GetInsensitive(suffixWithExt); ok {
					return result
				}
			}
		}
		return ""
	}

	// Fallback: linear scan (for backward compatibility)
	for i := 0; i < len(pathParts); i++ {
		suffix := strings.Join(pathParts[i:], "/")
		for _, ext := range Extensions {
			suffixWithExt := suffix + ext
			suffixPattern := "/" + suffixWithExt
			for j, filePath := range normalizedFileList {
				if strings.HasSuffix(filePath, suffixPattern) ||
					strings.EqualFold(filePath[len(filePath)-len(suffixPattern):], suffixPattern) {
					return allFileList[j]
				}
			}
		}
	}
	return ""
}