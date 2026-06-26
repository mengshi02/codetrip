package cpp

// C++ Import Target Resolution — resolve #include and using declarations.
//
// C++ imports are fundamentally different from other languages:
//   - #include "file.h" is a textual inclusion (wildcard import)
//   - #include <system_header> is a system import (not resolved locally)
//   - using X::name is a named import
//   - using namespace X is a wildcard namespace import
//
// This module resolves import targets to workspace file paths.
// Ported from TS languages/cpp/import-target.ts.

import (
	"path/filepath"
	"strings"
)

// ResolveCppImportTarget resolves a C++ import target to a workspace file path.
// Returns nil for unresolvable/system imports.
// TODO: full implementation — handle angle-bracket includes, relative paths, and header augmentation.
func ResolveCppImportTarget(targetRaw string, fromFile string, allFilePaths map[string]bool) []string {
	// System headers (angle-bracket includes) are not resolved locally
	if strings.HasPrefix(targetRaw, "<") && strings.HasSuffix(targetRaw, ">") {
		return nil
	}

	// Strip quotes from user includes
	target := strings.Trim(targetRaw, "\"'")

	if target == "" {
		return nil
	}

	// Try exact match first
	if allFilePaths[target] {
		return []string{target}
	}

	// Try relative resolution from the importing file's directory
	dir := filepath.Dir(fromFile)
	relPath := filepath.Join(dir, target)
	relPath = filepath.Clean(relPath)

	if allFilePaths[relPath] {
		return []string{relPath}
	}

	// Try common header search paths
	for _, candidate := range resolveCppHeaderCandidates(target) {
		if allFilePaths[candidate] {
			return []string{candidate}
		}
	}

	return nil
}

// resolveCppHeaderCandidates generates candidate paths for a header include.
// TODO: full implementation — search include directories, src/include, etc.
func resolveCppHeaderCandidates(target string) []string {
	return []string{
		filepath.Join("src", target),
		filepath.Join("include", target),
	}
}