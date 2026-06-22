package orm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ORMQueryExtractor ORM query extraction interface
type ORMQueryExtractor interface {
	// Framework returns the ORM framework name for the extractor
	Framework() string
	// ExtractQueries extracts ORM queries from parsed files
	ExtractQueries(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ORMQuery, error)
}

// ORMQuery ORM query
type ORMQuery struct {
	Model     string // data model name
	Operation string // findMany/create/update/delete/upsert
	CallerID  string // caller node ID
	TargetID  string // queried model node ID
	FilePath  string
	Line      int
}

// ============ Prisma Query Detection ============

// PrismaQueryExtractor detects Prisma ORM queries
// Detects prisma.xxx.findMany()/findFirst()/create()/update()/delete()/$queryRaw()/$executeRaw()
// Extracts from call chain: prisma → model → operation
type PrismaQueryExtractor struct{}

// NewPrismaQueryExtractor creates Prisma query detector
func NewPrismaQueryExtractor() *PrismaQueryExtractor { return &PrismaQueryExtractor{} }

// Framework returns framework name
func (e *PrismaQueryExtractor) Framework() string { return "prisma" }

// DetectTools → ExtractQueries
// Detects Prisma query patterns
func (e *PrismaQueryExtractor) ExtractQueries(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ORMQuery, error) {
	queries := acquireQuerySlice()
	defer releaseQuerySlice(queries)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		for _, cs := range f.CallSites {
			if isPrismaQuery(cs) {
				model, operation := parsePrismaCall(cs)
				if model != "" && operation != "" {
					q := &ORMQuery{
						Model:     model,
						Operation: operation,
						CallerID:  cs.CallerID,
						FilePath:  cs.FilePath,
						Line:      cs.Line,
					}

					// Try to find model node from graph
					nodes, err := g.GetNodesByName(g.Repo(), model)
					if err == nil && len(nodes) > 0 {
						q.TargetID = nodes[0].ID
					}

					*queries = append(*queries, q)
				}
			}
		}
	}

	result := make([]*ORMQuery, len(*queries))
	copy(result, *queries)
	return result, nil
}

// isPrismaQuery checks if it's a Prisma query call
func isPrismaQuery(cs *pipeline.CallSite) bool {
	return cs.Receiver == "prisma" || strings.HasPrefix(cs.Receiver, "prisma.")
}

// parsePrismaCall extracts model and operation from Prisma call chain
// Call chain pattern: prisma → model → operation
// e.g., prisma.user.findMany() → model=user, operation=findMany
func parsePrismaCall(cs *pipeline.CallSite) (model, operation string) {
	operation = cs.Name

	// Prisma operation list
	prismaOps := map[string]bool{
		"findMany": true, "findFirst": true, "findUnique": true,
		"create": true, "update": true, "delete": true,
		"upsert": true, "count": true, "aggregate": true,
		"groupBy": true, "$queryRaw": true, "$executeRaw": true,
	}

	if !prismaOps[operation] {
		return "", ""
	}

	// Extract model from Receiver
	// Receiver can be "prisma" (for $queryRaw etc.) or "prisma.user" etc.
	receiver := cs.Receiver
	if strings.HasPrefix(receiver, "prisma.") {
		model = strings.TrimPrefix(receiver, "prisma.")
		// Handle nesting: prisma.user.todos → model=user (take first)
		if idx := strings.Index(model, "."); idx >= 0 {
			model = model[:idx]
		}
	}

	return model, operation
}

// ============ Supabase Query Detection ============

// SupabaseQueryExtractor detects Supabase ORM queries
// Detects supabase.from('table').select()/insert()/update()/delete()/upsert()
// Extracts from call chain: supabase → from(table) → operation
type SupabaseQueryExtractor struct{}

// NewSupabaseQueryExtractor creates Supabase query detector
func NewSupabaseQueryExtractor() *SupabaseQueryExtractor { return &SupabaseQueryExtractor{} }

// Framework returns framework name
func (e *SupabaseQueryExtractor) Framework() string { return "supabase" }

// ExtractQueries detects Supabase queries
func (e *SupabaseQueryExtractor) ExtractQueries(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ORMQuery, error) {
	queries := acquireQuerySlice()
	defer releaseQuerySlice(queries)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		for _, cs := range f.CallSites {
			if isSupabaseQuery(cs) {
				model, operation := parseSupabaseCall(cs)
				if model != "" && operation != "" {
					q := &ORMQuery{
						Model:     model,
						Operation: operation,
						CallerID:  cs.CallerID,
						FilePath:  cs.FilePath,
						Line:      cs.Line,
					}

					// Try to find model node from graph
					nodes, err := g.GetNodesByName(g.Repo(), model)
					if err == nil && len(nodes) > 0 {
						q.TargetID = nodes[0].ID
					}

					*queries = append(*queries, q)
				}
			}
		}
	}

	result := make([]*ORMQuery, len(*queries))
	copy(result, *queries)
	return result, nil
}

// isSupabaseQuery checks if it's a Supabase query call
func isSupabaseQuery(cs *pipeline.CallSite) bool {
	return cs.Receiver == "supabase" || strings.HasPrefix(cs.Receiver, "supabase.")
}

