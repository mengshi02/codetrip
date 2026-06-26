package cpp

// C++ Header Scan — discover header files for import resolution.
//
// C++ #include directives reference header files (.h, .hpp, .hxx, .hh)
// that may not be classified as C++ by language detection (e.g. a .h file
// could be C or C++). This module scans the workspace to find all header
// files and returns them as a set for import resolution augmentation.
// Ported from TS languages/cpp/header-scan.ts.

import (
	"os"
	"path/filepath"
	"strings"
)

// headerExtensions are the file extensions considered C++ headers.
var headerExtensions = map[string]bool{
	".h":   true,
	".hpp": true,
	".hxx": true,
	".hh":  true,
	".inl": true, // inline definition files
}

// ScanCppHeaderFiles scans the workspace for C++ header files.
// Returns a map of header file paths for use as resolutionConfig.
// TODO: full implementation — use gitignore-aware file walker.
func ScanCppHeaderFiles(repoPath string) map[string]bool {
	headers := make(map[string]bool)
	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if headerExtensions[ext] {
			rel, relErr := filepath.Rel(repoPath, path)
			if relErr == nil {
				headers[rel] = true
			}
		}
		return nil
	})
	return headers
}