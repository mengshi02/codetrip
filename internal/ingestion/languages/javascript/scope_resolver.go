// Package javascript — JavaScript ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for JavaScript,
// providing per-language hooks for the generic scope-resolution orchestrator.
// Follows the same minimal wiring-only pattern as TypeScript.
//
// Key differences from TypeScript resolver:
//   - FieldFallbackOnMethodLookup: true — JS is dynamically typed; enable the
//     field-fallback heuristic so member-call receivers without type annotations
//     can still resolve through declared class fields (e.g. JSDoc-typed fields).
//   - LoadResolutionConfig: omitted — JS projects don't use tsconfig.json path
//     aliases. tsconfigPaths: nil is threaded through the resolver adapter.
//   - HoistTypeBindingsToModule: true — JSDoc @returns bindings are synthesized
//     on the function scope and hoisted, matching TypeScript's strategy.
//   - allowGlobalFreeCallFallback: true — CJS require patterns and global helpers
//     benefit from workspace-wide unique-name fallback.
//
// Ported from TS languages/javascript/scope-resolver.ts.
package javascript

import (
	"regexp"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JavaScriptScopeResolver implements scope_resolution.ScopeResolver for JavaScript.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for JavaScript source code.
var JavaScriptScopeResolver scope_resolution.ScopeResolver = &jsScopeResolverImpl{}

type jsScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *jsScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageJavaScript
}

func (r *jsScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return JavaScriptProvider()
}

func (r *jsScopeResolverImpl) ImportEdgeReason() string {
	return "javascript-scope: import"
}

// --- Import resolution ---

func (r *jsScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	ctx := typescript.TsResolveContext{
		FromFile:     fromFile,
		AllFilePaths: allFilePaths,
		Language:     shared.SupportedLanguageJavaScript,
		TsconfigPaths: nil,
	}
	result := typescript.ResolveTsTarget(targetRaw, ctx)
	if result == nil {
		return nil
	}
	return []string{*result}
}

func (r *jsScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	return nil // TODO: implement wildcard expansion for JS re-exports
}

func (r *jsScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// JavaScript projects don't use tsconfig.json path aliases.
	return nil
}

// --- Binding merge ---

func (r *jsScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return JsMergeBindings(append(existing, incoming...))
}

// --- Arity / constraint compatibility ---

func (r *jsScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	return JsArityCompatibility(def, callsite)
}

func (r *jsScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	return nil // JS has no constrained-overload semantics
}

// --- MRO ---

func (r *jsScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// TODO: wire to buildMro(graph, parsedFiles, nodeLookup, defaultLinearize)
	return nil
}

func (r *jsScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	return nil
}

// --- Owner population ---

func (r *jsScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	// TODO: wire to populateClassOwnedMembers(parsed)
}

// jsSuperRegex matches `super` keyword — same pattern as TypeScript.
var jsSuperRegex = regexp.MustCompile(`^super(\s*\(|\s*\.|\s*\[|\s*$)`)

func (r *jsScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	return jsSuperRegex.MatchString(receiverName)
}

// --- Optional toggle hooks ---

func (r *jsScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// Return-type propagation across ESM imports mirrors TypeScript's default.
	// JSDoc @returns bindings are hoisted to Module scope and propagated.
	return true
}

func (r *jsScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// JavaScript is dynamically typed — enable the field-fallback heuristic
	// so member-call receivers without type annotations can still resolve.
	return true
}

func (r *jsScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	return nil
}

func (r *jsScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	return false
}

func (r *jsScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	return nil
}

func (r *jsScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// JSDoc @returns bindings are synthesized on the function/method node
	// and hoisted to Module scope by jsBindingScopeFor (identical to the
	// TypeScript tsBindingScopeFor @type-binding.return branch).
	return true
}

// --- Heritage / implicit import hooks ---

func (r *jsScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// TODO: wire to JavaScript heritage edge emitter
	return nil
}

func (r *jsScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	return nil
}

func (r *jsScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *jsScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	return nil
}

func (r *jsScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	return false
}

func (r *jsScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	return nil
}

func (r *jsScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	return nil
}

func (r *jsScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	return nil
}

func (r *jsScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	return nil
}