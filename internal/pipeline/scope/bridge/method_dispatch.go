package bridge

import (
	"fmt"
	"log/slog"

	"github.com/mengshi02/codetrip"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// MethodDispatcher method dispatcher
// Handles method call resolution: receiver type → MRO traversal → method matching
// Follows emission order contract I1:
//   emitReceiverBoundCalls → emitFreeCallFallback → emitReferencesViaLookup → emitImportEdges
type MethodDispatcher struct {
	graph    *graph.GraphStore
	repo     string
	resolver codetrip.ScopeResolver
	emitter  *EdgeEmitter
	idResolver *IDResolver
}

// NewMethodDispatcher creates a method dispatcher
func NewMethodDispatcher(gs *graph.GraphStore, repo string, resolver codetrip.ScopeResolver) *MethodDispatcher {
	return &MethodDispatcher{
		graph:      gs,
		repo:       repo,
		resolver:   resolver,
		emitter:    NewEdgeEmitter(gs, repo, 64),
		idResolver: NewIDResolver(gs, repo),
	}
}

// DispatchCallSite dispatches a call site
// Finds target method node and emits CALLS edge based on CallSite's receiver and method name
func (d *MethodDispatcher) DispatchCallSite(cs *pipeline.CallSite) error {
	if cs.CallerID == "" {
		return nil
	}

	// 1. Try receiver-bound method call
	if cs.Receiver != "" {
		targetIDs := d.idResolver.ResolveMethod(cs.Receiver, cs.Name)
		if len(targetIDs) > 0 {
			// Check arity compatibility
			for _, targetID := range targetIDs {
				target, err := d.graph.GetNode(targetID)
				if err != nil {
					continue
				}
				if d.resolver.ArityCompatibility(cs, target) {
					confidence := 0.95 // Same file Tier1
					if err := d.emitter.EmitCall(cs.CallerID, targetID, confidence); err != nil {
						return err
					}
				}
			}
			return d.emitter.Flush()
		}

		// 2. Try MRO traversal (inheritance/interface method lookup)
		targetIDs = d.dispatchViaMRO(cs.Receiver, cs.Name)
		if len(targetIDs) > 0 {
			for _, targetID := range targetIDs {
				if err := d.emitter.EmitCall(cs.CallerID, targetID, 0.9); err != nil {
					return err
				}
			}
			return d.emitter.Flush()
		}

		// 3. Untyped receiver fallback (dynamic languages)
		if d.resolver.IsSuperReceiver(cs.Receiver) {
			// super call: find parent class method
			targetIDs = d.dispatchSuperCall(cs)
			if len(targetIDs) > 0 {
				for _, targetID := range targetIDs {
					if err := d.emitter.EmitCall(cs.CallerID, targetID, 0.85); err != nil {
						return err
					}
				}
				return d.emitter.Flush()
			}
		}
	}

	// 4. Free function call fallback
	targetIDs := d.idResolver.ResolveByName(cs.Name)
	for _, targetID := range targetIDs {
		target, err := d.graph.GetNode(targetID)
		if err != nil {
			continue
		}
		// Only match function nodes (exclude class/variable etc. with same name)
		if target.Label == graph.LabelFunction || target.Label == graph.LabelMethod {
			if err := d.emitter.EmitCall(cs.CallerID, targetID, 0.9); err != nil {
				return err
			}
		}
	}

	return d.emitter.Flush()
}

// dispatchViaMRO finds method via MRO traversal
func (d *MethodDispatcher) dispatchViaMRO(receiver, methodName string) []string {
	// Find receiver type node
	receiverNodes := d.idResolver.ResolveByName(receiver)
	if len(receiverNodes) == 0 {
		return nil
	}

	// Traverse EMBRACES edges to find base class methods
	for _, recvID := range receiverNodes {
		visited := make(map[string]bool)
		queue := []string{recvID}

		for len(queue) > 0 {
			currID := queue[0]
			queue = queue[1:]

			if visited[currID] {
				continue
			}
			visited[currID] = true

			// Find EMBRACES out edges (inheritance relationships)
			outEdges, err := d.graph.GetAllOutEdges(currID)
			if err != nil {
				slog.Warn("method_dispatch: failed to get out-edges", "node_id", currID, "error", err)
			}
			for _, edge := range outEdges {
				if edge.Type == graph.RelEmbraces || edge.Type == graph.RelExtends || edge.Type == graph.RelImplements {
					// Find method in base class
					baseNode, err := d.graph.GetNode(edge.Target)
					if err != nil {
						continue
					}
					methodIDs := d.idResolver.ResolveMethod(baseNode.Name, methodName)
					if len(methodIDs) > 0 {
						return methodIDs
					}
					queue = append(queue, edge.Target)
				}
			}
		}
	}

	return nil
}

// dispatchSuperCall dispatches super call
func (d *MethodDispatcher) dispatchSuperCall(cs *pipeline.CallSite) []string {
	// Find the class containing the caller
	caller, err := d.graph.GetNode(cs.CallerID)
	if err != nil {
		return nil
	}

	// Find the class the caller belongs to (MEMBER_OF / CONTAINS in edges)
	inEdges, err := d.graph.GetAllInEdges(caller.ID)
	if err != nil {
		slog.Warn("method_dispatch: failed to get in-edges", "node_id", caller.ID, "error", err)
	}
	for _, edge := range inEdges {
		if edge.Type == graph.RelMemberOf || edge.Type == graph.RelContains {
			classNode, err := d.graph.GetNode(edge.Source)
			if err != nil {
				continue
			}
			// Find base class of this class
			outEdges, err := d.graph.GetAllOutEdges(classNode.ID)
			if err != nil {
				slog.Warn("method_dispatch: failed to get out-edges for class", "node_id", classNode.ID, "error", err)
			}
			for _, oe := range outEdges {
				if oe.Type == graph.RelEmbraces || oe.Type == graph.RelExtends {
					baseNode, err := d.graph.GetNode(oe.Target)
					if err != nil {
						continue
					}
					// Find method in base class
					methodIDs := d.idResolver.ResolveMethod(baseNode.Name, cs.Name)
					if len(methodIDs) > 0 {
						return methodIDs
					}
				}
			}
		}
	}

	return nil
}

// SetEmitter sets edge emitter (for custom emission behavior)
func (d *MethodDispatcher) SetEmitter(emitter *EdgeEmitter) {
	d.emitter = emitter
}

// Stats returns dispatch statistics
type DispatchStats struct {
	CallSitesProcessed int
	EdgesEmitted       int
	Unresolved         int
}

// String formats statistics
func (s DispatchStats) String() string {
	return fmt.Sprintf("dispatched=%d, edges=%d, unresolved=%d", s.CallSitesProcessed, s.EdgesEmitted, s.Unresolved)
}