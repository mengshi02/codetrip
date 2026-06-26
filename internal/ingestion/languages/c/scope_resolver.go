// Package c — C ScopeResolver implementation.
// Implements the scope_resolution.ScopeResolver interface for C,
// providing per-language hooks for the generic scope-resolution orchestrator.
//
// Key C-specific behaviors:
//   - No namespaces — flat symbol space with file-level scoping via static
//   - #include as imports (importEdgeReason = "c-scope: include")
//   - Static linkage: file-local functions excluded from cross-file free-call fallback
//   - No overloading — arity compatibility is straightforward
//   - No MRO, no constraints, no wildcard imports
//   - isSuperReceiver always false (C has no OOP)
//   - propagatesReturnTypesAcrossImports = false
//   - fieldFallbackOnMethodLookup = false
//
// Ported from TS languages/c/scope-resolver.ts.
package c

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CScopeResolver implements scope_resolution.ScopeResolver for C.
// It is the EMIT-SIDE contract — how the resolution pipeline dispatches
// references to graph edges for C source code.
var CScopeResolver scope_resolution.ScopeResolver = &cScopeResolverImpl{}

// cScopeResolverImpl is the concrete implementation.
type cScopeResolverImpl struct{}

// --- Core identity methods ---

func (r *cScopeResolverImpl) Language() shared.SupportedLanguage {
	return shared.SupportedLanguageC
}

func (r *cScopeResolverImpl) LanguageProvider() core.LanguageProvider {
	return CProvider()
}

func (r *cScopeResolverImpl) ImportEdgeReason() string {
	return "c-scope: include"
}

// --- Import resolution ---

func (r *cScopeResolverImpl) ResolveImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	// Augment allFilePaths with .h files discovered via LoadResolutionConfig
	// since the phase only passes .c files to the C resolver but #include
	// targets .h files (classified as C++ in language detection).
	// Ported from TS c/scope-resolver.ts resolveImportTarget.
	if resolutionConfig != nil {
		if headerPaths, ok := resolutionConfig.(map[string]bool); ok && len(headerPaths) > 0 {
			augmented := make(map[string]bool, len(allFilePaths)+len(headerPaths))
			for k, v := range allFilePaths {
				augmented[k] = v
			}
			for k, v := range headerPaths {
				augmented[k] = v
			}
			resolved := ResolveCImportTarget(targetRaw, fromFile, augmented)
			if resolved != "" {
				return []string{resolved}
			}
			return nil
		}
	}
	resolved := ResolveCImportTarget(targetRaw, fromFile, allFilePaths)
	if resolved != "" {
		return []string{resolved}
	}
	return nil
}

func (r *cScopeResolverImpl) ExpandsWildcardTo() func(shared.ScopeID, []*shared.ParsedFile) []string {
	// C #include brings in all symbols — enable global free call fallback.
	// ExpandCWildcardNames filters out static (file-local) names.
	// Ported from TS c/scope-resolver.ts expandsWildcardTo.
	return func(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
		// Find the target parsed file by module scope ID
		for _, pf := range parsedFiles {
			if pf.ModuleScope != nil && pf.ModuleScope.ID == targetModuleScope {
				// Collect symbol defs as SymbolDef slice
				var defs []SymbolDef
				for _, d := range pf.LocalDefs {
					qn := ""
					if d.QualifiedName != nil {
						qn = *d.QualifiedName
					}
					defs = append(defs, SymbolDef{
						QualifiedName: qn,
						FilePath:      d.FilePath,
					})
				}
				return ExpandCWildcardNames(pf.FilePath, defs)
			}
		}
		return nil
	}
}

func (r *cScopeResolverImpl) LoadResolutionConfig() func(string) interface{} {
	// Scan for .h files so the C resolver can resolve #include targets.
	// Ported from TS c/scope-resolver.ts loadResolutionConfig.
	return func(repoPath string) interface{} {
		return ScanHeaderFiles(repoPath)
	}
}

// --- Binding merge ---

func (r *cScopeResolverImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID shared.ScopeID) []shared.BindingRef {
	return CMergeBindings(existing, incoming)
}

// --- Arity compatibility ---

func (r *cScopeResolverImpl) ArityCompatibility(callsite shared.Callsite, def shared.SymbolDefinition) scope_resolution.ArityVerdict {
	return CArityCompatibility(callsite, def)
}

// --- Constraint compatibility ---

