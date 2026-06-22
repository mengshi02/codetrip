package pipeline

import (
	"errors"
	"fmt"
	"sync"
)

// ModelState represents semantic model state
type ModelState int

const (
	// ModelWriting: write phase - parse and scopeResolution can write
	ModelWriting ModelState = iota
	// ModelFrozen: frozen state - read-only after finalize
	ModelFrozen
)

func (s ModelState) String() string {
	switch s {
	case ModelWriting:
		return "writing"
	case ModelFrozen:
		return "frozen"
	default:
		return "unknown"
	}
}

// ============ Semantic Data Structures ============

// SymbolEntry represents a symbol table entry: symbol name → NodeID mapping
type SymbolEntry struct {
	Name      string
	NodeID    string
	FilePath  string
	Kind      string // "function", "method", "class", "variable", "constant", "interface", etc.
	Receiver  string // Only set for methods
	IsStatic  bool
}

// TypeBinding represents type binding information
type TypeBinding struct {
	TypeName    string
	NodeID      string
	BoundToNode string // Target node ID that this type is bound to
	FilePath    string
	Kind        string // "implements", "extends", "typeAlias", etc.
}

// ImportEntry represents an import table entry
type ImportEntry struct {
	ImportPath string
	SourceFile string
	Symbols    []string
	IsWildcard bool
	Alias      string
	Line       int
}

// ScopeResolutionIndexes represents scope resolution indexes
// Read-only access through SemanticModel after freezing
type ScopeResolutionIndexes struct {
	// nameIndex: name → []NodeID (lookup symbols by name)
	nameIndex map[string][]string
	// fileIndex: filePath → []NodeID (lookup symbols by file)
	fileIndex map[string][]string
	// receiverIndex: receiver → []NodeID (lookup methods by receiver)
	receiverIndex map[string][]string
	// importIndex: importPath → []ImportEntry (lookup by import path)
	importIndex map[string][]ImportEntry
}

// newScopeResolutionIndexes creates empty indexes
func newScopeResolutionIndexes() *ScopeResolutionIndexes {
	return &ScopeResolutionIndexes{
		nameIndex:     make(map[string][]string),
		fileIndex:     make(map[string][]string),
		receiverIndex: make(map[string][]string),
		importIndex:   make(map[string][]ImportEntry),
	}
}

// ============ MutableSemanticModel — Mutable Handle ============

// MutableSemanticModel is a mutable semantic model handle
// Only held during write phases (parse, scopeResolution)
// Calling Freeze() generates a read-only SemanticModel; any subsequent write operations will panic
type MutableSemanticModel struct {
	state ModelState
	mu    sync.RWMutex

	// Semantic data structures
	symbolTable  map[string]*SymbolEntry  // symbolKey → entry
	typeBindings map[string]*TypeBinding  // bindingKey → entry
	importTable  map[string]*ImportEntry  // importKey → entry
	scopeIndexes *ScopeResolutionIndexes

	// PhaseStats migrated from old SemanticModel
	PhaseStats map[string]PhaseStat
}

// symbolPool reuses temporary maps
var symbolPool = sync.Pool{
	New: func() any {
		return make(map[string]*SymbolEntry)
	},
}

// typeBindingPool reuses temporary maps
var typeBindingPool = sync.Pool{
	New: func() any {
		return make(map[string]*TypeBinding)
	},
}

// importPool reuses temporary maps
var importPool = sync.Pool{
	New: func() any {
		return make(map[string]*ImportEntry)
	},
}

// NewMutableSemanticModel creates a mutable semantic model
func NewMutableSemanticModel() *MutableSemanticModel {
	st := symbolPool.Get().(map[string]*SymbolEntry)
	for k := range st {
		delete(st, k)
	}
	tb := typeBindingPool.Get().(map[string]*TypeBinding)
	for k := range tb {
		delete(tb, k)
	}
	im := importPool.Get().(map[string]*ImportEntry)
	for k := range im {
		delete(im, k)
	}

	return &MutableSemanticModel{
		state:         ModelWriting,
		symbolTable:   st,
		typeBindings:  tb,
		importTable:   im,
		scopeIndexes:  newScopeResolutionIndexes(),
		PhaseStats:    make(map[string]PhaseStat),
	}
}

