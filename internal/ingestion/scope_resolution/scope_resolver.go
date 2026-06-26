// Package scope_resolution implements the scope-resolution engine — the core
// analysis pipeline that resolves cross-file references and emits graph edges.
//
// It mirrors the TS scope-resolution/ directory, adapted for Go:
//   - contract/scope-resolver.ts → scope_resolver.go (this file)
//   - pipeline/*.ts               → pipeline files
//   - graph-bridge/*.ts           → graph bridge files
//   - passes/*.ts                 → per-pass resolution logic
//   - scope/*.ts                  → scope helpers
//
// The ScopeResolver interface is the per-language contract consumed by the
// generic scope-resolution orchestrator (runScopeResolution).
package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ArityVerdict is the result of ScopeResolver.ArityCompatibility — mirrors
// RegistryProviders.arityCompatibility.
type ArityVerdict string

const (
	ArityCompatible   ArityVerdict = "compatible"
	ArityUnknown      ArityVerdict = "unknown"
	ArityIncompatible ArityVerdict = "incompatible"
)

// ReceiverMemberResolution is the result of resolving a receiver.method lookup.
type ReceiverMemberResolution struct {
	Kind         string   // "resolved" or "ambiguous"
	Definition   *shared.SymbolDefinition // non-nil when Kind == "resolved"
	CandidateIDs []string // non-empty when Kind == "ambiguous"
}

// ResolveQualifiedReceiverMemberResult is the result of a qualified
// namespace-receiver member lookup. When Ambiguous is true, the edge
// emission should be suppressed; otherwise Def holds the resolved target.
type ResolveQualifiedReceiverMemberResult struct {
	Def       *shared.SymbolDefinition
	Ambiguous bool
}

// LinearizeStrategy computes the method-resolution order for a class.
// It receives the full ancestor map so C3-style algorithms can merge
// each parent's MRO. Python's depth-first first-seen only consumes
// directParents and parentsByDefId.
type LinearizeStrategy func(
	classDefID string,
	directParents []string,
	parentsByDefID map[string][]string,
) []string

