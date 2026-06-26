// Package java — Java ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for Java,
// providing per-language hooks for the generic scope-resolution orchestrator.
// Ported from TS languages/java/scope-resolver.ts.
package java

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JavaScopeResolver implements scope_resolution.ScopeResolver for Java.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for Java source code.
var JavaScopeResolver scope_resolution.ScopeResolver = &javaScopeResolverImpl{}

// javaScopeResolverImpl is the concrete implementation.
type javaScopeResolverImpl struct{}

// --- Core identity methods --- (3 methods)

func (r *javaScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageJava
}

func (r *javaScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return JavaProvider()
}

func (r *javaScopeResolverImpl) ImportEdgeReason() string {
	return "java-scope: import"
}

// --- Import resolution --- (3 methods)

func (r *javaScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	return ResolveJavaImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig)
}

func (r *javaScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// Java has no wildcard import expansion in scope resolution.
	// Import-on-demand declarations (e.g. import com.foo.*) are resolved
	// by PopulateJavaPackageSiblings, not by ExpandsWildcardTo.
	return nil
}

func (r *javaScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// Java imports are absolute (fully qualified); no tsconfig-like config needed.
	return nil
}

// --- Binding merge --- (1 method)

func (r *javaScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return JavaMergeBindings(existing, incoming, string(scopeID))
}

// --- Arity / constraint compatibility --- (2 methods)

func (r *javaScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	// Note: TS signature is javaArityCompatibility(def, callsite);
	// Go interface is (callsite, def). JavaArityCompatibility takes (def, callsite).
	return JavaArityCompatibility(def, callsite)
}

func (r *javaScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// Java has no constrained-overload semantics (unlike C++ templates).
	return nil
}

// --- MRO --- (2 methods)

func (r *javaScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Java uses C3 linearization + interface augmentation (buildJavaMro).
	// TODO: implement buildJavaMro when graph emission is ready.
	return nil
}

func (r *javaScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Java doesn't need extends-only MRO (no mixin augmentation).
	return nil
}

// --- Owner population --- (1 method)

func (r *javaScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	PopulateJavaClassOwnedMembers(parsed)
}

// --- Super receiver --- (1 method)

func (r *javaScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// Java uses "super" keyword for superclass method calls.
	return strings.TrimSpace(receiverName) == "super"
}

// --- Optional toggle hooks --- (7 methods)

func (r *javaScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// Java is statically typed but signatures cross files —
	// return types propagate across imports for type-inference heuristics.
	return true
}

func (r *javaScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// Java is statically typed; field access is explicit, not inferred.
	return false
}

func (r *javaScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// Java doesn't use property-style collection accessors.
	return nil
}

func (r *javaScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// Java collapses member calls by caller/target — one edge per caller→target pair.
	return true
}

func (r *javaScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	return PopulateJavaPackageSiblings
}

func (r *javaScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// Java stores method return types on Module scopes after package population.
	return true
}

// --- Heritage / implicit import hooks --- (3 methods)

func (r *javaScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// Java explicit inheritance (class A extends B, implements C)
	// is handled by the generic pass.
	return nil
}

func (r *javaScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// Java package siblings provide implicit same-package visibility —
	// handled by PopulateNamespaceSiblings.
	return nil
}

func (r *javaScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// Java doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *javaScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// Java uses text-only IsSuperReceiver.
	return nil
}

func (r *javaScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// Java resolves `this.method()` via explicit typeBinding; no special `this` handling.
	return false
}

func (r *javaScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// Java uses the generic MRO walk for member resolution.
	return nil
}

func (r *javaScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// Java doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *javaScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// Java has no static-only filtering.
	return nil
}

func (r *javaScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// Java has no conversion-rank scoring for overload resolution.
	return nil
}