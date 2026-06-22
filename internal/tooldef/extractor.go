package tooldef

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ToolDefinitionExtractor MCP/RPC tool definition detection interface
type ToolDefinitionExtractor interface {
	// Framework returns the framework/protocol name for the extractor
	Framework() string
	// DetectTools detects tool definitions from parsed files
	DetectTools(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ToolDef, error)
}

// ToolDef tool definition
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
	HandlerID   string
	FilePath    string
	Line        int
}

// ============ MCP Tool Detection ============

// MCPToolExtractor detects MCP tool definitions
// Scans name + description + inputSchema array patterns
// Recognizes tool() function calls or @tool decorators
type MCPToolExtractor struct{}

// NewMCPToolExtractor creates MCP tool detector
func NewMCPToolExtractor() *MCPToolExtractor { return &MCPToolExtractor{} }

// Framework returns framework name
func (e *MCPToolExtractor) Framework() string { return "mcp" }

// DetectTools detects MCP tool definitions
func (e *MCPToolExtractor) DetectTools(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ToolDef, error) {
	tools := acquireToolSlice()
	defer releaseToolSlice(tools)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 1. Detect tool() function call patterns from CallSites
		for _, cs := range f.CallSites {
			if isMCPToolCall(cs) {
				def := &ToolDef{
					Name:      extractToolNameFromCall(cs),
					HandlerID: cs.CallerID,
					FilePath:  cs.FilePath,
					Line:      cs.Line,
				}
				*tools = append(*tools, def)
			}
		}

		// 2. Detect @tool decorator patterns from ClassInfos
		for _, ci := range f.ClassInfos {
			for _, dec := range ci.Methods {
				if strings.HasPrefix(dec, "tool") || strings.HasPrefix(dec, "@tool") {
					def := &ToolDef{
						Name:      ci.Name + "." + dec,
						HandlerID: ci.NodeID,
						FilePath:  ci.FilePath,
					}
					*tools = append(*tools, def)
				}
			}
		}

		// 3. Detect name+description+inputSchema patterns from Symbols
		for _, sym := range f.Symbols {
			if sym.Props != nil {
				if _, hasName := sym.Props["name"]; hasName {
					if _, hasSchema := sym.Props["inputSchema"]; hasSchema {
						def := &ToolDef{
							Name:        fmt.Sprintf("%v", sym.Props["name"]),
							HandlerID:   sym.NodeID,
							FilePath:    sym.FilePath,
							Line:        sym.StartLine,
							InputSchema: extractInputSchema(sym.Props),
						}
						if desc, ok := sym.Props["description"]; ok {
							def.Description = fmt.Sprintf("%v", desc)
						}
						*tools = append(*tools, def)
					}
				}
			}
		}
	}

	result := make([]*ToolDef, len(*tools))
	copy(result, *tools)
	return result, nil
}

// isMCPToolCall checks if it's an MCP tool() call
func isMCPToolCall(cs *pipeline.CallSite) bool {
	// Match: tool(), server.tool(), defineTool(), registerTool()
	name := cs.Name
	return name == "tool" || name == "defineTool" || name == "registerTool" ||
		(cs.Receiver == "server" && (name == "tool" || name == "addTool")) ||
		strings.HasSuffix(name, "Tool")
}

// extractToolNameFromCall extracts tool name from call site
func extractToolNameFromCall(cs *pipeline.CallSite) string {
	if cs.Receiver != "" {
		return cs.Receiver + "." + cs.Name
	}
	return cs.Name
}

// extractInputSchema extracts inputSchema from properties
func extractInputSchema(props map[string]any) map[string]any {
	if v, ok := props["inputSchema"]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// ============ RPC Tool Detection ============

// RPCToolExtractor detects RPC tool definitions
// Recognizes handler bindings + inputSchema
// Scans registerTool/addTool/server.method registration calls
type RPCToolExtractor struct{}

// NewRPCToolExtractor creates RPC tool detector
func NewRPCToolExtractor() *RPCToolExtractor { return &RPCToolExtractor{} }

// Framework returns framework name
func (e *RPCToolExtractor) Framework() string { return "rpc" }

// DetectTools detects RPC tool definitions
func (e *RPCToolExtractor) DetectTools(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ToolDef, error) {
	tools := acquireToolSlice()
	defer releaseToolSlice(tools)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 1. Detect RPC registration calls from CallSites
		for _, cs := range f.CallSites {
			if isRPCToolRegistration(cs) {
				def := &ToolDef{
					Name:      extractRPCMethodName(cs),
					HandlerID: cs.CallerID,
					FilePath:  cs.FilePath,
					Line:      cs.Line,
				}
				*tools = append(*tools, def)
			}
		}

		// 2. Detect RPC service classes from ClassInfos
		for _, ci := range f.ClassInfos {
			if isRPCServiceClass(ci) {
				for _, method := range ci.Methods {
					def := &ToolDef{
						Name:      ci.Name + "." + method,
						HandlerID: ci.NodeID,
						FilePath:  ci.FilePath,
					}
					*tools = append(*tools, def)
				}
			}
		}
	}

	result := make([]*ToolDef, len(*tools))
	copy(result, *tools)
	return result, nil
}

