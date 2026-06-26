// Package rust — Rust ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for Rust,
// providing per-language hooks for the generic scope-resolution orchestrator.
//
// Key Rust-specific behaviors:
//   - impl T for S → IMPLEMENTS heritage edges (emitRustTraitImplEdges)
//   - isSuperReceiver always false (Rust has no super/base keyword)
//   - propagatesReturnTypesAcrossImports=true, fieldFallbackOnMethodLookup=false
//   - hoistTypeBindingsToModule=true
//
// Ported from TS languages/rust/scope-resolver.ts.
package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustScopeResolver implements scope_resolution.ScopeResolver for Rust.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for Rust source code.
var RustScopeResolver scope_resolution.ScopeResolver = &rustScopeResolverImpl{}

// rustScopeResolverImpl is the concrete implementation.
type rustScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *rustScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageRust
}

func (r *rustScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return RustProvider()
}

func (r *rustScopeResolverImpl) ImportEdgeReason() string {
	return "rust-scope: use"
}

// --- Import resolution ---

func (r *rustScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	return ResolveRustImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig)
}

func (r *rustScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// Rust supports glob imports (use module::*).
	// TODO: wire to Rust wildcard expansion when scope-resolution is integrated.
	return nil
}

func (r *rustScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// Rust uses Cargo.toml for module resolution.
	// TODO: wire to core.LoadLanguageConfigs or a dedicated Cargo.toml loader.
	return func(repoPath string) interface{} {
		configs := core.LoadLanguageConfigs(repoPath)
		for _, c := range configs {
			if c.Language == "rust" {
				return c
			}
		}
		return nil
	}
}

// --- Binding merge ---

func (r *rustScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return RustMergeBindings(existing, incoming, string(scopeID))
}

// --- Arity / constraint compatibility ---

func (r *rustScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	// Adapter: RustArityCompatibility uses (def, callsite); the contract is (callsite, def).
	return RustArityCompatibility(def, callsite)
}

func (r *rustScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// Rust has no constrained-overload semantics; return nil.
	return nil
}

// --- MRO ---

func (r *rustScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Rust uses C3 linearization for trait-impl MRO with interface augmentation.
	// TODO: wire to buildRustMro(graph, parsedFiles, nodeLookup) when the MRO pass is integrated.
	return nil
}

func (r *rustScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Rust doesn't need extends-only MRO.
	return nil
}

// --- Owner population ---

func (r *rustScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	PopulateRustOwners(parsed)
}

func (r *rustScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// Rust has no "super" or "base" keyword for dispatch.
	return false
}

// --- Optional toggle hooks ---

func (r *rustScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// Rust is statically typed but trait-impl return types propagate across modules.
	return true
}

func (r *rustScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// Rust is statically typed; struct fields are explicit, not inferred.
	return false
}

func (r *rustScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// Rust doesn't use property-style collection accessors.
	return nil
}

func (r *rustScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// Rust method calls should not be collapsed — each call site is distinct.
	return false
}

func (r *rustScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	// Rust doesn't use namespace-sibling visibility (modules are explicit).
	return nil
}

func (r *rustScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// Rust hoists method return-type bindings to the enclosing Module scope
	// so propagateImportedReturnTypes can mirror them across files.
	return true
}

// --- Heritage / implicit import hooks ---

func (r *rustScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// Rust trait implementation edges: impl T for S → S IMPLEMENTS T.
	// This is the key heritage hook for Rust — the generic preEmitInheritanceEdges
	// pass cannot produce these edges because impl_item scopes own no class-like def.
	// TODO: wire to emitRustTraitImplEdges when graph emission is ready.
	return nil
}

func (r *rustScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// Rust doesn't have implicit cross-file visibility (modules are explicit).
	return nil
}

func (r *rustScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// Rust doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *rustScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// Rust has no super/base keyword.
	return nil
}

func (r *rustScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// Rust has no `this`/`self` class-based dispatch.
	return false
}

func (r *rustScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// Rust uses the generic MRO walk for member resolution.
	return nil
}

func (r *rustScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// Rust doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *rustScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// Rust has no static-only filtering.
	return nil
}

func (r *rustScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// Rust has no conversion-rank scoring for overload resolution.
	return nil
}