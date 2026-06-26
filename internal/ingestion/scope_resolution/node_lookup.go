package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// GraphNodeLookup provides fast lookups from scope-resolution IDs to graph nodes.
// Mirrors TS scope-resolution/graph-bridge/node-lookup.ts.
//
// Two key functions:
//   - simpleKey — map a SymbolDefinition to a graph node via label + simple name
//   - qualifiedKey — map via label + qualified name (used when simple key is ambiguous)
//
// BuildGraphNodeLookup constructs the lookup indices by iterating the graph.
type GraphNodeLookup struct {
	// bySimple maps "Label:simpleName" → []*graph.Node
	bySimple map[string][]*graph.Node
	// byQualified maps "Label:qualifiedName" → []*graph.Node
	byQualified map[string][]*graph.Node
	// byFilePath maps filePath → []*graph.Node
	byFilePath map[string][]*graph.Node
	// byID maps nodeID → *graph.Node
	byID map[string]*graph.Node
}

// SimpleKey builds a simple lookup key from a label and a name.
func SimpleKey(label shared.NodeLabel, name string) string {
	return string(label) + ":" + name
}

// QualifiedKey builds a qualified lookup key from a label and a qualified name.
func QualifiedKey(label shared.NodeLabel, qualifiedName string) string {
	return string(label) + ":" + qualifiedName
}

// BuildGraphNodeLookup constructs lookup indices from the KnowledgeGraph.
// Iterates all graph nodes to populate 4 lookup indices: bySimple, byQualified,
// byFilePath, and byID. The resulting lookup is shared across all languages.
func BuildGraphNodeLookup(g shared.KnowledgeGraph) *GraphNodeLookup {
	lookup := &GraphNodeLookup{
		bySimple:    make(map[string][]*graph.Node),
		byQualified: make(map[string][]*graph.Node),
		byFilePath:  make(map[string][]*graph.Node),
		byID:        make(map[string]*graph.Node),
	}

	g.ForEachNode(func(n *graph.Node) {
		lookup.byID[n.ID] = n
		key := SimpleKey(shared.NodeLabel(n.Label), n.Name)
		lookup.bySimple[key] = append(lookup.bySimple[key], n)
		// QualifiedName is stored in Extra map
		if qn, _ := n.Props.Extra["qualifiedName"].(string); qn != "" {
			qKey := QualifiedKey(shared.NodeLabel(n.Label), qn)
			lookup.byQualified[qKey] = append(lookup.byQualified[qKey], n)
		}
		if n.FilePath != "" {
			lookup.byFilePath[n.FilePath] = append(lookup.byFilePath[n.FilePath], n)
		}
	})

	return lookup
}

// LookupBySimple returns nodes matching a label + simple name.
func (l *GraphNodeLookup) LookupBySimple(label shared.NodeLabel, name string) []*graph.Node {
	return l.bySimple[SimpleKey(label, name)]
}

// LookupByQualified returns nodes matching a label + qualified name.
func (l *GraphNodeLookup) LookupByQualified(label shared.NodeLabel, qualifiedName string) []*graph.Node {
	return l.byQualified[QualifiedKey(label, qualifiedName)]
}

// LookupByID returns the node with the given ID, or nil.
func (l *GraphNodeLookup) LookupByID(id string) *graph.Node {
	return l.byID[id]
}

// LookupByFilePath returns all nodes in the given file.
func (l *GraphNodeLookup) LookupByFilePath(filePath string) []*graph.Node {
	return l.byFilePath[filePath]
}
