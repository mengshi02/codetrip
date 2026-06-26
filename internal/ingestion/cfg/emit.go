package cfg

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
)

// EmitCFG emits FunctionCFG to GraphStore
// Creates Node(LabelBasicBlock) for each BasicBlock, creates CFG edges and REACHING_DEF edges
// Uses GraphStore.Batch for batch writing
func EmitCFG(gs *graph.GraphStore, fcfg *FunctionCFG) error {
	if gs == nil {
		return fmt.Errorf("graphstore is nil")
	}
	if fcfg == nil {
		return fmt.Errorf("functioncfg is nil")
	}

	repo := gs.Repo()

	return gs.Batch(func(b *graph.Batch) error {
		// 1. Create Node for each BasicBlock
		for i := range fcfg.Blocks {
			block := &fcfg.Blocks[i]
			node := graph.NewNode(repo, graph.LabelBasicBlock, blockLabel(fcfg.FuncName, block.ID))
			node.ID = block.ID
			node.WithProp("funcID", fcfg.FuncID)
			node.WithProp("funcName", fcfg.FuncName)
			node.WithProp("blockLabel", block.Label)
			node.WithProp("startLine", block.StartLine)
			node.WithProp("endLine", block.EndLine)
			if len(block.NodeIDs) > 0 {
				node.WithProp("nodeIDs", block.NodeIDs)
			}
			if len(block.StatementIDs) > 0 {
				node.WithProp("statementIDs", block.StatementIDs)
			}
			if err := b.AddNode(node); err != nil {
				return fmt.Errorf("add basic block node %s: %w", block.ID, err)
			}
		}

		// 2. Create CFG edges
		for i := range fcfg.Edges {
			edge := &fcfg.Edges[i]
			e := graph.NewEdge(graph.RelCFG, edge.From, edge.To)
			e.WithProp("edgeType", edge.EdgeType)
			if edge.Condition != "" {
				e.WithProp("condition", edge.Condition)
			}
			if err := b.AddEdge(e); err != nil {
				return fmt.Errorf("add cfg edge %s->%s: %w", edge.From, edge.To, err)
			}
		}

		// 3. Emit binding information to node properties
		for i := range fcfg.Bindings {
			binding := &fcfg.Bindings[i]
			// Create corresponding variable node for parameters and local variables
			varNode := graph.NewNode(repo, graph.LabelVariable, binding.Name)
			varNode.ID = binding.NodeID
			varNode.WithProp("funcID", fcfg.FuncID)
			varNode.WithProp("isParam", binding.IsParam)
			varNode.WithProp("line", binding.Line)
			if err := b.AddNode(varNode); err != nil {
				return fmt.Errorf("add binding node %s: %w", binding.NodeID, err)
			}
		}

		// 4. Emit call sites
		for i := range fcfg.Sites {
			site := &fcfg.Sites[i]
			siteNode := graph.NewNode(repo, graph.LabelCallSite, site.Symbol)
			siteNode.ID = site.NodeID
			siteNode.WithProp("funcID", fcfg.FuncID)
			siteNode.WithProp("siteType", site.SiteType)
			siteNode.WithProp("line", site.Line)
			if err := b.AddNode(siteNode); err != nil {
				return fmt.Errorf("add site node %s: %w", site.NodeID, err)
			}
		}

		return nil
	})
}

// EmitReachingDefs emits reaching definition analysis results to GraphStore
// Creates REACHING_DEF edges connecting definition points to use points
func EmitReachingDefs(gs *graph.GraphStore, fcfg *FunctionCFG, reachingDefs map[string][]string) error {
	if gs == nil {
		return fmt.Errorf("graphstore is nil")
	}

	return gs.Batch(func(b *graph.Batch) error {
		for defPoint, reachableBlocks := range reachingDefs {
			for _, blockID := range reachableBlocks {
				e := graph.NewEdge(graph.RelReachingDef, defPoint, blockID)
				e.WithProp("funcID", fcfg.FuncID)
				if err := b.AddEdge(e); err != nil {
					return fmt.Errorf("add reaching def edge %s->%s: %w", defPoint, blockID, err)
				}
			}
		}
		return nil
	})
}

// blockLabel generates a readable name for a basic block
func blockLabel(funcName, blockID string) string {
	return fmt.Sprintf("%s:%s", funcName, blockID)
}