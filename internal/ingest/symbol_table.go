package ingest

// SymbolTable provides O(1) symbol lookup with two indexes:
//   - fileIndex: FilePath → (SymbolName → SymbolDefinition) — for same-file resolution (high confidence)
//   - globalIndex: SymbolName → []SymbolDefinition — for cross-file fuzzy resolution (low confidence)

import (
	"strings"
	"sync"
)

// SymbolDefinition holds metadata for a named symbol (function, class, etc.).
type SymbolDefinition struct {
	NodeID         string
	FilePath       string
	Type           string // "Function", "Class", "Method", etc.
	ParameterCount *int
	ReturnType     string
	OwnerID        string // Links Method/Constructor to owning Class/Struct/Trait
	StartByte      uint   // Byte offset where the symbol definition starts
	EndByte        uint   // Byte offset where the symbol definition ends
}

func (st *SymbolTable) SetReturnType(nodeID string, startByte uint, returnType string) {
	if returnType == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, defs := range st.globalIndex {
		for _, def := range defs {
			if def.NodeID == nodeID && def.StartByte == startByte {
				def.ReturnType = returnType
			}
		}
	}
}

func (st *SymbolTable) LookupNodeIDWithArity(nodeID string, argCount int) *SymbolDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()
	var matches []*SymbolDefinition
	for _, defs := range st.globalIndex {
		for _, def := range defs {
			if def.NodeID == nodeID && (argCount < 0 || def.ParameterCount == nil || *def.ParameterCount == argCount) {
				matches = append(matches, def)
			}
		}
	}
	if len(matches) != 1 {
		return nil
	}
	copy := *matches[0]
	return &copy
}

func (st *SymbolTable) LookupNodeID(nodeID string) *SymbolDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()
	for _, defs := range st.globalIndex {
		for _, def := range defs {
			if def.NodeID == nodeID {
				copy := *def
				return &copy
			}
		}
	}
	return nil
}

type TypeAliasDefinition struct {
	FilePath   string
	TargetName string
}

// Function-like symbol types for enclosing function lookup.
var functionSymbolTypes = map[string]bool{
	"Function":    true,
	"Method":      true,
	"Constructor": true,
}

// SymbolTable provides concurrent-safe symbol registration and lookup.
type SymbolTable struct {
	mu              sync.RWMutex
	fileIndex       map[string]map[string]*SymbolDefinition // FilePath → Name → Def
	globalIndex     map[string][]*SymbolDefinition          // Name → Defs
	rangeIndex      map[string][]*SymbolDefinition          // FilePath → function-like defs sorted by StartByte (for enclosing lookup)
	ownerRangeIndex map[string][]*SymbolDefinition          // FilePath → class-like defs sorted by StartByte
	typeAliases     map[string][]TypeAliasDefinition
}

// NewSymbolTable creates an empty SymbolTable.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		fileIndex:       make(map[string]map[string]*SymbolDefinition),
		globalIndex:     make(map[string][]*SymbolDefinition),
		rangeIndex:      make(map[string][]*SymbolDefinition),
		ownerRangeIndex: make(map[string][]*SymbolDefinition),
		typeAliases:     make(map[string][]TypeAliasDefinition),
	}
}

func (st *SymbolTable) AddTypeAlias(filePath, aliasName, targetName string) {
	if aliasName == "" || targetName == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.typeAliases[aliasName] = append(st.typeAliases[aliasName], TypeAliasDefinition{FilePath: filePath, TargetName: targetName})
}

func (st *SymbolTable) LookupTypeAliases(aliasName string) []TypeAliasDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return append([]TypeAliasDefinition(nil), st.typeAliases[aliasName]...)
}

// Add registers a new symbol definition in both the file and global indexes.
func (st *SymbolTable) Add(filePath, name, nodeID, symType string, parameterCount *int, ownerID string, startByte, endByte uint) {
	st.mu.Lock()
	defer st.mu.Unlock()

	def := &SymbolDefinition{
		NodeID:         nodeID,
		FilePath:       filePath,
		Type:           symType,
		ParameterCount: parameterCount,
		OwnerID:        ownerID,
		StartByte:      startByte,
		EndByte:        endByte,
	}

	// A. Add to file index
	fileMap, ok := st.fileIndex[filePath]
	if !ok {
		fileMap = make(map[string]*SymbolDefinition)
		st.fileIndex[filePath] = fileMap
	}
	fileMap[name] = def

	// B. Add to global index (same pointer — zero additional memory)
	st.globalIndex[name] = append(st.globalIndex[name], def)

	// C. Add to range index if function-like (for findEnclosingFunctionIDFromByte)
	if functionSymbolTypes[symType] {
		st.rangeIndex[filePath] = append(st.rangeIndex[filePath], def)
	}
	if symType == "Class" || symType == "Interface" || symType == "Struct" || symType == "Trait" || symType == "Impl" {
		st.ownerRangeIndex[filePath] = append(st.ownerRangeIndex[filePath], def)
	}
}

