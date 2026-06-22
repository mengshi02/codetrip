package bridge

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// IDResolver graph ID resolver
// Resolves symbol names/paths/receivers to graph node IDs
type IDResolver struct {
	graph *graph.GraphStore
	repo  string
}

// NewIDResolver creates an ID resolver
func NewIDResolver(gs *graph.GraphStore, repo string) *IDResolver {
	return &IDResolver{graph: gs, repo: repo}
}

// ResolveByName resolves node IDs by name
// Returns all node IDs matching the name
func (r *IDResolver) ResolveByName(name string) []string {
	nodes, err := r.graph.GetNodesByName(r.repo, name)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

// ResolveByNameAndLabel resolves node IDs by name and label
func (r *IDResolver) ResolveByNameAndLabel(name string, label graph.Label) []string {
	nodes, err := r.graph.GetNodesByName(r.repo, name)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Label == label {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// ResolveByFile resolves node IDs by file path
func (r *IDResolver) ResolveByFile(filePath string) []string {
	nodes, err := r.graph.GetNodesByFile(r.repo, filePath)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

// ResolveMethod resolves method node IDs by receiver and method name
// receiver: receiver type name, methodName: method name
func (r *IDResolver) ResolveMethod(receiver, methodName string) []string {
	nodes, err := r.graph.GetNodesByName(r.repo, methodName)
	if err != nil {
		return nil
	}
	var ids []string
	for _, n := range nodes {
		if n.Label == graph.LabelMethod {
			recv := n.GetPropString("receiver")
			if recv == receiver || recv == "" {
				ids = append(ids, n.ID)
			}
		}
	}
	return ids
}

// ResolveImport resolves target node IDs by import path
// importPath: import path (e.g., "fmt", "github.com/...")
func (r *IDResolver) ResolveImport(importPath string) []string {
	nodes, err := r.graph.GetNodesByLabel(r.repo, string(graph.LabelImport))
	if err != nil {
		return nil
	}
	var ids []string
	for _, n := range nodes {
		path := n.GetPropString("path")
		if path == "" {
			path = n.Name
		}
		if path == importPath || strings.HasSuffix(path, importPath) {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// ResolveQualified resolves node IDs by qualified name
// qualifiedName: e.g., "pkg.FuncName", "Type.MethodName"
func (r *IDResolver) ResolveQualified(qualifiedName string) []string {
	parts := strings.SplitN(qualifiedName, ".", 2)
	if len(parts) == 2 {
		return r.ResolveMethod(parts[0], parts[1])
	}
	return r.ResolveByName(qualifiedName)
}

// ResolveNodeByID gets node by ID
func (r *IDResolver) ResolveNodeByID(nodeID string) (*graph.Node, error) {
	return r.graph.GetNode(nodeID)
}

// MakeNodeID generates node ID (deterministic)
func MakeNodeID(repo, label, name, filePath string, line int) string {
	return fmt.Sprintf("%s:%s:%s:%s:%d", repo, label, name, filePath, line)
}