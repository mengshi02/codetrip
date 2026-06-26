package c

import (
	"os"
	"path/filepath"
	"strings"
)

// Header extensions to scan for in the workspace.
var cHeaderExtensions = map[string]bool{
	".h": true,
}

// Directories to skip during header scanning.
var cSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true,
	"_build":       true,
	".next":        true,
}

// ScanHeaderFiles walks repoPath recursively and returns relative paths of all .h files.
// Used by the C resolver so it can resolve #include targets that live in .h files.
// Ported from GitNexus c/header-scan.ts.
func ScanHeaderFiles(repoPath string) map[string]bool {
	headers := make(map[string]bool)
	walkHeaderDir(repoPath, repoPath, headers)
	return headers
}

func walkHeaderDir(dir, root string, out map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // permission denied, etc.
	}
	for _, entry := range entries {
		name := entry.Name()
		full := filepath.Join(dir, name)

		if entry.IsDir() {
			// Skip common non-source directories and build output dirs.
			if cSkipDirs[name] || strings.HasPrefix(name, "cmake-build") {
				continue
			}
			walkHeaderDir(full, root, out)
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if cHeaderExtensions[ext] {
				// Normalize to forward slashes for cross-platform consistency.
				rel, err := filepath.Rel(root, full)
				if err != nil {
					continue
				}
				normalized := strings.ReplaceAll(rel, "\\", "/")
				out[normalized] = true
			}
		}
	}
}