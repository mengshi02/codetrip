package cpp

// C++ Inline Namespaces — handle inline namespace visibility.
//
// `inline namespace v1 { void foo(); }` has two ISO C++ semantics:
//
//   1. Transitive unqualified visibility: names declared in an inline namespace
//      are reachable by unqualified lookup from the enclosing namespace's scope.
//   2. Transitive qualified visibility: outer::foo() resolves to outer::v1::foo()
//      when v1 is inline. The qualified-namespace receiver resolver walks
//      inline-namespace children transitively when collecting candidates.
//
// State lifecycle: capture-time markCppInlineNamespaceRange records each inline
// namespace's source range; populateCppInlineNamespaceScopes resolves ranges to
// ScopeIDs during populateOwners. Cleared via clearCppInlineNamespaces.
//
// STL idiom: std::__1::vector (libc++) and std::__cxx11 (libstdc++) are inline
// namespaces of std. With this support, std::vector qualified calls resolve to
// the inline-namespace declaration transparently.
//
// Ported from TS languages/cpp/inline-namespaces.ts.

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ────────────────────────────────────────────────────────────────────────────
// Module-level state
// ────────────────────────────────────────────────────────────────────────────

var inlineNsMutex sync.RWMutex

// inlineNamespaceRangesByFile maps filePath → set of range keys for inline namespace blocks.
var inlineNamespaceRangesByFile map[string]map[string]bool

// inlineNamespaceScopeIDs records ScopeIDs resolved from inline-namespace ranges.
var inlineNamespaceScopeIDs map[shared.ScopeID]bool

// legacyInlineNsRegistry maps inline namespace scope ID → enclosing namespace scope ID.
// Used by the legacy RegisterCppInlineNamespace/IsCppInlineNamespace API.
var legacyInlineNsRegistry map[string]string

func init() {
	inlineNamespaceRangesByFile = make(map[string]map[string]bool)
	inlineNamespaceScopeIDs = make(map[shared.ScopeID]bool)
	legacyInlineNsRegistry = make(map[string]string)
}

// ────────────────────────────────────────────────────────────────────────────
// Range key helpers
// ────────────────────────────────────────────────────────────────────────────

func inlineNsRangeKey(r shared.Range) string {
	return fmt.Sprintf("%d:%d:%d:%d", r.StartLine, r.StartCol, r.EndLine, r.EndCol)
}

// ────────────────────────────────────────────────────────────────────────────
// Capture-time recording
// ────────────────────────────────────────────────────────────────────────────

// MarkCppInlineNamespaceRange records an inline namespace_definition source range during capture.
func MarkCppInlineNamespaceRange(filePath string, r shared.Range) {
	inlineNsMutex.Lock()
	defer inlineNsMutex.Unlock()
	if inlineNamespaceRangesByFile[filePath] == nil {
		inlineNamespaceRangesByFile[filePath] = make(map[string]bool)
	}
	inlineNamespaceRangesByFile[filePath][inlineNsRangeKey(r)] = true
}

// ────────────────────────────────────────────────────────────────────────────
// Side-channel serialization
// ────────────────────────────────────────────────────────────────────────────

// CollectCppInlineNamespaceSideChannel snapshots this file's captured inline-namespace ranges.
func CollectCppInlineNamespaceSideChannel(filePath string) []string {
	inlineNsMutex.RLock()
	defer inlineNsMutex.RUnlock()
	set := inlineNamespaceRangesByFile[filePath]
	var result []string
	for k := range set {
		result = append(result, k)
	}
	return result
}

