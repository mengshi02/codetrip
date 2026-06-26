// Package importresolvers — JVM import resolution helpers.
//
// Resolves Java/Kotlin wildcard and member imports.
// Mirrors TS import-resolvers/jvm.ts.
package importresolvers

import (
	"path/filepath"
	"strings"
	"unicode"
)

// KotlinExtensions lists Kotlin source file extensions.
var KotlinExtensions = []string{".kt", ".kts"}

// ResolveJvmWildcard resolves a JVM wildcard import (e.g. "com.example.*")
// to all matching files in the corresponding directory.
//
// Converts dot-separated package path to directory path, then finds
// all files with the given extensions in that directory.
func ResolveJvmWildcard(importPath string, normalizedFileList, allFileList []string, extensions []string, index *SuffixIndex) []string {
	// Strip trailing ".*"
	if !strings.HasSuffix(importPath, ".*") {
		return nil
	}
	pkgPath := strings.TrimSuffix(importPath, ".*")
	dirPath := strings.ReplaceAll(pkgPath, ".", "/")

	var files []string

	// Try using the suffix index for directory lookups
	if index != nil {
		for _, ext := range extensions {
			dirFiles := index.GetFilesInDir(dirPath, ext)
			files = append(files, dirFiles...)
		}
		if len(files) > 0 {
			return files
		}
	}

	// Fallback: linear scan
	dirSuffix := "/" + dirPath + "/"
	for i, normalized := range normalizedFileList {
		if !strings.Contains(normalized, dirSuffix) {
			continue
		}
		// Check extension
		ext := filepath.Ext(normalized)
		for _, allowedExt := range extensions {
			if ext == allowedExt {
				// Verify it's a direct child (no subdirectory after dirSuffix)
				idx := strings.Index(normalized, dirSuffix)
				afterDir := normalized[idx+len(dirSuffix):]
				if !strings.Contains(afterDir, "/") {
					files = append(files, allFileList[i])
				}
				break
			}
		}
	}

	return files
}

// ResolveJvmMemberImport resolves a JVM member import by stripping the
// last segment (the member name) and looking for the class file.
//
// e.g. "com.example.models.User" → strip "User" → look for "com/example/models" + extensions
//
// Skips if:
//   - fewer than 3 segments (not enough for package.class.member)
//   - last segment is ALL_CAPS (constant field, not a class)
//   - last segment starts with lowercase (method/field, not a class)
func ResolveJvmMemberImport(importPath string, normalizedFileList, allFileList []string, extensions []string, index *SuffixIndex) string {
	segments := strings.Split(importPath, ".")
	if len(segments) < 3 {
		return ""
	}

	lastSegment := segments[len(segments)-1]
	if len(lastSegment) == 0 {
		return ""
	}

	// Skip ALL_CAPS (constants like "MAX_VALUE")
	if isAllCaps(lastSegment) {
		return ""
	}

	// Skip lowercase-starting segments (methods/fields)
	if unicode.IsLower(rune(lastSegment[0])) {
		return ""
	}

	// Strip last segment to get the class path
	classPath := strings.Join(segments[:len(segments)-1], "/")

	// Try suffix resolve
	pathParts := strings.Split(classPath, "/")
	if result := SuffixResolve(pathParts, normalizedFileList, allFileList, index); result != "" {
		// Verify the result has the right extension
		ext := filepath.Ext(result)
		for _, allowedExt := range extensions {
			if ext == allowedExt {
				return result
			}
		}
	}

	return ""
}

// AppendKotlinWildcard checks if a Kotlin import has a wildcard_import sibling
// and appends ".*" if so. This handles Kotlin's syntax where wildcard imports
// are written as "import com.example.*" with the wildcard as a sibling node.
func AppendKotlinWildcard(importPath string, hasWildcardSibling bool) string {
	if hasWildcardSibling && !strings.HasSuffix(importPath, ".*") {
		return importPath + ".*"
	}
	return importPath
}

// isAllCaps checks if a string consists entirely of uppercase letters,
// digits, and underscores (i.e. a constant name like "MAX_VALUE").
func isAllCaps(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return len(s) > 0
}