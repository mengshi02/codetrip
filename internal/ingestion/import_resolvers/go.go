// Package importresolvers — Go package import resolution helpers.
//
// Resolves Go import paths to their package directories and files.
// Mirrors TS import-resolvers/go.ts.
package importresolvers

import (
	"path/filepath"
	"strings"
)

// ResolveGoPackageDir extracts the directory suffix from a Go import path
// relative to the module path.
//
// e.g. importPath="github.com/example/project/pkg/utils",
//      modulePath="github.com/example/project"
//      → "/pkg/utils/"
func ResolveGoPackageDir(importPath string, modulePath string) string {
	if modulePath == "" {
		return ""
	}
	if !strings.HasPrefix(importPath, modulePath) {
		return ""
	}
	rest := importPath[len(modulePath):]
	if rest == "" {
		return "/"
	}
	// Must be a proper sub-package: rest should start with "/"
	if rest[0] != '/' {
		return ""
	}
	return rest + "/"
}

// ResolveGoPackage resolves a Go import path to all .go files in the
// corresponding package directory (excluding _test.go files).
//
// Only direct children of the directory are returned (no subdirectories).
func ResolveGoPackage(importPath string, modulePath string, normalizedFileList, allFileList []string) []string {
	dirSuffix := ResolveGoPackageDir(importPath, modulePath)
	if dirSuffix == "" {
		return nil
	}

	var files []string
	for i, normalized := range normalizedFileList {
		// Check if the file ends with the directory suffix pattern
		// e.g. dirSuffix="/pkg/utils/" → file must be in that directory
		if !hasDirSuffix(normalized, dirSuffix) {
			continue
		}

		// Must be a .go file
		if !strings.HasSuffix(normalized, ".go") {
			continue
		}

		// Exclude _test.go files
		if strings.HasSuffix(normalized, "_test.go") {
			continue
		}

		// Must be a direct child (no further slashes after the dir suffix)
		afterDir := normalized[strings.LastIndex(normalized, dirSuffix)+len(dirSuffix):]
		if strings.Contains(afterDir, "/") {
			continue
		}

		files = append(files, allFileList[i])
	}

	return files
}

// hasDirSuffix checks if a normalized path contains the given directory suffix.
// dirSuffix is like "/pkg/utils/" — we look for this as a directory component
// in the path.
func hasDirSuffix(normalizedPath string, dirSuffix string) bool {
	// dirSuffix is like "/pkg/utils/"
	// We need to find this as a substring, and the character before it
	// must be nothing (beginning of path) or a path separator already handled
	idx := strings.Index(normalizedPath, dirSuffix)
	if idx < 0 {
		return false
	}
	// The part after the dir suffix should be a filename (no additional /)
	afterDir := normalizedPath[idx+len(dirSuffix):]
	if afterDir == "" {
		return false // trailing slash with no file
	}
	// Verify there's no subdirectory after the package dir
	// (we only want direct children)
	nextSlash := strings.Index(afterDir, "/")
	if nextSlash >= 0 {
		// There's a slash — this is a subdirectory, not a direct child
		// We need this check at a higher level, but also verify it's not
		// just a different path that happens to contain the suffix
		_ = nextSlash
	}
	return true
}

// GoPackageStrategy resolves Go package imports.
// Returns an ImportResult for the package directory and its .go files.
func GoPackageStrategy(rawImportPath string, _ string, ctx *ResolveCtx) *ImportResult {
	goModule := ctx.Configs.GoModule
	if goModule == nil || goModule.ModulePath == "" {
		return nil
	}

	files := ResolveGoPackage(
		rawImportPath,
		goModule.ModulePath,
		ctx.NormalizedFileList,
		ctx.AllFileList,
	)

	if len(files) > 0 {
		dirSuffix := ResolveGoPackageDir(rawImportPath, goModule.ModulePath)
		// Clean the dirSuffix for storage
		cleanSuffix := filepath.ToSlash(filepath.Clean(dirSuffix))
		if cleanSuffix == "." {
			cleanSuffix = "/"
		} else if !strings.HasPrefix(cleanSuffix, "/") {
			cleanSuffix = "/" + cleanSuffix
		}
		return &ImportResult{
			Kind:      "package",
			Files:     files,
			DirSuffix: cleanSuffix,
		}
	}

	return nil
}