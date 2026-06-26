// BindingAccumulator — read-only accumulator that collects typeEnv
// bindings across file in the ingestion pipeline.
//
// Mirrors TS binding-accumulator.ts (~300 lines), simplified for codetrip:
//   - Single implementation (no worker/sequential split, codetrip is sequential)
//   - dispose() not needed (no multi-threaded lifecycle concerns)
//   - enrichExportedTypeMap simplified (uses shared.KnowledgeGraph interface)
//
// Lifecycle contract: append-only finalize. After finalize(), appendFile() throws.
// After dispose() (codetrip doesn't call this), read methods return empty/undefined.

package core

import (
	"fmt"
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ─── Types ──────────────────────────────────────────────────

// BindingEntry represents a single binding captured during parsing.
type BindingEntry struct {
	Scope   string // '' for file-level, 'funcName@startIndex' for function-local
	VarName string
	TypeName string
}

// ─── BindingAccumulator ──────────────────────────────────────

// BindingAccumulator is a read-only accumulator that collects typeEnv
// bindings across file in the ingestion pipeline.
//
// Lifecycle: append-only until finalize(), then frozen.
// After finalize(), calling appendFile() will panic.
type BindingAccumulator struct {
	mu       sync.RWMutex
	entries  map[string][]BindingEntry // filePath → entries
	frozen   bool
	fileCnt  int
}

// NewBindingAccumulator creates a new, empty BindingAccumulator.
func NewBindingAccumulator() *BindingAccumulator {
	return &BindingAccumulator{
		entries: make(map[string][]BindingEntry),
	}
}

// ─── Mutation (pre-finalize only) ────────────────────────────

// AppendFile adds bindings for a single file.
// Panics if called after finalize().
func (ba *BindingAccumulator) AppendFile(filePath string, entries []BindingEntry) {
	ba.mu.Lock()
	if ba.frozen {
		ba.mu.Unlock()
		panic("BindingAccumulator: AppendFile called after finalize")
	}
	ba.entries[filePath] = entries
	ba.fileCnt++
	ba.mu.Unlock()
}

// ─── Finalize ────────────────────────────────────────────────

// Finalize freezes the accumulator — no more appendFile calls allowed.
// After this, read methods are safe to call concurrently (if needed).
func (ba *BindingAccumulator) Finalize() {
	ba.mu.Lock()
	ba.frozen = true
	ba.mu.Unlock()
}

// ─── Read methods (always safe) ────────────────────────────────

// FileCount returns the total number of files with bindings.
func (ba *BindingAccumulator) FileCount() int {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.fileCnt
}

// Files returns all file paths that have bindings.
func (ba *BindingAccumulator) Files() []string {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	result := make([]string, 0, len(ba.entries))
	for k := range ba.entries {
		result = append(result, k)
	}
	return result
}

// FileScopeEntries returns all file-scope (scope='') bindings for a given file.
func (ba *BindingAccumulator) FileScopeEntries(filePath string) []BindingEntry {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	entries := ba.entries[filePath]
	var result []BindingEntry
	for _, e := range entries {
		if e.Scope == "" {
			result = append(result, e)
		}
	}
	return result
}

// AllEntries returns all entries for a given file (including function-local).
func (ba *BindingAccumulator) AllEntries(filePath string) []BindingEntry {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.entries[filePath]
}

// ─── Enrichment ──────────────────────────────────────────────

// EnrichExportedTypeMap merges file-scope bindings into an exportedTypeMap
// for symbols whose graph nodes are marked as exported.
//
// Node ID candidates: Function:{filePath}:{name}, Variable:{filePath}:{name},
// Const:{filePath}:{name}. First match wins.
//
// Tier 0 priority: if exportedTypeMap already has an entry for a (filePath, name)
// pair, the accumulator entry does NOT overwrite — SymbolTable tier-0 is
// authoritative.
//
// Returns the number of new entries written into exportedTypeMap.
func EnrichExportedTypeMap(
	ba *BindingAccumulator,
	graph shared.KnowledgeGraph,
	exportedTypeMap map[string]map[string]string,
) int {
	if ba.FileCount() == 0 {
		return 0
	}
	enriched := 0
	for _, filePath := range ba.Files() {
		for _, entry := range ba.FileScopeEntries(filePath) {
			// Three-candidate-ID lookup
			functionID := fmt.Sprintf("Function:%s:%s", filePath, entry.VarName)
			variableID := fmt.Sprintf("Variable:%s:%s", filePath, entry.VarName)
			constID := fmt.Sprintf("Const:%s:%s", filePath, entry.VarName)

			var isExported bool
			node := graph.GetNode(functionID)
			if node != nil {
				isExported = node.Props.IsExported
			} else {
				node = graph.GetNode(variableID)
				if node != nil {
					isExported = node.Props.IsExported
				} else {
					node = graph.GetNode(constID)
					if node != nil {
						isExported = node.Props.IsExported
					}
				}
			}

			if isExported {
				// Tier 0 guard: don't overwrite existing entries
				if existing, ok := exportedTypeMap[filePath]; ok {
					if _, exists := existing[entry.VarName]; exists {
						continue // SymbolTable tier-0 wins
					}
				}
				if exportedTypeMap[filePath] == nil {
					exportedTypeMap[filePath] = make(map[string]string)
				}
				exportedTypeMap[filePath][entry.VarName] = entry.TypeName
				enriched++
			}
		}
	}
	return enriched
}