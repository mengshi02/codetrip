package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mengshi02/codetrip/internal/graph"
)

// ============ Impact Tool ============

// ImpactRequest represents an impact analysis request
type ImpactRequest struct {
	Target        string
	Repo          string   // repository name for graph store selection
	TargetUID     string
	Direction     string   // upstream | downstream
	FilePath      string
	Kind          string
	MaxDepth      int
	RelationTypes []string
	MinConfidence float64
	IncludeTests  bool
	SummaryOnly   bool
	Limit         int
	Offset        int
	TimeoutMs     int
}

// ImpactResult represents an impact analysis result
type ImpactResult struct {
	Risk              string
	AffectedProcesses []ProcessRef
	AffectedModules   []ModuleRef
	ByDepth           []DepthGroup
	ByDepthCounts     map[int]int
}

// ProcessRef represents a process reference
type ProcessRef struct {
	ID    string
	Label string
}

// ModuleRef represents a module reference
type ModuleRef struct {
	Name     string
	FilePath string
}

// DepthGroup represents a depth grouping
type DepthGroup struct {
	Depth   int
	Symbols []SymbolRef
}

// SymbolRef represents a symbol reference
type SymbolRef struct {
	NodeID   string
	Name     string
	Kind     string
	FilePath string
}

// RunImpact executes impact analysis
func RunImpact(ctx context.Context, gs *graph.GraphStore, req *ImpactRequest) (*ImpactResult, error) {
	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 32 {
		maxDepth = 32
	}

	// Find target node
	var startNodes []*graph.Node
	var err error
	if req.TargetUID != "" {
		node, e := gs.FindByUID(req.TargetUID)
		if e != nil {
			return nil, e
		}
		startNodes = []*graph.Node{node}
	} else {
		startNodes, err = gs.GetNodesByName(gs.Repo(), req.Target)
		if err != nil {
			return nil, err
		}
	}

	if len(startNodes) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", req.Target)
	}

	result := &ImpactResult{
		ByDepthCounts: make(map[int]int),
	}

	direction := graph.TraverseOut
	if req.Direction == "upstream" {
		direction = graph.TraverseIn
	}

	// Relation filtering
	relationSet := make(map[string]bool)
	for _, rt := range req.RelationTypes {
		relationSet[rt] = true
	}
	filter := func(e *graph.Edge) bool {
		if req.MinConfidence > 0 && e.Confidence() < req.MinConfidence {
			return false
		}
		if len(relationSet) > 0 && !relationSet[string(e.Type)] {
			return false
		}
		return true
	}

	totalAffected := 0
	for _, startNode := range startNodes {
		bfsNodes, _ := gs.BFS(ctx, startNode.ID, direction, maxDepth, filter)
		totalAffected += len(bfsNodes)

		// Manual BFS to collect depth information
		depthMap := make(map[int][]SymbolRef)
		visited := make(map[string]int)
		visited[startNode.ID] = 0

		queue := []struct {
			id    string
			depth int
		}{{id: startNode.ID, depth: 0}}

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]

			if cur.depth >= maxDepth {
				continue
			}

			var edges []*graph.Edge
			if direction == graph.TraverseIn {
				edges, err = gs.GetAllInEdges(cur.id)
			} else {
				edges, err = gs.GetAllOutEdges(cur.id)
			}
			if err != nil {
				slog.Warn("impact traverse: get edges failed", "nodeID", cur.id, "direction", direction, "error", err)
				continue
			}

			for _, edge := range edges {
				if filter != nil && !filter(edge) {
					continue
				}
				nextID := edge.Target
				if direction == graph.TraverseIn {
					nextID = edge.Source
				}
				if _, seen := visited[nextID]; seen {
					continue
				}
				visited[nextID] = cur.depth + 1

				node, e := gs.GetNode(nextID)
				if e != nil {
					continue
				}

				symRef := SymbolRef{
					NodeID:   node.ID,
					Name:     node.Name,
					Kind:     string(node.Label),
					FilePath: node.FilePath,
				}
				depthMap[cur.depth+1] = append(depthMap[cur.depth+1], symRef)
				queue = append(queue, struct {
					id    string
					depth int
				}{nextID, cur.depth + 1})
			}
		}

		for depth, symbols := range depthMap {
			result.ByDepthCounts[depth] = len(symbols)
			result.ByDepth = append(result.ByDepth, DepthGroup{
				Depth:   depth,
				Symbols: symbols,
			})
		}
	}

	switch {
	case totalAffected >= 10:
		result.Risk = "CRITICAL"
	case totalAffected >= 6:
		result.Risk = "HIGH"
	case totalAffected >= 3:
		result.Risk = "MEDIUM"
	default:
		result.Risk = "LOW"
	}

	return result, nil
}

