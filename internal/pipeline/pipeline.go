package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mengshi02/codetrip/internal/graph"
	"golang.org/x/sync/errgroup"
)

// Phase represents a pipeline phase interface
type Phase interface {
	Name() string
	Dependencies() []string
	Run(ctx context.Context, input *PhaseInput) (*PhaseOutput, error)
}

// PhaseInput represents Phase input
// ImportSemantics defines import semantic strategies
type ImportSemantics string

const (
	ImportSemanticsNamed              ImportSemantics = "named"               // Go/Java: explicit named import
	ImportSemanticsWildcardLeaf       ImportSemantics = "wildcard-leaf"       // Python from xxx import *
	ImportSemanticsWildcardTransitive ImportSemantics = "wildcard-transitive" // JS re-export
	ImportSemanticsNamespace          ImportSemantics = "namespace"           // TS namespace import
)

// Provider is the minimal interface that parse-phase needs from language providers.
// Defined here to avoid circular dependencies (pipeline must not import codetrip).
// The full codetrip.LanguageProvider satisfies this interface.
type Provider interface {
	QuerySet() *LangQuerySet
	InterpretScope(captures []LangCapture, source []byte, filePath string) []*ScopeInfo
	InterpretDeclaration(captures []LangCapture, source []byte, filePath string) []*SymbolInfo
	InterpretImport(captures []LangCapture, source []byte, filePath string) []*ImportInfo
	InterpretTypeBinding(captures []LangCapture, source []byte, filePath string) []*TypeBindingInfo
	InterpretReference(captures []LangCapture, source []byte, filePath string) []*ReferenceInfo
}

type PhaseInput struct {
	Repo          string
	Graph         *graph.GraphStore
	Files         []*ParsedFile
	SemanticModel *SemanticModel        // Read-only semantic model (used by read phases)
	MutableModel  *MutableSemanticModel // Mutable semantic model (write phases: parse, scopeResolution)
	Config        PipelineConfig
	Providers     map[string]Provider // Language name → Provider (populated by Trip)
}

// PhaseOutput represents Phase output
type PhaseOutput struct {
	NodesAdded   int
	EdgesAdded   int
	Stats        map[string]any
	Files        []*ParsedFile // Files to pass to downstream phases; nil means "no update" (preserve upstream)
	FilesUpdated bool          // true means Files was explicitly set (even if empty); false means no update
}

// PipelineConfig represents pipeline configuration
type PipelineConfig struct {
	RepoPath      string
	TripDir       string // Trip directory for persistent indexes (BM25, vector, etc.)
	MaxWorkers    int
	ByteBudget    int64
	WithCFG       bool
	WithPDG       bool
	BM25ChunkSize int // BM25 batch chunk size for large repos (0 = auto 10000)
}

// Pipeline is the pipeline engine
// High-performance design: topological sort + parallel execution of independent phases
type Pipeline struct {
	mu     sync.RWMutex
	phases map[string]Phase
	order  []string // Topological order
	dirty  atomic.Bool
}

// NewPipeline creates a pipeline
func NewPipeline() *Pipeline {
	return &Pipeline{
		phases: make(map[string]Phase),
	}
}

// Register registers a Phase
func (p *Pipeline) Register(phase Phase) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.phases[phase.Name()] = phase
	p.dirty.Store(true)
}

