// Package cpp — C++ ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for C++,
// providing per-language hooks for the generic scope-resolution orchestrator.
//
// Key C++-specific behaviors:
//   - ADL (argument-dependent lookup) for free function resolution
//   - Two-phase name lookup for template instantiation
//   - Capture side channel for template/constraint resolution
//   - Constraint compatibility (concepts, SFINAE)
//   - User-defined conversion rank for overload resolution
//   - Inline namespaces and file-local linkage
//   - Dependent base class resolution
//   - isSuperReceiver always false (C++ has no super/base keyword)
//   - propagatesReturnTypesAcrossImports=true
//   - hoistTypeBindingsToModule=true
//   - loadResolutionConfig clears ADL/inline-namespace/conversion caches + scans headers
//
// Ported from TS languages/cpp/scope-resolver.ts.
package cpp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CppScopeResolver implements scope_resolution.ScopeResolver for C++.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for C++ source code.
var CppScopeResolver scope_resolution.ScopeResolver = &cppScopeResolverImpl{}

// cppScopeResolverImpl is the concrete implementation.
type cppScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *cppScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageCpp
}

func (r *cppScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return CppProvider()
}

func (r *cppScopeResolverImpl) ImportEdgeReason() string {
	return "cpp-scope: include"
}

// --- Import resolution ---

func (r *cppScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	// Augment allFilePaths with header files discovered via loadResolutionConfig.
	// C++ .h/.hpp/.hxx/.hh files may be classified differently by language
	// detection but are importable from .cpp files via #include.
	headerPaths, ok := resolutionConfig.(map[string]bool)
	if ok && len(headerPaths) > 0 {
		augmented := make(map[string]bool, len(allFilePaths)+len(headerPaths))
		for k, v := range allFilePaths {
			augmented[k] = v
		}
		for k, v := range headerPaths {
			augmented[k] = v
		}
		return ResolveCppImportTarget(targetRaw, fromFile, augmented)
	}
	return ResolveCppImportTarget(targetRaw, fromFile, allFilePaths)
}

func (r *cppScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// C++ "using namespace X;" expands to all symbols in namespace X.
	// TODO: wire to C++ wildcard expansion when scope-resolution is integrated.
	return nil
}

func (r *cppScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// C++ clears stale per-pipeline state and scans for header files.
	// The config is a set of header file paths for import resolution.
	return func(repoPath string) interface{} {
		ClearCppAdlState()
		ClearCppInlineNamespaces()
		ClearCppUserDefinedConversions()
		ClearCppMemberLookupState()
		ClearFileLocalNames()
		ClearCppDependentBases()
		return ScanCppHeaderFiles(repoPath)
	}
}

// --- Binding merge ---

func (r *cppScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	return CppMergeBindings(existing, incoming, string(scopeID))
}

// --- Arity / constraint compatibility ---

func (r *cppScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	return CppScopeArityCompatibility(callsite, def)
}

func (r *cppScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// C++ uses constraint compatibility for concepts and SFINAE-based overload resolution.
	// TODO: wire to CppConstraintCompatibility when constraint extraction is ready.
	return CppConstraintCompatibilityHook
}

// --- MRO ---

func (r *cppScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// C++ uses C3 linearization for multiple inheritance MRO with virtual base resolution.
	// TODO: wire to buildCppMro(graph, parsedFiles, nodeLookup) when the MRO pass is integrated.
	return nil
}

func (r *cppScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// C++ needs extends-only MRO for non-virtual diamond resolution.
	return nil
}

// --- Owner population ---

func (r *cppScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	PopulateCppOwners(parsed)
}

func (r *cppScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// C++ has no "super" or "base" keyword for dispatch (though some compilers
	// support __super as an extension, it's not standard).
	return false
}

// --- Optional toggle hooks ---

func (r *cppScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// C++ propagates return types across #include boundaries via header declarations.
	return true
}

func (r *cppScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// C++ is statically typed; no dynamic field fallback.
	return false
}

func (r *cppScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// C++ doesn't use property-style collection accessors.
	return nil
}

func (r *cppScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	// C++ method calls should not be collapsed — overload resolution is context-dependent.
	return false
}

func (r *cppScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	// C++ uses namespace siblings for ADL (argument-dependent lookup).
	// Inline namespaces contribute their members to the enclosing namespace.
	// TODO: wire to PopulateCppNamespaceSiblings when namespace resolution is ready.
	return nil
}

func (r *cppScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// C++ hoists return-type bindings to module scope so
	// propagateImportedReturnTypes can mirror them across translation units.
	return true
}

// --- Heritage / implicit import hooks ---

func (r *cppScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// C++ heritage edges include:
	//   - class : public Base → EXTENDS edges
	//   - virtual inheritance diamond resolution
	// TODO: wire to emitCppHeritageEdges when graph emission is ready.
	return nil
}

func (r *cppScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// C++ has implicit includes from the standard library and
	// ADL-triggered transitive visibility.
	// TODO: wire to emitCppImplicitImportEdges when graph emission is ready.
	return nil
}

func (r *cppScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// C++ is the primary consumer of the capture side channel.
	// Template instantiation and constraint resolution write auxiliary
	// bindings to the side channel that must be restored before
	// the binding-merge pass.
	return RestoreCppCaptureSideChannel
}

// --- Optional receiver-resolution hooks ---

func (r *cppScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// C++ has no super/base keyword, but uses context-dependent
	// qualified-name resolution for dependent base classes.
	// TODO: wire to isCppSuperReceiverInContext when scope-resolution is integrated.
	return nil
}

func (r *cppScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// C++ resolves implicit `this->member` calls against the enclosing
	// class + MRO even without an explicit typeBinding in scope.
	return true
}

func (r *cppScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// C++ qualified-name lookup needs context-aware member resolution.
	// TODO: wire to resolveCppReceiverMember when scope-resolution is integrated.
	return nil
}

func (r *cppScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// C++ qualified namespace resolution (outer::inner::foo).
	// TODO: wire to resolveCppQualifiedReceiverMember when scope-resolution is integrated.
	return nil
}

func (r *cppScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// C++ has no static-only filtering (no companion-object promotion).
	return nil
}

func (r *cppScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// C++ uses conversion-rank scoring for overload resolution.
	// TODO: wire to cppConversionRank when scope-resolution is integrated.
	return nil
}