// ============ Context Tool ============

// ContextRequest represents a 360-degree symbol view request
type ContextRequest struct {
	Name           string
	Repo           string   // repository name for graph store selection
	UID            string
	FilePath       string
	Kind           string
	IncludeContent bool
}

// ContextResult represents a 360-degree symbol view result
type ContextResult struct {
	Symbol         *ContextSymbolInfo
	Incoming       []ReferenceGroup
	Outgoing       []ReferenceGroup
	Processes      []ProcessRef
	Disambiguation []Candidate
}

// ContextSymbolInfo represents context symbol information
type ContextSymbolInfo struct {
	NodeID   string
	Name     string
	Kind     string
	FilePath string
}

// ReferenceGroup represents a reference grouping
type ReferenceGroup struct {
	Type string
	Refs []SymbolRef
}

// Candidate represents a disambiguation candidate
type Candidate struct {
	NodeID   string
	Name     string
	Kind     string
	FilePath string
}

// RunContext executes a 360-degree symbol view
func RunContext(ctx context.Context, gs *graph.GraphStore, req *ContextRequest) (*ContextResult, error) {
	var node *graph.Node
	var err error
	var candidates []*graph.Node

	if req.UID != "" {
		node, err = gs.FindByUID(req.UID)
		if err != nil {
			return nil, err
		}
	} else {
		candidates, err = gs.GetNodesByName(gs.Repo(), req.Name)
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("symbol not found: %s", req.Name)
		}
		if req.FilePath != "" {
			for _, c := range candidates {
				if c.FilePath == req.FilePath {
					node = c
					break
				}
			}
		}
		if node == nil {
			node = candidates[0]
		}
	}

	result := &ContextResult{
		Symbol: &ContextSymbolInfo{
			NodeID:   node.ID,
			Name:     node.Name,
			Kind:     string(node.Label),
			FilePath: node.FilePath,
		},
	}

	// Group incoming edges
	inEdges, err := gs.GetAllInEdges(node.ID)
	if err != nil {
		slog.Warn("context: failed to get in-edges", "node_id", node.ID, "error", err)
	}
	groupIncoming := make(map[string][]SymbolRef)
	for _, edge := range inEdges {
		src, err := gs.GetNode(edge.Source)
		if err != nil {
			slog.Warn("context: failed to get source node", "node_id", edge.Source, "error", err)
		}
		if src != nil {
			ref := SymbolRef{NodeID: src.ID, Name: src.Name, Kind: string(src.Label), FilePath: src.FilePath}
			groupIncoming[string(edge.Type)] = append(groupIncoming[string(edge.Type)], ref)
		}
	}
	for typ, refs := range groupIncoming {
		result.Incoming = append(result.Incoming, ReferenceGroup{Type: typ, Refs: refs})
	}

	// Group outgoing edges
	outEdges, err := gs.GetAllOutEdges(node.ID)
	if err != nil {
		slog.Warn("context: failed to get out-edges", "node_id", node.ID, "error", err)
	}
	groupOutgoing := make(map[string][]SymbolRef)
	for _, edge := range outEdges {
		tgt, err := gs.GetNode(edge.Target)
		if err != nil {
			slog.Warn("context: failed to get target node", "node_id", edge.Target, "error", err)
		}
		if tgt != nil {
			ref := SymbolRef{NodeID: tgt.ID, Name: tgt.Name, Kind: string(tgt.Label), FilePath: tgt.FilePath}
			groupOutgoing[string(edge.Type)] = append(groupOutgoing[string(edge.Type)], ref)
		}
	}
	for typ, refs := range groupOutgoing {
		result.Outgoing = append(result.Outgoing, ReferenceGroup{Type: typ, Refs: refs})
	}

	if len(candidates) > 1 {
		for _, c := range candidates {
			result.Disambiguation = append(result.Disambiguation, Candidate{
				NodeID: c.ID, Name: c.Name, Kind: string(c.Label), FilePath: c.FilePath,
			})
		}
	}

	return result, nil
}