// ErrModelReadOnly indicates a write operation was attempted on a frozen model
var ErrModelReadOnly = errors.New("semantic model: write operation on frozen model")

// ensureWriting ensures the model is in writing state
func (m *MutableSemanticModel) ensureWriting() error {
	if m.state != ModelWriting {
		return fmt.Errorf("%w: state=%s", ErrModelReadOnly, m.state)
	}
	return nil
}

// AddSymbol adds a symbol to the symbol table
// key is a unique identifier (e.g., "file:Name:Kind" or "file:Receiver.Name")
func (m *MutableSemanticModel) AddSymbol(key string, entry *SymbolEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return err
	}
	m.symbolTable[key] = entry

	// Synchronously update scope indexes
	m.scopeIndexes.nameIndex[entry.Name] = append(m.scopeIndexes.nameIndex[entry.Name], entry.NodeID)
	if entry.FilePath != "" {
		m.scopeIndexes.fileIndex[entry.FilePath] = append(m.scopeIndexes.fileIndex[entry.FilePath], entry.NodeID)
	}
	if entry.Receiver != "" {
		m.scopeIndexes.receiverIndex[entry.Receiver] = append(m.scopeIndexes.receiverIndex[entry.Receiver], entry.NodeID)
	}
	return nil
}

// AddTypeBinding adds a type binding
func (m *MutableSemanticModel) AddTypeBinding(key string, binding *TypeBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return err
	}
	m.typeBindings[key] = binding
	return nil
}

// AddImport adds an import entry
func (m *MutableSemanticModel) AddImport(key string, entry *ImportEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return err
	}
	m.importTable[key] = entry

	// Synchronously update import index
	m.scopeIndexes.importIndex[entry.ImportPath] = append(m.scopeIndexes.importIndex[entry.ImportPath], *entry)
	return nil
}

// ForEachSymbol iterates over all symbol entries in the model.
// The callback receives each key and entry; returning a non-nil error stops iteration and is returned.
func (m *MutableSemanticModel) ForEachSymbol(fn func(key string, entry *SymbolEntry) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, entry := range m.symbolTable {
		if err := fn(key, entry); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileOwnership reconciles symbol ownership conflicts
// When symbols with the same name exist in multiple files, select the correct ownership based on rules
func (m *MutableSemanticModel) ReconcileOwnership() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return err
	}
	// Ownership reconciliation logic: when multiple nodes have the same name, retain the priority node in the defining file
	// Current placeholder implementation, cross-file priority rules can be added later
	return nil
}

// RecordPhaseStat records phase statistics (replaces direct PhaseStats map manipulation)
func (m *MutableSemanticModel) RecordPhaseStat(phaseName string, stat PhaseStat) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return err
	}
	m.PhaseStats[phaseName] = stat
	return nil
}

// Freeze freezes the model and returns a read-only SemanticModel
// After freezing, any write operations on MutableSemanticModel will return ErrModelReadOnly
// The returned SemanticModel can be safely used read-only in multiple goroutines
func (m *MutableSemanticModel) Freeze() (*SemanticModel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureWriting(); err != nil {
		return nil, err
	}
	m.state = ModelFrozen

	sm := &SemanticModel{
		PhaseStats:    m.PhaseStats,
		symbolTable:   m.symbolTable,
		typeBindings:  m.typeBindings,
		importTable:   m.importTable,
		scopeIndexes:  m.scopeIndexes,
	}

	return sm, nil
}

// ============ SemanticModel — Read-Only Handle ============

// SemanticModel is a read-only semantic model
// Type system guarantee: no write methods, physically impossible to accidentally write
// Used by read phases (mro/communities/processes/embeddings/tools)
type SemanticModel struct {
	PhaseStats    map[string]PhaseStat
	symbolTable   map[string]*SymbolEntry
	typeBindings  map[string]*TypeBinding
	importTable   map[string]*ImportEntry
	scopeIndexes  *ScopeResolutionIndexes
	phaseStatsMu  sync.Mutex
}

