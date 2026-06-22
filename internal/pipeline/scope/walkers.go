package scope

import (
	"fmt"
	"log/slog"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ScopeWalker scope tree walker
// Traverses file scope hierarchy, collecting symbol bindings and import information
type ScopeWalker struct {
	graph *graph.GraphStore
	repo  string
}

// NewScopeWalker creates a scope walker
func NewScopeWalker(gs *graph.GraphStore, repo string) *ScopeWalker {
	return &ScopeWalker{graph: gs, repo: repo}
}

// WalkResult walk result
type WalkResult struct {
	// FileScopes file → symbol bindings
	FileScopes map[string]*FileScope
	// ImportGraph import relation graph
	ImportGraph map[string][]string // sourceFile → []targetFile
}

// FileScope file scope
type FileScope struct {
	FilePath    string
	Definitions []SymbolBinding // symbols defined in this file
	Imports     []ImportBinding // symbols imported by this file
	References  []SymbolRef     // symbols referenced by this file
}

// SymbolBinding symbol binding
type SymbolBinding struct {
	Name      string
	NodeID    string
	Label     graph.Label
	Receiver  string // method receiver (for methods only)
	IsStatic  bool
	IsExported bool
}

// ImportBinding import binding
type ImportBinding struct {
	ImportPath string
	Alias      string
	Symbols    []string // explicitly imported symbol names
	IsWildcard bool
	SourceNodeID string
}

// SymbolRef symbol reference
type SymbolRef struct {
	Name       string
	Receiver   string
	FilePath   string
	Line       int
	CallerID   string // caller node ID
}

// WalkAll walks all file scopes
func (w *ScopeWalker) WalkAll(files []*pipeline.ParsedFile) *WalkResult {
	result := &WalkResult{
		FileScopes: make(map[string]*FileScope),
		ImportGraph: make(map[string][]string),
	}

	for _, pf := range files {
		fs := &FileScope{FilePath: pf.Path}

		// Collect definitions
		for _, sym := range pf.Symbols {
			fs.Definitions = append(fs.Definitions, SymbolBinding{
				Name:       sym.Name,
				NodeID:     sym.NodeID,
				Label:      sym.Label,
				IsExported: isExported(sym.Name, sym.Label),
			})
		}

		// Collect imports
		for _, imp := range pf.Imports {
			binding := ImportBinding{
				ImportPath:   imp.Path,
				Alias:        imp.Alias,
				Symbols:      imp.Symbols,
				IsWildcard:   imp.IsWildcard,
			}
			fs.Imports = append(fs.Imports, binding)

			// Build import graph
			if imp.Path != "" {
				w.resolveImportPath(pf.Path, imp.Path, result.ImportGraph)
			}
		}

		// Collect references (call sites)
		for _, cs := range pf.CallSites {
			fs.References = append(fs.References, SymbolRef{
				Name:     cs.Name,
				Receiver: cs.Receiver,
				FilePath: cs.FilePath,
				Line:     cs.Line,
				CallerID: cs.CallerID,
			})
		}

		result.FileScopes[pf.Path] = fs
	}

	return result
}

// WalkFile walks a single file scope
func (w *ScopeWalker) WalkFile(pf *pipeline.ParsedFile) *FileScope {
	fs := &FileScope{FilePath: pf.Path}

	for _, sym := range pf.Symbols {
		fs.Definitions = append(fs.Definitions, SymbolBinding{
			Name:       sym.Name,
			NodeID:     sym.NodeID,
			Label:      sym.Label,
			IsExported: isExported(sym.Name, sym.Label),
		})
	}

	for _, imp := range pf.Imports {
		fs.Imports = append(fs.Imports, ImportBinding{
			ImportPath: imp.Path,
			Alias:      imp.Alias,
			Symbols:    imp.Symbols,
			IsWildcard: imp.IsWildcard,
		})
	}

	for _, cs := range pf.CallSites {
		fs.References = append(fs.References, SymbolRef{
			Name:     cs.Name,
			Receiver: cs.Receiver,
			FilePath: cs.FilePath,
			Line:     cs.Line,
			CallerID: cs.CallerID,
		})
	}

	return fs
}

// WalkClassHierarchy walks class inheritance hierarchy
// Starting from specified class, BFS traverses EMBRACES/EXTENDS/IMPLEMENTS edges
func (w *ScopeWalker) WalkClassHierarchy(className string, maxDepth int) []*graph.Node {
	seedNodes, err := w.graph.GetNodesByName(w.repo, className)
	if err != nil || len(seedNodes) == 0 {
		return nil
	}

	visited := make(map[string]bool)
	var result []*graph.Node
	queue := []struct {
		node  *graph.Node
		depth int
	}{}

	for _, n := range seedNodes {
		queue = append(queue, struct {
			node  *graph.Node
			depth int
		}{n, 0})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if visited[item.node.ID] {
			continue
		}
		if maxDepth > 0 && item.depth > maxDepth {
			continue
		}

		visited[item.node.ID] = true
		result = append(result, item.node)

		// Traverse inheritance/implementation edges
		outEdges, err := w.graph.GetAllOutEdges(item.node.ID)
		if err != nil {
			slog.Warn("scope_walker: failed to get out-edges", "node_id", item.node.ID, "error", err)
		}
		for _, edge := range outEdges {
			if edge.Type == graph.RelEmbraces || edge.Type == graph.RelExtends || edge.Type == graph.RelImplements {
				target, err := w.graph.GetNode(edge.Target)
				if err == nil && !visited[target.ID] {
					queue = append(queue, struct {
						node  *graph.Node
						depth int
					}{target, item.depth + 1})
				}
			}
		}
	}

	return result
}

// resolveImportPath resolves import path to file path
func (w *ScopeWalker) resolveImportPath(sourceFile, importPath string, importGraph map[string][]string) {
	// Simplified implementation: directly add import path to import graph
	importGraph[sourceFile] = append(importGraph[sourceFile], importPath)
}

// isExported determines if symbol is exported
func isExported(name string, label graph.Label) bool {
	// Go: uppercase letter prefix means exported
	if len(name) == 0 {
		return false
	}
	// General rule: non-underscore prefix
	if name[0] == '_' {
		return false
	}
	// Uppercase letter prefix considered exported (Go/Java/C# convention)
	return name[0] >= 'A' && name[0] <= 'Z'
}

// WalkCallChain walks call chain
// Starting from specified node, BFS traverses CALLS edges
func (w *ScopeWalker) WalkCallChain(startNodeID string, direction Direction, maxDepth int) []*graph.Node {
	visited := make(map[string]bool)
	var result []*graph.Node
	queue := []struct {
		nodeID string
		depth  int
	}{{startNodeID, 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if visited[item.nodeID] {
			continue
		}
		if maxDepth > 0 && item.depth > maxDepth {
			continue
		}

		visited[item.nodeID] = true
		node, err := w.graph.GetNode(item.nodeID)
		if err != nil {
			continue
		}
		result = append(result, node)

		var edges []*graph.Edge
		if direction == DirectionDownstream {
			edges, err = w.graph.GetAllOutEdges(item.nodeID)
		} else {
			edges, err = w.graph.GetAllInEdges(item.nodeID)
		}
		if err != nil {
			slog.Warn("scope traverse: get edges failed", "nodeID", item.nodeID, "direction", direction, "error", err)
			continue
		}

		for _, edge := range edges {
			if edge.Type == graph.RelCalls {
				nextID := edge.Target
				if direction == DirectionUpstream {
					nextID = edge.Source
				}
				if !visited[nextID] {
					queue = append(queue, struct {
						nodeID string
						depth  int
					}{nextID, item.depth + 1})
				}
			}
		}
	}

	return result
}

// Direction walk direction
type Direction int

const (
	// DirectionDownstream walk downstream (caller → callee)
	DirectionDownstream Direction = iota
	// DirectionUpstream walk upstream (callee → caller)
	DirectionUpstream
)

// String formats direction
func (d Direction) String() string {
	switch d {
	case DirectionDownstream:
		return "downstream"
	case DirectionUpstream:
		return "upstream"
	default:
		return fmt.Sprintf("unknown(%d)", d)
	}
}