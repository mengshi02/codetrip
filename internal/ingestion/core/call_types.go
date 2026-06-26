package core

import "github.com/odvcencio/gotreesitter"

// CallForm represents the form of a call site: free call, member call, or constructor.
type CallForm string

const (
	CallFormFree        CallForm = "free"
	CallFormMember      CallForm = "member"
	CallFormConstructor CallForm = "constructor"
)

// MixedChainStep represents one step in a mixed receiver chain (field access or call).
// Defined in call-analysis.ts; placed here for package cohesion.
type MixedChainStep struct {
	Kind string // "field" or "call"
	Name string
}

// ExtractedCallSite is the per-node call extraction result.
// The parse worker enriches this with file-level context (filePath, sourceId,
// TypeEnv lookups, arg types) to produce the final ExtractedCall that enters
// the resolution pipeline.
type ExtractedCallSite struct {
	CalledName               string
	CallForm                 *CallForm        // optional: nil means unspecified
	ReceiverName             *string          // optional
	ArgCount                 *int             // optional
	ReceiverMixedChain       []MixedChainStep // optional: nil = absent
	TypeAsReceiverHeuristic *bool            // optional
}

// CallExtractor extracts a call site from captured AST nodes.
// Produced by createCallExtractor() per language.
type CallExtractor interface {
	// Language returns the language this extractor handles.
	Language() SupportedLanguage
	// Extract extracts a call site from captured AST nodes.
	// callNode is the @call capture; callNameNode is the @call.name capture (may be nil).
	// source is the source text bytes for node text extraction.
	// lang is the tree-sitter Language for node type queries.
	// Returns nil when no call can be derived.
	Extract(callNode *gotreesitter.Node, callNameNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *ExtractedCallSite
}

// CallExtractionConfig defines per-language call extraction configuration.
// One config per language / language group.
type CallExtractionConfig struct {
	Language SupportedLanguage

	// ExtractLanguageCallSite is a language-specific call site extraction function.
	// Called BEFORE the generic path. If it returns non-nil, the generic
	// inferCallForm / extractReceiverName path is skipped entirely.
	// Use this for call shapes that don't follow the standard @call / @call.name
	// pattern (e.g. Java method_reference via ::).
	ExtractLanguageCallSite func(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *ExtractedCallSite // optional: nil = not set

	// TypeAsReceiverHeuristic: when true and the receiver name starts with an
	// uppercase letter, the receiver is treated as a type name when no TypeEnv
	// binding exists. Applies to JVM and C# languages where Type.method() and
	// Type::method are common patterns.
	TypeAsReceiverHeuristic bool
}