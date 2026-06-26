// TypeRegistry: Class/Struct/Interface/Enum/Record/Trait index.
// Ported from gitnexus model/type-registry.ts (151 lines).
package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// TypeRegistry — read-only interface
// ---------------------------------------------------------------------------

// TypeRegistry provides O(1) lookups for class-like definitions by
// name, qualified name, impl name, and owner-scoped nested type.
type TypeRegistry interface {
	// LookupClassByName returns all class-like definitions with the given simple name.
	LookupClassByName(name string) []*shared.SymbolDefinition

	// LookupClassByQualifiedName returns all class-like definitions with the given
	// canonical qualified name (e.g. "App.Models.User", "com.example.User").
	LookupClassByQualifiedName(qualifiedName string) []*shared.SymbolDefinition

	// LookupImplByName returns all Impl definitions with the given name.
	// Used by Tier 3 resolution for Rust impl blocks.
	LookupImplByName(name string) []*shared.SymbolDefinition

	// LookupAllByOwner returns nested-type defs registered under (ownerNodeId, simpleName).
	// Returns empty slice on miss (never nil) — callers can concatenate without nil checks.
	// Used by Step 2 Receiver/MRO resolution when the receiver's owner declares
	// nested classes/structs/enums/typedefs.
	LookupAllByOwner(ownerNodeId, simpleName string) []*shared.SymbolDefinition
}

// ---------------------------------------------------------------------------
// MutableTypeRegistry — read-write interface
// ---------------------------------------------------------------------------

// MutableTypeRegistry extends TypeRegistry with registration and clear.
type MutableTypeRegistry interface {
	TypeRegistry
	// RegisterClass adds a class-like definition indexed by both simple and qualified name.
	RegisterClass(name, qualifiedName string, def *shared.SymbolDefinition)
	// RegisterImpl adds a Rust Impl block indexed by name.
	RegisterImpl(name string, def *shared.SymbolDefinition)
	// RegisterByOwner adds a nested type under its owner.
	RegisterByOwner(ownerNodeId, simpleName string, def *shared.SymbolDefinition)
	// Clear removes all entries.
	Clear()
}

// ---------------------------------------------------------------------------
// Factory: CreateTypeRegistry
// ---------------------------------------------------------------------------

// CreateTypeRegistry creates an empty TypeRegistry.
func CreateTypeRegistry() MutableTypeRegistry {
	return &typeRegistryImpl{
		classByName:          make(map[string][]*shared.SymbolDefinition),
		classByQualifiedName: make(map[string][]*shared.SymbolDefinition),
		implByName:           make(map[string][]*shared.SymbolDefinition),
		nestedByOwner:        make(map[string][]*shared.SymbolDefinition),
	}
}

type typeRegistryImpl struct {
	classByName          map[string][]*shared.SymbolDefinition
	classByQualifiedName map[string][]*shared.SymbolDefinition
	implByName           map[string][]*shared.SymbolDefinition
	nestedByOwner        map[string][]*shared.SymbolDefinition
}

func (r *typeRegistryImpl) LookupClassByName(name string) []*shared.SymbolDefinition {
	if defs, ok := r.classByName[name]; ok {
		return defs
	}
	return []*shared.SymbolDefinition{}
}

func (r *typeRegistryImpl) LookupClassByQualifiedName(qualifiedName string) []*shared.SymbolDefinition {
	if defs, ok := r.classByQualifiedName[qualifiedName]; ok {
		return defs
	}
	return []*shared.SymbolDefinition{}
}

func (r *typeRegistryImpl) LookupImplByName(name string) []*shared.SymbolDefinition {
	if defs, ok := r.implByName[name]; ok {
		return defs
	}
	return []*shared.SymbolDefinition{}
}

func (r *typeRegistryImpl) LookupAllByOwner(ownerNodeId, simpleName string) []*shared.SymbolDefinition {
	key := ownerNodeId + "\x00" + simpleName
	if defs, ok := r.nestedByOwner[key]; ok {
		return defs
	}
	return []*shared.SymbolDefinition{}
}

func (r *typeRegistryImpl) RegisterClass(name, qualifiedName string, def *shared.SymbolDefinition) {
	r.classByName[name] = append(r.classByName[name], def)
	r.classByQualifiedName[qualifiedName] = append(r.classByQualifiedName[qualifiedName], def)
}

func (r *typeRegistryImpl) RegisterImpl(name string, def *shared.SymbolDefinition) {
	r.implByName[name] = append(r.implByName[name], def)
}

func (r *typeRegistryImpl) RegisterByOwner(ownerNodeId, simpleName string, def *shared.SymbolDefinition) {
	key := ownerNodeId + "\x00" + simpleName
	r.nestedByOwner[key] = append(r.nestedByOwner[key], def)
}

func (r *typeRegistryImpl) Clear() {
	for k := range r.classByName {
		delete(r.classByName, k)
	}
	for k := range r.classByQualifiedName {
		delete(r.classByQualifiedName, k)
	}
	for k := range r.implByName {
		delete(r.implByName, k)
	}
	for k := range r.nestedByOwner {
		delete(r.nestedByOwner, k)
	}
}