// ============ Check Tool ============

// CheckRequest represents a structure check request
type CheckRequest struct {
	Repo   string // Repository name, uses default if empty
	Cycles bool   // Detect circular dependencies (default true)
}

// CheckResult represents a structure check result
type CheckResult struct {
	Cycles [][]string
}

// RunCheck executes structure check
func RunCheck(ctx context.Context, gs *graph.GraphStore, req *CheckRequest) (*CheckResult, error) {
	result := &CheckResult{}
	if req.Cycles {
		cycles, _ := gs.DetectCycles(ctx, gs.Repo())
		result.Cycles = cycles
	}
	return result, nil
}

// ============ Search Tool ============

// SearchRequest represents a search request
type SearchRequest struct {
	Query    string
	Limit    int
	Repo     string
	Semantic bool
}

// SearchResult represents a search result
type SearchResult struct {
	Results  []SearchItem
	Fallback string // Set to "bm25" when semantic search falls back to BM25 due to no embed data
}

// SearchItem represents a search result item
type SearchItem struct {
	NodeID    string
	Name      string
	Kind      string
	FilePath  string
	Score     float64
	StartLine int
	EndLine   int
}

// ============ Rename Tool ============

// RenameRequest represents a multi-file collaborative rename request
type RenameRequest struct {
	SymbolName string
	Repo       string // repository name for graph store selection
	SymbolUID  string
	NewName    string
	FilePath   string
	DryRun     bool
}

// RenameResult represents a rename result
type RenameResult struct {
	Edits []RenameEdit
}

// RenameEdit represents a rename edit
type RenameEdit struct {
	FilePath   string
	OldText    string
	NewText    string
	Confidence string
}

// ============ DetectChanges Tool ============

// DetectChangesRequest represents a change detection request
type DetectChangesRequest struct {
	Scope   string
	BaseRef string
	Repo    string
}

// DetectChangesResult represents a change detection result
type DetectChangesResult struct {
	ChangedSymbols    []SymbolChange
	AffectedProcesses []ProcessRef
	RiskSummary       RiskSummary
}

// SymbolChange represents a symbol change
type SymbolChange struct {
	NodeID     string
	Name       string
	Kind       string
	FilePath   string
	ChangeType string
}

// RiskSummary represents a risk summary
type RiskSummary struct {
	Level        string
	TotalChanges int
	HighRisk     int
}

// ============ Route/Tool/Shape/API Impact ============

// RouteMapRequest represents a route mapping request
type RouteMapRequest struct {
	Route string
	Repo  string // repository name for graph store selection
}

// RouteMapResult represents a route mapping result
type RouteMapResult struct{ Routes []RouteInfo }

// RouteInfo represents route information
type RouteInfo struct {
	Path       string
	Method     string
	HandlerID  string
	Middleware []string
	Consumers  []string
}

// ToolMapRequest represents a tool mapping request
type ToolMapRequest struct {
	Tool string
	Repo string // repository name for graph store selection
}

// ToolMapResult represents a tool mapping result
type ToolMapResult struct{ Tools []ToolInfo }

// ToolInfo represents tool information
type ToolInfo struct {
	Name        string
	Description string
	HandlerID   string
}

// ShapeCheckRequest represents a response shape check request
type ShapeCheckRequest struct {
	Route string
	Repo  string // repository name for graph store selection
}

// ShapeCheckResult represents a response shape check result
type ShapeCheckResult struct{ Mismatches []ShapeMismatch }

// ShapeMismatch represents a shape mismatch
type ShapeMismatch struct {
	Route    string
	Field    string
	Producer string
	Consumer string
}

// ApiImpactRequest represents an API impact analysis request
type ApiImpactRequest struct {
	Route string
	Repo  string // repository name for graph store selection
	File  string
}

