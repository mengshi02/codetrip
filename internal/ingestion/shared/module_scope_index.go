// Package shared — ModuleScopeIndex for file→module-scope mapping.
// Ported from gitnexus-shared scope-resolution/module-scope-index.ts (73 lines).
package shared

// ModuleScopeEntry is the per-file entry in the ModuleScopeIndex.
type ModuleScopeEntry struct {
	// ModuleScopeID is the ScopeID of the file's root module scope.
	ModuleScopeID ScopeID
	// FilePath is the absolute file path.
	FilePath string
	// Language is the source file's language.
	Language SupportedLanguage
}

// ModuleScopeIndex maps FilePath → ModuleScopeEntry for O(1) lookup
// of a file's root module scope. Used during import resolution to find
// the target module scope when linking ImportEdges.
type ModuleScopeIndex struct {
	entries map[string]ModuleScopeEntry // key = FilePath
}

// NewModuleScopeIndex creates an empty ModuleScopeIndex.
func NewModuleScopeIndex() *ModuleScopeIndex {
	return &ModuleScopeIndex{entries: make(map[string]ModuleScopeEntry)}
}

// Get returns the ModuleScopeEntry for the given file path, or false if not found.
func (m *ModuleScopeIndex) Get(filePath string) (ModuleScopeEntry, bool) {
	e, ok := m.entries[filePath]
	return e, ok
}

// Entries returns the full entries map.
func (m *ModuleScopeIndex) Entries() map[string]ModuleScopeEntry {
	return m.entries
}

// BuildModuleScopeIndex constructs a ModuleScopeIndex from ParsedFiles.
// Each ParsedFile's ModuleScope becomes an entry.
func BuildModuleScopeIndex(files []ParsedFile) *ModuleScopeIndex {
	idx := NewModuleScopeIndex()
	for _, f := range files {
		if f.ModuleScope != nil {
			idx.entries[f.FilePath] = ModuleScopeEntry{
				ModuleScopeID: f.ModuleScope.ID,
				FilePath:      f.FilePath,
				// Language is set separately by the finalize orchestrator
				// since ParsedFile doesn't carry language info directly.
			}
		}
	}
	return idx
}

// Set adds or replaces an entry. Used for post-build augmentation.
func (m *ModuleScopeIndex) Set(filePath string, entry ModuleScopeEntry) {
	m.entries[filePath] = entry
}