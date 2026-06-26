// Filesystem walker — scans repository files and reads contents.
//
// Mirrors TS filesystem-walker.ts, adapted for codetrip:
//   - Uses os/filepath instead of Node.js fs/glob
//   - No ignore-service dependency (codetrip uses its own .gitignore handling)
//   - Sequential file reads (no worker pool, no Promise.allSettled)
//   - WalkRepositoryPaths: stat-only scan (paths + sizes, no content)
//   - ReadFileContents: load content for a subset of paths
//   - WalkRepository: legacy API combining scan + read
//
// The TS version uses glob + ignore-service for filtering; codetrip
// uses filepath.Walk with .gitignore-aware filtering handled by the
// scan phase upstream.

package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ─── Constants ────────────────────────────────────────────────

const (
	// DefaultMaxFileSizeBytes is the default maximum file size to include.
	// Files larger than this are skipped (likely generated/vendored).
	DefaultMaxFileSizeBytes = 2 * 1024 * 1024 // 2MB

	// ReadConcurrency controls batch size for sequential reads.
	// In Go we don't need concurrency for file reads — os.ReadFile
	// is already efficient — but we keep the constant for structural
	// consistency with the TS version.
	ReadConcurrency = 32

	// SkippedPreviewCap limits how many skipped-large-file paths to show.
	SkippedPreviewCap = 5
)

// ─── Types ──────────────────────────────────────────────────

// ScannedFile holds a file path and its size from stat, no content in memory.
type ScannedFile struct {
	Path string
	Size int64
}

// FileEntry holds a file path and its content.
type FileEntry struct {
	Path    string
	Content string
}

// ─── WalkRepositoryPaths ────────────────────────────────────

// WalkRepositoryPaths scans a repository — stat files to get paths + sizes,
// no content loaded. Memory: ~10MB for 100K files vs ~1GB+ with content.
//
// Files larger than maxFileSizeBytes are skipped. OnProgress is called
// for each file processed (path, processed count, total count).
func WalkRepositoryPaths(
	repoPath string,
	maxFileSizeBytes int64,
	onProgress func(current int, total int, filePath string),
) ([]ScannedFile, error) {
	var entries []ScannedFile
	var skippedLarge int
	var skippedLargePaths []string
	total := 0

	err := filepath.WalkDir(repoPath, func(relPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			// Skip common non-source directories
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" ||
				base == "dist" || base == "build" || base == "__pycache__" ||
				base == ".venv" || base == "target" || base == "bin" ||
				base == ".next" || base == ".cache" {
				return filepath.SkipDir
			}
			return nil
		}

		total++
		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.Size() > maxFileSizeBytes {
			skippedLarge++
			normalized := strings.ReplaceAll(relPath, "\\", "/")
			skippedLargePaths = append(skippedLargePaths, normalized)
			if onProgress != nil {
				onProgress(total, total, normalized)
			}
			return nil
		}

		normalized := strings.ReplaceAll(relPath, "\\", "/")
		entries = append(entries, ScannedFile{
			Path: normalized,
			Size: info.Size(),
		})
		if onProgress != nil {
			onProgress(total, total, normalized)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking repository: %w", err)
	}

	// Log skipped large files
	if skippedLarge > 0 {
		isDefault := maxFileSizeBytes == DefaultMaxFileSizeBytes
		suffix := ""
		if isDefault {
			suffix = ", likely generated/vendored"
		}
		fmt.Printf("  Skipped %d large files (>%.0fKB%s)\n",
			skippedLarge, float64(maxFileSizeBytes)/1024, suffix)

		// Sort for stable preview
		sortedPaths := make([]string, len(skippedLargePaths))
		copy(sortedPaths, skippedLargePaths)
		for i := 0; i < len(sortedPaths)-1; i++ {
			for j := i + 1; j < len(sortedPaths); j++ {
				if sortedPaths[i] > sortedPaths[j] {
					sortedPaths[i], sortedPaths[j] = sortedPaths[j], sortedPaths[i]
				}
			}
		}

		showAll := len(sortedPaths) <= SkippedPreviewCap
		preview := sortedPaths
		if !showAll {
			preview = sortedPaths[:SkippedPreviewCap]
		}
		for _, p := range preview {
			fmt.Printf("  - %s\n", p)
		}
		if !showAll {
			remaining := len(sortedPaths) - SkippedPreviewCap
			fmt.Printf("  ...and %d more\n", remaining)
		}
		if isDefault {
			fmt.Printf("  Set GITNEXUS_MAX_FILE_SIZE=<KB> to include files above the default cap.\n")
		}
	}

	return entries, nil
}

// ─── ReadFileContents ────────────────────────────────────────

// ReadFileContents reads file contents for a specific set of relative paths.
// Returns a map for O(1) lookup. Silently skips files that fail to read.
func ReadFileContents(repoPath string, relativePaths []string) map[string]string {
	contents := make(map[string]string, len(relativePaths))

	for _, relPath := range relativePaths {
		fullPath := filepath.Join(repoPath, relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue // silently skip
		}
		contents[relPath] = string(data)
	}

	return contents
}

// ─── WalkRepository (legacy API) ────────────────────────────

// WalkRepository scans and reads everything into memory.
// Used by sequential fallback path only.
func WalkRepository(
	repoPath string,
	maxFileSizeBytes int64,
	onProgress func(current int, total int, filePath string),
) ([]FileEntry, error) {
	scanned, err := WalkRepositoryPaths(repoPath, maxFileSizeBytes, onProgress)
	if err != nil {
		return nil, err
	}

	paths := make([]string, len(scanned))
	for i, f := range scanned {
		paths[i] = f.Path
	}
	contents := ReadFileContents(repoPath, paths)

	var entries []FileEntry
	for _, f := range scanned {
		if content, ok := contents[f.Path]; ok {
			entries = append(entries, FileEntry{Path: f.Path, Content: content})
		}
	}
	return entries, nil
}