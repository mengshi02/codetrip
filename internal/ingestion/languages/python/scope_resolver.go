// Package python — Python ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for Python,
// providing per-language hooks for the generic scope-resolution orchestrator.
//
// The provider is a thin wiring object — Python's specific bits
// (super recognizer, LEGB merge precedence, Python's relative-import
// resolver, the C3 MRO walk) plug into runScopeResolution.
//
// Ported from TS languages/python/scope-resolver.ts.
package python

import (
	"regexp"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PythonScopeResolver implements scope_resolution.ScopeResolver for Python.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for Python source code.
var PythonScopeResolver scope_resolution.ScopeResolver = &pythonScopeResolverImpl{}

// pythonScopeResolverImpl is the concrete implementation.
type pythonScopeResolverImpl struct{}

// superReceiverRegex matches Python super() calls.
// Mirrors TS /^super\s*\(/ pattern.
var superReceiverRegex = regexp.MustCompile(`^super\s*\(`)

// --- Core identity methods ---\n

func (r *pythonScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguagePython
}

func (r *pythonScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return PythonProvider()
}

func (r *pythonScopeResolverImpl) ImportEdgeReason() string {
	return "python-scope: import"
}

// --- Import resolution ---\n

func (r *pythonScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	return ResolvePythonImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig)
}

func (r *pythonScopeResolverImpl) ExpandsWildcardTo() func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// Python supports wildcard imports (from m import *).
	// TODO: wire to Python wildcard expansion when scope-resolution is integrated.
	return nil
}

func (r *pythonScopeResolverImpl) LoadResolutionConfig() func(repoPath string) interface{} {
	// Python imports are relative (PEP-328); no config file needed.
	// Returns nil — no resolution config loading required.
	return nil
}

// --- Binding merge ---\n

func (r *pythonScopeResolverImpl) MergeBindings(
	existing []shared.BindingRef,
	incoming []shared.BindingRef,
	scopeID shared.ScopeID,
) []shared.BindingRef {
	// Python LEGB precedence: local > import/namespace/reexport > wildcard.
	// The per-scope id is unused by pythonMergeBindings (tier ordering
	// is computed purely from BindingRef.origin), so we merge all and apply LEGB.
	return PythonMergeBindings(append(existing, incoming...))
}

// --- Arity / constraint compatibility ---\n

func (r *pythonScopeResolverImpl) ArityCompatibility(
	callsite shared.Callsite,
	def shared.SymbolDefinition,
) scope_resolution.ArityVerdict {
	// Adapter: pythonArityCompatibility predates RegistryProviders and
	// uses (def, callsite). ScopeResolver contract is (callsite, def).
	// Wrapper kept to honor both contracts.
	return PythonArityCompatibility(def, callsite)
}

func (r *pythonScopeResolverImpl) ConstraintCompatibility() func(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// Python has no constrained-overload semantics; return nil.
	return nil
}

// --- MRO ---\n

func (r *pythonScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Python uses C3 linearization for MRO (defaultLinearize).
	// TODO: wire to buildMro(graph, parsedFiles, nodeLookup, defaultLinearize)
	// when the MRO pass is integrated.
	return nil
}

func (r *pythonScopeResolverImpl) BuildExtendsOnlyMro() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// Python doesn't need extends-only MRO (no mixin/trait augmentation separate from inheritance).
	return nil
}

// --- Owner population ---\n

func (r *pythonScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	// Python class-body methods need owner population.
	// Mirrors TS populateClassOwnedMembers(parsed).
	// TODO: wire to populateClassOwnedMembers when scope-resolution walkers are integrated.
	populatePythonOwners(parsed)
}

func (r *pythonScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// Python: "super(...)" calls indicate a super receiver.
	// Mirrors TS /^super\s*\(/ pattern.
	return superReceiverRegex.MatchString(receiverName)
}

// --- Optional toggle hooks ---\n

func (r *pythonScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	// Python is dynamically typed — return-type propagation across imports ON.
	// Listed explicitly for documentation (default true).
	return true
}

func (r *pythonScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	// Python is dynamically typed — field-fallback heuristic ON.
	// Listed explicitly for documentation (default true).
	return true
}

func (r *pythonScopeResolverImpl) UnwrapCollectionAccessor() func(def *shared.SymbolDefinition) *shared.SymbolDefinition {
	// Python doesn't use property-style collection accessors.
	return nil
}

func (r *pythonScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	return false
}

func (r *pythonScopeResolverImpl) PopulateNamespaceSiblings() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	// Python doesn't use namespace-sibling visibility (no implicit cross-package visibility).
	return nil
}

func (r *pythonScopeResolverImpl) HoistTypeBindingsToModule() bool {
	// Python method return types are on Function scopes; no Module-level hoisting needed.
	return false
}

// --- Heritage / implicit import hooks ---\n

func (r *pythonScopeResolverImpl) EmitHeritageEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	scopes *model.ScopeResolutionIndexes,
) {
	// Python class inheritance edges are handled by the generic preEmitInheritanceEdges pass.
	return nil
}

func (r *pythonScopeResolverImpl) EmitImplicitImportEdges() func(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	resolutionConfig interface{},
) {
	// Python doesn't have implicit cross-file visibility (no package-level auto-import).
	return nil
}

func (r *pythonScopeResolverImpl) RestoreCaptureSideChannel() func(parsed *shared.ParsedFile) {
	// Python doesn't use capture side channels (C++ is the only consumer).
	return nil
}

// --- Optional receiver-resolution hooks ---

func (r *pythonScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// Python uses text-only IsSuperReceiver (super() pattern matching).
	return nil
}

func (r *pythonScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// Python resolves `self.method()` via explicit typeBinding; no special `this` handling.
	return false
}

func (r *pythonScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// Python uses the generic MRO walk for member resolution.
	return nil
}

func (r *pythonScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// Python doesn't have C++-style qualified namespace resolution.
	return nil
}

func (r *pythonScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	// Python has no static-only filtering (no companion-object promotion).
	return nil
}

func (r *pythonScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	// Python is dynamically typed — no conversion-rank scoring.
	return nil
}

// populatePythonOwners assigns owner IDs to Python class-body methods
// that the legacy parse pass missed.
// TODO: full implementation — requires scope tree traversal to find
// class_definition parent scopes and assign their method defs as owned members.
func populatePythonOwners(parsed *shared.ParsedFile) {
	// TODO: implement when scope-resolution walkers are integrated.
	// Mirrors TS populateClassOwnedMembers(parsed):
	//   - Walk all scopes in parsed.Scopes
	//   - For Class scopes, collect their child Function scopes' defs
	//   - Set OwnerID on each method def to point to the class scope's first def
}