// ApiImpactResult represents an API impact analysis result
type ApiImpactResult struct {
	Risk       string
	Consumers  []ConsumerInfo
	Mismatches []ShapeMismatch
	Middleware []string
	Processes  []ProcessRef
}

// ConsumerInfo represents consumer information
type ConsumerInfo struct {
	NodeID   string
	Name     string
	FilePath string
}

// ExplainRequest represents a taint explanation request
type ExplainRequest struct {
	Target string
	Repo   string // repository name for graph store selection
	Limit  int
}

// ExplainResult represents a taint explanation result
type ExplainResult struct {
	Findings      []TaintFinding
	TotalFindings int
	Truncated     bool
}

// TaintFinding represents a taint finding
type TaintFinding struct {
	Category   string
	SourceLine int
	SinkLine   int
	HopPath    []HopInfo
}

// HopInfo represents a hop information
type HopInfo struct {
	NodeID string
	Line   int
}

// ============ Request Validation ============

// Validate checks the ImpactRequest for invalid fields
func (r *ImpactRequest) Validate() error {
	if r.Target == "" && r.TargetUID == "" {
		return fmt.Errorf("impact request: Target or TargetUID is required")
	}
	if r.Direction != "" && r.Direction != "upstream" && r.Direction != "downstream" {
		return fmt.Errorf("impact request: Direction must be 'upstream' or 'downstream', got %q", r.Direction)
	}
	if r.Limit < 0 {
		return fmt.Errorf("impact request: Limit must be non-negative, got %d", r.Limit)
	}
	return nil
}

// Validate checks the SearchRequest for invalid fields
func (r *SearchRequest) Validate() error {
	if r.Query == "" {
		return fmt.Errorf("search request: Query is required")
	}
	if r.Limit < 0 {
		return fmt.Errorf("search request: Limit must be non-negative, got %d", r.Limit)
	}
	return nil
}

// Validate checks the RenameRequest for invalid fields
func (r *RenameRequest) Validate() error {
	if r.SymbolName == "" && r.SymbolUID == "" {
		return fmt.Errorf("rename request: SymbolName or SymbolUID is required")
	}
	if r.NewName == "" {
		return fmt.Errorf("rename request: NewName is required")
	}
	return nil
}

// Validate checks the ExplainRequest for invalid fields
func (r *ExplainRequest) Validate() error {
	if r.Target == "" {
		return fmt.Errorf("explain request: Target is required")
	}
	if r.Limit < 0 {
		return fmt.Errorf("explain request: Limit must be non-negative, got %d", r.Limit)
	}
	return nil
}

// Validate checks the RouteMapRequest for invalid fields
func (r *RouteMapRequest) Validate() error {
	if r.Route == "" {
		return fmt.Errorf("route_map request: Route is required")
	}
	return nil
}

// Validate checks the ToolMapRequest for invalid fields
func (r *ToolMapRequest) Validate() error {
	if r.Tool == "" {
		return fmt.Errorf("tool_map request: Tool is required")
	}
	return nil
}

// Validate checks the ContextRequest for invalid fields
func (r *ContextRequest) Validate() error {
	if r.Name == "" && r.UID == "" {
		return fmt.Errorf("context request: Name or UID is required")
	}
	return nil
}

// Validate checks the CheckRequest for invalid fields
func (r *CheckRequest) Validate() error {
	// CheckRequest has no required fields; Repo defaults to "" (default store)
	return nil
}

// Validate checks the DetectChangesRequest for invalid fields
func (r *DetectChangesRequest) Validate() error {
	if r.BaseRef == "" {
		return fmt.Errorf("detect_changes request: BaseRef is required")
	}
	return nil
}

// Validate checks the ShapeCheckRequest for invalid fields
func (r *ShapeCheckRequest) Validate() error {
	if r.Route == "" {
		return fmt.Errorf("shape_check request: Route is required")
	}
	return nil
}

// Validate checks the ApiImpactRequest for invalid fields
func (r *ApiImpactRequest) Validate() error {
	if r.Route == "" {
		return fmt.Errorf("api_impact request: Route is required")
	}
	return nil
}