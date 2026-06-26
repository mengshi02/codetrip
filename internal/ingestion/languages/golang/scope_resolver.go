// Package golang — Go ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for Go,
// providing per-language hooks for the generic scope-resolution orchestrator.
// Ported from TS languages/go/scope-resolver.ts.
package golang

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// GoScopeResolver implements scope_resolution.ScopeResolver for Go.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for Go source code.
var GoScopeResolver scope_resolution.ScopeResolver = &goScopeResolverImpl{}

// goScopeResolverImpl is the concrete implementation.
type goScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *goScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageGo
}

func (r *goScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return GoProvider()
}

func (r *goScopeResolverImpl) ImportEdgeReason() string {
	return "go-import"
}

// --- Import resolution ---

func (r *goScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	return ResolveGoImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig)
}

func (r *goScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	return ExpandGoWildcardNames
}

func (r *goScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// Go loads go.mod module path as resolution config.
	// TODO: wire to core.LoadLanguageConfigs or a dedicated go.mod loader.
	return func(repoPath string) interface{} {
		configs := core.LoadLanguageConfigs(repoPath)
		for _, c := range configs {
			if c.Language == "go" && c.GoModule != nil {
				return c.GoModule
			}
		}
		return nil
	}
}

// --- Binding merge ---

func (r *goScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return GoMergeBindings(existing, incoming, string(scopeID))
}

// --- Arity / constraint compatibility ---

func (r *goScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	// Note: TS signature is (def, callsite); Go interface is (callsite, def).
	// GoArityCompatibility takes (def, callsite), so we swap arguments.
	return GoArityCompatibility(def, callsite)
}

func (r *goScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// Go has no constrained-overload semantics; return nil.
	return nil
}

// --- MRO ---

func (r *goScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Go uses BFS first-wins MRO (no class inheritance chain).
	// TODO: implement BFS MRO for Go struct embedding.
	return nil
}

func (r *goScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Go doesn't need extends-only MRO (no mixin/trait augmentation).
	return nil
}

// --- Owner population ---

func (r *goScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	PopulateGoOwners(parsed)
}

func (r *goScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// Go has no "super" keyword. Method dispatch uses the receiver type.
	return false
}

// --- Optional toggle hooks ---

func (r *goScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// Go is statically typed; return types are on the declaration, not
	// inferred through import chains. Turn OFF to avoid over-connecting.
	return false
}

func (r *goScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// Go is statically typed; struct fields are explicit, not inferred.
	// Turn OFF to avoid over-connecting.
	return false
}

func (r *goScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// Go doesn't use property-style collection accessors.
	return nil
}

func (r *goScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// Go method calls should not be collapsed — each call site is distinct.
	return false
}

func (r *goScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	return PopulateGoPackageSiblings
}

func (r *goScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// Go stores method return types on Module scopes after mirroring.
	return false
}

// --- Heritage / implicit import hooks ---

func (r *goScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// Go structural interface implementation detection.
	// DetectGoInterfaceImplementations is available but graph emission
	// requires the KnowledgeGraph write API — currently returns nil
	// until the graph mutation layer is stabilized.
	return nil
}

func (r *goScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// Go files in the same package implicitly see each other's declarations.
	// TODO: wire to a function that emits intra-package import edges.
	return nil
}

func (r *goScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// Go doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *goScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// Go has no super/base keyword.
	return nil
}

func (r *goScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// Go has no `this`/`self` class-based dispatch.
	return false
}

func (r *goScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// Go uses the generic MRO walk for member resolution.
	return nil
}

func (r *goScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// Go doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *goScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// Go has no static-only filtering.
	return nil
}

func (r *goScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// Go has no conversion-rank scoring for overload resolution.
	return nil
}