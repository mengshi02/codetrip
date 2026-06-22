package phases

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// StructurePhase is the file/folder/Section node creation phase
type StructurePhase struct{}

func NewStructurePhase() *StructurePhase { return &StructurePhase{} }

func (p *StructurePhase) Name() string          { return "structure" }
func (p *StructurePhase) Dependencies() []string { return []string{"scan"} }

func (p *StructurePhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// Collect all file paths
	filePaths := make(map[string]bool)
	for _, f := range input.Files {
		filePaths[f.Path] = true
	}

	// Create folder nodes and file nodes
	folderCache := make(map[string]string) // path → nodeID

	// Ensure root folder exists
	_, err := p.ensureFolder(input, "", folderCache)
	if err != nil {
		return nil, err
	}
	nodesAdded++

	for _, f := range input.Files {
		// Ensure parent folder chain exists
		parentID, err := p.ensureFolderChain(input, f.Path, folderCache)
		if err != nil {
			continue
		}

		// Create file node
		fileNode := graph.NewNode(input.Repo, graph.LabelFile, filepath.Base(f.Path)).
			WithFile(f.Path).
			WithProp("language", f.Language).
			WithProp("contentHash", f.ContentHash).
			WithProp("lineCount", 0) // will be updated in parse phase
		if err := input.Graph.BufferNode(fileNode); err != nil {
			continue
		}
		nodesAdded++

		// CONTAINS edge: Folder → File
		edge := graph.NewEdge(graph.RelContains, parentID, fileNode.ID)
		if err := input.Graph.BufferEdge(edge); err != nil {
			continue
		}
		edgesAdded++

		f.NodeIDs = append(f.NodeIDs, fileNode.ID)
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
	}, nil
}

// ensureFolderChain ensures folder chain exists, returns direct parent folder ID
func (p *StructurePhase) ensureFolderChain(input *pipeline.PhaseInput, filePath string, cache map[string]string) (string, error) {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return p.ensureFolder(input, "", cache)
	}

	parts := strings.Split(dir, "/")
	var parentID string
	var err error

	currentPath := ""
	for _, part := range parts {
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}
		parentID, err = p.ensureFolder(input, currentPath, cache)
		if err != nil {
			return "", err
		}
	}

	return parentID, nil
}

// ensureFolder ensures folder node exists
func (p *StructurePhase) ensureFolder(input *pipeline.PhaseInput, path string, cache map[string]string) (string, error) {
	if id, ok := cache[path]; ok {
		return id, nil
	}

	name := filepath.Base(path)
	if name == "" || name == "." {
		name = filepath.Base(input.Config.RepoPath)
	}

	folderNode := graph.NewNode(input.Repo, graph.LabelFolder, name).WithFile(path)
	if err := input.Graph.BufferNode(folderNode); err != nil {
		return "", err
	}

	cache[path] = folderNode.ID
	return folderNode.ID, nil
}