// Run executes the pipeline (topological layer parallel execution)
// Phases in the same layer with no dependencies execute in parallel; different layers execute sequentially
func (p *Pipeline) Run(ctx context.Context, input *PhaseInput) error {
	slog.Info("pipeline: starting", "repo", input.Repo)
	p.ensureOrder()

	// Set of write phase names (these phases can write to MutableModel)
	writePhases := map[string]bool{
		"parse":           true,
		"crossFile":       true,
		"scopeResolution": true,
	}
	finalizePhase := "finalize"

	// Divide topological sort results into layers (same layer can be parallelized)
	layers := p.computeLayers()

	for _, layer := range layers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if len(layer) == 1 {
			// Single Phase, execute directly
			if err := p.runPhase(ctx, input, layer[0], writePhases, finalizePhase); err != nil {
				return err
			}
		} else {
			// Execute phases in the same layer in parallel
			g, gCtx := errgroup.WithContext(ctx)
			for _, name := range layer {
				name := name
				g.Go(func() error {
					return p.runPhase(gCtx, input, name, writePhases, finalizePhase)
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	slog.Info("pipeline: completed", "repo", input.Repo)
	return nil
}
func (p *Pipeline) runPhase(ctx context.Context, input *PhaseInput, name string, writePhases map[string]bool, finalizePhase string) error {
	phase, ok := p.phases[name]
	if !ok {
		return nil
	}

	start := time.Now()
	output, err := phase.Run(ctx, input)
	if err != nil {
		return fmt.Errorf("phase %s: %w", name, err)
	}

	// Pass output to downstream
	if output != nil && output.FilesUpdated {
		input.Files = output.Files // explicitly set, even if empty
	}

	// Record phase statistics
	stat := PhaseStat{
		Duration:   time.Since(start),
		NodesAdded: output.NodesAdded,
		EdgesAdded: output.EdgesAdded,
	}
	if output != nil && output.Stats != nil {
		stat.Extra = output.Stats
	}

	// Write phase: write to MutableModel
	if input.MutableModel != nil && writePhases[name] {
		if err := input.MutableModel.RecordPhaseStat(name, stat); err != nil {
			slog.Warn("record phase stat failed", "phase", name, "error", err)
		}
	}
	// Compatibility: also write to SemanticModel.PhaseStats
	if input.SemanticModel != nil {
		input.SemanticModel.phaseStatsMu.Lock()
		input.SemanticModel.PhaseStats[name] = stat
		input.SemanticModel.phaseStatsMu.Unlock()
	}

	slog.Debug("pipeline phase completed",
		"phase", name,
		"nodes_added", output.NodesAdded,
		"edges_added", output.EdgesAdded,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	// Finalize phase: freeze MutableModel → SemanticModel
	if name == finalizePhase && input.MutableModel != nil {
		sm, err := input.MutableModel.Freeze()
		if err != nil {
			return fmt.Errorf("freeze semantic model: %w", err)
		}
		input.SemanticModel = sm
		input.MutableModel = nil
	}

	return nil
}

// computeLayers divides topological sort results into parallelizable layers
func (p *Pipeline) computeLayers() [][]string {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for name := range p.phases {
		inDegree[name] = 0
	}
	for name, phase := range p.phases {
		for _, dep := range phase.Dependencies() {
			if _, ok := p.phases[dep]; ok {
				adj[dep] = append(adj[dep], name)
				inDegree[name]++
			}
		}
	}

	var layers [][]string
	for {
		// Find all nodes with in-degree 0 (current layer)
		var layer []string
		for name, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			break
		}
		sort.Strings(layer)
		layers = append(layers, layer)

		// Remove current layer nodes and update in-degrees
		for _, name := range layer {
			delete(inDegree, name)
			for _, dep := range adj[name] {
				inDegree[dep]--
			}
		}
	}

	return layers
}

// ensureOrder ensures topological order (lazy computation)
func (p *Pipeline) ensureOrder() {
	if !p.dirty.Load() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.dirty.Load() {
		return
	}

	p.order = p.topologicalSort()
	p.dirty.Store(false)
}

// topologicalSort performs topological sorting (Kahn's algorithm)
func (p *Pipeline) topologicalSort() []string {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for name := range p.phases {
		inDegree[name] = 0
	}

	for name, phase := range p.phases {
		for _, dep := range phase.Dependencies() {
			if _, ok := p.phases[dep]; ok {
				adj[dep] = append(adj[dep], name)
				inDegree[name]++
			}
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // Deterministic order

	var result []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, name)

		deps := adj[name]
		sort.Strings(deps)
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	return result
}

// ============ Helper Types ============

// ParsedFile represents a parsed file
type ParsedFile struct {
	Path        string
	Language    string
	Content     []byte
	ContentHash string
	Size        int64
	Symbols     []*SymbolInfo
	Imports     []*ImportInfo
	CallSites   []*CallSite
	ClassInfos  []*ClassInfo
	FieldInfos  []*FieldInfo
	NodeIDs     []string // Node IDs created by this file

	// Scope-based pipeline types
	Scopes       []*ScopeInfo       // Nested scope tree
	TypeBindings []*TypeBindingInfo // Type bindings
	References   []*ReferenceInfo   // Classified references (supersedes CallSite)
}

// SymbolInfo represents symbol information
type SymbolInfo struct {
	Name      string
	Label     graph.Label
	FilePath  string
	StartLine int
	EndLine   int
	NodeID    string
	Props     map[string]any

	// Structured fields (replace ad-hoc Props for type safety)
	QualifiedName string // Fully qualified name with namespace
	Visibility    string // "public" | "private" | "protected" | "internal"
	IsStatic      bool
	IsAbstract    bool
	IsFinal       bool
	IsVirtual     bool
	IsOverride    bool
	IsAsync       bool
	Parameters    []ParamInfo // Parameter list
	ReturnType    string      // Return type
	Annotations   []string    // Decorators / annotations
	ScopeID       string      // Owning scope ID
}

// ImportInfo represents import information
type ImportInfo struct {
	Path       string
	SourceFile string
	Symbols    []string // Imported symbol names
	IsWildcard bool
	Alias      string
	Line       int
}

// CallSite represents a call site
type CallSite struct {
	Name     string
	Receiver string // Receiver of method call
	Args     int    // Number of arguments
	FilePath string
	Line     int
	CallerID string // Caller node ID
}

// ClassInfo represents class information
type ClassInfo struct {
	Name          string
	Methods       []string
	Fields        []string
	IsAbstract    bool
	Parents       []string // Inherited parent class names
	Implements    []string // Implemented interface names
	FilePath      string
	NodeID        string
	QualifiedName string   // Fully qualified name with namespace
	TemplateArgs  []string // Generic type arguments
	ScopeID       string   // Owning scope ID
}

// FieldInfo represents field information
type FieldInfo struct {
	Name       string
	ClassName  string
	IsStatic   bool
	FilePath   string
	NodeID     string
	TypeName   string // Field type
	Visibility string // "public" | "private" | "protected"
	IsReadonly bool
}

// ============ Scope-Based Pipeline Types ============

// ScopeInfo represents a nested scope node in the scope tree
type ScopeInfo struct {
	ID         string // Unique identifier
	Kind       string // "module" | "class" | "function" | "method" | "block" | "loop" | "closure"
	Name       string // Scope name (function name / class name / empty for blocks)
	ParentID   string // Parent scope ID (empty for root)
	FilePath   string
	StartLine  int
	EndLine    int
	Symbols    []string // Symbol NodeIDs defined in this scope
	References []string // Reference NodeIDs within this scope
}

// TypeBindingInfo represents type binding information
type TypeBindingInfo struct {
	Kind       string   // "parameter" | "return" | "variable" | "field" | "alias" | "receiver" | "constructor"
	TypeName   string   // Type name
	TypeArgs   []string // Generic type arguments
	BoundNode  string   // Symbol NodeID this type is bound to
	ScopeID    string   // Owning scope ID
	FilePath   string
	StartLine  int
	IsOptional bool
	IsVariadic bool
}

// ReferenceInfo represents a classified reference (supersedes CallSite)
type ReferenceInfo struct {
	Kind           string   // "free_call" | "member_call" | "constructor" | "macro" | "field_read" | "field_write"
	Name           string   // Referenced name
	Receiver       string   // Receiver expression (empty for free_call)
	ReceiverType   string   // Receiver type reference (filled after type-binding)
	Arity          int      // Number of arguments
	Args           []string // Argument expressions
	EnclosingScope string   // Owning scope ID
	FilePath       string
	StartLine      int
}

// ParamInfo represents a function/method parameter
type ParamInfo struct {
	Name       string
	Type       string
	IsOptional bool
	IsVariadic bool
	Default    string
}

// PhaseStat represents Phase statistics
type PhaseStat struct {
	Duration   time.Duration
	NodesAdded int
	EdgesAdded int
	Extra      map[string]any // Additional statistics (e.g., fileCount)
}

// FileInfo represents file information (produced by scan phase)
type FileInfo struct {
	Path     string
	Language string
	Size     int64
	Hash     string
}