func (r *cScopeResolverImpl) ConstraintCompatibility() func(shared.ReferenceSite, shared.SymbolDefinition, shared.ParameterTypeClass) scope_resolution.ArityVerdict {
	// C has no constrained-overload semantics
	return nil
}

// --- MRO ---

func (r *cScopeResolverImpl) BuildMro(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]string {
	// C has no classes or inheritance
	return nil
}

func (r *cScopeResolverImpl) BuildExtendsOnlyMro() func(shared.KnowledgeGraph, []*shared.ParsedFile, *scope_resolution.GraphNodeLookup) map[string][]string {
	// C has no class hierarchy
	return nil
}

// --- Owners ---

func (r *cScopeResolverImpl) PopulateOwners(parsed *shared.ParsedFile) {
	// C has no out-of-class method definitions that need owner assignment
}

// --- Super receiver ---

func (r *cScopeResolverImpl) IsSuperReceiver(receiverName string) bool {
	// C has no OOP — no super/base keyword
	return false
}

// --- Optional toggles / hooks ---

func (r *cScopeResolverImpl) PropagatesReturnTypesAcrossImports() bool {
	return false
}

func (r *cScopeResolverImpl) FieldFallbackOnMethodLookup() bool {
	return false
}

func (r *cScopeResolverImpl) UnwrapCollectionAccessor() func(*shared.SymbolDefinition) *shared.SymbolDefinition {
	// C has no property-style collection accessors
	return nil
}

func (r *cScopeResolverImpl) CollapseMemberCallsByCallerTarget() bool {
	return false
}

func (r *cScopeResolverImpl) PopulateNamespaceSiblings() func(shared.KnowledgeGraph, []*shared.ParsedFile, *scope_resolution.GraphNodeLookup, *model.ScopeResolutionIndexes) {
	// C has no namespaces
	return nil
}

func (r *cScopeResolverImpl) HoistTypeBindingsToModule() bool {
	return false
}

func (r *cScopeResolverImpl) EmitHeritageEdges() func(shared.KnowledgeGraph, []*shared.ParsedFile, *scope_resolution.GraphNodeLookup, *model.ScopeResolutionIndexes) {
	// C has no inheritance
	return nil
}

func (r *cScopeResolverImpl) EmitImplicitImportEdges() func(shared.KnowledgeGraph, []*shared.ParsedFile, *scope_resolution.GraphNodeLookup, interface{}) {
	// C has no implicit imports (all includes are explicit)
	return nil
}

func (r *cScopeResolverImpl) RestoreCaptureSideChannel() func(*shared.ParsedFile) {
	// Restore static-linkage side channel from worker-serialized snapshot.
	// Ported from TS c/scope-resolver.ts applyCaptureSideChannel.
	return func(parsed *shared.ParsedFile) {
		if parsed == nil {
			return
		}
		// The capture side channel data is stored on ParsedFile.CaptureSideChannel.
		// ApplyCStaticLinkageSideChannel restores it into the module-level map.
		ApplyCStaticLinkageSideChannel(parsed.FilePath, parsed.CaptureSideChannel)
	}
}

// --- Optional receiver-resolution hooks ---

func (r *cScopeResolverImpl) IsSuperReceiverInContext() func(receiverName string, scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) bool {
	// C has no super/base keyword and no classes.
	return nil
}

func (r *cScopeResolverImpl) ResolveThisViaEnclosingClass() bool {
	// C has no `this`/`self` class-based dispatch.
	return false
}

func (r *cScopeResolverImpl) ResolveReceiverMember() func(
	ownerDef *shared.SymbolDefinition,
	memberName string,
	site shared.ReferenceSite,
	indexes *model.ScopeResolutionIndexes,
	model model.SemanticModel,
) *scope_resolution.ReceiverMemberResolution {
	// C has no class-based member resolution.
	return nil
}

func (r *cScopeResolverImpl) ResolveQualifiedReceiverMember() func(
	receiverName string,
	memberName string,
	scopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	parsedFiles []*shared.ParsedFile,
	site shared.ReferenceSite,
) *scope_resolution.ResolveQualifiedReceiverMemberResult {
	// C has no qualified namespace resolution.
	return nil
}

func (r *cScopeResolverImpl) IsStaticOnly() func(def *shared.SymbolDefinition) bool {
	return nil
}

func (r *cScopeResolverImpl) ConversionRankFn() func(callType string, paramType string) int {
	return nil
}
