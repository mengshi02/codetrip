package core

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ProcessStructure builds the file/folder graph from a list of file paths.
// For each path, it splits by "/" and creates Folder nodes for every directory
// segment and a File node for the final segment, with CONTAINS relationships
// linking parent → child segments.
//
// This mirrors the TS implementation's behavior:
//   - Each intermediate segment → Folder node (label="Folder", name=segment)
//   - Final segment → File node (label="File", name=segment)
//   - Parent folder CONTAINS child folder/file
//   - Node IDs use GenerateID format: "${label}:${name}"
//
// The graph parameter is the full shared.KnowledgeGraph interface (unified
// from the former core.KnowledgeGraph). Only the AddNode and AddEdge mutation
// methods are used in this function; the rest of the interface is available
// for later pipeline phases.
func ProcessStructure(graphObj shared.KnowledgeGraph, paths []string) {
	for _, path := range paths {
		segments := splitPath(path)
		if len(segments) == 0 {
			continue
		}

		// Build folder nodes for intermediate segments
		var parentID string
		for i := 0; i < len(segments)-1; i++ {
			seg := segments[i]
			nodeID := shared.GenerateID("Folder", seg)

			graphObj.AddNode(&graph.Node{
				ID:    nodeID,
				Label: graph.LabelFolder,
				Name:  seg,
			})

			if parentID != "" {
				edgeID := shared.GenerateID("CONTAINS", parentID+"->"+nodeID)
				graphObj.AddEdge(&graph.Edge{
					ID:     edgeID,
					Type:   graph.RelContains,
					Source: parentID,
					Target: nodeID,
				})
			}
			parentID = nodeID
		}

		// Build file node for the final segment
		fileSeg := segments[len(segments)-1]
		fileID := shared.GenerateID("File", fileSeg)

		graphObj.AddNode(&graph.Node{
			ID:    fileID,
			Label: graph.LabelFile,
			Name:  fileSeg,
		})

		if parentID != "" {
			edgeID := shared.GenerateID("CONTAINS", parentID+"->"+fileID)
			graphObj.AddEdge(&graph.Edge{
				ID:     edgeID,
				Type:   graph.RelContains,
				Source: parentID,
				Target: fileID,
			})
		}
	}
}

// splitPath splits a file path into non-empty segments.
// Handles both Unix (/) and Windows (\\) separators.
func splitPath(path string) []string {
	// Replace Windows backslashes with forward slashes
	normalized := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(normalized, "/")
	// Filter out empty segments (e.g. from leading "/" or double "/")
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}