package phases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mengshi02/codetrip/internal/community"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/orm"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/mengshi02/codetrip/internal/process"
	"github.com/mengshi02/codetrip/internal/search"
	"github.com/mengshi02/codetrip/internal/tooldef"
)

// ============ PruneLocal Phase ============

// PruneLocalPhase prunes purely local symbols
// Removes local symbol nodes with no external references, non-exported, and no callers
type PruneLocalPhase struct{}

func NewPruneLocalPhase() *PruneLocalPhase { return &PruneLocalPhase{} }

func (p *PruneLocalPhase) Name() string          { return "pruneLocal" }
func (p *PruneLocalPhase) Dependencies() []string { return []string{"scopeResolution"} }

func (p *PruneLocalPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesPruned := 0
	edgesPruned := 0

	// Iterate over all nodes, identifying local symbols that can be pruned
	iter := input.Graph.IterNodes(input.Repo)
	defer iter.Close()

	var nodesToDelete []string

	for iter.Next() {
		node := iter.Node()
		if !node.Label.IsSymbol() {
			continue
		}

		// Retention conditions (satisfy any one to retain):
		// 1. Exported symbols
		if isExported(node) {
			continue
		}
		// 2. Has incoming edges (referenced/called)
		inEdges, err := input.Graph.GetAllInEdges(node.ID)
		if err == nil && len(inEdges) > 0 {
			continue
		}
		// 3. Functions/methods/classes/interfaces — retain key structures
		if node.Label == graph.LabelFunction || node.Label == graph.LabelMethod ||
			node.Label == graph.LabelClass || node.Label == graph.LabelInterface ||
			node.Label == graph.LabelStruct {
			continue
		}
		// 4. Symbols with call out-edges
		outEdges, err := input.Graph.GetAllOutEdges(node.ID)
		if err == nil {
			hasCalls := false
			for _, e := range outEdges {
				if e.Type == graph.RelCalls || e.Type == graph.RelAccesses {
					hasCalls = true
					break
				}
			}
			if hasCalls {
				continue
			}
		}

		nodesToDelete = append(nodesToDelete, node.ID)
	}

	// Execute deletion
	for _, id := range nodesToDelete {
		// Delete node's edges
		outEdges, err := input.Graph.GetAllOutEdges(id)
		if err != nil {
			slog.Warn("prune_dead_code: failed to get out-edges", "node_id", id, "error", err)
		}
		edgesPruned += len(outEdges)
		inEdges, err := input.Graph.GetAllInEdges(id)
		if err != nil {
			slog.Warn("prune_dead_code: failed to get in-edges", "node_id", id, "error", err)
		}
		edgesPruned += len(inEdges)

		if err := input.Graph.DeleteNode(id); err == nil {
			nodesPruned++
		}
	}

	return &pipeline.PhaseOutput{
		NodesAdded: -nodesPruned,
		EdgesAdded: -edgesPruned,
		Stats:      map[string]any{"nodesPruned": nodesPruned, "edgesPruned": edgesPruned},
	}, nil
}

// isExported determines whether a node is an exported symbol
func isExported(node *graph.Node) bool {
	if v, ok := node.Props.GetProp("exported"); ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	// Go: starts with uppercase letter
	if len(node.Name) > 0 && node.Name[0] >= 'A' && node.Name[0] <= 'Z' {
		return true
	}
	return false
}

// ============ MRO Phase ============

// MROPhase Method Resolution Order
// Per ARCHITECTURE.md: collect inheritance relationships → linearization → create METHOD_OVERRIDES / METHOD_IMPLEMENTS edges
type MROPhase struct{}

func NewMROPhase() *MROPhase { return &MROPhase{} }

func (p *MROPhase) Name() string          { return "mro" }
func (p *MROPhase) Dependencies() []string { return []string{"pruneLocal"} }