// FinalizeRangeIndex sorts the range index entries by StartByte for binary search.
// Must be called once after all Add() calls, before any FindEnclosingFunctionID calls.
func (st *SymbolTable) FinalizeRangeIndex() {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, defs := range st.rangeIndex {
		sortDefsByStartByte(defs)
	}
	for _, defs := range st.ownerRangeIndex {
		sortDefsByStartByte(defs)
	}
}

// sortDefsByStartByte sorts symbol definitions by StartByte using insertion sort
// (small N per file, insertion sort is cache-friendly and fast).
func sortDefsByStartByte(defs []*SymbolDefinition) {
	for i := 1; i < len(defs); i++ {
		j := i
		for j > 0 && defs[j].StartByte < defs[j-1].StartByte {
			defs[j], defs[j-1] = defs[j-1], defs[j]
			j--
		}
	}
}

// FindEnclosingFunctionID finds the innermost function-like symbol that contains
// the given byte offset in the specified file.
// Returns the NodeID of the enclosing function, or "" if the byte offset is at
// top-level (not inside any function).
func (st *SymbolTable) FindEnclosingFunctionID(filePath string, byteOffset uint) string {
	best := st.FindEnclosingFunction(filePath, byteOffset)
	if best == nil {
		return ""
	}
	// Use extractFunctionName's label, not the definition label.
	// For Constructor definitions, extractFunctionName returns label="Method".
	enclosingLabel := best.Type
	if enclosingLabel == "Constructor" {
		enclosingLabel = "Method"
	}
	idx := strings.Index(best.NodeID, ":")
	if idx >= 0 {
		return enclosingLabel + best.NodeID[idx:]
	}
	return best.NodeID
}

// FindEnclosingFunction returns the innermost function-like definition that
// contains byteOffset, including its semantic owner.
func (st *SymbolTable) FindEnclosingFunction(filePath string, byteOffset uint) *SymbolDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()

	defs, ok := st.rangeIndex[filePath]
	if !ok || len(defs) == 0 {
		return nil
	}

	// Find the innermost function containing byteOffset.
	// Since defs are sorted by StartByte, we scan and keep the last match
	// that contains the offset (last match = smallest/innermost enclosing range
	// because inner functions have larger StartByte and smaller range).
	var best *SymbolDefinition
	for _, def := range defs {
		if def.StartByte <= byteOffset && byteOffset <= def.EndByte {
			best = def
		} else if def.StartByte > byteOffset {
			// Past the offset, no more enclosing functions possible
			break
		}
	}
	if best != nil {
		copy := *best
		return &copy
	}
	return nil
}

// FindEnclosingOwnerID returns the innermost class-like definition containing
// byteOffset. Enhanced mode uses this for constructor calls in class headers.
func (st *SymbolTable) FindEnclosingOwnerID(filePath string, byteOffset uint) string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	var best *SymbolDefinition
	for _, def := range st.ownerRangeIndex[filePath] {
		if def.StartByte <= byteOffset && byteOffset <= def.EndByte {
			best = def
		} else if def.StartByte > byteOffset {
			break
		}
	}
	if best != nil {
		return best.NodeID
	}
	return ""
}

// LookupExact returns the NodeID for a symbol in a specific file (high confidence).
func (st *SymbolTable) LookupExact(filePath, name string) string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if fileMap, ok := st.fileIndex[filePath]; ok {
		if def, ok := fileMap[name]; ok {
			return def.NodeID
		}
	}
	return ""
}

// LookupExactFull returns the full SymbolDefinition for a symbol in a specific file.
func (st *SymbolTable) LookupExactFull(filePath, name string) *SymbolDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if fileMap, ok := st.fileIndex[filePath]; ok {
		return fileMap[name]
	}
	return nil
}

// LookupFuzzy returns all definitions for a symbol name across the project (low confidence).
func (st *SymbolTable) LookupFuzzy(name string) []*SymbolDefinition {
	st.mu.RLock()
	defer st.mu.RUnlock()
	docs := st.globalIndex[name]
	if docs == nil {
		return nil
	}
	// Return a copy to avoid data races
	result := make([]*SymbolDefinition, len(docs))
	copy(result, docs)
	return result
}

// Stats returns debugging information about the symbol table.
func (st *SymbolTable) Stats() (fileCount int, globalSymbolCount int) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.fileIndex), len(st.globalIndex)
}

// Clear removes all entries from the symbol table.
func (st *SymbolTable) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.fileIndex = make(map[string]map[string]*SymbolDefinition)
	st.globalIndex = make(map[string][]*SymbolDefinition)
	st.rangeIndex = make(map[string][]*SymbolDefinition)
	st.ownerRangeIndex = make(map[string][]*SymbolDefinition)
}
