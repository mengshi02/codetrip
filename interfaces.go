package codetrip

import (
	"context"
	"fmt"

	"github.com/mengshi02/codetrip/internal/collection"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/odvcencio/gotreesitter"
)

// ============ Extension Interfaces ============

// ImportSemantics defines import semantic strategies
type ImportSemantics = collection.ImportSemantics

const (
	ImportSemanticsNamed              = collection.ImportSemanticsNamed
	ImportSemanticsWildcardLeaf       = collection.ImportSemanticsWildcardLeaf
	ImportSemanticsWildcardTransitive = collection.ImportSemanticsWildcardTransitive
	ImportSemanticsNamespace          = collection.ImportSemanticsNamespace
)

// LanguageProvider is the language provider interface.
// It defines S-expression queries for structured extraction and interpret
// functions that synthesize typed results from tree-sitter captures.
//
// Providers can implement the minimal interface (QuerySet only) and the
// default SharedInterpreter will handle capture-to-struct conversion,
// or implement InterpretXxx methods for language-specific synthesis logic.
//
// Providers may additionally implement ScopeCaptureProvider (ParseFile method)
// to use runtime AST synthesis instead of the legacy query-execute-and-interpret
// path. When ParseFile is present, the parse phase calls it directly and skips
// the five separate QuerySet+InterpretXxx steps.
type LanguageProvider interface {
	Language() graph.Label

	// QuerySet returns all tree-sitter S-expression queries for this language.
	// Each query targets a specific extraction dimension (scope, declaration,
	// import, type-binding, reference). The parse phase executes each query
	// against the same AST and passes captures to the corresponding Interpret method.
	//
	// NOTE: Providers that implement ScopeCaptureProvider (ParseFile) may return
	// an empty/minimal QuerySet since the parse phase will call ParseFile directly
	// and skip the query+interpret flow.
	QuerySet() *collection.LangQuerySet

	// InterpretScope synthesizes ScopeInfo from scope query captures.
	InterpretScope(captures []collection.LangCapture, source []byte, filePath string) []*collection.ScopeInfo
	// InterpretDeclaration synthesizes SymbolInfo from declaration query captures.
	InterpretDeclaration(captures []collection.LangCapture, source []byte, filePath string) []*collection.SymbolInfo
	// InterpretImport synthesizes ImportInfo from import query captures.
	InterpretImport(captures []collection.LangCapture, source []byte, filePath string) []*collection.ImportInfo
	// InterpretTypeBinding synthesizes TypeBindingInfo from type-binding query captures.
	InterpretTypeBinding(captures []collection.LangCapture, source []byte, filePath string) []*collection.TypeBindingInfo
	// InterpretReference synthesizes ReferenceInfo from reference query captures.
	InterpretReference(captures []collection.LangCapture, source []byte, filePath string) []*collection.ReferenceInfo

	// TreeSitterLanguage returns the tree-sitter Language for this provider.
	TreeSitterLanguage() *gotreesitter.Language

	ImportSemantics() ImportSemantics
}

// ScopeCaptureProvider is an optional interface that language providers may
// implement to use runtime AST synthesis (emitScopeCaptures pattern) instead
// of the legacy QuerySet + InterpretXxx pattern.
//
// When a provider implements this interface, the parse phase calls ParseFile
// directly and skips the query-execute-and-interpret flow entirely.
// This mirrors GitNexus's emitScopeCaptures hook — a provider with ParseFile
// does its own AST walk and capture synthesis internally.
type ScopeCaptureProvider interface {
	LanguageProvider
	ParseFile(f *collection.ParsedFile)
}

// BindingSet is re-exported from the pipeline package.
// It represents a set of bindings (symbol name → candidate node ID list).
type BindingSet = collection.BindingSet

// NewBindingSet creates an empty binding set (delegates to collection.NewBindingSet)
var NewBindingSet = collection.NewBindingSet

// ============ Concrete Type Aliases (replacing interface{} in ScopeResolver) ============

// ScopeModel is the concrete type for semantic model parameters.
// Providers should type-assert to *collection.MutableSemanticModel or *collection.SemanticModel as needed.
type ScopeModel = collection.MutableSemanticModel

// ImportRef is the concrete type for import parameters.
// Providers should type-assert to *collection.ImportInfo or *collection.ImportEntry as needed.
type ImportRef = collection.ImportInfo

// CallSiteRef is the concrete type for call site parameters.
type CallSiteRef = collection.CallSite

// FileSet is the concrete type for file list parameters.
type FileSet = []*collection.ParsedFile

// IndexSet is the concrete type for scope resolution index parameters.
type IndexSet = collection.ScopeResolutionIndexes

// ScopeMapType is the concrete type for scope map parameters.
type ScopeMapType = collection.ScopeResolutionIndexes

// ============ ScopeResolver — Split into composed interfaces (ISP) ============

