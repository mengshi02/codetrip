// Package shared — DefIndex type and builder.
// Ported from gitnexus-shared scope-resolution/def-index.ts.
package shared

// DefIndex maps DefID to SymbolDefinition for O(1) lookup.
// First-write-wins on collision: the first definition registered for a
// given DefID is kept, subsequent duplicates are silently discarded.
type DefIndex struct {
	entries map[DefID]SymbolDefinition
}

// NewDefIndex creates an empty DefIndex.
func NewDefIndex() *DefIndex {
	return &DefIndex{entries: make(map[DefID]SymbolDefinition)}
}

// Get returns the SymbolDefinition for the given DefID, or nil if not found.
func (d *DefIndex) Get(id DefID) *SymbolDefinition {
	if def, ok := d.entries[id]; ok {
		return &def
	}
	return nil
}

// Entries returns all entries in the index.
func (d *DefIndex) Entries() map[DefID]SymbolDefinition {
	return d.entries
}

// Len returns the number of entries in the index.
func (d *DefIndex) Len() int {
	return len(d.entries)
}

// BuildDefIndex constructs a DefIndex from a slice of SymbolDefinitions.
// First-write-wins: if two defs share the same NodeID, the first one wins.
func BuildDefIndex(defs []SymbolDefinition) *DefIndex {
	idx := NewDefIndex()
	for _, def := range defs {
		if _, exists := idx.entries[def.NodeID]; !exists {
			idx.entries[def.NodeID] = def
		}
	}
	return idx
}

// BuildDefIndexFromMap constructs a DefIndex from a pre-built map.
// First-write-wins on collision within the map itself is the caller's responsibility.
func BuildDefIndexFromMap(entries map[DefID]SymbolDefinition) *DefIndex {
	return &DefIndex{entries: entries}
}