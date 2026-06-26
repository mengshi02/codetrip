package core

import "github.com/odvcencio/gotreesitter"

// ClassLikeNodeLabel represents the set of class-like node labels.
// Mirrors TS's Extract<NodeLabel, 'Class'|'Struct'|'Interface'|'Enum'|'Record'>.
type ClassLikeNodeLabel string

const (
	NodeLabelClass     ClassLikeNodeLabel = "Class"
	NodeLabelStruct    ClassLikeNodeLabel = "Struct"
	NodeLabelInterface ClassLikeNodeLabel = "Interface"
	NodeLabelEnum      ClassLikeNodeLabel = "Enum"
	NodeLabelRecord    ClassLikeNodeLabel = "Record"
)

// ExtractedClassSymbol is the result of extracting a class-like declaration.
type ExtractedClassSymbol struct {
	Name             string
	Type             ClassLikeNodeLabel
	QualifiedName    string
	TemplateArguments []string // nil = absent (TS optional)
}

// ClassCaptureContext holds the AST capture map and definition/name nodes
// for a class-like query match.
type ClassCaptureContext struct {
	CaptureMap     map[string]*gotreesitter.Node
	DefinitionNode *gotreesitter.Node // nil = null
	NameNode       *gotreesitter.Node // nil = undefined
}

// ClassExtractFallback provides optional name and type hints when the
// primary extraction path cannot determine them from the node alone.
// Replaces TS's optional { name?: string; type?: NodeLabel | null } parameter.
type ClassExtractFallback struct {
	Name *string
	Type *ClassLikeNodeLabel // nil = null (different from absent)
}

// ClassExtractor extracts class-like declarations from AST nodes.
// Implementations are per-language, produced by createClassExtractor().
type ClassExtractor interface {
	// Language returns the language this extractor handles.
	Language() SupportedLanguage

	// QualifiedNodeId: when true, this language's nested-type graph nodes are keyed
	// by their fully-qualified path (e.g. "Class:file:Outer.Inner") instead of the
	// simple tail name, so same-tail nested types in one file stay distinct (#1978).
	QualifiedNodeId() bool

	// IsTypeDeclaration checks whether the node is a type declaration.
	IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool

	// Extract extracts a class symbol from the node, optionally using fallback hints.
	Extract(node *gotreesitter.Node, fallback *ClassExtractFallback, source []byte, lang *gotreesitter.Language) *ExtractedClassSymbol

	// ExtractQualifiedName builds the qualified name for a type declaration node.
	ExtractQualifiedName(node *gotreesitter.Node, simpleName string, source []byte, lang *gotreesitter.Language) *string

	// QualifyScopeName qualifies a scope-defining node that maps to a class-like
	// registry label (e.g. a Ruby module → Trait) but is NOT a typeDeclaration.
	// Optional — only providers that materialize such nodes implement it.
	// Returns empty string when not implemented; callers check for non-empty.
	QualifyScopeName(node *gotreesitter.Node, simpleName string, source []byte, lang *gotreesitter.Language) string

	// ShouldSkipClassCapture optionally filters class captures that should be skipped.
	// Returns false when not implemented.
	ShouldSkipClassCapture(ctx *ClassCaptureContext, nodeLabel ClassLikeNodeLabel, lang *gotreesitter.Language) bool

	// ExtractTemplateArgumentsFromCapture optionally extracts template arguments from captures.
	// Returns nil when not implemented.
	ExtractTemplateArgumentsFromCapture(ctx *ClassCaptureContext, source []byte, lang *gotreesitter.Language) []string
}

// ClassExtractionConfig defines per-language class extraction configuration.
type ClassExtractionConfig struct {
	Language               SupportedLanguage
	TypeDeclarationNodes   []string
	FileScopeNodeTypes     []string // optional: nil = not set
	AncestorScopeNodeTypes []string // optional: nil = not set
	QualifiedNodeId        bool     // opt-in (#1978)
	ScopeNameNodeTypes     []string // optional: nil = not set
	ExtractName            func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractType            func(node *gotreesitter.Node, lang *gotreesitter.Language) *ClassLikeNodeLabel
	ExtractScopeSegments   func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string            // optional: nil = not set
	ExtractTemplateArguments func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string           // optional: nil = not set
	ShouldSkipClassCapture func(ctx *ClassCaptureContext, nodeLabel ClassLikeNodeLabel, lang *gotreesitter.Language) bool // optional
	ExtractTemplateArgumentsFromCapture func(ctx *ClassCaptureContext, source []byte, lang *gotreesitter.Language) []string // optional
}