func (p *MROPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// 1. Collect all class/interface nodes
	iter := input.Graph.IterNodes(input.Repo)
	defer iter.Close()

	var classes []*graph.Node
	for iter.Next() {
		node := iter.Node()
		if isClassLikeLabel(node.Label) {
			classes = append(classes, node)
		}
	}

	// 2. Build MRO (Method Resolution Order) for each class
	// Go uses depth-first linearization (no diamond problem)
	for _, cls := range classes {
		mro := buildMRO(input, cls)

		// 3. Check method overrides/implementations
		// Find all methods of this class
		outEdges, err := input.Graph.GetAllOutEdges( cls.ID)
		if err != nil {
			continue
		}

		for _, edge := range outEdges {
			if edge.Type != graph.RelHasMethod && edge.Type != graph.RelContains && edge.Type != graph.RelDefines {
				continue
			}
			method, err := input.Graph.GetNode(edge.Target)
			if err != nil {
				continue
			}
			if method.Label != graph.LabelMethod {
				continue
			}

			// Find methods with same name in MRO chain (in parent classes)
			for _, ancestor := range mro {
				ancestorMethods := getMethodsOfClass(input, ancestor)
				for _, ancestorMethod := range ancestorMethods {
					if ancestorMethod.Name == method.Name && ancestorMethod.ID != method.ID {
						// Create METHOD_OVERRIDES or METHOD_IMPLEMENTS edge
						relType := graph.RelMethodOverrides
						if ancestor.Label == graph.LabelInterface {
							relType = graph.RelMethodImplements
						}
						mEdge := graph.NewEdge(relType, method.ID, ancestorMethod.ID).
							WithProp("confidence", 0.95)
						if err := input.Graph.BufferEdge(mEdge); err == nil {
							edgesAdded++
						}
					}
				}
			}
		}
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
	}, nil
}

// buildMRO builds method resolution order (depth-first, Go style)
func buildMRO(input *pipeline.PhaseInput, cls *graph.Node) []*graph.Node {
	var mro []*graph.Node
	visited := make(map[string]bool)

	var dfs func(n *graph.Node)
	dfs = func(n *graph.Node) {
		if visited[n.ID] {
			return
		}
		visited[n.ID] = true

		// Find parent classes (via INHERITS edge — unified EXTENDS/IMPLEMENTS/EMBRACES)
		outEdges, err := input.Graph.GetAllOutEdges(n.ID)
		if err != nil {
			return
		}
		for _, edge := range outEdges {
			if edge.Type == graph.RelInherits || edge.Type == graph.RelEmbraces {
				parent, err := input.Graph.GetNode(edge.Target)
				if err == nil && isClassLikeLabel(parent.Label) {
					dfs(parent)
				}
			}
		}

		mro = append(mro, n)
	}

	dfs(cls)
	return mro
}

// getMethodsOfClass gets all methods of a class
func getMethodsOfClass(input *pipeline.PhaseInput, cls *graph.Node) []*graph.Node {
	var methods []*graph.Node
	outEdges, err := input.Graph.GetAllOutEdges( cls.ID)
	if err != nil {
		return nil
	}
	for _, edge := range outEdges {
		if edge.Type == graph.RelHasMethod || edge.Type == graph.RelContains || edge.Type == graph.RelDefines {
			target, err := input.Graph.GetNode(edge.Target)
			if err == nil && (target.Label == graph.LabelMethod || target.Label == graph.LabelFunction) {
				methods = append(methods, target)
			}
		}
	}

	// Fallback to name lookup: match via receiver property
	if len(methods) == 0 {
		iter := input.Graph.IterNodes(input.Repo)
		defer iter.Close()
		for iter.Next() {
			node := iter.Node()
			if node.Label == graph.LabelMethod {
				if recv, ok := node.Props.GetProp("receiver"); ok {
					if fmt.Sprintf("%v", recv) == cls.Name {
						methods = append(methods, node)
					}
				}
			}
		}
	}

	return methods
}

// ============ Communities Phase ============

// Community detection result cache (for use by Processes Phase)
var lastCommunityResult *community.CommunityResult

// CommunitiesPhase Leiden community detection
type CommunitiesPhase struct{}

