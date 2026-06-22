package phases

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// CrossFilePhase handles cross-file import resolution (3-Tier confidence)
type CrossFilePhase struct{}

func NewCrossFilePhase() *CrossFilePhase { return &CrossFilePhase{} }

func (p *CrossFilePhase) Name() string          { return "crossFile" }
func (p *CrossFilePhase) Dependencies() []string { return []string{"parse" } }

func (p *CrossFilePhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// 1. Collect import declarations from all files
	// Build name → nodeID index (cross-file lookup)
	nameIndex := make(map[string][]string) // symbolName → []nodeID
	fileIndex := make(map[string]string)    // filePath → fileNodeID

	for _, f := range input.Files {
		if len(f.NodeIDs) > 0 {
			fileIndex[f.Path] = f.NodeIDs[0]
		}
		for _, sym := range f.Symbols {
			if sym.NodeID != "" {
				nameIndex[sym.Name] = append(nameIndex[sym.Name], sym.NodeID)
			}
		}
	}

	// 2. Resolve cross-file import relationships
	for _, f := range input.Files {
		for _, imp := range f.Imports {
			// 3-Tier confidence
			confidence := 0.5 // Tier 3: Global (default)

			// Match import path to file
			importPath := imp.Path
			var targetIDs []string

			// Check if import path matches repository files
			for otherPath, otherFileID := range fileIndex {
				if isImportMatch(importPath, otherPath, f.Path) {
					// Tier 2: Via explicit import reference (0.9)
					confidence = 0.9
					targetIDs = append(targetIDs, otherFileID)
				}
			}

			// If no file matched, try symbol name matching (Tier 3)
			if len(targetIDs) == 0 {
				// Go: import path last segment might be package name
				pkgName := lastSegment(importPath)
				if ids, ok := nameIndex[pkgName]; ok {
					targetIDs = ids
				}
			}

			// Create IMPORTS edge
			if len(f.NodeIDs) > 0 {
				for _, tid := range targetIDs {
					edge := (&graph.Edge{
						Type:   graph.RelImports,
						Source: f.NodeIDs[0],
						Target: tid,
					}).WithProp("confidence", confidence).
						WithProp("importPath", importPath).
						WithProp("alias", imp.Alias)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}
			}
		}

		// Tier 1: Same-file reference (0.95)
		// DEFINES edges within same file were created in parse phase
		// Here we add CALLS edges within same file (if symbol references other symbols in same file)
		if len(f.Symbols) > 1 && len(f.NodeIDs) > 0 {
			for i, sym := range f.Symbols {
				// Find other symbols with matching names in same file
				for j, other := range f.Symbols {
					if i != j && other.NodeID != "" && sym.NodeID != "" {
						// If method receiver is other.Name or sym references other
						if sym.Props != nil {
							if recv, ok := sym.Props["receiver"]; ok {
								if recv == other.Name {
									edge := (&graph.Edge{
										Type:   graph.RelCalls,
										Source: sym.NodeID,
										Target: other.NodeID,
									}).WithProp("confidence", 0.95).
										WithProp("sameFile", true)
									if err := input.Graph.BufferEdge(edge); err == nil {
										edgesAdded++
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded:   nodesAdded,
		EdgesAdded:   edgesAdded,
		Files:        input.Files,
		FilesUpdated: true,
	}, nil
}

// isImportMatch checks if import path matches file path
func isImportMatch(importPath, filePath, sourceFile string) bool {
	// Go: import "github.com/mengshi02/codetrip/internal/graph" → pkg/graph/store.go
	// Simplified matching: last few segments of import path match file path prefix

	// Standard library cannot match
	if !strings.Contains(importPath, "/") {
		return false
	}

	// Check if import path is a parent path of file path
	// e.g., importPath="pkg/graph" matches filePath="pkg/graph/store.go"
	importParts := strings.Split(importPath, "/")
	fileParts := strings.Split(strings.TrimSuffix(filePath, "/"), "/")

	// Match last N segments
	minLen := min(len(importParts), len(fileParts))
	for i := 0; i < minLen; i++ {
		ip := importParts[len(importParts)-1-i]
		fp := fileParts[len(fileParts)-1-i]
		if ip != fp {
			return false
		}
	}
	return true
}

// lastSegment gets the last segment of a path
func lastSegment(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}