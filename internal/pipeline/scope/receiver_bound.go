package scope

import (
	"fmt"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ScopeContext holds shared state for scope resolution sub-passes.
type ScopeContext struct {
	Graph        *graph.GraphStore
	Repo         string
	Files        []*pipeline.ParsedFile
	HandledSites map[string]bool // tracks processed (callerID, callSiteName) pairs
}

// siteKey generates a unique key for a call site based on callerID and callSiteName.
func siteKey(callerID, callSiteName string) string {
	return fmt.Sprintf("%s:%s", callerID, callSiteName)
}

// nodeSlicePool is a sync.Pool for temporary []*graph.Node slices,
// reducing heap allocations during per-receiver resolution.
var nodeSlicePool = sync.Pool{
	New: func() any {
		s := make([]*graph.Node, 0, 8)
		return &s
	},
}

func getNodeSlice() *[]*graph.Node  { return nodeSlicePool.Get().(*[]*graph.Node) }
func putNodeSlice(s *[]*graph.Node) { *s = (*s)[:0]; nodeSlicePool.Put(s) }

// EmitReceiverBoundCalls implements precise per-receiver edge emission.
// For each CallSite with a Receiver it:
//  1. Finds the receiver's class/struct node in the graph
//  2. Follows HAS_METHOD edges to locate method nodes
//  3. Creates CALLS edges (caller → target method) with confidence=0.95
//  4. Marks (callerID, callSiteName) in handledSites
func EmitReceiverBoundCalls(ctx *ScopeContext) int {
	edgesAdded := 0

	for _, f := range ctx.Files {
		for _, cs := range f.CallSites {
			if cs.Receiver == "" {
				continue
			}

			// Find receiver class/struct nodes by name
			receiverNodes, err := ctx.Graph.GetNodesByName(ctx.Repo, cs.Receiver)
			if err != nil || len(receiverNodes) == 0 {
				continue
			}

			// Resolve caller ID
			callerID := cs.CallerID
			if callerID == "" {
				callerID = findCallerID(ctx, cs)
			}
			if callerID == "" {
				continue
			}

			matched := false

			for _, recvNode := range receiverNodes {
				if !isClassLikeLabel(recvNode.Label) {
					continue
				}

				// Follow HAS_METHOD edges to find target methods
				outEdges, err := ctx.Graph.GetAllOutEdges(recvNode.ID)
				if err != nil {
					continue
				}

				methods := getNodeSlice()
				for _, edge := range outEdges {
					if edge.Type != graph.RelHasMethod {
						continue
					}
					methodNode, err := ctx.Graph.GetNode(edge.Target)
					if err != nil {
						continue
					}
					if methodNode.Name == cs.Name {
						*methods = append(*methods, methodNode)
					}
				}

				// Create CALLS edges from caller to each matched method
				for _, methodNode := range *methods {
					e := graph.NewEdge(graph.RelCalls, callerID, methodNode.ID).
						WithProp("confidence", 0.95).
						WithProp("line", cs.Line).
						WithProp("file", cs.FilePath)
					if err := ctx.Graph.BufferEdge(e); err == nil {
						edgesAdded++
					}
				}
				putNodeSlice(methods)
				matched = true
			}

			if matched {
				ctx.HandledSites[siteKey(callerID, cs.Name)] = true
			}
		}
	}

	return edgesAdded
}

// isClassLikeLabel checks whether a label represents a class-like type.
func isClassLikeLabel(label graph.Label) bool {
	switch label {
	case graph.LabelClass, graph.LabelStruct, graph.LabelInterface, graph.LabelTrait:
		return true
	}
	return false
}

// findCallerID resolves the caller node ID for a call site by locating the
// enclosing function/method in the same file that spans the call site line.
func findCallerID(ctx *ScopeContext, cs *pipeline.CallSite) string {
	nodes, err := ctx.Graph.GetNodesByFile(ctx.Repo, cs.FilePath)
	if err != nil {
		return ""
	}
	for _, n := range nodes {
		if n.Label != graph.LabelFunction && n.Label != graph.LabelMethod {
			continue
		}
		startLine := n.GetPropInt("startLine")
		endLine := n.GetPropInt("endLine")
		if startLine > 0 && endLine > 0 && cs.Line >= startLine && cs.Line <= endLine {
			return n.ID
		}
	}
	return ""
}