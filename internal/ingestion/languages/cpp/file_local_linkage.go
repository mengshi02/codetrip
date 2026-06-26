package cpp

// C++ File-Local Linkage — track anonymous-namespace and static declarations.
//
// In C++, entities in an anonymous namespace or declared with the `static`
// keyword have file-local linkage — they are only visible within the
// translation unit (source file) where they are defined.
//
// This module tracks file-local definitions and non-globally-visible defs so
// the scope resolver can avoid emitting cross-file resolution edges for them.
//
// Ported from TS languages/cpp/file-local-linkage.ts.

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ────────────────────────────────────────────────────────────────────────────
// Module-level state
// ────────────────────────────────────────────────────────────────────────────

var fileLocalMutex sync.RWMutex

// fileLocalNames maps filePath → set of file-local symbol names (static / anonymous namespace).
var fileLocalNames map[string]map[string]bool

// nonGloballyVisibleNodeIDs maps filePath → set of nodeIDs NOT visible by unqualified lookup from outside the file.
var nonGloballyVisibleNodeIDs map[string]map[string]bool

// anonymousNamespaceRangesByFile maps filePath → set of range keys identifying namespace { ... } blocks.
var anonymousNamespaceRangesByFile map[string]map[string]bool

// anonymousNamespaceScopeIDs records ScopeIDs resolved from anonymous-namespace ranges.
var anonymousNamespaceScopeIDs map[shared.ScopeID]bool

func init() {
	fileLocalNames = make(map[string]map[string]bool)
	nonGloballyVisibleNodeIDs = make(map[string]map[string]bool)
	anonymousNamespaceRangesByFile = make(map[string]map[string]bool)
	anonymousNamespaceScopeIDs = make(map[shared.ScopeID]bool)
}

// ────────────────────────────────────────────────────────────────────────────
// Range key helpers
// ────────────────────────────────────────────────────────────────────────────

func fileLocalRangeKey(r shared.Range) string {
	return fmt.Sprintf("%d:%d:%d:%d", r.StartLine, r.StartCol, r.EndLine, r.EndCol)
}

// ────────────────────────────────────────────────────────────────────────────
// Capture-time recording functions
// ────────────────────────────────────────────────────────────────────────────

// MarkFileLocal records a symbol name as file-local (static or anonymous namespace).
func MarkFileLocal(filePath string, name string) {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()
	if fileLocalNames[filePath] == nil {
		fileLocalNames[filePath] = make(map[string]bool)
	}
	fileLocalNames[filePath][name] = true
}

// IsFileLocal checks whether a symbol name has file-local linkage in the given file.
func IsFileLocal(filePath string, name string) bool {
	fileLocalMutex.RLock()
	defer fileLocalMutex.RUnlock()
	return fileLocalNames[filePath][name]
}

// MarkCppAnonymousNamespaceRange records an anonymous namespace_definition source range during capture.
func MarkCppAnonymousNamespaceRange(filePath string, r shared.Range) {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()
	if anonymousNamespaceRangesByFile[filePath] == nil {
		anonymousNamespaceRangesByFile[filePath] = make(map[string]bool)
	}
	anonymousNamespaceRangesByFile[filePath][fileLocalRangeKey(r)] = true
}

// IsCppAnonymousNamespaceScope checks whether a scope is an anonymous namespace.
func IsCppAnonymousNamespaceScope(scopeID shared.ScopeID) bool {
	fileLocalMutex.RLock()
	defer fileLocalMutex.RUnlock()
	return anonymousNamespaceScopeIDs[scopeID]
}

// ────────────────────────────────────────────────────────────────────────────
// Side-channel serialization (worker → main boundary)
// ────────────────────────────────────────────────────────────────────────────

// CppFileLocalSideChannel is a JSON-serializable snapshot of per-file capture state.
type CppFileLocalSideChannel struct {
	FileLocalNames           []string `json:"fileLocalNames"`
	AnonymousNamespaceRanges []string `json:"anonymousNamespaceRanges"`
}

