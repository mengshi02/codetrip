package model

import (
	"sort"
	"sync"
)

// KnowledgeGraph is an in-memory graph storing nodes and relationships.
type KnowledgeGraph struct {
	mu              sync.RWMutex
	nodeMap         map[string]*GraphNode
	relationshipMap map[string]*GraphRelationship
}

// NewKnowledgeGraph creates a new empty KnowledgeGraph.
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		nodeMap:         make(map[string]*GraphNode),
		relationshipMap: make(map[string]*GraphRelationship),
	}
}

// AddNode adds a node to the graph. If a node with the same ID already exists, it is not replaced.
func (g *KnowledgeGraph) AddNode(node *GraphNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.nodeMap[node.ID]; !exists {
		g.nodeMap[node.ID] = node
	}
}

// AddNodeMergingRange preserves the first node payload while expanding its
// source range. It is used for semantic nodes that intentionally collapse
// several physical declarations, such as C++ constructor overload families.
func (g *KnowledgeGraph) AddNodeMergingRange(node *GraphNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	existing, exists := g.nodeMap[node.ID]
	if !exists {
		g.nodeMap[node.ID] = node
		return
	}
	if node.Properties.StartLine != nil && (existing.Properties.StartLine == nil || *node.Properties.StartLine < *existing.Properties.StartLine) {
		value := *node.Properties.StartLine
		existing.Properties.StartLine = &value
	}
	if node.Properties.EndLine != nil && (existing.Properties.EndLine == nil || *node.Properties.EndLine > *existing.Properties.EndLine) {
		value := *node.Properties.EndLine
		existing.Properties.EndLine = &value
	}
}

// AddRelationship adds a relationship to the graph. If a relationship with the same ID already exists, it is not replaced.
func (g *KnowledgeGraph) AddRelationship(rel *GraphRelationship) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.relationshipMap[rel.ID]; !exists {
		g.relationshipMap[rel.ID] = rel
	}
}

// GetNode returns a node by ID.
func (g *KnowledgeGraph) GetNode(id string) (*GraphNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodeMap[id]
	return node, ok
}

// NodeCount returns the number of nodes in the graph.
func (g *KnowledgeGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodeMap)
}

// RelationshipCount returns the number of relationships in the graph.
func (g *KnowledgeGraph) RelationshipCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.relationshipMap)
}

// ForEachNode iterates over all nodes in the graph in sorted order by ID.
func (g *KnowledgeGraph) ForEachNode(fn func(*GraphNode)) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	nodes := make([]*GraphNode, 0, len(g.nodeMap))
	for _, node := range g.nodeMap {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	for _, node := range nodes {
		fn(node)
	}
}

// ForEachRelationship iterates over all relationships in the graph in sorted order.
// Sorting by (source, target, type) ensures deterministic CSV output.
func (g *KnowledgeGraph) ForEachRelationship(fn func(*GraphRelationship)) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	rels := make([]*GraphRelationship, 0, len(g.relationshipMap))
	for _, rel := range g.relationshipMap {
		rels = append(rels, rel)
	}
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].SourceID != rels[j].SourceID {
			return rels[i].SourceID < rels[j].SourceID
		}
		if rels[i].TargetID != rels[j].TargetID {
			return rels[i].TargetID < rels[j].TargetID
		}
		return string(rels[i].Type) < string(rels[j].Type)
	})
	for _, rel := range rels {
		fn(rel)
	}
}

// Nodes returns a slice of all nodes.
func (g *KnowledgeGraph) Nodes() []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*GraphNode, 0, len(g.nodeMap))
	for _, node := range g.nodeMap {
		result = append(result, node)
	}
	return result
}

// Relationships returns a slice of all relationships.
func (g *KnowledgeGraph) Relationships() []*GraphRelationship {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*GraphRelationship, 0, len(g.relationshipMap))
	for _, rel := range g.relationshipMap {
		result = append(result, rel)
	}
	return result
}

// RemoveNode removes a node and all relationships involving it.
func (g *KnowledgeGraph) RemoveNode(nodeID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.nodeMap[nodeID]; !exists {
		return false
	}
	delete(g.nodeMap, nodeID)
	for id, rel := range g.relationshipMap {
		if rel.SourceID == nodeID || rel.TargetID == nodeID {
			delete(g.relationshipMap, id)
		}
	}
	return true
}

// RemoveNodesByFile removes all nodes (and their relationships) belonging to a file path.
func (g *KnowledgeGraph) RemoveNodesByFile(filePath string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	removed := 0
	for id, node := range g.nodeMap {
		if node.Properties.FilePath == filePath {
			delete(g.nodeMap, id)
			removed++
		}
	}
	// Remove orphaned relationships
	for id, rel := range g.relationshipMap {
		if _, ok := g.nodeMap[rel.SourceID]; !ok {
			delete(g.relationshipMap, id)
		} else if _, ok := g.nodeMap[rel.TargetID]; !ok {
			delete(g.relationshipMap, id)
		}
	}
	return removed
}

// GenerateID creates a node/relationship ID in the format "Label:name".
func GenerateID(label string, name string) string {
	return label + ":" + name
}