func NewCommunitiesPhase() *CommunitiesPhase { return &CommunitiesPhase{} }

func (p *CommunitiesPhase) Name() string          { return "communities" }
func (p *CommunitiesPhase) Dependencies() []string { return []string{"mro"} }

func (p *CommunitiesPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// 1. Build adjacency matrix from graph
	adjGraph, err := community.BuildAdjGraph(input.Graph, input.Repo)
	if err != nil {
		return nil, fmt.Errorf("build adj graph: %w", err)
	}

	if len(adjGraph.Nodes) == 0 {
		return &pipeline.PhaseOutput{}, nil
	}

	// 2. Run Leiden algorithm
	leiden := community.NewLeidenAlgorithm()
	result := leiden.Detect(adjGraph)

	// 3. Create Community nodes and MEMBER_OF edges
	for _, cn := range result.Communities {
		commNode := graph.NewNode(input.Repo, graph.LabelCommunity, cn.HeuristicLabel).
			WithProp("cohesion", cn.Cohesion).
			WithProp("symbolCount", cn.SymbolCount)
		// Use CommunityNode.ID as the node ID (it is itself a graph node ID)
		commNode.ID = cn.ID
		if err := input.Graph.BufferNode(commNode); err == nil {
			nodesAdded++
		}
	}

	// Create MEMBER_OF edge for each member
	for _, m := range result.Memberships {
		edge := graph.NewEdge(graph.RelMemberOf, m.NodeID, m.CommunityID).
			WithProp("confidence", 0.85)
		if err := input.Graph.BufferEdge(edge); err == nil {
			edgesAdded++
		}
	}

	// Cache result for use by Processes Phase
	lastCommunityResult = result

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
		Stats:      map[string]any{"communities": len(result.Communities)},
	}, nil
}

// ============ Processes Phase ============

// ProcessesPhase process flow detection
type ProcessesPhase struct{}

func NewProcessesPhase() *ProcessesPhase { return &ProcessesPhase{} }

func (p *ProcessesPhase) Name() string          { return "processes" }
func (p *ProcessesPhase) Dependencies() []string { return []string{"communities"} }

func (p *ProcessesPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// 1. Get community detection result (cached by Communities Phase)
	if lastCommunityResult == nil {
		return &pipeline.PhaseOutput{}, nil // Skip if no community data
	}

	// 2. Use ProcessDetector to detect processes (requires community memberships)
	detector := process.NewProcessDetector()
	result, err := detector.Detect(input.Graph, input.Repo, lastCommunityResult.Memberships)
	if err != nil {
		return nil, fmt.Errorf("detect processes: %w", err)
	}

	// 3. Create Process nodes and STEP_IN_PROCESS edges
	for _, proc := range result.Processes {
		procNode := graph.NewNode(input.Repo, graph.LabelProcess, proc.HeuristicLabel).
			WithProp("entryPointID", proc.EntryPointID).
			WithProp("stepCount", proc.StepCount).
			WithProp("processType", proc.ProcessType)
		procNode.ID = proc.ID
		if err := input.Graph.BufferNode(procNode); err == nil {
			nodesAdded++

			// Create STEP_IN_PROCESS edge for each trace node
			for stepIdx, nodeID := range proc.Trace {
				if nodeID != "" {
					edge := graph.NewEdge(graph.RelStepInProcess, procNode.ID, nodeID).
						WithProp("order", stepIdx)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}
			}
		}
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
		Stats:      map[string]any{"processes": len(result.Processes)},
	}, nil
}

// ============ Embeddings Phase ============

// ============ Index Phase ============

// IndexPhase FTS index construction
type IndexPhase struct{}

func NewIndexPhase() *IndexPhase { return &IndexPhase{} }

func (p *IndexPhase) Name() string          { return "index" }
func (p *IndexPhase) Dependencies() []string { return []string{"processes"} }