// ApplyCppInlineNamespaceSideChannel restores this file's captured ranges from the side-channel.
func ApplyCppInlineNamespaceSideChannel(filePath string, ranges []string) {
	if len(ranges) == 0 {
		return
	}
	inlineNsMutex.Lock()
	defer inlineNsMutex.Unlock()
	if inlineNamespaceRangesByFile[filePath] == nil {
		inlineNamespaceRangesByFile[filePath] = make(map[string]bool)
	}
	for _, r := range ranges {
		inlineNamespaceRangesByFile[filePath][r] = true
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Clear state
// ────────────────────────────────────────────────────────────────────────────

// ClearCppInlineNamespaces resets all inline-namespace state.
func ClearCppInlineNamespaces() {
	inlineNsMutex.Lock()
	defer inlineNsMutex.Unlock()
	inlineNamespaceRangesByFile = make(map[string]map[string]bool)
	inlineNamespaceScopeIDs = make(map[shared.ScopeID]bool)
	legacyInlineNsRegistry = make(map[string]string)
}

// ────────────────────────────────────────────────────────────────────────────
// Populate inline namespace scopes
// ────────────────────────────────────────────────────────────────────────────

// PopulateCppInlineNamespaceScopes resolves captured inline-namespace ranges to ScopeIDs.
// Run from the cpp resolver's populateOwners hook.
func PopulateCppInlineNamespaceScopes(parsed *shared.ParsedFile) {
	inlineNsMutex.Lock()
	defer inlineNsMutex.Unlock()

	ranges := inlineNamespaceRangesByFile[parsed.FilePath]
	if len(ranges) == 0 {
		return
	}
	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindNamespace {
			continue
		}
		rk := inlineNsRangeKey(scope.Range)
		if ranges[rk] {
			inlineNamespaceScopeIDs[scope.ID] = true
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Scope predicates
// ────────────────────────────────────────────────────────────────────────────

// IsCppInlineNamespaceScope checks whether a scope is an inline namespace.
// Consumed by populateCppNonGloballyVisible to exempt inline-namespace members
// from cross-file unqualified-lookup exclusion.
func IsCppInlineNamespaceScope(scopeID shared.ScopeID) bool {
	inlineNsMutex.RLock()
	defer inlineNsMutex.RUnlock()
	return inlineNamespaceScopeIDs[scopeID]
}

// ────────────────────────────────────────────────────────────────────────────
// Qualified namespace member resolution
// ────────────────────────────────────────────────────────────────────────────

// ResolveCppQualifiedNamespaceMember walks every parsed file looking for a
// Namespace scope whose qualified name matches receiverName, collects its
// callable ownedDefs matching memberName, transitively descending into any
// inline-namespace children.
//
// Returns the most specific (innermost) match, "ambiguous", or nil.
func ResolveCppQualifiedNamespaceMember(
	receiverName string,
	memberName string,
	parsedFiles []*shared.ParsedFile,
) (*shared.SymbolDefinition, string) {
	inlineNsMutex.RLock()
	defer inlineNsMutex.RUnlock()

	var allHits []shared.SymbolDefinition
	seenNodeID := make(map[string]bool)

	for _, parsed := range parsedFiles {
		scopesByID := make(map[shared.ScopeID]*shared.Scope)
		for _, sc := range parsed.Scopes {
			scopesByID[sc.ID] = sc
		}

		for _, scope := range parsed.Scopes {
			if scope.Kind != shared.ScopeKindNamespace {
				continue
			}
			nsDef := findInlineNsNamespaceDef(scope.OwnedDefs)
			if nsDef == nil {
				continue
			}
			nsName := inlineNsSimpleName(*nsDef)
			if nsName != receiverName {
				continue
			}
			// Found a matching namespace scope. Collect members transitively.
			hits := findMemberInNamespaceTransitive(scope, scopesByID, memberName)
			for _, hit := range hits {
				if seenNodeID[hit.NodeID] {
					continue
				}
				seenNodeID[hit.NodeID] = true
				allHits = append(allHits, hit)
			}
		}
	}

	if len(allHits) == 0 {
		return nil, ""
	}
	if len(allHits) == 1 {
		return &allHits[0], ""
	}

	// Multi-candidate: conservative approach — check arity compatibility if available.
	// For now, return ambiguous if multiple distinct candidates survive.
	// TODO: wire narrowOverloadCandidates + cppConversionRank when available.

	return nil, "ambiguous"
}

// findMemberInNamespaceTransitive recursively searches a namespace scope and
// any inline-namespace descendants for callable defs with the given simple name.
func findMemberInNamespaceTransitive(
	scope *shared.Scope,
	scopesByID map[shared.ScopeID]*shared.Scope,
	memberName string,
) []shared.SymbolDefinition {
	var results []shared.SymbolDefinition

	// Check this scope's own ownedDefs first
	for _, def := range scope.OwnedDefs {
		if def.Type != "Function" && def.Type != "Method" && def.Type != "Constructor" {
			continue
		}
		simple := inlineNsSimpleName(def)
		if simple == memberName {
			results = append(results, def)
		}
	}

	// Descend into inline-namespace children
	for _, childScope := range scopesByID {
		if childScope.Parent == nil {
			continue
		}
		if *childScope.Parent != scope.ID {
			continue
		}
		if childScope.Kind != shared.ScopeKindNamespace {
			continue
		}
		if !inlineNamespaceScopeIDs[childScope.ID] {
			continue
		}
		childHits := findMemberInNamespaceTransitive(childScope, scopesByID, memberName)
		results = append(results, childHits...)
	}
	return results
}

func findInlineNsNamespaceDef(defs []shared.SymbolDefinition) *shared.SymbolDefinition {
	for i := range defs {
		if defs[i].Type == "Namespace" {
			return &defs[i]
		}
	}
	return nil
}

func inlineNsSimpleName(def shared.SymbolDefinition) string {
	if def.QualifiedName != nil {
		parts := strings.Split(*def.QualifiedName, ".")
		return parts[len(parts)-1]
	}
	return ""
}

// ────────────────────────────────────────────────────────────────────────────
// Legacy API compatibility
// ────────────────────────────────────────────────────────────────────────────

// RegisterCppInlineNamespace registers an inline namespace relationship (legacy API).
func RegisterCppInlineNamespace(inlineScopeID string, enclosingScopeID string) {
	inlineNsMutex.Lock()
	defer inlineNsMutex.Unlock()
	legacyInlineNsRegistry[inlineScopeID] = enclosingScopeID
}

// IsCppInlineNamespace checks whether a namespace is an inline namespace (legacy API).
func IsCppInlineNamespace(scopeID string) bool {
	inlineNsMutex.RLock()
	defer inlineNsMutex.RUnlock()
	_, ok := legacyInlineNsRegistry[scopeID]
	return ok
}

// GetCppEnclosingNamespace returns the enclosing namespace for an inline namespace (legacy API).
func GetCppEnclosingNamespace(inlineScopeID string) string {
	inlineNsMutex.RLock()
	defer inlineNsMutex.RUnlock()
	return legacyInlineNsRegistry[inlineScopeID]
}