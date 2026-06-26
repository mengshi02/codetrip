// Package shared — PositionIndex for O(log N) scope lookup by source position.
// Ported from gitnexus-shared scope-resolution/position-index.ts (167 lines).
package shared

import "sort"

// PositionIndex supports O(log N) lookup of the innermost scope containing
// a given source position. Built from a ScopeTree, sorted by range start,
// with binary search + reverse scan for innermost match.
type PositionIndex struct {
	// entries sorted by (startLine, startCol) for binary search.
	// Each file has its own index; fileMap partitions by filePath.
	fileMap map[string][]positionEntry
}

type positionEntry struct {
	scope     *Scope
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

// NewPositionIndex creates an empty PositionIndex.
func NewPositionIndex() *PositionIndex {
	return &PositionIndex{
		fileMap: make(map[string][]positionEntry),
	}
}

// BuildPositionIndex constructs a PositionIndex from a ScopeTree.
// For each scope in the tree, creates a sorted entry for binary search.
func BuildPositionIndex(tree *ScopeTree) *PositionIndex {
	idx := &PositionIndex{
		fileMap: make(map[string][]positionEntry),
	}

	for _, s := range tree.ByID() {
		entry := positionEntry{
			scope:     s,
			startLine: s.Range.StartLine,
			startCol:  s.Range.StartCol,
			endLine:   s.Range.EndLine,
			endCol:    s.Range.EndCol,
		}
		idx.fileMap[s.FilePath] = append(idx.fileMap[s.FilePath], entry)
	}

	// Sort each file's entries by start position
	for fp := range idx.fileMap {
		sort.Slice(idx.fileMap[fp], func(i, j int) bool {
			a, b := idx.fileMap[fp][i], idx.fileMap[fp][j]
			if a.startLine != b.startLine {
				return a.startLine < b.startLine
			}
			return a.startCol < b.startCol
		})
	}

	return idx
}

// FindInnermostScope returns the innermost scope containing the given position
// in the specified file. Returns nil if no scope contains the position.
//
// Algorithm:
//  1. Binary search to find the last entry whose start <= (line, col)
//  2. Reverse scan from that entry to find the innermost containing scope
func (pi *PositionIndex) FindInnermostScope(filePath string, line, col int) *Scope {
	entries, ok := pi.fileMap[filePath]
	if !ok {
		return nil
	}

	// Binary search: find the rightmost entry whose start <= (line, col)
	lo, hi := 0, len(entries)
	for lo < hi {
		mid := lo + (hi-lo)/2
		e := entries[mid]
		if e.startLine < line || (e.startLine == line && e.startCol <= col) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	// lo is now the first entry past the position; scan backwards
	// to find the innermost containing scope
	for i := lo - 1; i >= 0; i-- {
		e := entries[i]
		if positionInRange(line, col, e.startLine, e.startCol, e.endLine, e.endCol) {
			return e.scope
		}
		// If we've gone past entries that could contain the position, stop
		if e.endLine < line {
			break
		}
	}

	return nil
}

// FindAllScopesAt returns all scopes containing the given position,
// from outermost to innermost.
func (pi *PositionIndex) FindAllScopesAt(filePath string, line, col int) []*Scope {
	entries, ok := pi.fileMap[filePath]
	if !ok {
		return nil
	}

	var result []*Scope
	for _, e := range entries {
		if positionInRange(line, col, e.startLine, e.startCol, e.endLine, e.endCol) {
			result = append(result, e.scope)
		}
	}
	return result
}

// positionInRange returns true if (line, col) is within the range
// [startLine:startCol, endLine:endCol], inclusive.
func positionInRange(line, col, startLine, startCol, endLine, endCol int) bool {
	if line < startLine || line > endLine {
		return false
	}
	if line == startLine && col < startCol {
		return false
	}
	if line == endLine && col > endCol {
		return false
	}
	return true
}