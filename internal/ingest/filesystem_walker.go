package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileEntry represents a file discovered during repository walking.
type FileEntry struct {
	RelativePath string
	AbsolutePath string
	Size         int64
	IsDir        bool
	Extension    string
	LanguageID   string
	ParserID     string
}

// WalkResult holds the result of a repository walk.
type WalkResult struct {
	Files        []FileEntry
	TotalFiles   int
	TotalSize    int64
	SkippedDirs  int
	SkippedFiles int
}

// shouldIgnoreDir checks if a directory should be skipped during walking.
func shouldIgnoreDir(name string) bool {
	if SkipDirectories[name] {
		return true
	}
	// Skip any directory ending with -csv (output directories)
	if strings.HasSuffix(name, "-csv") {
		return true
	}
	return false
}

// shouldIgnoreFile checks if a file should be skipped based on extension or name.
func shouldIgnoreFile(name string) bool {
	if SkipFiles[name] {
		return true
	}
	nameLower := strings.ToLower(name)
	// Check extension (last dot)
	lastDot := strings.LastIndex(nameLower, ".")
	if lastDot != -1 {
		ext := nameLower[lastDot:]
		if SkipExtensions[ext] {
			return true
		}
		// Handle compound extensions like .min.js, .bundle.js, .d.ts
		secondDot := strings.LastIndex(nameLower[:lastDot], ".")
		if secondDot != -1 {
			compoundExt := nameLower[secondDot:]
			if SkipExtensions[compoundExt] {
				return true
			}
		}
	}
	// Ignore generated/bundled code patterns
	if strings.Contains(nameLower, ".bundle.") ||
		strings.Contains(nameLower, ".chunk.") ||
		strings.Contains(nameLower, ".generated.") {
		return true
	}
	return false
}

// maxFileSize is the maximum file size (in bytes) that will be processed.
const maxFileSize int64 = 512 * 1024

// WalkRepositoryPaths walks the repository and discovers all files.
func WalkRepositoryPaths(repoPath string) (*WalkResult, error) {
	result := &WalkResult{}
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		base := filepath.Base(relPath)

		// Skip dot-entries (files and directories) to match dot:false behavior.
		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				result.SkippedDirs++
				return filepath.SkipDir
			}
			result.SkippedFiles++
			return nil
		}

		if info.IsDir() {
			// Skip directories in the ignore list (uses specific ignore list, not blanket dot-prefix skip)
			if shouldIgnoreDir(base) {
				result.SkippedDirs++
				return filepath.SkipDir
			}
			return nil
		}

		// Skip specific files by name or extension
		if shouldIgnoreFile(base) {
			result.SkippedFiles++
			return nil
		}

		if info.Size() > maxFileSize {
			result.SkippedFiles++
			return nil
		}

		ext := filepath.Ext(path)
		langID := LanguageID(ext)
		parserID := ParserID(ext)

		entry := FileEntry{
			RelativePath: relPath,
			AbsolutePath: path,
			Size:         info.Size(),
			IsDir:        false,
			Extension:    ext,
			LanguageID:   langID,
			ParserID:     parserID,
		}

		result.Files = append(result.Files, entry)
		result.TotalFiles++
		result.TotalSize += info.Size()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk repository: %w", err)
	}

	return result, nil
}

// ReadFileContents reads the content of a file, respecting the max buffer size.
func ReadFileContents(absPath string, fileSize int64) (string, error) {
	bufSize := GetTreeSitterBufferSize(fileSize)
	if fileSize > int64(bufSize) {
		// File too large, skip reading
		return "", fmt.Errorf("file too large: %d bytes (max %d)", fileSize, bufSize)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", absPath, err)
	}

	return string(data), nil
}

// WalkRepository is the main entry point combining walk + optional content reading.
func WalkRepository(repoPath string) (*WalkResult, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat repo path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repo path is not a directory: %s", absPath)
	}

	return WalkRepositoryPaths(absPath)
}
