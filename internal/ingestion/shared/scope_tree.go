// Package shared — ScopeTree type, builder, and invariant checks.
// Ported from gitnexus-shared scope-resolution/scope-tree.ts (296 lines).
package shared

import (
	"fmt"
	"sort"
)

// ScopeTreeInvariantError is returned when a scope tree violates one of its invariants.
type ScopeTreeInvariantError struct {
	Violation string
	ScopeID   ScopeID
	Detail    string
}

func (e *ScopeTreeInvariantError) Error() string {
	return fmt.Sprintf("scope tree invariant violation (%s): scope=%s %s", e.Violation, e.ScopeID, e.Detail)
}

// ScopeTree is the hierarchical scope tree supporting parent/child traversal,
// scope lookup by ID, and position-based innermost-scope queries.
type ScopeTree struct {
	// byID maps ScopeID → *Scope for O(1) lookup.
	byID map[ScopeID]*Scope
	// children maps parent ScopeID → child scopes sorted by range start.
	children map[ScopeID][]*Scope
	// roots are the top-level module scopes (no parent).
	roots []*Scope
}

// NewScopeTree creates an empty ScopeTree.
func NewScopeTree() *ScopeTree {
	return &ScopeTree{
		byID:     make(map[ScopeID]*Scope),
		children: make(map[ScopeID][]*Scope),
	}
}

// GetScope returns the Scope for the given ScopeID, or nil if not found.
func (st *ScopeTree) GetScope(id ScopeID) *Scope {
	return st.byID[id]
}

// Roots returns the top-level module scopes.
func (st *ScopeTree) Roots() []*Scope {
	return st.roots
}

// Children returns the child scopes of the given parent ScopeID.
func (st *ScopeTree) Children(parent ScopeID) []*Scope {
	return st.children[parent]
}

// ByID returns the full byID map.
func (st *ScopeTree) ByID() map[ScopeID]*Scope {
	return st.byID
}

// AllScopes returns all scopes in the tree.
func (st *ScopeTree) AllScopes() []*Scope {
	result := make([]*Scope, 0, len(st.byID))
	for _, s := range st.byID {
		result = append(result, s)
	}
	return result
}

// ScopeCount returns the number of scopes in the tree.
func (st *ScopeTree) ScopeCount() int {
	return len(st.byID)
}

// BuildScopeTree constructs a ScopeTree from a flat list of scopes.
// Validates 5 invariants after construction:
//   1. Non-module scopes must have a parent.
//   2. Parent scope must exist in the tree.
//   3. Parent scope's range must contain child scope's range.
//   4. Sibling scopes must not have overlapping ranges.
//   5. Parent and child must share the same file path (Module→non-Module exempted).
func BuildScopeTree(scopes []*Scope) (*ScopeTree, error) {
	st := &ScopeTree{
		byID:     make(map[ScopeID]*Scope, len(scopes)),
		children: make(map[ScopeID][]*Scope),
	}

	// Phase 1: register all scopes by ID
	for _, s := range scopes {
		st.byID[s.ID] = s
	}

	// Phase 2: link children to parents; collect roots
	for _, s := range scopes {
		if s.Parent == nil {
			st.roots = append(st.roots, s)
		} else {
			st.children[*s.Parent] = append(st.children[*s.Parent], s)
		}
	}

	// Phase 3: sort children by range start for deterministic iteration
	for parentID := range st.children {
		sort.Slice(st.children[parentID], func(i, j int) bool {
			a, b := st.children[parentID][i], st.children[parentID][j]
			if a.Range.StartLine != b.Range.StartLine {
				return a.Range.StartLine < b.Range.StartLine
			}
			return a.Range.StartCol < b.Range.StartCol
		})
	}

	// Phase 4: validate invariants
	if err := st.validateInvariants(); err != nil {
		return nil, err
	}

	return st, nil
}

// validateInvariants checks the 5 scope tree invariants.
func (st *ScopeTree) validateInvariants() error {
	for _, s := range st.byID {
		// Invariant 1: non-module scopes must have a parent
		if s.Kind != ScopeKindModule && s.Parent == nil {
			return &ScopeTreeInvariantError{
				Violation: "non-module-requires-parent",
				ScopeID:   s.ID,
				Detail:    "non-module scope has no parent",
			}
		}

		// Invariant 2: parent scope must exist
		if s.Parent != nil {
			parent, ok := st.byID[*s.Parent]
			if !ok {
				return &ScopeTreeInvariantError{
					Violation: "parent-not-found",
					ScopeID:   s.ID,
					Detail:    fmt.Sprintf("parent scope %s not found", *s.Parent),
				}
			}

			// Invariant 3: parent must contain child range
			if !canParentScope(parent, s) {
				return &ScopeTreeInvariantError{
					Violation: "parent-must-contain-child",
					ScopeID:   s.ID,
					Detail:    fmt.Sprintf("parent %s range does not contain child range", *s.Parent),
				}
			}

			// Invariant 5: parent and child must share file path (Module→non-Module exempted)
			if parent.FilePath != s.FilePath && !(parent.Kind == ScopeKindModule) {
				return &ScopeTreeInvariantError{
					Violation: "parent-must-share-filepath",
					ScopeID:   s.ID,
					Detail:    fmt.Sprintf("parent file %s != child file %s", parent.FilePath, s.FilePath),
				}
			}
		}
	}

	// Invariant 4: sibling ranges must not overlap
	for parentID, siblings := range st.children {
		for i := 0; i < len(siblings)-1; i++ {
			a, b := siblings[i], siblings[i+1]
			if rangesOverlap(a.Range, b.Range) {
				return &ScopeTreeInvariantError{
					Violation: "sibling-ranges-overlap",
					ScopeID:   a.ID,
					Detail:    fmt.Sprintf("sibling %s overlaps with %s under parent %s", a.ID, b.ID, parentID),
				}
			}
		}
	}

	return nil
}

// canParentScope checks whether the parent scope's range can contain the child scope's range.
// Module scopes are allowed to "contain" non-module scopes even when the module range
// technically doesn't encompass the child range (e.g., module spans the file,
// but children are namespace imports that live outside the text range).
func canParentScope(parent, child *Scope) bool {
	// Module → non-Module: always allowed (module represents the file)
	if parent.Kind == ScopeKindModule {
		return true
	}
	return rangeContains(parent.Range, child.Range)
}

// rangeContains returns true if outer range fully contains inner range.
func rangeContains(outer, inner Range) bool {
	if outer.StartLine < inner.StartLine {
		return true
	}
	if outer.StartLine == inner.StartLine && outer.StartCol <= inner.StartCol {
		if outer.EndLine > inner.EndLine {
			return true
		}
		if outer.EndLine == inner.EndLine && outer.EndCol >= inner.EndCol {
			return true
		}
	}
	return false
}

// rangesOverlap returns true if two ranges overlap (exclusive of endpoints).
// Assumes ranges are sorted by start position.
func rangesOverlap(a, b Range) bool {
	// a starts before b; check if a ends after b starts
	if a.EndLine < b.StartLine {
		return false
	}
	if a.EndLine == b.StartLine && a.EndCol <= b.StartCol {
		return false
	}
	return true
}