// isRPCToolRegistration checks if it's an RPC tool registration call
func isRPCToolRegistration(cs *pipeline.CallSite) bool {
	name := cs.Name
	receiver := cs.Receiver
	// Match: registerTool, addTool, server.method, registerHandler, addHandler
	return name == "registerTool" || name == "addTool" ||
		name == "registerHandler" || name == "addHandler" ||
		name == "registerMethod" || name == "addMethod" ||
		(receiver == "server" && (name == "method" || name == "register")) ||
		(receiver == "grpc" && name == "register")
}

// extractRPCMethodName extracts RPC method name from call site
func extractRPCMethodName(cs *pipeline.CallSite) string {
	if cs.Receiver != "" {
		return cs.Receiver + "." + cs.Name
	}
	return cs.Name
}

// isRPCServiceClass checks if it's an RPC service class
func isRPCServiceClass(ci *pipeline.ClassInfo) bool {
	name := ci.Name
	return strings.HasSuffix(name, "Service") ||
		strings.HasSuffix(name, "Handler") ||
		strings.HasSuffix(name, "Impl") ||
		strings.HasSuffix(name, "Server")
}

// ============ ToolDef Registry ============

// ToolDefRegistry tool definition extractor registry
type ToolDefRegistry struct {
	mu         sync.Mutex
	extractors []ToolDefinitionExtractor
}

// NewToolDefRegistry creates tool definition extractor registry
func NewToolDefRegistry() *ToolDefRegistry {
	return &ToolDefRegistry{
		extractors: make([]ToolDefinitionExtractor, 0),
	}
}

// Register registers tool definition extractor
func (r *ToolDefRegistry) Register(extractor ToolDefinitionExtractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors = append(r.extractors, extractor)
}

// DetectAll executes all registered extractors and aggregates tool definitions
func (r *ToolDefRegistry) DetectAll(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ToolDef, error) {
	r.mu.Lock()
	extractors := make([]ToolDefinitionExtractor, len(r.extractors))
	copy(extractors, r.extractors)
	r.mu.Unlock()

	var allTools []*ToolDef
	for _, ext := range extractors {
		select {
		case <-ctx.Done():
			return allTools, ctx.Err()
		default:
		}

		tools, err := ext.DetectTools(ctx, g, files)
		if err != nil {
			return allTools, fmt.Errorf("extractor %s: %w", ext.Framework(), err)
		}
		allTools = append(allTools, tools...)
	}
	return allTools, nil
}

// ============ sync.Pool Reuse ============

var toolSlicePool = sync.Pool{
	New: func() any {
		s := make([]*ToolDef, 0, 32)
		return &s
	},
}

func acquireToolSlice() *[]*ToolDef {
	return toolSlicePool.Get().(*[]*ToolDef)
}

func releaseToolSlice(s *[]*ToolDef) {
	*s = (*s)[:0]
	toolSlicePool.Put(s)
}

// ============ ToolDef Phase ============

// ToolDefPhase tool definition detection pipeline phase
// Implements pipeline.Phase interface
type ToolDefPhase struct {
	registry *ToolDefRegistry
}

// NewToolDefPhase creates tool definition detection phase
func NewToolDefPhase(registry *ToolDefRegistry) *ToolDefPhase {
	return &ToolDefPhase{registry: registry}
}

// Name implements pipeline.Phase
func (p *ToolDefPhase) Name() string { return "tools" }

// Dependencies implements pipeline.Phase
func (p *ToolDefPhase) Dependencies() []string { return []string{"parse"} }

// Run implements pipeline.Phase
func (p *ToolDefPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	tools, err := p.registry.DetectAll(ctx, input.Graph, input.Files)
	if err != nil {
		return nil, err
	}

	nodesAdded, edgesAdded, err := p.persistTools(ctx, input.Graph, tools)
	if err != nil {
		return nil, err
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
		Stats:      map[string]any{"tools": len(tools)},
	}, nil
}

// persistTools batch persists tool definitions to graph store
func (p *ToolDefPhase) persistTools(ctx context.Context, g *graph.GraphStore, tools []*ToolDef) (int, int, error) {
	var nodesAdded, edgesAdded int

	err := g.Batch(func(b *graph.Batch) error {
		for _, t := range tools {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Create Tool node
			toolNode := graph.NewNode(g.Repo(), graph.LabelTool, t.Name).
				WithFile(t.FilePath).
				WithProp("line", t.Line)

			if t.Description != "" {
				toolNode.WithProp("description", t.Description)
			}
			if t.HandlerID != "" {
				toolNode.WithProp("handlerId", t.HandlerID)
			}
			if t.InputSchema != nil {
				toolNode.WithProp("inputSchema", t.InputSchema)
			}

			if err := b.AddNode(toolNode); err != nil {
				return err
			}
			nodesAdded++

			// HANDLES_TOOL: handler → tool
			if t.HandlerID != "" {
				edge := graph.NewEdge(graph.RelHandlesTool, t.HandlerID, toolNode.ID).
					WithProp("confidence", 0.9)
				if err := b.AddEdge(edge); err != nil {
					return err
				}
				edgesAdded++
			}
		}
		return nil
	})

	return nodesAdded, edgesAdded, err
}