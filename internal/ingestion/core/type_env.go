// Type Environment — lightweight type inference / environment for tracking symbol types.
//
// Mirrors TS type-env.ts, skeleton for codetrip.
// The type environment tracks what type each local symbol has within a
// single file's scope. It is built during tree-sitter parsing and consumed
// by scope resolution. Deferred to Phase 3.

package core

// TypeEntry represents a single binding of varName → typeName in a scope.
type TypeEntry struct {
	ScopeID  string
	VarName  string
	TypeName string
	Source   string // "inferred", "explicit", "parameter", "return"
}

// TypeEnv is a per-file type environment mapping scope → variable → type.
type TypeEnv struct {
	entries []TypeEntry
}

// NewTypeEnv creates an empty type environment.
func NewTypeEnv() *TypeEnv {
	return &TypeEnv{entries: []TypeEntry{}}
}

// Add records a type binding in the environment.
func (te *TypeEnv) Add(scopeID, varName, typeName, source string) {
	te.entries = append(te.entries, TypeEntry{
		ScopeID:  scopeID,
		VarName:  varName,
		TypeName: typeName,
		Source:   source,
	})
}

// Entries returns all type bindings in the environment.
func (te *TypeEnv) Entries() []TypeEntry {
	return te.entries
}

// Lookup finds all entries for a given scope+variable.
func (te *TypeEnv) Lookup(scopeID, varName string) []TypeEntry {
	var result []TypeEntry
	for _, e := range te.entries {
		if e.ScopeID == scopeID && e.VarName == varName {
			result = append(result, e)
		}
	}
	return result
}

// ScopeEntries returns all type entries for a given scope.
func (te *TypeEnv) ScopeEntries(scopeID string) []TypeEntry {
	var result []TypeEntry
	for _, e := range te.entries {
		if e.ScopeID == scopeID {
			result = append(result, e)
		}
	}
	return result
}

// Merge combines another TypeEnv into this one (append-only).
func (te *TypeEnv) Merge(other *TypeEnv) {
	te.entries = append(te.entries, other.entries...)
}

// LookupExact finds the TypeName for a variable in any scope.
// Returns "" if not found.
func (te *TypeEnv) LookupExact(typeName string) string {
	for _, e := range te.entries {
		if e.TypeName == typeName {
			return e.TypeName
		}
	}
	return ""
}