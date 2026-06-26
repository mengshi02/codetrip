package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// SemanticModel -- read-only interface
// ---------------------------------------------------------------------------

// SemanticModel -- aggregated read-only view: three owner-scoped registries + SymbolTable.
// Resolver only holds this interface; physically cannot modify indices.
type SemanticModel interface {
	Types() TypeRegistry
	Methods() MethodRegistry
	Fields() FieldRegistry
	Symbols() SymbolTableReader
	// Scopes -- immutable index bundle from finalize.
	// nil means not yet attached (before finalize-orchestrator call).
	Scopes() *ScopeResolutionIndexes
}

// ---------------------------------------------------------------------------
// MutableSemanticModel -- read-write interface
// ---------------------------------------------------------------------------

// MutableSemanticModel -- extends SemanticModel with mutable registries,
// Writer-typed symbols, full cascade clear, one-shot scope attach.
type MutableSemanticModel interface {
	SemanticModel
	// Mutable registries
	TypesMut() MutableTypeRegistry
	MethodsMut() MutableMethodRegistry
	FieldsMut() MutableFieldRegistry
	SymbolsMut() SymbolTableWriter
	// Clear -- cascade clear all registries + SymbolTable + attached scopes.
	Clear()
	// AttachScopeIndexes -- one-time write of finalize product.
	// Second call panics (indexes should materialize once per ingestion run).
	// Clear() resets the attached bundle, allowing re-ingestion.
	AttachScopeIndexes(indexes *ScopeResolutionIndexes)
}

// ---------------------------------------------------------------------------
// Factory: CreateSemanticModel
// ---------------------------------------------------------------------------

// CreateSemanticModel -- create top-level orchestrator.
// Composes pure SymbolTable + three owner-scoped registries + dispatch table,
// exposing wrapped add() that fans out to all layers.
func CreateSemanticModel() MutableSemanticModel {
	// 1. Create pure SymbolTable leaf node
	rawSymbols := CreateSymbolTable()

	// 2. Create three owner-scoped registries
	types := CreateTypeRegistry()
	methods := CreateMethodRegistry()
	fields := CreateFieldRegistry()

	// 3. Build dispatch table; closures capture current instance's registries
	dispatchTable := CreateRegistrationTable(RegistrationTableDeps{
		Types:   types,
		Methods: methods,
		Fields:  fields,
	})

	// 4. wrappedAdd -- fan out add() to SymbolTable + dispatch hooks
	wrappedAdd := func(
		filePath string,
		name string,
		nodeID string,
		typ shared.NodeLabel,
		meta *AddMetadata,
	) *shared.SymbolDefinition {
		// Step 1: write to SymbolTable (fileIndex + callableByName)
		def := rawSymbols.Add(filePath, name, nodeID, typ, meta)

		// Step 2: Function-with-ownerId normalized to Method
		// Python class body def / Rust trait method / Kotlin companion method
		dispatchKey := typ
		if typ == shared.LabelFunction && meta != nil && meta.OwnerID != nil {
			dispatchKey = shared.LabelMethod
		}

		// Step 3: dispatch table hook writes to owner-scoped registry
		hook, ok := dispatchTable[dispatchKey]
		if ok {
			hook(name, def)
		}

		return def
	}

	return &semanticModelImpl{
		rawSymbols:     rawSymbols,
		types:          types,
		methods:        methods,
		fields:         fields,
		dispatchTable:  dispatchTable,
		wrappedAdd:     wrappedAdd,
		attachedScopes: nil,
	}
}

type semanticModelImpl struct {
	rawSymbols     internalSymbolTable
	types          MutableTypeRegistry
	methods        MutableMethodRegistry
	fields         MutableFieldRegistry
	dispatchTable  map[shared.NodeLabel]RegistrationHook
	wrappedAdd     func(string, string, string, shared.NodeLabel, *AddMetadata) *shared.SymbolDefinition
	attachedScopes *ScopeResolutionIndexes
}

// --- SemanticModel interface implementation (read-only views) ---

