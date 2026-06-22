package route

import (
	"context"
	"fmt"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// RouteExtractor route extractor interface
type RouteExtractor interface {
	// Framework returns the framework name
	Framework() string
	// Extract extracts route information from parsed files
	Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error)
}

// Route route information
type Route struct {
	Path         string
	Method       string
	HandlerID    string
	Middleware   []string
	ResponseKeys []string
	ErrorKeys    []string
	Consumers    []RouteConsumer
	FilePath     string
	Line         int
}

// RouteConsumer route consumer (fetch caller)
type RouteConsumer struct {
	FunctionID string
	FilePath   string
}

// RouteRegistry route extractor registry
type RouteRegistry struct {
	mu         sync.Mutex
	extractors []RouteExtractor
}

// NewRouteRegistry creates a route extractor registry
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		extractors: make([]RouteExtractor, 0),
	}
}

// Register registers a route extractor
func (r *RouteRegistry) Register(extractor RouteExtractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors = append(r.extractors, extractor)
}

// ExtractAll executes all registered extractors and aggregates routes
func (r *RouteRegistry) ExtractAll(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	r.mu.Lock()
	extractors := make([]RouteExtractor, len(r.extractors))
	copy(extractors, r.extractors)
	r.mu.Unlock()

	var allRoutes []*Route
	for _, ext := range extractors {
		select {
		case <-ctx.Done():
			return allRoutes, ctx.Err()
		default:
		}

		routes, err := ext.Extract(ctx, g, files)
		if err != nil {
			return allRoutes, fmt.Errorf("extractor %s: %w", ext.Framework(), err)
		}
		allRoutes = append(allRoutes, routes...)
	}
	return allRoutes, nil
}

// ============ sync.Pool Reuse ============

var routeSlicePool = sync.Pool{
	New: func() any {
		s := make([]*Route, 0, 32)
		return &s
	},
}

func acquireRouteSlice() *[]*Route {
	return routeSlicePool.Get().(*[]*Route)
}

func releaseRouteSlice(s *[]*Route) {
	*s = (*s)[:0]
	routeSlicePool.Put(s)
}

// ============ Route Phase ============

// RoutePhase 路由提取管线阶段
type RoutePhase struct {
	registry *RouteRegistry
}

// NewRoutePhase 创建路由提取阶段
func NewRoutePhase(registry *RouteRegistry) *RoutePhase {
	return &RoutePhase{registry: registry}
}

// Name 实现 pipeline.Phase
func (p *RoutePhase) Name() string { return "route" }

// Dependencies 实现 pipeline.Phase
func (p *RoutePhase) Dependencies() []string { return []string{"parse"} }

// Run 实现 pipeline.Phase
func (p *RoutePhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	routes, err := p.registry.ExtractAll(ctx, input.Graph, input.Files)
	if err != nil {
		return nil, err
	}

	nodesAdded, edgesAdded, err := p.persistRoutes(ctx, input.Graph, routes)
	if err != nil {
		return nil, err
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
	}, nil
}

// persistRoutes 批量持久化路由到图存储
func (p *RoutePhase) persistRoutes(ctx context.Context, g *graph.GraphStore, routes []*Route) (int, int, error) {
	var nodesAdded, edgesAdded int

	err := g.Batch(func(b *graph.Batch) error {
		for _, r := range routes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			routeNode := graph.NewNode(g.Repo(), graph.LabelRoute, r.Method+" "+r.Path).
				WithFile(r.FilePath).
				WithProp("path", r.Path).
				WithProp("method", r.Method).
				WithProp("line", r.Line)

			if r.HandlerID != "" {
				routeNode.WithProp("handlerId", r.HandlerID)
			}
			if len(r.Middleware) > 0 {
				routeNode.WithProp("middleware", r.Middleware)
			}
			if len(r.ResponseKeys) > 0 {
				routeNode.WithProp("responseKeys", r.ResponseKeys)
			}
			if len(r.ErrorKeys) > 0 {
				routeNode.WithProp("errorKeys", r.ErrorKeys)
			}

			if err := b.AddNode(routeNode); err != nil {
				return err
			}
			nodesAdded++

			// HANDLES_ROUTE: handler → route
			if r.HandlerID != "" {
				edge := graph.NewEdge(graph.RelHandlesRoute, r.HandlerID, routeNode.ID).
					WithProp("method", r.Method)
				if err := b.AddEdge(edge); err != nil {
					return err
				}
				edgesAdded++
			}

			// FETCHES: consumer → route
			for _, consumer := range r.Consumers {
				if consumer.FunctionID != "" {
					edge := graph.NewEdge(graph.RelFetches, consumer.FunctionID, routeNode.ID)
					if err := b.AddEdge(edge); err != nil {
						return err
					}
					edgesAdded++
				}
			}
		}
		return nil
	})

	return nodesAdded, edgesAdded, err
}