func (p *IndexPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// Build BM25 full-text index (persistent on disk)
	tripDir := input.Config.TripDir
	bm25Index, err := search.NewBM25IndexWithDir(tripDir, input.Repo, input.Graph.Store())
	if err != nil {
		slog.Warn("bm25 index create failed, falling back to in-memory",
			"repo", input.Repo, "error", err)
		bm25Index = search.NewBM25Index(input.Graph.Store(), input.Repo)
	}

	// Collect searchable nodes for batch indexing (much faster than one-by-one)
	var batchNodes []*graph.Node
	iter := input.Graph.IterNodes(input.Repo)
	defer iter.Close()

	for iter.Next() {
		node := iter.Node()
		if !isSearchableNode(node) {
			continue
		}
		batchNodes = append(batchNodes, node)
	}

	// Batch index searchable nodes using chunked writes for large repos.
	// BatchIndexChunked breaks the batch into segments to limit memory usage,
	// which is critical for 1M+ node repos where a single batch would consume
	// excessive memory.
	if len(batchNodes) > 0 {
		if batchErr := bm25Index.BatchIndexChunked(batchNodes, input.Config.BM25ChunkSize); batchErr != nil {
			slog.Warn("bm25 chunked batch index failed",
				"repo", input.Repo, "nodes", len(batchNodes), "error", batchErr)
		}
	}

	// Finalize BM25 two-phase build: atomically rename build directory to final directory
	if finalizeErr := bm25Index.FinalizeBuild(); finalizeErr != nil {
		slog.Warn("bm25 build finalization failed", "repo", input.Repo, "error", finalizeErr)
	}

	// Close BM25 index writer to persist data to disk
	if closeErr := bm25Index.Close(); closeErr != nil {
		slog.Warn("bm25 index close failed", "repo", input.Repo, "error", closeErr)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
		Stats:      map[string]any{"indexedNodes": len(batchNodes)},
	}, nil
}

// isSearchableNode determines whether a node is searchable
func isSearchableNode(node *graph.Node) bool {
	switch node.Label {
	case graph.LabelFile, graph.LabelFunction, graph.LabelMethod,
		graph.LabelCallSite, graph.LabelClass, graph.LabelInterface,
		graph.LabelStruct, graph.LabelVariable, graph.LabelConst,
		graph.LabelField, graph.LabelEnum, graph.LabelTypeAlias,
		graph.LabelTrait, graph.LabelTypedef, graph.LabelMacro,
		graph.LabelUnion, graph.LabelNamespace, graph.LabelModule,
		graph.LabelConstructor, graph.LabelStatic, graph.LabelRecord,
		graph.LabelDelegate, graph.LabelAnnotation, graph.LabelDecorator,
		graph.LabelImpl, graph.LabelTemplate, graph.LabelCodeElement:
		return true
	}
	return false
}



// ============ Framework-specific Phases (routes/tools/orm) ============

// RoutesPhase route extraction phase
type RoutesPhase struct{}

func NewRoutesPhase() *RoutesPhase { return &RoutesPhase{} }

func (p *RoutesPhase) Name() string          { return "routes" }
func (p *RoutesPhase) Dependencies() []string { return []string{"parse"} }

func (p *RoutesPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	// TODO: Multi-framework route extraction
	return &pipeline.PhaseOutput{}, nil
}

// ToolsPhase tool definition detection phase
type ToolsPhase struct {
	registry *tooldef.ToolDefRegistry
}

// NewToolsPhase creates a tool definition detection phase
func NewToolsPhase(registry *tooldef.ToolDefRegistry) *ToolsPhase {
	return &ToolsPhase{registry: registry}
}

func (p *ToolsPhase) Name() string          { return "tools" }
func (p *ToolsPhase) Dependencies() []string { return []string{"parse"} }

func (p *ToolsPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	if p.registry == nil {
		// Use default registry (MCP + RPC detectors)
		reg := tooldef.NewToolDefRegistry()
		reg.Register(tooldef.NewMCPToolExtractor())
		reg.Register(tooldef.NewRPCToolExtractor())
		p.registry = reg
	}

	phase := tooldef.NewToolDefPhase(p.registry)
	return phase.Run(ctx, input)
}

