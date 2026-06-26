package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// Constants: class labels + callable labels + call target labels
// ---------------------------------------------------------------------------

// classTypesTuple -- NodeLabel subset that participates in class heritage / qualifiedName fallback.
var classTypesTuple = []shared.NodeLabel{
	shared.LabelClass,
	shared.LabelStruct,
	shared.LabelInterface,
	shared.LabelEnum,
	shared.LabelRecord,
	shared.LabelTrait,
}

// CLASS_TYPES -- Go equivalent of Set<NodeLabel>: map[shared.NodeLabel]bool.
// Determines qualifiedName fallback logic in SymbolTable.add().
var CLASS_TYPES = makeClassTypesSet()

func makeClassTypesSet() map[shared.NodeLabel]bool {
	m := make(map[shared.NodeLabel]bool, len(classTypesTuple))
	for _, l := range classTypesTuple {
		m[l] = true
	}
	return m
}

// freeCallableTuple -- callable labels without owner scope.
var freeCallableTuple = []shared.NodeLabel{
	shared.LabelFunction,
	shared.LabelMacro,
	shared.LabelDelegate,
}

// FREE_CALLABLE_TYPES -- callableByName index gate.
// Only Function/Macro/Delegate enter callableByName; Method/Constructor
// require ownerId (except orphan fallback).
var FREE_CALLABLE_TYPES = makeFreeCallableSet()

func makeFreeCallableSet() map[shared.NodeLabel]bool {
	m := make(map[shared.NodeLabel]bool, len(freeCallableTuple))
	for _, l := range freeCallableTuple {
		m[l] = true
	}
	return m
}

// CALL_TARGET_TYPES -- full set of call target labels (including owner-scoped).
// Used for kind filtering in Tier 3 resolve.
var CALL_TARGET_TYPES = makeCallTargetSet()

func makeCallTargetSet() map[shared.NodeLabel]bool {
	m := make(map[shared.NodeLabel]bool, len(freeCallableTuple)+2)
	for k := range FREE_CALLABLE_TYPES {
		m[k] = true
	}
	m[shared.LabelMethod] = true
	m[shared.LabelConstructor] = true
	return m
}

// ---------------------------------------------------------------------------
// AddMetadata -- optional metadata for SymbolTable.add()
// ---------------------------------------------------------------------------

// AddMetadata -- 1:1 counterpart of TS AddMetadata.
// Optional fields use pointer types (nil = undefined).
type AddMetadata struct {
	ParameterCount         *int
	RequiredParameterCount *int
	ParameterTypes         []string
	ParameterTypeClasses   []shared.ParameterTypeClass
	ReturnType             *string
	DeclaredType           *string
	TemplateArguments      []string
	OwnerID                *string
	QualifiedName          *string
	IsDeleted              bool
}

// ---------------------------------------------------------------------------
// SymbolTableReader -- read-only interface
// ---------------------------------------------------------------------------

// SymbolTableReader -- read-only view, no add() or clear().
type SymbolTableReader interface {
	// LookupExact -- look up a symbol name in the specified file, return nodeId.
	LookupExact(filePath string, name string) *string

	// LookupExactFull -- look up a symbol name in the specified file, return full definition.
	LookupExactFull(filePath string, name string) *shared.SymbolDefinition

	// LookupExactAll -- return all definitions with the same name in the specified file (including overloads).
	LookupExactAll(filePath string, name string) []*shared.SymbolDefinition

	// LookupCallableByName -- look up callable symbols by name (Function/Macro/Delegate).
	LookupCallableByName(name string) []*shared.SymbolDefinition

	// GetFiles -- return all indexed file paths.
	GetFiles() []string

	// GetStats -- debug statistics.
	GetStats() SymbolTableStats
}

type SymbolTableStats struct {
	FileCount int
}

// ---------------------------------------------------------------------------
// SymbolTableWriter -- read-write interface (no clear)
// ---------------------------------------------------------------------------

// SymbolTableWriter -- extends Reader with add().
type SymbolTableWriter interface {
	SymbolTableReader
	// Add -- register a symbol into fileIndex + (if callable) callableByName index.
	Add(filePath string, name string, nodeID string, typ shared.NodeLabel, meta *AddMetadata) *shared.SymbolDefinition
}

// ---------------------------------------------------------------------------
// internalSymbolTable -- internal type with clear()
// ---------------------------------------------------------------------------

// internalSymbolTable -- the full type returned by the factory.
// Only SemanticModel holds this reference (rawSymbols); externals see Writer/Reader.
type internalSymbolTable interface {
	SymbolTableWriter
	Clear()
}

// ---------------------------------------------------------------------------
// Factory: CreateSymbolTable
// ---------------------------------------------------------------------------