// CollectCppFileLocalSideChannel snapshots this file's capture state for the side-channel.
func CollectCppFileLocalSideChannel(filePath string) CppFileLocalSideChannel {
	fileLocalMutex.RLock()
	defer fileLocalMutex.RUnlock()

	names := fileLocalNames[filePath]
	var nameSlice []string
	for n := range names {
		nameSlice = append(nameSlice, n)
	}

	ranges := anonymousNamespaceRangesByFile[filePath]
	var rangeSlice []string
	for r := range ranges {
		rangeSlice = append(rangeSlice, r)
	}

	return CppFileLocalSideChannel{
		FileLocalNames:           nameSlice,
		AnonymousNamespaceRanges: rangeSlice,
	}
}

// ApplyCppFileLocalSideChannel restores this file's capture state from the side-channel.
func ApplyCppFileLocalSideChannel(filePath string, data CppFileLocalSideChannel) {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()

	for _, name := range data.FileLocalNames {
		if fileLocalNames[filePath] == nil {
			fileLocalNames[filePath] = make(map[string]bool)
		}
		fileLocalNames[filePath][name] = true
	}

	if len(data.AnonymousNamespaceRanges) > 0 {
		if anonymousNamespaceRangesByFile[filePath] == nil {
			anonymousNamespaceRangesByFile[filePath] = make(map[string]bool)
		}
		for _, r := range data.AnonymousNamespaceRanges {
			anonymousNamespaceRangesByFile[filePath][r] = true
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Clear state
// ────────────────────────────────────────────────────────────────────────────

// ClearFileLocalNames resets all file-local-linkage state at the start of each resolution pass.
func ClearFileLocalNames() {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()
	fileLocalNames = make(map[string]map[string]bool)
	nonGloballyVisibleNodeIDs = make(map[string]map[string]bool)
	anonymousNamespaceRangesByFile = make(map[string]map[string]bool)
	anonymousNamespaceScopeIDs = make(map[shared.ScopeID]bool)
}

// ────────────────────────────────────────────────────────────────────────────
// Populate anonymous namespace scopes — resolve recorded ranges to ScopeIDs
// ────────────────────────────────────────────────────────────────────────────

// PopulateCppAnonymousNamespaceScopes resolves recorded anonymous-namespace source ranges
// to ScopeIDs. Must run inside PopulateOwners BEFORE PopulateCppNonGloballyVisible consults
// the resolved set.
func PopulateCppAnonymousNamespaceScopes(parsed *shared.ParsedFile) {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()

	ranges := anonymousNamespaceRangesByFile[parsed.FilePath]
	if len(ranges) == 0 {
		return
	}

	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindNamespace {
			continue
		}
		rk := fileLocalRangeKey(scope.Range)
		if ranges[rk] {
			anonymousNamespaceScopeIDs[scope.ID] = true
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Populate non-globally-visible defs
// ────────────────────────────────────────────────────────────────────────────

// PopulateCppNonGloballyVisible walks parsed.scopes to mark defs that are NOT
// visible by unqualified lookup from outside the file. A def is "not globally
// visible" when its nearest structurally enclosing scope is a Namespace or
// Class — those require qualification (ns::name, Class::method) for cross-file
// unqualified lookup. Module-scoped defs remain globally visible.
//
// Inline namespaces and anonymous namespaces are exempt.
func PopulateCppNonGloballyVisible(parsed *shared.ParsedFile) {
	fileLocalMutex.Lock()
	defer fileLocalMutex.Unlock()

	if nonGloballyVisibleNodeIDs[parsed.FilePath] == nil {
		nonGloballyVisibleNodeIDs[parsed.FilePath] = make(map[string]bool)
	}
	s := nonGloballyVisibleNodeIDs[parsed.FilePath]

	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindNamespace && scope.Kind != shared.ScopeKindClass {
			continue
		}
		// Inline namespaces propagate their members to the enclosing namespace's
		// unqualified-lookup scope per ISO C++ [namespace.def]/p4. Skip them.
		if scope.Kind == shared.ScopeKindNamespace && IsCppInlineNamespaceScope(scope.ID) {
			continue
		}
		// Anonymous namespaces: internal linkage but contents are visible at the
		// enclosing scope within the same TU. The IsFileLocal mark (recorded on the
		// def's name) still blocks cross-file unqualified lookup that does not go
		// through #include, so dropping the structural visibility exclusion is safe.
		if scope.Kind == shared.ScopeKindNamespace && anonymousNamespaceScopeIDs[scope.ID] {
			continue
		}
		for _, def := range scope.OwnedDefs {
			s[def.NodeID] = true
		}
	}
}

// IsCppDefGloballyVisible checks whether a def is visible by unqualified lookup
// from outside its own file. Returns false for class-owned and namespace-nested defs.
func IsCppDefGloballyVisible(filePath string, nodeID string) bool {
	fileLocalMutex.RLock()
	defer fileLocalMutex.RUnlock()
	return !nonGloballyVisibleNodeIDs[filePath][nodeID]
}

// ────────────────────────────────────────────────────────────────────────────
// Wildcard import expansion — expandCppWildcardNames
// ────────────────────────────────────────────────────────────────────────────

// ExpandCppWildcardNames returns the names visible through a C++ wildcard import
// (#include or using namespace). Only defs whose nearest enclosing scope is the
// header's Module scope are emitted as wildcard-binding names.
//
// Class members and namespace-nested symbols are NOT visible by unqualified
// lookup from a free function in an including TU — they require qualification.
func ExpandCppWildcardNames(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	fileLocalMutex.RLock()
	defer fileLocalMutex.RUnlock()

	// Find the parsed file for the target module scope
	var target *shared.ParsedFile
	for _, p := range parsedFiles {
		if p.ModuleScope != nil && p.ModuleScope.ID == targetModuleScope {
			target = p
			break
		}
	}
	if target == nil {
		return nil
	}

	// Build nodeId → owning Scope map from the structural scope tree
	ownerScopeByNodeID := make(map[string]*shared.Scope)
	for _, scope := range target.Scopes {
		for _, ownedDef := range scope.OwnedDefs {
			ownerScopeByNodeID[ownedDef.NodeID] = scope
		}
	}

	seen := make(map[string]bool)
	var names []string
	for _, def := range target.LocalDefs {
		// Skip class-owned methods
		if def.OwnerID != nil && *def.OwnerID != "" {
			continue
		}

		// Structural visibility check
		ownerScope := ownerScopeByNodeID[def.NodeID]
		ownerIsAnonymousNamespace := false
		if ownerScope != nil &&
			ownerScope.Kind == shared.ScopeKindNamespace &&
			anonymousNamespaceScopeIDs[ownerScope.ID] {
			ownerIsAnonymousNamespace = true
		}
		if ownerScope != nil &&
			!ownerIsAnonymousNamespace &&
			(ownerScope.Kind == shared.ScopeKindNamespace || ownerScope.Kind == shared.ScopeKindClass) {
			continue
		}

		name := fileLocalSimpleName(def)
		if name == "" {
			continue
		}
		// Exempt anonymous-namespace names from the isFileLocal filter
		if !ownerIsAnonymousNamespace && IsFileLocal(target.FilePath, name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func fileLocalSimpleName(def shared.SymbolDefinition) string {
	if def.QualifiedName != nil {
		parts := strings.Split(*def.QualifiedName, ".")
		return parts[len(parts)-1]
	}
	return ""
}

// ────────────────────────────────────────────────────────────────────────────
// Legacy API compatibility helpers
// ────────────────────────────────────────────────────────────────────────────

// RegisterCppFileLocalDef registers a file-local definition.
// This is the legacy name used by some files.
func RegisterCppFileLocalDef(filePath string, defNodeID string) {
	MarkFileLocal(filePath, defNodeID)
}

// IsCppFileLocal checks whether a definition has file-local linkage.
// Legacy name accepting defNodeID.
func IsCppFileLocal(filePath string, defNodeID string) bool {
	return IsFileLocal(filePath, defNodeID)
}
