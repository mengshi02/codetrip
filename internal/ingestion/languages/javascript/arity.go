// Package javascript — Arity compatibility for JavaScript.
// Delegates to TypeScriptArityCompatibility unchanged — JavaScript supports
// the same arity constructs (rest parameters ...args, default parameters p = v)
// and the metadata shape (parameterCount, requiredParameterCount, parameterTypes)
// is synthesized by the same ComputeTsArityMetadata function (which understands
// both TS and JS parameter node types via extractTsJsParameters).
//
// Ported from TS languages/javascript/arity.ts.
package javascript

import (
	typescript "github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	scope_resolution "github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JsArityCompatibility delegates to TypeScriptArityCompatibility.
func JsArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	return typescript.TypeScriptArityCompatibility(def, callsite)
}