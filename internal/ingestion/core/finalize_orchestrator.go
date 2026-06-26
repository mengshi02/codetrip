// Finalize Orchestrator — orchestrates the finalize algorithm across all files.

package core

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ─── Types ──────────────────────────────────────────────────

// FinalizeOrchestratorResult holds the result of the finalize orchestration.
type FinalizeOrchestratorResult struct {
	Imports  map[shared.ScopeID][]shared.ImportEdge
	Bindings map[shared.ScopeID]map[string][]shared.BindingRef
	Sccs     []shared.FinalizedSccExt
	Stats    shared.FinalizeStatsExt
}

// ─── Main orchestration function ──────────────────────────────

// RunFinalize orchestrates the finalize algorithm across all files.
func RunFinalize(
	files []shared.FinalizeFile,
	workspaceIndex shared.WorkspaceIndex,
	adapter ImportTargetAdapter,
) (*FinalizeOrchestratorResult, error) {
	input := shared.FinalizeInput{
		Files:          files,
		WorkspaceIndex: workspaceIndex,
	}

	hooks := &finalizeHooksImpl{adapter: adapter}
	output := shared.Finalize(input, hooks)

	return &FinalizeOrchestratorResult{
		Imports:  output.Imports,
		Bindings: output.Bindings,
		Sccs:     output.Sccs,
		Stats:    output.Stats,
	}, nil
}

// ─── Hooks implementation ──────────────────────────────────

type finalizeHooksImpl struct {
	adapter ImportTargetAdapter
}

func (h *finalizeHooksImpl) ResolveImportTarget(targetRaw string, fromFile string, workspace shared.WorkspaceIndex) []string {
	filePath, _ := h.adapter.ResolveImportTarget(fromFile, targetRaw, nil) // TODO: pass workspace paths
	if filePath == "" {
		return nil
	}
	return []string{filePath}
}

func (h *finalizeHooksImpl) ExpandsWildcardTo(targetModuleScope shared.ScopeID, workspace shared.WorkspaceIndex) []string {
	// TODO(Phase 6): delegate to LanguageProvider.ExpandsWildcardTo
	return nil
}

func (h *finalizeHooksImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scope shared.ScopeID) []shared.BindingRef {
	// TODO(Phase 6): delegate to LanguageProvider.MergeBindings
	result := make([]shared.BindingRef, 0, len(existing)+len(incoming))
	result = append(result, existing...)
	result = append(result, incoming...)
	return result
}