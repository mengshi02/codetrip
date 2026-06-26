package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// Edge emission helpers for the scope-resolution engine.
// Mirrors TS scope-resolution/graph-bridge/edges.ts.

// MapReferenceKindToEdgeType translates a scope-resolution ReferenceKind
// into the corresponding graph edge type.
// Mirrors TS mapReferenceKindToEdgeType.
// "import-use" is dropped — provenance lives on the IMPORTS edge.
func MapReferenceKindToEdgeType(kind shared.ReferenceKind) graph.RelType {
	switch kind {
	case shared.ReferenceCall:
		return graph.RelCalls
	case shared.ReferenceRead, shared.ReferenceWrite:
		return graph.RelAccesses
	case shared.ReferenceInherits:
		return graph.RelExtends
	case shared.ReferenceTypeReference, shared.ReferenceMacro:
		return graph.RelUses
	case shared.ReferenceImportUse:
		return "" // no edge type for import-use
	default:
		return ""
	}
}

// TryEmitEdge resolves caller + target to graph IDs and emits the edge.
// Returns true if the edge was emitted (not deduped, not skipped).
//
// Dedup key format:
//   - Normal:  (edgeType):(callerGraphId)->(targetGraphId):(line):(col)
//   - Collapsed (CALLS only when collapseByCallerTarget): (edgeType):(callerGraphId)->(targetGraphId)
//
// Mirrors TS tryEmitEdge.
func TryEmitEdge(
	g shared.KnowledgeGraph,
	indexes *model.ScopeResolutionIndexes,
	lookup *GraphNodeLookup,
	site SiteInfo,
	targetDef *shared.SymbolDefinition,
	reason string,
	seen map[string]bool,
	confidence float64,
	collapseByCallerTarget bool,
) bool {
	callerGraphId := ResolveCallerGraphID(site.InScope, indexes, lookup, &site.AtRange)
	targetGraphId := ResolveDefGraphID(targetDef.FilePath, targetDef, lookup)
	edgeType := MapReferenceKindToEdgeType(shared.ReferenceKind(site.Kind))

	if callerGraphId == "" || targetGraphId == "" || edgeType == "" {
		return false
	}

	useCollapsed := collapseByCallerTarget && edgeType == graph.RelCalls
	dedupKey := ""
	if useCollapsed {
		dedupKey = fmt.Sprintf("%s:%s->%s", edgeType, callerGraphId, targetGraphId)
	} else {
		dedupKey = fmt.Sprintf("%s:%s->%s:%d:%d", edgeType, callerGraphId, targetGraphId, site.AtRange.StartLine, site.AtRange.StartCol)
	}

	if seen[dedupKey] {
		return false
	}
	seen[dedupKey] = true

	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:%s", dedupKey),
		Type:   edgeType,
		Source: callerGraphId,
		Target: targetGraphId,
	}
	edge.Props.SetProp("confidence", confidence)
	edge.Props.SetProp("reason", reason)
	shared.SetEdgeEvidence(edge, []shared.Evidence{
		{Kind: "scope-resolution", Weight: confidence, Note: reason},
	})

	g.AddEdge(edge)
	return true
}

// TryEmitEdgeWithExplicitTargetId is a variant of TryEmitEdge that takes a
// pre-resolved target graph ID instead of resolving it from a SymbolDefinition.
// Used by the value-receiver-owner bridge where the picked owner-indexed method
// def carries no qualifiedName (object literals have no class owner).
//
// Mirrors TS tryEmitEdgeWithExplicitTargetId.
func TryEmitEdgeWithExplicitTargetId(
	g shared.KnowledgeGraph,
	indexes *model.ScopeResolutionIndexes,
	lookup *GraphNodeLookup,
	site SiteInfo,
	targetGraphId string,
	reason string,
	seen map[string]bool,
	confidence float64,
	collapseByCallerTarget bool,
) bool {
	callerGraphId := ResolveCallerGraphID(site.InScope, indexes, lookup, &site.AtRange)
	edgeType := MapReferenceKindToEdgeType(shared.ReferenceKind(site.Kind))

	if callerGraphId == "" || edgeType == "" {
		return false
	}

	useCollapsed := collapseByCallerTarget && edgeType == graph.RelCalls
	dedupKey := ""
	if useCollapsed {
		dedupKey = fmt.Sprintf("%s:%s->%s", edgeType, callerGraphId, targetGraphId)
	} else {
		dedupKey = fmt.Sprintf("%s:%s->%s:%d:%d", edgeType, callerGraphId, targetGraphId, site.AtRange.StartLine, site.AtRange.StartCol)
	}

	if seen[dedupKey] {
		return false
	}
	seen[dedupKey] = true

	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:%s", dedupKey),
		Type:   edgeType,
		Source: callerGraphId,
		Target: targetGraphId,
	}
	edge.Props.SetProp("confidence", confidence)
	edge.Props.SetProp("reason", reason)
	shared.SetEdgeEvidence(edge, []shared.Evidence{
		{Kind: "scope-resolution", Weight: confidence, Note: reason},
	})

	g.AddEdge(edge)
	return true
}

// EmitInheritanceEdge emits an EXTENDS or IMPLEMENTS edge from child to parent.
func EmitInheritanceEdge(g shared.KnowledgeGraph, childID string, parentID string, relType graph.RelType, evidence []shared.Evidence) {
	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:%s:%s->%s", relType, childID, parentID),
		Type:   relType,
		Source: childID,
		Target: parentID,
	}
	if len(evidence) > 0 {
		shared.SetEdgeEvidence(edge, evidence)
	}
	g.AddEdge(edge)
}

// EmitCallEdge emits a CALLS edge from caller to callee with evidence.
func EmitCallEdge(g shared.KnowledgeGraph, callerID string, calleeID string, evidence []shared.Evidence) {
	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:CALLS:%s->%s", callerID, calleeID),
		Type:   graph.RelCalls,
		Source: callerID,
		Target: calleeID,
	}
	if len(evidence) > 0 {
		shared.SetEdgeEvidence(edge, evidence)
	}
	g.AddEdge(edge)
}

// EmitAccessEdge emits an ACCESSES edge from source to target with an access kind note.
func EmitAccessEdge(g shared.KnowledgeGraph, sourceID string, targetID string, accessKind string, evidence []shared.Evidence) {
	edge := &graph.Edge{
		ID:     fmt.Sprintf("rel:ACCESSES:%s->%s", sourceID, targetID),
		Type:   graph.RelAccesses,
		Source: sourceID,
		Target: targetID,
	}
	edge.Props.SetProp("accessKind", accessKind)
	if len(evidence) > 0 {
		shared.SetEdgeEvidence(edge, evidence)
	}
	g.AddEdge(edge)
}

// SiteInfo holds the minimal reference-site information needed for edge emission.
// Mirrors the TS `{ inScope, atRange, kind }` site parameter.
type SiteInfo struct {
	InScope shared.ScopeID
	AtRange shared.Range
	Kind    string
}