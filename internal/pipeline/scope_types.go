package pipeline

import "github.com/mengshi02/codetrip/internal/graph"

// ============ Scope Resolution Types ============
//
// These types were previously defined in the codetrip root package.
// They are moved here so the lang providers (internal/pipeline/lang) can
// depend only on the pipeline package, avoiding a circular import on codetrip.
// The codetrip package re-exports them as type aliases for backward compatibility.

// BindingSet represents a set of bindings (symbol name → candidate node ID list)
type BindingSet struct {
	// Name → []NodeID
	Bindings map[string][]string
	// FilePath is the file path where the binding belongs
	FilePath string
	// IsImported indicates whether this is an imported binding set
	IsImported bool
}

// NewBindingSet creates an empty binding set
func NewBindingSet() *BindingSet {
	return &BindingSet{Bindings: make(map[string][]string)}
}

// Add adds a binding
func (b *BindingSet) Add(name string, nodeID string) {
	b.Bindings[name] = append(b.Bindings[name], nodeID)
}

// Lookup looks up a binding
func (b *BindingSet) Lookup(name string) []string {
	return b.Bindings[name]
}

// Merge merges another binding set into the current one
func (b *BindingSet) Merge(other *BindingSet) {
	for name, ids := range other.Bindings {
		b.Bindings[name] = append(b.Bindings[name], ids...)
	}
}

// RangeBindContext provides context for for-range variable binding
type RangeBindContext struct {
	GraphStore *graph.GraphStore
	Repo       string
	Model      *MutableSemanticModel
}

// ScopeContextOptions provides options for scope context
type ScopeContextOptions struct {
	FilePath string
	Language graph.Label
}

// EmitContext provides context for post-processing edge emission
type EmitContext struct {
	GraphStore *graph.GraphStore
	Repo       string
	Model      *MutableSemanticModel
}

// GraphNodeLookup is a function type for looking up graph nodes
type GraphNodeLookup func(nodeID string) (*graph.Node, bool)