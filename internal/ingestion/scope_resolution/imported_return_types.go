package scope_resolution

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RECHAIN_MAX_DEPTH limits how many hops FollowChainPostFinalize will chase
// through type binding chains before giving up (mirrors TS RECHAIN_MAX_DEPTH).
const RECHAIN_MAX_DEPTH = 8

// typeRefEqual compares two TypeRef values for equality. Since TypeRef
// contains a []TypeRef (slice) field, Go's == operator cannot be used
// directly on structs with slices. We compare the identity fields
// (RawName + DeclaredAtScope) which are sufficient for chain-follow
// cycle detection — two TypeRefs with the same RawName and scope are
// considered the same binding reference.
func typeRefEqual(a, b shared.TypeRef) bool {
	return a.RawName == b.RawName && a.DeclaredAtScope == b.DeclaredAtScope
}

// PropagateImportedReturnTypes propagates method return types across import
// boundaries. After finalize, some type bindings reference imported types
// whose full chain hasn't been followed (e.g. `models.User` → the User class
// in another file).
//
// Mirrors TS scope-resolution/passes/imported-return-types.ts.
//
// Contract invariant I3: this pass MUST run AFTER finalizeScopeModel
// (so indexes.bindings is populated) and BEFORE resolveReferenceSites
// (so resolution sees the propagated types). Files are walked in SCC
// reverse-topological order (leaves first) so multi-hop alias chains
// collapse in a single pass.
func PropagateImportedReturnTypes(
	parsedFiles []*shared.ParsedFile,
	provider ScopeResolver,
	indexes *model.ScopeResolutionIndexes,
) {
	// Build the workspace resolution index for module-scope-by-file lookups.
	wsIndex := BuildWorkspaceResolutionIndex(parsedFiles)
	moduleScopeByFile := wsIndex.ModuleScopeByFile

	// Phase 1: SCC-ordered pass — propagate imported type bindings into
	// importer module scopes, then chain-follow each importer's own bindings.
	for _, scc := range indexes.Sccs() {
		for _, filePath := range scc.FilePaths {
			importerModule, ok := moduleScopeByFile[filePath]
			if !ok {
				continue
			}
			// For each name visible at the importer module scope that doesn't
			// already have a local type binding, look for an import/reexport/
			// wildcard binding and propagate the source's terminal type.
			for _, localName := range NamesAtScope(importerModule.ID, indexes) {
				if _, hasBinding := importerModule.TypeBindings[localName]; hasBinding {
					continue
				}
				refs := LookupBindingsAt(importerModule.ID, localName, indexes)
				for _, ref := range refs {
					if ref.Origin != shared.OriginImport && ref.Origin != shared.OriginReexport && ref.Origin != shared.OriginWildcard {
						continue
					}
					sourceModule, ok := moduleScopeByFile[ref.Def.FilePath]
					if !ok {
						continue
					}
					qn := ref.Def.QualifiedName
					if qn == nil {
						continue
					}
					// Extract the simple name from the qualified name (last segment after dot).
					dot := strings.LastIndex(*qn, ".")
					sourceName := *qn
					if dot != -1 {
						sourceName = (*qn)[dot+1:]
					}
					sourceTypeRef, ok := sourceModule.TypeBindings[sourceName]
					if !ok {
						continue
					}
					terminal := FollowChainPostFinalize(sourceTypeRef, sourceModule.ID, indexes)
					importerModule.TypeBindings[localName] = terminal
					break // first matching import/ref wins
				}
			}
			// Chain-follow this importer's own module typeBindings now.
			for name, ref := range importerModule.TypeBindings {
				resolved := FollowChainPostFinalize(ref, importerModule.ID, indexes)
				if !typeRefEqual(resolved, ref) {
					importerModule.TypeBindings[name] = resolved
				}
			}
		}
	}

	// Phase 2: Final pass — chain-follow non-module scopes.
	for _, parsed := range parsedFiles {
		var moduleScopeID shared.ScopeID
		if ms, ok := moduleScopeByFile[parsed.FilePath]; ok {
			moduleScopeID = ms.ID
		}
		for _, scope := range parsed.Scopes {
			if scope.ID == moduleScopeID {
				continue // already handled in Phase 1
			}
			for name, ref := range scope.TypeBindings {
				resolved := FollowChainPostFinalize(ref, scope.ID, indexes)
				if !typeRefEqual(resolved, ref) {
					scope.TypeBindings[name] = resolved
				}
			}
		}
	}
}

// FollowChainPostFinalize follows a type binding chain after finalize,
// resolving through imports to find the terminal type definition.
//
// It walks the scope chain upward looking for type bindings that resolve the
// current rawName, falling back to namespace and workspace type bindings.
// It stops when the rawName contains a dot (already qualified), no next
// binding is found, or a cycle is detected.
//
// Mirrors TS followChainPostFinalize.
func FollowChainPostFinalize(
	start shared.TypeRef,
	fromScopeID shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
) shared.TypeRef {
	current := start
	visited := make(map[string]bool)
	moduleScopeID := ModuleScopeIdOf(fromScopeID, indexes)

	for depth := 0; depth < RECHAIN_MAX_DEPTH; depth++ {
		// If the rawName is already dotted (e.g. "models.User"), it's
		// qualified — no further resolution needed.
		if strings.Contains(current.RawName, ".") {
			return current
		}

		// Walk the scope chain upward looking for a type binding that
		// resolves current.RawName to something different.
		scopeTree := indexes.ScopeTree()
		var next *shared.TypeRef
		scopeID := fromScopeID
		visitedScopes := make(map[shared.ScopeID]bool)

		for scopeID != "" && !visitedScopes[scopeID] {
			visitedScopes[scopeID] = true
			scope := scopeTree.GetScope(scopeID)
			if scope == nil {
				break
			}
			if tr, ok := scope.TypeBindings[current.RawName]; ok {
				if !typeRefEqual(tr, current) {
					next = &tr
					break
				}
			}
			if scope.Parent == nil {
				break
			}
			scopeID = *scope.Parent
		}

		// Fallback 1: namespace type binding
		if next == nil {
			nsHit := NamespaceTypeBindingFor(moduleScopeID, current.RawName, indexes)
			if nsHit != nil && !typeRefEqual(*nsHit, current) {
				next = nsHit
			}
		}

		// Fallback 2: workspace type binding
		if next == nil {
			if wsTypes := indexes.WorkspaceTypeBindings(); wsTypes != nil {
				if wsRef, ok := wsTypes[current.RawName]; ok {
					if wsRef != nil && !typeRefEqual(*wsRef, current) {
						next = wsRef
					}
				}
			}
		}

		// No resolution found — return current.
		if next == nil {
			return current
		}

		// Cycle detection — if we've already visited this rawName, stop.
		if visited[next.RawName] {
			return current
		}
		visited[next.RawName] = true
		current = *next
	}

	return current
}