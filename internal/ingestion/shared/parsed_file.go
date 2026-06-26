// Package shared — ParsedFile type definition.
// Ported from gitnexus-shared scope-resolution/parsed-file.ts.
package shared

// ParsedFile is the per-file extraction product: the module scope, child scopes,
// parsed imports, local definitions, reference sites, and capture/CFG side channels.
// This is the primary data structure flowing from the parse phase to the finalize phase.
type ParsedFile struct {
	// FilePath is the absolute path of the source file.
	FilePath string
	// ModuleScope is the file's root scope (always ScopeKindModule).
	ModuleScope *Scope
	// Scopes is the full list of scopes extracted from this file (including ModuleScope).
	Scopes []*Scope
	// ParsedImports is the raw imports emitted by the provider's interpretImport hook.
	ParsedImports []ParsedImport
	// LocalDefs is the definitions owned by this file's scopes.
	LocalDefs []SymbolDefinition
	// ReferenceSites is the pre-resolved use-site references.
	ReferenceSites []ReferenceSite
	// CaptureSideChannel carries language-specific side-channel data (e.g. C/C++
	// static-linkage snapshots). Typed as interface{} because each language stores
	// its own struct (e.g. c.CCaptureSideChannel). Mirrors TS ParsedFile.captureSideChannel.
	CaptureSideChannel interface{}
	// CfgSideChannel carries CFG-related data for taint/reachability analysis.
	CfgSideChannel interface{} // typed concretely in CFG engine
}