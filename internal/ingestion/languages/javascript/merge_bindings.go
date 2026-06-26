// Package javascript — Binding-merge precedence for JavaScript.
// JavaScript has no TypeScript declaration-merging (no interface + class
// coexisting in the same scope, no namespace + class dual-space declarations).
// However, TypeScriptMergeBindings handles these by falling back to
// ['value'] for any NodeLabel not explicitly mapped to multiple spaces —
// which is what every JavaScript declaration produces. The result is pure
// LEGB precedence without any cross-space logic, which is exactly what
// JavaScript needs.
//
// Reuse rather than reimplementing to keep the single source of truth for
// the tier (local 0 / import-namespace-reexport 1 / wildcard 2) ordering.
//
// Ported from TS languages/javascript/merge-bindings.ts.
package javascript

import (
	typescript "github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JsMergeBindings delegates to TypeScriptMergeBindings (same LEGB tier ordering).
func JsMergeBindings(bindings []shared.BindingRef) []shared.BindingRef {
	return typescript.TypeScriptMergeBindings(bindings)
}