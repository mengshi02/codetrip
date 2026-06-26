// Package golang — Go wildcard (dot) import expansion.
// Go's dot imports (import . "pkg") make all exported names of the
// imported package visible without qualification. These functions
// enumerate those names after the target module scope is linked.
// Ported from TS languages/go/expand-wildcards.ts.
package golang

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ExpandGoDotImports expands a wildcard (dot) import into the list of
// exported names visible from the target module scope.
// Returns nil when the target scope has no bindings or no matching scope.
//
// Mirrors TS expandGoDotImports(targetModuleScope, parsedFiles).
func ExpandGoDotImports(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	// Locate the target module scope in parsedFiles.
	var targetScope *shared.Scope
	for _, pf := range parsedFiles {
		for _, scope := range pf.Scopes {
			if scope.ID == targetModuleScope {
				targetScope = scope
				break
			}
		}
		if targetScope != nil {
			break
		}
	}
	if targetScope == nil {
		return nil
	}

	// Collect all exported names from the target scope's bindings.
	// In Go, exported means first letter is uppercase.
	var names []string
	seen := map[string]bool{}

	// Collect from Bindings (BindingRef entries).
	for name, refs := range targetScope.Bindings {
		if len(refs) == 0 {
			continue
		}
		if !isGoExported(name) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	// Also collect from TypeBindings.
	for name := range targetScope.TypeBindings {
		if !isGoExported(name) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	return names
}

// ExpandGoWildcardNames is the ScopeResolver.ExpandsWildcardTo callback.
// It delegates to ExpandGoDotImports for the wiring.
func ExpandGoWildcardNames(targetModuleScope shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	return ExpandGoDotImports(targetModuleScope, parsedFiles)
}