// CoreResolver contains the core scope resolution methods (5 methods).
// All language providers must implement this interface.
type CoreResolver interface {
	Language() graph.Label
	LanguageProvider() LanguageProvider
	PopulateOwners(graphStore *graph.GraphStore, model *ScopeModel) error
	BuildMRO(graphStore *graph.GraphStore, classes []*graph.Node) error
	ResolveImportTarget(graphStore *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error)
}

// BindingResolver contains binding and compatibility methods (4 methods).
type BindingResolver interface {
	MergeBindings(local, imported *BindingSet) *BindingSet
	ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool
	ImportEdgeReason(imp *ImportRef) string
	IsSuperReceiver(recv string) bool
}

// HookResolver contains boolean hook switches (6 methods).
type HookResolver interface {
	PropagatesReturnTypesAcrossImports() bool  // default true
	FieldFallbackOnMethodLookup() bool          // default true (statically typed languages should disable)
	UnwrapCollectionAccessor() bool             // unwrap collection accessors
	CollapseMemberCallsByCallerTarget() bool    // one edge per caller-target
	PopulateNamespaceSiblings() bool            // cross-file implicit visibility
	HoistTypeBindingsToModule() bool            // hoist method return types to module
}

// EmitResolver contains function-type hook methods (5 methods).
type EmitResolver interface {
	PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext)
	CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{}
	EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext)
	EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int
}

// ScopeResolver is the full composed interface for backward compatibility.
// It composes CoreResolver + BindingResolver + HookResolver + EmitResolver.
// New code should prefer depending on the smallest interface needed.
type ScopeResolver interface {
	CoreResolver
	BindingResolver
	HookResolver
	EmitResolver
}

// RangeBindContext provides context for for-range variable binding (re-exported from pipeline)
type RangeBindContext = collection.RangeBindContext

// ScopeContextOptions provides options for scope context (re-exported from pipeline)
type ScopeContextOptions = collection.ScopeContextOptions

// EmitContext provides context for post-processing edge emission (re-exported from pipeline)
type EmitContext = collection.EmitContext

// GraphNodeLookup is a function type for looking up graph nodes (re-exported from pipeline)
type GraphNodeLookup = collection.GraphNodeLookup

// ============ Generic Tool Interface (1.2) ============

// Tool is the analysis tool interface (non-generic backward-compatible version).
// For new tools, prefer GenericTool[T, R] which provides compile-time type safety.
type Tool interface {
	Name() string
	Run(ctx context.Context, t *Trip, req interface{}) (interface{}, error)
}

// GenericTool is a type-safe tool interface using generics.
// T is the request type, R is the response type.
type GenericTool[T any, R any] interface {
	Name() string
	Run(ctx context.Context, t *Trip, req T) (R, error)
}

// ToolAdapter wraps a GenericTool to implement the legacy Tool interface.
// This allows gradual migration from Tool to GenericTool.
type ToolAdapter[T any, R any] struct {
	Impl GenericTool[T, R]
}

func (a *ToolAdapter[T, R]) Name() string { return a.Impl.Name() }

func (a *ToolAdapter[T, R]) Run(ctx context.Context, t *Trip, req interface{}) (interface{}, error) {
	typedReq, ok := req.(T)
	if !ok {
		return nil, fmt.Errorf("%w: expected %T, got %T", ErrInvalidRequest, typedReq, req)
	}
	return a.Impl.Run(ctx, t, typedReq)
}

// ============ Embedder Interface (1.3) ============

// Embedder is the embedding model interface
type Embedder interface {
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	EmbedBatch(ctx context.Context, nodes []*graph.Node, config EmbedConfig) error
}

// EmbedConfig represents embedding configuration
type EmbedConfig struct {
	ModelID          string
	Dimensions       int
	BatchSize        int
	SubBatchSize     int
	MaxSnippetLength int
	ChunkSize        int
	Overlap          int
}

// ContractType represents contract types
type ContractType string

const (
	ContractHTTP    ContractType = "http"
	ContractGRPC    ContractType = "grpc"
	ContractThrift  ContractType = "thrift"
	ContractTopic   ContractType = "topic"
	ContractLib     ContractType = "lib"
	ContractCustom  ContractType = "custom"
	ContractInclude ContractType = "include"
)

// Contract is the typed contract result from extraction
type Contract interface {
	ContractID() string
	ContractType() ContractType
}

// ContractExtractor is the contract extractor interface
type ContractExtractor interface {
	ContractType() ContractType
	Extract(ctx context.Context, repo string, graphStore *graph.GraphStore) ([]Contract, error)
}

// NoopEmbedder is a no-op embedder implementation
type NoopEmbedder struct{}

func (n *NoopEmbedder) Dimensions() int { return 0 }
func (n *NoopEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}
func (n *NoopEmbedder) EmbedBatch(_ context.Context, _ []*graph.Node, _ EmbedConfig) error {
	return nil
}