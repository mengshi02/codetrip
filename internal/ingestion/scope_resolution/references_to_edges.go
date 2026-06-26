package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ReferencesToEdges converts resolved reference sites into graph edges.
// Mirrors TS scope-resolution/graph-bridge/references-to-edges.ts.

// EmitReferencesViaLookup resolves reference sites through the generic
// lookup algorithm and emits edges for sites not already handled by
// specialized passes.
//
// Mirrors TS emitReferencesViaLookup. Iterates the ReferenceIndex,
// resolves each reference's caller and target to graph node IDs,
// and emits the appropriate edge type (CALLS/ACCESSES/EXTENDS/USES).
//
// handledSites is a set of site keys (`filePath:startLine:startCol`)
// that were already handled by specialized passes (receiver-bound,
// free-call fallback). These sites are skipped.
//
// Returns the count of edges emitted.
func EmitReferencesViaLookup(
	g shared.KnowledgeGraph,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
	handledSites map[string]bool,
	mro map[string][]string,
) int {
	emitted := 0
	skipped := 0
	seen := make(map[string]bool)

	scopeTree := indexes.ScopeTree()
	defs := indexes.Defs()

	// Iterate all reference sites
	WalkReferences(indexes, func(ref *shared.ReferenceSite, scopeID shared.ScopeID) {
		scope := scopeTree.GetScope(scopeID)
		if scope == nil {
			skipped++
			return
		}
		fromFilePath := scope.FilePath

		// Skip sites already handled by specialized passes
		if handledSites != nil && fromFilePath != "" {
			siteKey := fmt.Sprintf("%s:%d:%d", fromFilePath, ref.Range.StartLine, ref.Range.StartCol)
			if handledSites[siteKey] {
				skipped++
				return
			}
		}

		// Resolve caller graph ID
		atRange := ref.Range
		callerGraphId := ResolveCallerGraphID(scopeID, indexes, lookup, &atRange)
		if callerGraphId == "" {
			skipped++
			return
		}

		// Resolve target def
		targetDef := defs.Get(shared.DefID(ref.DefID))
		if targetDef == nil {
			skipped++
			return
		}

		// Resolve target graph ID
		targetGraphId := ResolveDefGraphID(targetDef.FilePath, targetDef, lookup)
		if targetGraphId == "" {
			skipped++
			return
		}

		// Map reference kind to edge type
		edgeType := MapReferenceKindToEdgeType(ref.Kind)
		if edgeType == "" {
			skipped++
			return
		}

		// Dedup
		dedupKey := fmt.Sprintf("%s:%s->%s:%d:%d", edgeType, callerGraphId, targetGraphId, ref.Range.StartLine, ref.Range.StartCol)
		if seen[dedupKey] {
			return
		}
		seen[dedupKey] = true

		// Emit edge
		reason := fmt.Sprintf("scope-resolution: %s", ref.Kind)
		confidence := 1.0

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
		emitted++
	})

	_ = skipped // consumed
	return emitted
}

// EmitPreInheritanceEdges emits EXTENDS edges from inherits-type reference sites.
// Must run BEFORE BuildMro so the graph has the EXTENDS edges when MRO is computed.
//
// Mirrors TS emitPreInheritanceEdges. Iterates all reference sites with
// kind=inherits, resolves the target class, and emits an EXTENDS edge.
// Also adds these sites to handledSites so the generic EmitReferencesViaLookup
// pass doesn't re-emit them.
//
// Returns the count of edges emitted.
func EmitPreInheritanceEdges(
	g shared.KnowledgeGraph,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
	handledSites map[string]bool,
) int {
	emitted := 0
	seen := make(map[string]bool)

	scopeTree := indexes.ScopeTree()
	defs := indexes.Defs()

	// Iterate all reference sites, looking for inherits-type references
	WalkReferences(indexes, func(ref *shared.ReferenceSite, scopeID shared.ScopeID) {
		if ref.Kind != shared.ReferenceInherits {
			return
		}

		scope := scopeTree.GetScope(scopeID)
		if scope == nil {
			return
		}
		fromFilePath := scope.FilePath

		// Mark site as handled
		if handledSites != nil && fromFilePath != "" {
			siteKey := fmt.Sprintf("%s:%d:%d", fromFilePath, ref.Range.StartLine, ref.Range.StartCol)
			handledSites[siteKey] = true
		}

		// Resolve caller graph ID (the class that inherits)
		atRange := ref.Range
		callerGraphId := ResolveCallerGraphID(scopeID, indexes, lookup, &atRange)
		if callerGraphId == "" {
			return
		}

		// Resolve target def
		targetDef := defs.Get(shared.DefID(ref.DefID))
		if targetDef == nil {
			return
		}

		// Resolve target graph ID
		targetGraphId := ResolveDefGraphID(targetDef.FilePath, targetDef, lookup)
		if targetGraphId == "" {
			return
		}

		// Dedup
		dedupKey := fmt.Sprintf("EXTENDS:%s->%s", callerGraphId, targetGraphId)
		if seen[dedupKey] {
			return
		}
		seen[dedupKey] = true

		// Emit EXTENDS edge
		reason := "scope-resolution: inherits"
		EmitInheritanceEdge(g, callerGraphId, targetGraphId, graph.RelExtends,
			[]shared.Evidence{
				{Kind: "scope-resolution", Weight: 1.0, Note: reason},
			},
		)
		emitted++
	})

	return emitted
}