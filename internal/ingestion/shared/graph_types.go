// Package shared — CLI-specific graph types for the ingestion pipeline.
//
// Shared types (NodeLabel, Label constants, etc.) are defined in types.go.
// This file defines the full KnowledgeGraph interface (mirroring the TS
// core/graph/types.ts) that the pipeline uses for mutation and querying.
// It reuses the internal/graph package's Node and Edge structs for data
// interchange with the Pebble-backed GraphStore, so that ingestion can
// write directly into the same storage layer that the analysis queries use.

package shared

import (
	"github.com/mengshi02/codetrip/internal/graph"
)

// ─── Evidence ────────────────────────────────────────────────

// Evidence records a per-signal trace for edges emitted by the scope-based
// resolution pipeline.  Populated by emit-references when draining
// ReferenceIndex into the graph, so downstream query/audit tools can
// inspect *why* a given edge was emitted with its confidence value.
//
// Mirrors TS GraphRelationship.evidence (optional, additive).
type Evidence struct {
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
	Note   string  `json:"note,omitempty"`
}

// ─── KnowledgeGraph interface ────────────────────────────────

// KnowledgeGraph is the full CLI interface for building and querying the
// knowledge graph during ingestion.  Implementations may wrap an
// in-memory graph, the Pebble-backed GraphStore, or other storage.
//
// Mirrors TS core/graph/types.ts KnowledgeGraph (38 lines):
//   - nodes, relationships, iterNodes, iterRelationships
//   - iterRelationshipsByType (per-type index, hot path)
//   - forEachNode, forEachRelationship
//   - getNode, nodeCount, relationshipCount
//   - addNode, addRelationship
//   - removeNode, removeNodesByFile, removeRelationship
//
// Go adaptation:
//   - Uses graph.Node and graph.Edge instead of TS's GraphNode/GraphRelationship.
//   - Returns slices instead of IterableIterator (Go idiom).
//   - Evidence is carried on EdgeProps.Extra["evidence"] as []Evidence
//     (typed access via GetEdgeEvidence helper below).
type KnowledgeGraph interface {
	// ── Mutation ──

	// AddNode adds a node to the graph. If a node with the same ID
	// already exists, the implementation may silently ignore the duplicate.
	AddNode(node *graph.Node)

	// AddEdge adds a directed edge (relationship) between two nodes.
	AddEdge(edge *graph.Edge)

	// RemoveNode removes the node with the given ID and returns whether
	// the node existed.  Implementations should also remove all edges
	// referencing the removed node.
	RemoveNode(nodeID string) bool

	// RemoveNodesByFile removes all nodes whose FilePath matches the given
	// path and returns the count of removed nodes.  Implementations should
	// also remove all edges referencing any removed node.
	RemoveNodesByFile(filePath string) int

	// RemoveEdge removes the edge with the given ID and returns whether
	// the edge existed.
	RemoveEdge(edgeID string) bool

	// ── Lookup ──

	// GetNode returns the node with the given ID, or nil if not found.
	GetNode(id string) *graph.Node

	// NodeCount returns the total number of nodes in the graph.
	NodeCount() int

	// EdgeCount returns the total number of edges in the graph.
	EdgeCount() int

	// ── Iteration ──

	// ForEachNode calls fn for every node in the graph.
	ForEachNode(fn func(*graph.Node))

	// ForEachEdge calls fn for every edge in the graph.
	ForEachEdge(fn func(*graph.Edge))

	// EdgesByType returns all edges of the given relationship type,
	// backed by a per-type index maintained during mutation.
	// Prefer this over ForEachEdge + per-edge type filtering for
	// hot paths (MRO setup, heritage walks).
	EdgesByType(relType graph.RelType) []*graph.Edge
}

// ─── Helpers ──────────────────────────────────────────────────

// GetEdgeEvidence retrieves the Evidence slice from an Edge's Extra map.
// Returns nil if no evidence is stored.  This is the typed accessor for
// the TS-style evidence field that we encode into EdgeProps.Extra.
func GetEdgeEvidence(edge *graph.Edge) []Evidence {
	if edge == nil || edge.Props.Extra == nil {
		return nil
	}
	raw, ok := edge.Props.Extra["evidence"]
	if !ok {
		return nil
	}
	ev, ok := raw.([]Evidence)
	if ok {
		return ev
	}
	// Fallback: accept []any and convert item-by-item (JSON/msgpack deserialization)
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]Evidence, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		e := Evidence{}
		if v, ok := m["kind"].(string); ok {
			e.Kind = v
		}
		if v, ok := m["weight"].(float64); ok {
			e.Weight = v
		}
		if v, ok := m["note"].(string); ok {
			e.Note = v
		}
		result = append(result, e)
	}
	return result
}

// SetEdgeEvidence stores the Evidence slice into an Edge's Extra map.
func SetEdgeEvidence(edge *graph.Edge, evidence []Evidence) {
	if edge == nil {
		return
	}
	if edge.Props.Extra == nil {
		edge.Props.Extra = make(map[string]any)
	}
	edge.Props.Extra["evidence"] = evidence
}