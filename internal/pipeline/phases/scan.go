package phases

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/mengshi02/codetrip/internal/util"
)

// ScanPhase is the file system scan phase
type ScanPhase struct{}

func NewScanPhase() *ScanPhase { return &ScanPhase{} }

func (p *ScanPhase) Name() string          { return "scan" }
func (p *ScanPhase) Dependencies() []string { return nil }

func (p *ScanPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	var fileCount atomic.Int64
	var totalSize atomic.Int64
	var files []*pipeline.FileInfo

	root := input.Config.RepoPath

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			// Skip hidden directories and common ignore directories
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "__pycache__" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Detect language
		lang := DetectLanguage(path)
		if lang == "" {
			return nil // skip unsupported files
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)

		// Read file content and compute hash
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fi := &pipeline.FileInfo{
			Path:     relPath,
			Language: lang,
			Size:     info.Size(),
			Hash:     util.ContentHash(content),
		}

		files = append(files, fi)
		fileCount.Add(1)
		totalSize.Add(info.Size())

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create project root node
	projectNode := graph.NewNode(input.Repo, graph.LabelProject, filepath.Base(root)).
		WithProp("fileCount", int(fileCount.Load())).
		WithProp("totalSize", int(totalSize.Load()))
	if err := input.Graph.BufferNode(projectNode); err != nil {
		return nil, err
	}

	// Convert FileInfo to ParsedFile and pass to downstream
	parsedFiles := make([]*pipeline.ParsedFile, 0, len(files))
	for _, fi := range files {
		parsedFiles = append(parsedFiles, &pipeline.ParsedFile{
			Path:        fi.Path,
			Language:    fi.Language,
			ContentHash: fi.Hash,
			Size:        fi.Size,
		})
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded:   1, // Project node
		Files:        parsedFiles,
		FilesUpdated: true,
		Stats: map[string]any{
			"fileCount": int(fileCount.Load()),
			"totalSize": int(totalSize.Load()),
		},
	}, nil
}

// DetectLanguage detects language based on file extension.
// Exported for use by Trip layer incremental re-indexing.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c", ".h":
		return "c"
	case ".md", ".mdx":
		return "markdown"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".vue":
		return "vue"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".dart":
		return "dart"
	default:
		return ""
	}
}