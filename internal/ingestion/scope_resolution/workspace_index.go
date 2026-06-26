package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// workspace_index.go — cross-file workspace scope-lookup index.
// Ported from TS scope-resolution/workspace-index.ts (177 lines).
// ---------------------------------------------------------------------------

// WorkspaceResolutionIndex is the scope-tied lookup index built once per
// resolution run, after populateOwners and before any resolution pass.
//
// It carries only the lookups that return a Scope — things SemanticModel
// structurally cannot provide:
//   - classScopeByDefId: class def nodeId → *Scope
//   - classScopeIdToDefId: inverse (Scope.id → class def nodeId)
//   - moduleScopeByFile: file path → module *Scope
//   - exportedCallableByName: simpleName → first module-local callable def
type WorkspaceResolutionIndex struct {
	// ClassScopeByDefId maps class def nodeId → *Scope.
	ClassScopeByDefId map[string]*shared.Scope
	// ClassScopeIdToDefId is the inverse: class Scope.id → class def nodeId.
	ClassScopeIdToDefId map[shared.ScopeID]string
	// ModuleScopeByFile maps file path → module *Scope.
	ModuleScopeByFile map[string]*shared.Scope
	// ExportedCallableByName maps simpleName → first module-local callable def.
	ExportedCallableByName map[string]*shared.SymbolDefinition
}

// BuildWorkspaceResolutionIndex builds the workspace scope-lookup index from
// parsed files. Iterates all scopes to collect class scopes and module scopes.
// Mirrors TS buildWorkspaceResolutionIndex.
func BuildWorkspaceResolutionIndex(parsedFiles []*shared.ParsedFile) *WorkspaceResolutionIndex {
	classScopeByDefId := make(map[string]*shared.Scope)
	classScopeIdToDefId := make(map[shared.ScopeID]string)
	moduleScopeByFile := make(map[string]*shared.Scope)
	exportedCallableByName := make(map[string]*shared.SymbolDefinition)

	for _, parsed := range parsedFiles {
		// Find the Module scope
		for _, scope := range parsed.Scopes {
			if scope.Kind == shared.ScopeKindModule {
				moduleScopeByFile[parsed.FilePath] = scope

				// Precompute the findExportedDefByName workspace fallback:
				// first module-local (origin 'local') callable per name,
				// first file wins.
				for name, refs := range scope.Bindings {
					if _, exists := exportedCallableByName[name]; exists {
						continue
					}
					for i := range refs {
						if refs[i].Origin != shared.OriginLocal {
							continue
						}
						t := refs[i].Def.Type
						if t == shared.LabelFunction || t == shared.LabelMethod || t == shared.LabelConstructor {
							defCopy := refs[i].Def
							exportedCallableByName[name] = &defCopy
							break
						}
					}
				}
				break // only one module scope per file
			}
		}

		// Collect Class scopes
		for _, scope := range parsed.Scopes {
			if scope.Kind != shared.ScopeKindClass {
				continue
			}
			cd := findClassLikeDef(scope)
			if cd != nil {
				classScopeByDefId[cd.NodeID] = scope
				classScopeIdToDefId[scope.ID] = cd.NodeID
			}
		}
	}

	return &WorkspaceResolutionIndex{
		ClassScopeByDefId:      classScopeByDefId,
		ClassScopeIdToDefId:    classScopeIdToDefId,
		ModuleScopeByFile:      moduleScopeByFile,
		ExportedCallableByName: exportedCallableByName,
	}
}

// LookupClassScopeByDefId returns the Class scope for a class def nodeId.
func (w *WorkspaceResolutionIndex) LookupClassScopeByDefId(defID string) (*shared.Scope, bool) {
	s, ok := w.ClassScopeByDefId[defID]
	return s, ok
}

// LookupClassDefIdByScopeId returns the class def nodeId for a class scope ID.
func (w *WorkspaceResolutionIndex) LookupClassDefIdByScopeId(scopeID shared.ScopeID) (string, bool) {
	id, ok := w.ClassScopeIdToDefId[scopeID]
	return id, ok
}

// LookupModuleScopeByFile returns the module scope for a file path.
func (w *WorkspaceResolutionIndex) LookupModuleScopeByFile(filePath string) (*shared.Scope, bool) {
	s, ok := w.ModuleScopeByFile[filePath]
	return s, ok
}

// LookupExportedCallable returns the first module-local callable def for a name.
func (w *WorkspaceResolutionIndex) LookupExportedCallable(name string) (*shared.SymbolDefinition, bool) {
	d, ok := w.ExportedCallableByName[name]
	return d, ok
}

// ---------------------------------------------------------------------------
// Backward compatibility: BuildWorkspaceIndex redirects to BuildWorkspaceResolutionIndex.
// ---------------------------------------------------------------------------

// WorkspaceIndexResult is the old name kept for backward compatibility.
// Deprecated: use WorkspaceResolutionIndex instead.
type WorkspaceIndexResult = WorkspaceResolutionIndex

// BuildWorkspaceIndex builds the workspace index from ScopeResolutionIndexes.
// Deprecated: use BuildWorkspaceResolutionIndex(parsedFiles) instead.
func BuildWorkspaceIndex(
	indexes interface{},
	lookup *GraphNodeLookup,
) *WorkspaceIndexResult {
	// This is a compatibility shim. The real build requires ParsedFile[].
	// Callers should migrate to BuildWorkspaceResolutionIndex.
	return &WorkspaceIndexResult{
		ClassScopeByDefId:      make(map[string]*shared.Scope),
		ClassScopeIdToDefId:    make(map[shared.ScopeID]string),
		ModuleScopeByFile:      make(map[string]*shared.Scope),
		ExportedCallableByName: make(map[string]*shared.SymbolDefinition),
	}
}