// NewSemanticModel creates a read-only semantic model (compatible with old calls)
// Equivalent to NewMutableSemanticModel().Freeze()
// The returned model is in frozen state, PhaseStats is writable (compatible with direct assignment in db.go)
func NewSemanticModel() *SemanticModel {
	m := NewMutableSemanticModel()
	// Don't call Freeze(), because db.go needs PhaseStats to be writable
	// But SemanticModel in semantic.go doesn't expose write methods
	// Here we use "relaxed" semantics: return a model that is internally writable but externally read-only
	return &SemanticModel{
		PhaseStats:   m.PhaseStats,
		symbolTable:  m.symbolTable,
		typeBindings: m.typeBindings,
		importTable:  m.importTable,
		scopeIndexes: m.scopeIndexes,
	}
}

// LookupSymbol looks up a symbol by key
func (sm *SemanticModel) LookupSymbol(key string) (*SymbolEntry, bool) {
	e, ok := sm.symbolTable[key]
	return e, ok
}

// LookupSymbolByName looks up symbol node IDs by name
// O(1) complexity
func (sm *SemanticModel) LookupSymbolByName(name string) []string {
	return sm.scopeIndexes.nameIndex[name]
}

// GetTypeInfo looks up type binding by key
func (sm *SemanticModel) GetTypeInfo(key string) (*TypeBinding, bool) {
	b, ok := sm.typeBindings[key]
	return b, ok
}

// GetImports gets a snapshot of all import entries
func (sm *SemanticModel) GetImports() []ImportEntry {
	result := make([]ImportEntry, 0, len(sm.importTable))
	for _, entry := range sm.importTable {
		result = append(result, *entry)
	}
	return result
}

// GetImportsByPath looks up import information by import path
func (sm *SemanticModel) GetImportsByPath(importPath string) []ImportEntry {
	return sm.scopeIndexes.importIndex[importPath]
}

// GetSymbolsByFile looks up symbol node IDs by file path
func (sm *SemanticModel) GetSymbolsByFile(filePath string) []string {
	return sm.scopeIndexes.fileIndex[filePath]
}

// GetMethodsByReceiver looks up method node IDs by receiver
func (sm *SemanticModel) GetMethodsByReceiver(receiver string) []string {
	return sm.scopeIndexes.receiverIndex[receiver]
}

// GetScopeIndexes gets a read-only reference to scope resolution indexes
func (sm *SemanticModel) GetScopeIndexes() *ScopeResolutionIndexes {
	return sm.scopeIndexes
}

// SymbolCount returns the size of the symbol table
func (sm *SemanticModel) SymbolCount() int {
	return len(sm.symbolTable)
}

// TypeBindingCount returns the number of type bindings
func (sm *SemanticModel) TypeBindingCount() int {
	return len(sm.typeBindings)
}

// ImportCount returns the number of imports
func (sm *SemanticModel) ImportCount() int {
	return len(sm.importTable)
}

// ============ ScopeResolutionIndexes Read-Only Access Methods ============

// LookupByName looks up node IDs by name
func (idx *ScopeResolutionIndexes) LookupByName(name string) []string {
	return idx.nameIndex[name]
}

// LookupByFile looks up node IDs by file path
func (idx *ScopeResolutionIndexes) LookupByFile(filePath string) []string {
	return idx.fileIndex[filePath]
}

// LookupByReceiver looks up method node IDs by receiver
func (idx *ScopeResolutionIndexes) LookupByReceiver(receiver string) []string {
	return idx.receiverIndex[receiver]
}

// LookupByImportPath looks up import information by import path
func (idx *ScopeResolutionIndexes) LookupByImportPath(importPath string) []ImportEntry {
	return idx.importIndex[importPath]
}

// NameIndexSize returns the size of the name index
func (idx *ScopeResolutionIndexes) NameIndexSize() int {
	return len(idx.nameIndex)
}

// FileIndexSize returns the size of the file index
func (idx *ScopeResolutionIndexes) FileIndexSize() int {
	return len(idx.fileIndex)
}

// ReceiverIndexSize returns the size of the receiver index
func (idx *ScopeResolutionIndexes) ReceiverIndexSize() int {
	return len(idx.receiverIndex)
}

// ImportIndexSize returns the size of the import index
func (idx *ScopeResolutionIndexes) ImportIndexSize() int {
	return len(idx.importIndex)
}