// ScopeResolver is the per-language contract consumed by the generic
// scope-resolution orchestrator (RunScopeResolution).
//
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges. 8+ fields total. Consumed by RunScopeResolution,
// once per workspace at resolve time.
//
// In contrast, LanguageProvider is the PARSING-SIDE contract — how to emit
// captures, classify scopes, interpret imports/typeBindings. ~40 fields.
// Consumed by ScopeExtractor, once per file at extract time.
//
// They share three concept names (ArityCompatibility, MergeBindings,
// ResolveImportTarget) because the emit pipeline reuses a few finalize hooks.
// Per-language wiring passes the SAME function reference through both interfaces.
type ScopeResolver interface {
	// Language returns the SupportedLanguage this resolver handles.
	Language() shared.SupportedLanguage

	// LanguageProvider returns the parsing-side hook bag consumed by extractParsedFile.
	// The same LanguageProvider reference flows through both interfaces.
	LanguageProvider() core.LanguageProvider

	// ImportEdgeReason returns the reason text on emitted IMPORTS edges.
	ImportEdgeReason() string

	// ResolveImportTarget resolves an import statement's targetRaw into an
	// absolute repo-relative file path, or nil for unresolvable/external modules.
	// allFilePaths is the workspace's file set.
	// resolutionConfig is the opaque value from LoadResolutionConfig (may be nil).
	ResolveImportTarget(
		targetRaw string,
		fromFile string,
		allFilePaths map[string]bool,
		resolutionConfig interface{},
	) []string // nil = unresolvable; single entry = resolved; multiple = ambiguous

	// ExpandsWildcardTo enumerates names visible through a wildcard import
	// after the target module scope has been linked.
	// Languages that don't support wildcard imports leave this nil.
	ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string

	// LoadResolutionConfig is an optional one-shot loader for cross-file
	// import-resolution config (e.g. tsconfig path aliases, go.mod paths).
	// The orchestrator calls this once per workspace pass.
	// Returns nil if not needed.
	LoadResolutionConfig() func(repoPath string) interface{}

	// MergeBindings implements per-scope binding-merge precedence.
	// Python uses LEGB: local > import / namespace / reexport > wildcard.
	MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID shared.ScopeID) []shared.BindingRef

	// ArityCompatibility returns per-language arity compatibility between
	// a callsite and a candidate def. Note arg order — (callsite, def).
	ArityCompatibility(callsite shared.Callsite, def shared.SymbolDefinition) ArityVerdict

	// ConstraintCompatibility returns per-language constraint compatibility
	// between a callsite and a candidate def with templateConstraints.
	// Optional — languages without constrained-overload semantics return nil.
	ConstraintCompatibility() func(
		callsite shared.ReferenceSite,
		def shared.SymbolDefinition,
		ctx shared.ParameterTypeClass,
	) ArityVerdict

	// BuildMro computes the method-dispatch order for every Class def in the
	// workspace. Returns map of DefId → ancestor DefIds.
	BuildMro(
		graph shared.KnowledgeGraph,
		parsedFiles []*shared.ParsedFile,
		nodeLookup *GraphNodeLookup,
	) map[string][]string

	// BuildExtendsOnlyMro is an optional parallel MRO that EXCLUDES mixin-like
	// augmentation (e.g. PHP traits). Returns nil if not needed.
	BuildExtendsOnlyMro() func(
		graph shared.KnowledgeGraph,
		parsedFiles []*shared.ParsedFile,
		nodeLookup *GraphNodeLookup,
	) map[string][]string

	// PopulateOwners assigns owner IDs to parsed definitions that the legacy
	// parse pass missed (primarily Python class-body methods).
	PopulateOwners(parsed *shared.ParsedFile)

	// IsSuperReceiver returns true if the receiver name indicates a super call
	// (e.g. "super" in Python, "base" in C#).
	IsSuperReceiver(receiverName string) bool

	// ─── Optional toggles / hooks ─────────────────────────────

	// PropagatesReturnTypesAcrossImports — default true for most languages.
	// Turn OFF for statically-typed languages where the heuristic over-connects.
	PropagatesReturnTypesAcrossImports() bool

	// FieldFallbackOnMethodLookup — default true. Turn OFF for statically-typed
	// languages where the heuristic over-connects.
	FieldFallbackOnMethodLookup() bool

	// UnwrapCollectionAccessor — property-style collection views. Optional.
	UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition

	// CollapseMemberCallsByCallerTarget — one edge per caller/target. Optional.
	CollapseMemberCallsByCallerTarget() bool

	// PopulateNamespaceSiblings — cross-file implicit visibility (C#). Optional.
	PopulateNamespaceSiblings() func(
		graph shared.KnowledgeGraph,
		parsedFiles []*shared.ParsedFile,
		nodeLookup *GraphNodeLookup,
		indexes *model.ScopeResolutionIndexes,
	)

	// HoistTypeBindingsToModule — enable ONLY when method return types are
	// stored on the enclosing Module scope. Most languages leave this OFF.
	HoistTypeBindingsToModule() bool

	// EmitHeritageEdges — optional pre-MRO hook to emit heritage edges (IMPLEMENTS)
	// that the generic preEmitInheritanceEdges pass cannot produce.
	EmitHeritageEdges() func(
		graph shared.KnowledgeGraph,
		parsedFiles []*shared.ParsedFile,
		nodeLookup *GraphNodeLookup,
		scopes *model.ScopeResolutionIndexes,
	)

	// EmitImplicitImportEdges — optional hook for implicit cross-file visibility
	// (e.g. every file in a build target sees its siblings' top-level declarations).
	EmitImplicitImportEdges() func(
		graph shared.KnowledgeGraph,
		parsedFiles []*shared.ParsedFile,
		nodeLookup *GraphNodeLookup,
		resolutionConfig interface{},
	)

	// RestoreCaptureSideChannel — restore capture-time per-file side-channel
	// state that emitScopeCaptures produces. C++ is the only consumer today.
	RestoreCaptureSideChannel() func(parsed *shared.ParsedFile)

	// ─── Optional receiver-resolution hooks ─────────────────────

	// IsSuperReceiverInContext — context-aware super-receiver classification.
	// When non-nil, the receiver-bound pass prefers it over IsSuperReceiver.
	// C++ defines this; simple text-only languages (Python, Java, PHP) leave it nil.
	IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool

	// ResolveThisViaEnclosingClass — when true, `this`-receiver callsites
	// resolve against the enclosing class + MRO even when there's no explicit
	// `this` typeBinding in scope. C++ is the sole consumer today.
	ResolveThisViaEnclosingClass() bool

	// ResolveReceiverMember — language-specific member resolution on a class.
	// When non-nil, the receiver-bound pass calls it before the generic MRO walk.
	// Returns a ReceiverMemberResolution with Kind "resolved" or "ambiguous".
	ResolveReceiverMember() func(
		ownerDef *shared.SymbolDefinition,
		memberName string,
		site shared.ReferenceSite,
		indexes *model.ScopeResolutionIndexes,
		model model.SemanticModel,
	) *ReceiverMemberResolution

	// ResolveQualifiedReceiverMember — namespace-scoped qualified member resolution.
	// Languages whose qualified-name semantics need workspace-wide namespace walking
	// (C++ `outer::foo()`) implement this. Returns nil if not resolvable, or
	// the special string "ambiguous" to suppress edge emission.
	ResolveQualifiedReceiverMember() func(
		receiverName string,
		memberName string,
		scopeID shared.ScopeID,
		indexes *model.ScopeResolutionIndexes,
		parsedFiles []*shared.ParsedFile,
		site shared.ReferenceSite,
	) *ResolveQualifiedReceiverMemberResult

	// IsStaticOnly — when non-nil, filters out static-only candidates (Kotlin
	// companion-promoted methods) during MRO chain walks.
	IsStaticOnly() func(def *shared.SymbolDefinition) bool

	// ConversionRankFn — per-language conversion-rank scoring for overload
	// resolution. C++ uses it to rank implicit conversion sequences.
	// Returns nil if not applicable.
	ConversionRankFn() func(callType string, paramType string) int
}