// ORMPhase ORM query detection phase
type ORMPhase struct {
	registry *orm.ORMRegistry
}

// NewORMPhase creates an ORM query detection phase
func NewORMPhase(registry *orm.ORMRegistry) *ORMPhase {
	return &ORMPhase{registry: registry}
}

func (p *ORMPhase) Name() string          { return "orm" }
func (p *ORMPhase) Dependencies() []string { return []string{"parse"} }

func (p *ORMPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	if p.registry == nil {
		// Use default registry (Prisma + Supabase detectors)
		reg := orm.NewORMRegistry()
		reg.Register(orm.NewPrismaQueryExtractor())
		reg.Register(orm.NewSupabaseQueryExtractor())
		p.registry = reg
	}

	phase := orm.NewORMPhase(p.registry)
	return phase.Run(ctx, input)
}

// ============ Markdown Phase ============

// MarkdownPhase Markdown Section extraction phase
// Extracts Section nodes (heading hierarchy) from Markdown files
type MarkdownPhase struct{}

func NewMarkdownPhase() *MarkdownPhase { return &MarkdownPhase{} }

func (p *MarkdownPhase) Name() string          { return "markdown" }
func (p *MarkdownPhase) Dependencies() []string { return []string{"structure"} }

func (p *MarkdownPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	for _, f := range input.Files {
		if f.Language != "markdown" {
			continue
		}

		// Get file node
		fileNodes, err := input.Graph.GetNodesByFile(input.Repo, f.Path)
		if err != nil || len(fileNodes) == 0 {
			continue
		}
		fileNode := fileNodes[0]

		// Extract Section nodes from content (based on heading lines)
		lines := strings.Split(string(f.Content), "\n")
		for lineNum, line := range lines {
			heading := ""
			level := 0

			if strings.HasPrefix(line, "# ") {
				heading = strings.TrimPrefix(line, "# ")
				level = 1
			} else if strings.HasPrefix(line, "## ") {
				heading = strings.TrimPrefix(line, "## ")
				level = 2
			} else if strings.HasPrefix(line, "### ") {
				heading = strings.TrimPrefix(line, "### ")
				level = 3
			} else if strings.HasPrefix(line, "#### ") {
				heading = strings.TrimPrefix(line, "#### ")
				level = 4
			}

			if heading == "" {
				continue
			}

			sectionNode := graph.NewNode(input.Repo, graph.LabelSection, heading).
				WithFile(f.Path).
				WithProp("level", level).
				WithProp("startLine", lineNum+1)
			if err := input.Graph.BufferNode(sectionNode); err != nil {
				continue
			}
			nodesAdded++

			// DEFINES edge: File → Section
			edge := graph.NewEdge(graph.RelDefines, fileNode.ID, sectionNode.ID)
			if err := input.Graph.BufferEdge(edge); err != nil {
				continue
			}
			edgesAdded++
		}
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
	}, nil
}

// ============ Cobol Phase ============

// CobolPhase COBOL code preprocessing phase (optional)
// Performs specific preprocessing on COBOL files (e.g., COPY statement expansion)
type CobolPhase struct{}

func NewCobolPhase() *CobolPhase { return &CobolPhase{} }

func (p *CobolPhase) Name() string          { return "cobol" }
func (p *CobolPhase) Dependencies() []string { return []string{"structure"} }

func (p *CobolPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	// COBOL preprocessing: currently a stub implementation, identifies and tags COBOL files
	nodesAdded := 0

	for _, f := range input.Files {
		if f.Language != "cobol" {
			continue
		}

		// Get file node and tag as COBOL
		fileNodes, err := input.Graph.GetNodesByFile(input.Repo, f.Path)
		if err != nil || len(fileNodes) == 0 {
			continue
		}
		for _, fn := range fileNodes {
			fn.Props.SetProp("cobol", true)
			if err := input.Graph.BufferNode(fn); err != nil {
				continue
			}
			nodesAdded++
		}
	}

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
	}, nil
}