func (sm *semanticModelImpl) Types() TypeRegistry     { return sm.types }
func (sm *semanticModelImpl) Methods() MethodRegistry { return sm.methods }
func (sm *semanticModelImpl) Fields() FieldRegistry   { return sm.fields }
func (sm *semanticModelImpl) Scopes() *ScopeResolutionIndexes {
	return sm.attachedScopes
}

// Symbols -- return read-only Reader view.
// Callers holding SymbolTableReader cannot add/clear.
func (sm *semanticModelImpl) Symbols() SymbolTableReader {
	return sm.readerFacade()
}

// --- MutableSemanticModel interface implementation ---

func (sm *semanticModelImpl) TypesMut() MutableTypeRegistry   { return sm.types }
func (sm *semanticModelImpl) MethodsMut() MutableMethodRegistry { return sm.methods }
func (sm *semanticModelImpl) FieldsMut() MutableFieldRegistry   { return sm.fields }

// SymbolsMut -- return Writer view (with wrappedAdd, without clear).
func (sm *semanticModelImpl) SymbolsMut() SymbolTableWriter {
	return &symbolTableWriterFacade{
		add:               sm.wrappedAdd,
		SymbolTableReader: sm.readerFacade(),
	}
}

// Clear -- cascade clear: all registries + SymbolTable + attached scopes.
// Single source of truth: no phantom-resolution failure mode where
// file/callable indices are cleared but owner-scoped indices still have residual data.
func (sm *semanticModelImpl) Clear() {
	sm.types.Clear()
	sm.methods.Clear()
	sm.fields.Clear()
	sm.rawSymbols.Clear()
	sm.attachedScopes = nil
}

// AttachScopeIndexes -- one-time write. Second call panics.
func (sm *semanticModelImpl) AttachScopeIndexes(indexes *ScopeResolutionIndexes) {
	if sm.attachedScopes != nil {
		panic("SemanticModel: scope indexes already attached. Call Clear() before re-attaching.")
	}
	sm.attachedScopes = indexes
}

// ---------------------------------------------------------------------------
// Internal facade types: Reader / Writer view separation
// ---------------------------------------------------------------------------

// symbolTableReaderFacade -- read-only view proxy.
// Does not expose Add/Clear, only query methods.
type symbolTableReaderFacade struct {
	impl *symbolTableImpl
}

func (f *symbolTableReaderFacade) LookupExact(filePath string, name string) *string {
	return f.impl.LookupExact(filePath, name)
}
func (f *symbolTableReaderFacade) LookupExactFull(filePath string, name string) *shared.SymbolDefinition {
	return f.impl.LookupExactFull(filePath, name)
}
func (f *symbolTableReaderFacade) LookupExactAll(filePath string, name string) []*shared.SymbolDefinition {
	return f.impl.LookupExactAll(filePath, name)
}
func (f *symbolTableReaderFacade) LookupCallableByName(name string) []*shared.SymbolDefinition {
	return f.impl.LookupCallableByName(name)
}
func (f *symbolTableReaderFacade) GetFiles() []string {
	return f.impl.GetFiles()
}
func (f *symbolTableReaderFacade) GetStats() SymbolTableStats {
	return f.impl.GetStats()
}

// symbolTableWriterFacade -- read-write view proxy.
// Exposes Add (wrappedAdd), but not Clear.
type symbolTableWriterFacade struct {
	SymbolTableReader // embed read-only interface
	add func(string, string, string, shared.NodeLabel, *AddMetadata) *shared.SymbolDefinition
}

func (f *symbolTableWriterFacade) Add(
	filePath string,
	name string,
	nodeID string,
	typ shared.NodeLabel,
	meta *AddMetadata,
) *shared.SymbolDefinition {
	return f.add(filePath, name, nodeID, typ, meta)
}

// readerFacade -- create read-only facade.
func (sm *semanticModelImpl) readerFacade() SymbolTableReader {
	return &symbolTableReaderFacade{impl: sm.rawSymbols.(*symbolTableImpl)}
}