// parseSupabaseCall extracts model and operation from Supabase call chain
// Call chain pattern: supabase → from(table) → operation
// e.g., supabase.from('users').select() → model=users, operation=select
func parseSupabaseCall(cs *pipeline.CallSite) (model, operation string) {
	// Supabase operation list
	supabaseOps := map[string]bool{
		"select": true, "insert": true, "update": true,
		"delete": true, "upsert": true,
	}

	name := cs.Name
	if supabaseOps[name] {
		operation = name
	} else if name == "from" {
		// from() call itself is not a query, but marks model
		// Real operation is in subsequent chain call
		return "", ""
	} else {
		return "", ""
	}

	// Extract model from Receiver
	// Receiver format can be "supabase" or "supabase.from('users')"
	receiver := cs.Receiver
	if strings.Contains(receiver, "from(") {
		// Try to extract table name from from('...')
		start := strings.Index(receiver, "from(")
		if start >= 0 {
			sub := receiver[start+5:]
			// Remove quotes
			sub = strings.TrimLeft(sub, "'\"`")
			if idx := strings.IndexAny(sub, "'\"`)"); idx >= 0 {
				model = sub[:idx]
			}
		}
	}

	return model, operation
}

// ============ ORM Registry ============

// ORMRegistry ORM query extractor registry
type ORMRegistry struct {
	mu         sync.Mutex
	extractors []ORMQueryExtractor
}

// NewORMRegistry creates ORM query extractor registry
func NewORMRegistry() *ORMRegistry {
	return &ORMRegistry{
		extractors: make([]ORMQueryExtractor, 0),
	}
}

// Register registers ORM query extractor
func (r *ORMRegistry) Register(extractor ORMQueryExtractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors = append(r.extractors, extractor)
}

// ExtractAll executes all registered extractors and aggregates queries
func (r *ORMRegistry) ExtractAll(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*ORMQuery, error) {
	r.mu.Lock()
	extractors := make([]ORMQueryExtractor, len(r.extractors))
	copy(extractors, r.extractors)
	r.mu.Unlock()

	var allQueries []*ORMQuery
	for _, ext := range extractors {
		select {
		case <-ctx.Done():
			return allQueries, ctx.Err()
		default:
		}

		queries, err := ext.ExtractQueries(ctx, g, files)
		if err != nil {
			return allQueries, fmt.Errorf("extractor %s: %w", ext.Framework(), err)
		}
		allQueries = append(allQueries, queries...)
	}
	return allQueries, nil
}

// ============ sync.Pool Reuse ============

var querySlicePool = sync.Pool{
	New: func() any {
		s := make([]*ORMQuery, 0, 32)
		return &s
	},
}

func acquireQuerySlice() *[]*ORMQuery {
	return querySlicePool.Get().(*[]*ORMQuery)
}

func releaseQuerySlice(s *[]*ORMQuery) {
	*s = (*s)[:0]
	querySlicePool.Put(s)
}

// ============ ORM Phase ============

// ORMPhase ORM query detection pipeline phase
// Implements pipeline.Phase interface
type ORMPhase struct {
	registry *ORMRegistry
}

// NewORMPhase creates ORM query detection phase
func NewORMPhase(registry *ORMRegistry) *ORMPhase {
	return &ORMPhase{registry: registry}
}

// Name implements pipeline.Phase
func (p *ORMPhase) Name() string { return "orm" }

// Dependencies implements pipeline.Phase
func (p *ORMPhase) Dependencies() []string { return []string{"parse"} }

// Run implements pipeline.Phase
func (p *ORMPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	queries, err := p.registry.ExtractAll(ctx, input.Graph, input.Files)
	if err != nil {
		return nil, err
	}

	nodesAdded, edgesAdded, err := p.persistQueries(ctx, input.Graph, queries)
	if err != nil {
		return nil, err
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
		Stats:      map[string]any{"queries": len(queries)},
	}, nil
}

// persistQueries batch persists ORM queries to graph store
// Creates QUERIES edge: caller → target model
func (p *ORMPhase) persistQueries(ctx context.Context, g *graph.GraphStore, queries []*ORMQuery) (int, int, error) {
	var nodesAdded, edgesAdded int

	err := g.Batch(func(b *graph.Batch) error {
		for _, q := range queries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// If target model node doesn't exist, create a placeholder model node
			if q.TargetID == "" && q.Model != "" {
				modelNode := graph.NewNode(g.Repo(), graph.LabelRecord, q.Model).
					WithFile(q.FilePath).
					WithProp("ormModel", true)
				if err := b.AddNode(modelNode); err != nil {
					return err
				}
				nodesAdded++
				q.TargetID = modelNode.ID
			}

			// QUERIES: caller → target model
			if q.CallerID != "" && q.TargetID != "" {
				edge := graph.NewEdge(graph.RelQueries, q.CallerID, q.TargetID).
					WithProp("operation", q.Operation).
					WithProp("model", q.Model).
					WithProp("line", q.Line).
					WithProp("confidence", 0.85)
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
