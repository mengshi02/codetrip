// Package typescript — TypeScript ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for TypeScript,
// providing per-language hooks for the generic scope-resolution orchestrator.
// Third migration after Python and C#. Follows the same minimal wiring-only
// pattern — per-hook logic lives in the sibling modules (arity.go,
// merge_bindings.go, import_target.go, etc.).
//
// Ported from TS languages/typescript/scope-resolver.ts.
package typescript

import (
	"regexp"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TypeScriptScopeResolver implements scope_resolution.ScopeResolver for TypeScript.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for TypeScript source code.
var TypeScriptScopeResolver scope_resolution.ScopeResolver = &tsScopeResolverImpl{}

type tsScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *tsScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageTypeScript
}

func (r *tsScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return TypeScriptProvider()
}

func (r *tsScopeResolverImpl) ImportEdgeReason() string {
	return "typescript-scope: import"
}

// --- Import resolution ---

func (r *tsScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	// TODO: wire to makeTsResolveImportTarget memoized adapter that caches
	// allFileList, normalizedFileList, and resolveCache per workspace pass.
	ctx := TsResolveContext{
		FromFile:        fromFile,
		AllFilePaths:    allFilePaths,
		Language:        shared.SupportedLanguageTypeScript,
		TsconfigPaths:   nil, // populated by LoadResolutionConfig
	}
	result := ResolveTsTarget(targetRaw, ctx)
	if result == nil {
		return nil
	}
	return []string{*result}
}

func (r *tsScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// TypeScript supports re-export wildcards (export * from './m').
	// TODO: implement wildcard expansion for TS namespace re-exports.
	return nil
}

func (r *tsScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// TypeScript loads tsconfig.json path aliases as resolution config.
	// The orchestrator calls this once per workspace pass.
	return func(repoPath string) interface{} {
		configs := core.LoadLanguageConfigs(repoPath)
		for _, c := range configs {
			if c.Language == string(shared.SupportedLanguageTypeScript) && c.TsconfigPaths != nil {
				return c.TsconfigPaths
			}
		}
		return nil
	}
}

// --- Binding merge ---

func (r *tsScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return TypeScriptMergeBindings(append(existing, incoming...))
}

// --- Arity / constraint compatibility ---

func (r *tsScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	// Note: TypeScriptArityCompatibility uses (def, callsite); interface is (callsite, def).
	return TypeScriptArityCompatibility(def, callsite)
}

func (r *tsScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// TypeScript has no constrained-overload semantics; return nil.
	return nil
}

// --- MRO ---

func (r *tsScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// TODO: wire to buildMro(graph, parsedFiles, nodeLookup, defaultLinearize)
	return nil
}

func (r *tsScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// TypeScript doesn't need extends-only MRO (no mixin/trait augmentation).
	return nil
}

// --- Owner population ---

func (r *tsScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	// TODO: wire to populateClassOwnedMembers(parsed)
}

// tsSuperRegex matches `super` keyword: super(...), super.foo, super[x], or bare super.
var tsSuperRegex = regexp.MustCompile(`^super(\s*\(|\s*\.|\s*\[|\s*$)`)

func (r *tsScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	return tsSuperRegex.MatchString(receiverName)
}

// --- Optional toggle hooks ---

func (r *tsScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// TypeScript is statically typed — return-type propagation across imports
	// is ON (matches legacy DAG behavior: explicit return-type annotations
	// flow across export boundaries and resolve chained member calls).
	return true
}

func (r *tsScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// TypeScript is statically typed — the type-binding layer produces
	// precise owner types. Turn OFF to avoid over-connecting.
	return false
}

func (r *tsScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// TypeScript uses .values() / .keys() method-call syntax — no property-style
	// collection accessors like C#'s Dictionary<K,V>.Values.
	return nil
}

func (r *tsScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// TypeScript legacy DAG emits one edge per call site, so per-site dedup
	// is the parity target. Leave false.
	return false
}

func (r *tsScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	// TypeScript requires an explicit import / namespace augmentation for
	// cross-file visibility; there's no implicit same-namespace sibling rule.
	return nil
}

func (r *tsScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// tsBindingScopeFor DOES hoist method return-type bindings to the
	// enclosing Module scope (mirrors C#), so enable the walk-up that lets
	// the compound-receiver resolver find them.
	return true
}

// --- Heritage / implicit import hooks ---

func (r *tsScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// TODO: wire to TypeScript heritage edge emitter when graph emission is ready.
	return nil
}

func (r *tsScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// TypeScript has no implicit import edges (everything is explicit import).
	return nil
}

func (r *tsScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// TypeScript doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *tsScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// TypeScript uses text-only IsSuperReceiver ("super" keyword).
	return nil
}

func (r *tsScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// TypeScript resolves `this.method()` via explicit typeBinding.
	return false
}

func (r *tsScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// TypeScript uses the generic MRO walk for member resolution.
	return nil
}

func (r *tsScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// TypeScript doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *tsScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// TypeScript has no static-only filtering.
	return nil
}

func (r *tsScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// TypeScript has no conversion-rank scoring for overload resolution.
	return nil
}