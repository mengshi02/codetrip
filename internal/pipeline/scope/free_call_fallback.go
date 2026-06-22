package scope

import (
	"github.com/mengshi02/codetrip/internal/graph"
)

// EmitFreeCallFallback handles call sites that were not resolved by
// receiver-bound resolution — i.e. calls without a Receiver or calls
// whose Receiver could not be resolved to a class/struct node.
//
// For each unhandled CallSite it:
//  1. Skips sites already present in handledSites
//  2. Looks up functions/methods by name via GetNodesByName
//  3. Creates CALLS edges with confidence=0.7
//  4. Adds the site to handledSites
func EmitFreeCallFallback(ctx *ScopeContext) int {
	edgesAdded := 0

	for _, f := range ctx.Files {
		for _, cs := range f.CallSites {
			// Resolve caller ID first (needed for handledSites key)
			callerID := cs.CallerID
			if callerID == "" {
				callerID = findCallerID(ctx, cs)
			}
			if callerID == "" {
				continue
			}

			// Skip sites already handled by receiver-bound pass
			key := siteKey(callerID, cs.Name)
			if ctx.HandledSites[key] {
				continue
			}

			// Only handle calls without a Receiver, or whose Receiver
			// was not resolved (no prior handled entry means it failed).
			if cs.Receiver != "" {
				// Receiver calls that landed here were NOT resolved;
				// still attempt free-call fallback.
			}

			// Look up target functions/methods by name
			targetNodes, err := ctx.Graph.GetNodesByName(ctx.Repo, cs.Name)
			if err != nil || len(targetNodes) == 0 {
				continue
			}

			matched := false
			for _, targetNode := range targetNodes {
				// Only match function / method labels
				if targetNode.Label != graph.LabelFunction && targetNode.Label != graph.LabelMethod {
					continue
				}

				e := graph.NewEdge(graph.RelCalls, callerID, targetNode.ID).
					WithProp("confidence", 0.7).
					WithProp("line", cs.Line).
					WithProp("file", cs.FilePath)
				if err := ctx.Graph.BufferEdge(e); err == nil {
					edgesAdded++
				}
				matched = true
			}

			if matched {
				ctx.HandledSites[key] = true
			}
		}
	}

	return edgesAdded
}