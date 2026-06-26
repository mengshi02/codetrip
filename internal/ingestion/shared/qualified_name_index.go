// Package shared — QualifiedNameIndex for qualified-name-based def lookup.
// Ported from gitnexus-shared scope-resolution/qualified-name-index.ts (93 lines).
package shared

// QualifiedNameIndex maps qualified names (e.g., "app.models.User") to
// the DefIDs that define them. Multiple defs can share a qualified name
// (partial classes in C#, overloads, re-exports from different modules).
type QualifiedNameIndex struct {
	// entries maps qualified name → list of DefIDs
	entries map[string][]DefID
}

// NewQualifiedNameIndex creates an empty QualifiedNameIndex.
func NewQualifiedNameIndex() *QualifiedNameIndex {
	return &QualifiedNameIndex{entries: make(map[string][]DefID)}
}

// Get returns the DefIDs for the given qualified name, or nil if not found.
func (q *QualifiedNameIndex) Get(qualifiedName string) []DefID {
	return q.entries[qualifiedName]
}

// Entries returns the full entries map.
func (q *QualifiedNameIndex) Entries() map[string][]DefID {
	return q.entries
}

// Has reports whether the given qualified name exists in the index.
func (q *QualifiedNameIndex) Has(qualifiedName string) bool {
	_, ok := q.entries[qualifiedName]
	return ok
}

// BuildQualifiedNameIndex constructs a QualifiedNameIndex from a slice of
// SymbolDefinitions. Only defs with a non-nil QualifiedName are indexed.
// Deduplication: same (qualifiedName, defID) pair is only added once.
func BuildQualifiedNameIndex(defs []SymbolDefinition) *QualifiedNameIndex {
	idx := NewQualifiedNameIndex()
	seen := make(map[string]bool) // pairKey = qualifiedName + "\x00" + defID

	for _, def := range defs {
		if def.QualifiedName == nil {
			continue
		}
		pairKey := *def.QualifiedName + "\x00" + def.NodeID
		if seen[pairKey] {
			continue
		}
		seen[pairKey] = true
		idx.entries[*def.QualifiedName] = append(idx.entries[*def.QualifiedName], def.NodeID)
	}

	return idx
}