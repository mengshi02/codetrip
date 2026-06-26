// Package python — Python import target resolution.
// Adapter from (ParsedImport, WorkspaceIndex) → concrete file path.
// Delegates to resolvePythonImportInternal (PEP-328 relative resolution
// + standard suffix matching).
//
// Ported from TS languages/python/import-target.ts (resolvePythonImportTarget).
package python

// PythonResolveContext carries the context needed for Python import resolution.
// Mirrors TS PythonResolveContext interface.
type PythonResolveContext struct {
	FromFile      string         // absolute path of the importing file
	AllFilePaths  map[string]bool // workspace file set for suffix matching
}

// ResolvePythonImportTarget resolves a raw import target path to concrete
// file paths within the workspace.
//
// Returns nil for unresolvable/external modules.
// Returns a single entry for resolved imports.
// Returns multiple entries for ambiguous imports.
//
// Python specifics:
//   - PEP-328 relative imports (from .m import x)
//   - Dotted imports (from m.sub import x) → path-based resolution
//   - External library imports return nil (no local file match)
//   - Relative imports that don't resolve must NOT fall through to suffix matching
//
// Mirrors TS resolvePythonImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig).
// TODO: full implementation — requires resolvePythonImportInternal wiring.
func ResolvePythonImportTarget(targetRaw string, fromFile string, allFilePaths map[string]bool, resolutionConfig interface{}) []string {
	// TODO: implement when import resolver is wired.
	// Steps (from TS):
	//   1. If targetRaw starts with '.', resolve as PEP-328 relative import
	//      (must NOT fall through to suffix matching if unresolved)
	//   2. For dotted imports (e.g. django.apps), check if leading segment
	//      has a repo candidate; skip suffix matching for external packages
	//   3. Try exact path, then ancestor walk, then suffix match for
	//      in-repo modules
	//   4. Return nil for external/standard-library imports
	return nil
}