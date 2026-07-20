package ingest

import (
	"path/filepath"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
)

// ProcessStructure creates File and Folder nodes and CONTAINS relationships
// from the walked file entries.
func ProcessStructure(g *graph.KnowledgeGraph, walkResult *WalkResult) {
	seenFolders := make(map[string]bool)

	for _, entry := range walkResult.Files {
		// Create File node
		fileID := graph.GenerateID("File", entry.RelativePath)
		fileNode := &graph.GraphNode{
			ID:    fileID,
			Label: graph.LabelFile,
			Properties: graph.NodeProperties{
				Name:       filepath.Base(entry.RelativePath),
				FilePath:   entry.RelativePath,
				FileSize:   entry.Size,
				LanguageID: entry.LanguageID,
			},
		}
		g.AddNode(fileNode)

		// Create Folder nodes and CONTAINS relationships along the path
		processFolderPath(g, entry.RelativePath, fileID, seenFolders)
	}
}

// processFolderPath creates Folder nodes and CONTAINS relationships for a file path.
// For a path like "src/core/graph/types.ts", it creates:
//   - Folder:misc (if path starts with misc/)
//   - Folder:src, Folder:src/core, Folder:src/core/graph
//   - CONTAINS(Folder:src, Folder:src/core)
//   - CONTAINS(Folder:src/core, Folder:src/core/graph)
//   - CONTAINS(Folder:src/core/graph, File:src/core/graph/types.ts)
func processFolderPath(g *graph.KnowledgeGraph, filePath string, fileID string, seenFolders map[string]bool) {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return
	}

	parts := splitPath(dir)
	var parentFolderID string

	for i := 0; i < len(parts); i++ {
		folderPath := strings.Join(parts[:i+1], "/")
		folderID := graph.GenerateID("Folder", folderPath)

		if !seenFolders[folderPath] {
			seenFolders[folderPath] = true
			folderNode := &graph.GraphNode{
				ID:    folderID,
				Label: graph.LabelFolder,
				Properties: graph.NodeProperties{
					Name:     parts[i],
					FilePath: folderPath,
				},
			}
			g.AddNode(folderNode)
		}

		// CONTAINS(ParentFolder, ChildFolder)
		if parentFolderID != "" {
			relID := graph.GenerateID("CONTAINS", parentFolderID+"->"+folderID)
			rel := &graph.GraphRelationship{
				ID:         relID,
				SourceID:   parentFolderID,
				TargetID:   folderID,
				Type:       graph.RelCONTAINS,
				Confidence: 1.0,
				Reason:     "",
			}
			g.AddRelationship(rel)
		}

		parentFolderID = folderID
	}

	// CONTAINS(DirectParentFolder, File)
	if parentFolderID != "" {
		relID := graph.GenerateID("CONTAINS", parentFolderID+"->"+fileID)
		rel := &graph.GraphRelationship{
			ID:         relID,
			SourceID:   parentFolderID,
			TargetID:   fileID,
			Type:       graph.RelCONTAINS,
			Confidence: 1.0,
			Reason:     "",
		}
		g.AddRelationship(rel)
	}
}

// splitPath splits a path into its components, handling both / and \ separators.
func splitPath(p string) []string {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.Split(p, "/")
}
