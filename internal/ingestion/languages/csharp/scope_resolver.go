// Package csharp — C# ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for C#,
// providing per-language hooks for the generic scope-resolution orchestrator.
// Ported from TS languages/csharp/scope-resolver.ts.
package csharp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CSharpScopeResolver implements scope_resolution.ScopeResolver for C#.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for C# source code.
var CSharpScopeResolver scope_resolution.ScopeResolver = &csharpScopeResolverImpl{}

// csharpScopeResolverImpl is the concrete implementation.
type csharpScopeResolverImpl struct{}

// --- Core identity methods --- (3 methods)

func (r *csharpScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageCSharp
}

func (r *csharpScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return CSharpProvider()
}

func (r *csharpScopeResolverImpl) ImportEdgeReason() string {
	return "csharp-scope: using"
}

// --- Import resolution --- (3 methods)

func (r *csharpScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	return ResolveCsharpImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig)
}

func (r *csharpScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// C# "using static" imports expose individual members, not wildcard expansion.
	// Namespace "using" imports expose all types in the namespace via PopulateCsharpNamespaceSiblings.
	return nil
}

func (r *csharpScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// C# loads .csproj RootNamespace as resolution config.
	return func(repoPath string) interface{} {
		return LoadCsharpResolutionConfig(repoPath)
	}
}

// --- Binding merge --- (1 method)

func (r *csharpScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return CSharpMergeBindings(existing, incoming, string(scopeID))
}

// --- Arity / constraint compatibility --- (2 methods)

func (r *csharpScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	return CSharpArityCompatibility(callsite, def)
}

func (r *csharpScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// C# has no constrained-overload semantics (unlike C++ templates).
	return nil
}

// --- MRO --- (2 methods)

func (r *csharpScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// C# uses single-inheritance MRO (depth-first, linear).
	// TODO: implement BFS/DFS MRO for C# class hierarchy.
	return nil
}

func (r *csharpScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// C# doesn't need extends-only MRO (no mixin augmentation).
	return nil
}

// --- Owner population --- (1 method)

func (r *csharpScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	// C# class-owned members are handled by the generic PopulateClassOwnedMembers.
	// No special C# owner pass needed beyond the standard nested-class stamping.
}

// --- Super receiver --- (1 method)

func (r *csharpScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// C# uses "base" keyword for super calls, not "super".
	return receiverName == "base"
}

// --- Optional toggle hooks --- (7 methods)

func (r *csharpScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// C# is statically typed but signatures are authoritative —
	// return types propagate across imports for type-inference heuristics.
	return true
}

func (r *csharpScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// C# is statically typed; field access is explicit, not inferred.
	return false
}

func (r *csharpScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	return UnwrapCsharpCollectionAccessor
}

func (r *csharpScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// C# collapses member calls by caller/target — one edge per caller→target pair.
	return true
}

func (r *csharpScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	return PopulateCsharpNamespaceSiblings
}

func (r *csharpScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// C# stores method return types on Module scopes after namespace prefix population.
	return true
}

// --- Heritage / implicit import hooks --- (3 methods)

func (r *csharpScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// C# explicit inheritance (class A : B) is handled by the generic pass.
	return nil
}

func (r *csharpScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// C# namespace siblings provide implicit visibility — handled by PopulateNamespaceSiblings.
	return nil
}

func (r *csharpScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// C# doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *csharpScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// C# uses text-only IsSuperReceiver ("base" keyword).
	return nil
}

func (r *csharpScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// C# resolves `this.method()` via explicit typeBinding.
	return false
}

func (r *csharpScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// C# uses the generic MRO walk for member resolution.
	return nil
}

func (r *csharpScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// C# doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *csharpScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// C# has no static-only filtering.
	return nil
}

func (r *csharpScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// C# has no conversion-rank scoring for overload resolution.
	return nil
}