package operators

import (
	"log/slog"

	"github.com/mengshi02/codetrip/internal/graph"
)

// ExpandIterator traverses relationships from nodes in the input rows.
// It implements the graph expansion (relationship traversal) part of a
// MATCH pattern, including variable-length path expansion.
//
// Optimization: target nodes are resolved in batch after edge filtering,
// and a node cache avoids redundant GetNode calls when the same node
// appears as the target of multiple edges.
type ExpandIterator struct {
	child Iterator
	rel   *RelationshipPattern
	gs    GraphStore
	ev    *Evaluator

	// current state
	curRow   Row
	curEdges []*graph.Edge
	edgeIdx  int
	closed   bool

	// node cache: nodeID → *graph.Node, avoids repeated GetNode calls
	nodeCache map[string]*graph.Node
}

// NewExpandIterator creates an ExpandIterator for the given relationship pattern.
func NewExpandIterator(child Iterator, rel *RelationshipPattern, gs GraphStore, ev *Evaluator) *ExpandIterator {
	return &ExpandIterator{
		child:     child,
		rel:       rel,
		gs:        gs,
		ev:        ev,
		nodeCache: make(map[string]*graph.Node),
	}
}

// getCachedNode retrieves a node, using the cache to avoid redundant lookups.
func (ex *ExpandIterator) getCachedNode(id string) (*graph.Node, error) {
	if n, ok := ex.nodeCache[id]; ok {
		return n, nil
	}
	n, err := ex.gs.GetNode(id)
	if err != nil || n == nil {
		return nil, err
	}
	ex.nodeCache[id] = n
	return n, nil
}

// Next returns the next row with the relationship and target node added.
func (ex *ExpandIterator) Next() (Row, error) {
	if ex.closed {
		return nil, nil
	}

	for {
		// If we still have edges to process from the current row
		if ex.edgeIdx < len(ex.curEdges) {
			edge := ex.curEdges[ex.edgeIdx]
			ex.edgeIdx++

			newRow := CopyRow(ex.curRow)
			if ex.rel.Variable != "" {
				newRow[ex.rel.Variable] = edge
			}

			// Resolve target node
			targetID := edge.Target
			if ex.rel.Direction == DirIn {
				targetID = edge.Source
			}
			targetNode, err := ex.getCachedNode(targetID)
			if err != nil || targetNode == nil {
				continue
			}

			// Assign target node variable
			if ex.rel.Target != nil && ex.rel.Target.Variable != "" {
				newRow[ex.rel.Target.Variable] = targetNode
			}

			// Filter target node by labels
			if ex.rel.Target != nil && len(ex.rel.Target.Labels) > 0 {
				labelMatch := false
				for _, label := range ex.rel.Target.Labels {
					if string(targetNode.Label) == label {
						labelMatch = true
						break
					}
				}
				if !labelMatch {
					continue
				}
			}

			return newRow, nil
		}

		// Get next input row
		row, err := ex.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}

		// Find source node
		sourceNode, ok := FindSourceNode(row)
		if !ok {
			continue
		}

		// Fetch edges
		var edges []*graph.Edge
		var edgeErr error
		switch ex.rel.Direction {
		case DirOut:
			edges, edgeErr = ex.gs.GetAllOutEdges(sourceNode.ID)
		case DirIn:
			edges, edgeErr = ex.gs.GetAllInEdges(sourceNode.ID)
		case DirBoth:
			var outE, inE []*graph.Edge
			outE, edgeErr = ex.gs.GetAllOutEdges(sourceNode.ID)
			if edgeErr != nil {
				slog.Warn("expand: get out-edges failed", "nodeID", sourceNode.ID, "error", edgeErr)
			}
			inE, edgeErr = ex.gs.GetAllInEdges(sourceNode.ID)
			if edgeErr != nil {
				slog.Warn("expand: get in-edges failed", "nodeID", sourceNode.ID, "error", edgeErr)
			}
			edges = append(outE, inE...)
		}
		if edgeErr != nil {
			slog.Warn("expand: get edges failed", "nodeID", sourceNode.ID, "direction", ex.rel.Direction, "error", edgeErr)
		}

		// Filter by relationship types
		if len(ex.rel.RelTypes) > 0 {
			typeSet := make(map[string]bool, len(ex.rel.RelTypes))
			for _, rt := range ex.rel.RelTypes {
				typeSet[rt] = true
			}
			filtered := make([]*graph.Edge, 0, len(edges))
			for _, edge := range edges {
				if typeSet[string(edge.Type)] {
					filtered = append(filtered, edge)
				}
			}
			edges = filtered
		}

		// Variable-length path expansion
		if ex.rel.MinHops != nil && ex.rel.MaxHops != nil && *ex.rel.MaxHops > 1 {
			edges = expandVariableLength(ex.gs, sourceNode.ID, ex.rel, edges)
		}

		ex.curRow = row
		ex.curEdges = edges
		ex.edgeIdx = 0
	}
}

// Close releases resources held by the iterator and its child.
func (ex *ExpandIterator) Close() error {
	ex.closed = true
	ex.nodeCache = nil
	return ex.child.Close()
}

// expandVariableLength performs BFS-based variable-length path expansion.
func expandVariableLength(gs GraphStore, sourceID string, rel *RelationshipPattern, directEdges []*graph.Edge) []*graph.Edge {
	minHops := *rel.MinHops
	maxHops := *rel.MaxHops

	var allPaths []*graph.Edge
	visited := make(map[string]bool)
	visited[sourceID] = true

	type hopState struct {
		nodeID string
		depth  int
	}

	queue := []hopState{{nodeID: sourceID, depth: 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxHops {
			continue
		}

		var edges []*graph.Edge
		var edgeErr error
		switch rel.Direction {
		case DirOut:
			edges, edgeErr = gs.GetAllOutEdges(cur.nodeID)
		case DirIn:
			edges, edgeErr = gs.GetAllInEdges(cur.nodeID)
		case DirBoth:
			var outE, inE []*graph.Edge
			outE, edgeErr = gs.GetAllOutEdges(cur.nodeID)
			inE, edgeErr = gs.GetAllInEdges(cur.nodeID)
			edges = append(outE, inE...)
		}
		if edgeErr != nil {
			slog.Warn("expand variable-length: get edges failed", "nodeID", cur.nodeID, "error", edgeErr)
			continue
		}

		for _, edge := range edges {
			nextID := edge.Target
			if rel.Direction == DirIn {
				nextID = edge.Source
			}
			if visited[nextID] {
				continue
			}

			if cur.depth+1 >= minHops {
				allPaths = append(allPaths, edge)
			}

			visited[nextID] = true
			queue = append(queue, hopState{nodeID: nextID, depth: cur.depth + 1})
		}
	}

	return allPaths
}