// CreateSymbolTable -- create a pure SymbolTable leaf instance.
func CreateSymbolTable() internalSymbolTable {
	fileIndex := make(map[string]map[string][]*shared.SymbolDefinition)
	callableByName := make(map[string][]*shared.SymbolDefinition)
	st := &symbolTableImpl{
		fileIndex:      fileIndex,
		callableByName: callableByName,
	}
	return st
}

type symbolTableImpl struct {
	fileIndex      map[string]map[string][]*shared.SymbolDefinition
	callableByName map[string][]*shared.SymbolDefinition
}

// Add -- register a symbol. Core logic:
//   A. Always write to fileIndex
//   B. If FREE_CALLABLE or orphaned owner-scoped -> write to callableByName
func (st *symbolTableImpl) Add(
	filePath string,
	name string,
	nodeID string,
	typ shared.NodeLabel,
	meta *AddMetadata,
) *shared.SymbolDefinition {
	// qualifiedName fallback: class-like uses name, others use metadata value
	var qualifiedName *string
	if CLASS_TYPES[typ] {
		if meta != nil && meta.QualifiedName != nil {
			qualifiedName = meta.QualifiedName
		} else {
			fallback := name
			qualifiedName = &fallback
		}
	} else if meta != nil && meta.QualifiedName != nil {
		qualifiedName = meta.QualifiedName
	}

	// Build SymbolDefinition -- only write non-zero fields
	def := &shared.SymbolDefinition{
		NodeID:   nodeID,
		FilePath: filePath,
		Type:     typ,
	}
	if qualifiedName != nil {
		def.QualifiedName = qualifiedName
	}
	if meta != nil {
		if meta.ParameterCount != nil {
			def.ParameterCount = meta.ParameterCount
		}
		if meta.RequiredParameterCount != nil {
			def.RequiredParameterCount = meta.RequiredParameterCount
		}
		if meta.ParameterTypes != nil {
			def.ParameterTypes = meta.ParameterTypes
		}
		if meta.ParameterTypeClasses != nil {
			def.ParameterTypeClasses = meta.ParameterTypeClasses
		}
		if meta.ReturnType != nil {
			def.ReturnType = meta.ReturnType
		}
		if meta.DeclaredType != nil {
			def.DeclaredType = meta.DeclaredType
		}
		if meta.TemplateArguments != nil {
			def.TemplateArguments = meta.TemplateArguments
		}
		if meta.OwnerID != nil {
			def.OwnerID = meta.OwnerID
		}
		if meta.IsDeleted {
			def.IsDeleted = true
		}
	}

	// A. File Index -- unconditional
	fileMap, ok := st.fileIndex[filePath]
	if !ok {
		fileMap = make(map[string][]*shared.SymbolDefinition)
		st.fileIndex[filePath] = fileMap
	}
	fileMap[name] = append(fileMap[name], def)

	// B. Callable Index -- gated by FREE_CALLABLE_TYPES + orphan fallback
	isOrphanedOwnerScoped := false
	if (typ == shared.LabelMethod || typ == shared.LabelConstructor) &&
		(meta == nil || meta.OwnerID == nil) {
		isOrphanedOwnerScoped = true
	}

	if FREE_CALLABLE_TYPES[typ] || isOrphanedOwnerScoped {
		st.callableByName[name] = append(st.callableByName[name], def)
	}

	return def
}

func (st *symbolTableImpl) LookupExact(filePath string, name string) *string {
	defs := st.lookupDefsInFile(filePath, name)
	if len(defs) == 0 {
		return nil
	}
	return &defs[0].NodeID
}

func (st *symbolTableImpl) LookupExactFull(filePath string, name string) *shared.SymbolDefinition {
	defs := st.lookupDefsInFile(filePath, name)
	if len(defs) == 0 {
		return nil
	}
	return defs[0]
}

func (st *symbolTableImpl) LookupExactAll(filePath string, name string) []*shared.SymbolDefinition {
	return st.lookupDefsInFile(filePath, name)
}

func (st *symbolTableImpl) LookupCallableByName(name string) []*shared.SymbolDefinition {
	defs, ok := st.callableByName[name]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	return defs
}

func (st *symbolTableImpl) GetFiles() []string {
	files := make([]string, 0, len(st.fileIndex))
	for f := range st.fileIndex {
		files = append(files, f)
	}
	return files
}

func (st *symbolTableImpl) GetStats() SymbolTableStats {
	return SymbolTableStats{FileCount: len(st.fileIndex)}
}

func (st *symbolTableImpl) Clear() {
	for k := range st.fileIndex {
		delete(st.fileIndex, k)
	}
	for k := range st.callableByName {
		delete(st.callableByName, k)
	}
}

// lookupDefsInFile -- internal helper: fetch definitions by file and name from fileIndex.
func (st *symbolTableImpl) lookupDefsInFile(filePath string, name string) []*shared.SymbolDefinition {
	fileMap, ok := st.fileIndex[filePath]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	defs, ok := fileMap[name]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	return defs
}
