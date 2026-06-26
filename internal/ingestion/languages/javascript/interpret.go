// Package javascript — Capture-match → semantic-shape interpreters for JavaScript.
// interpretJsImport delegates to interpretTsImport for all cases because
// emitJsScopeCaptures synthesizes the same @import.kind/name/alias/source
// markers for both ESM and CJS imports.
// interpretJsTypeBinding handles the JS-only @type-binding.class-field tag
// before delegating to interpretTsTypeBinding.
//
// Ported from TS languages/javascript/interpret.ts.
package javascript

import (
	typescript "github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterpretJsImport delegates to InterpretTsImport for all cases.
// CJS import kinds (named, named-alias, namespace, side-effect) match
// the kinds InterpretTsImport already handles for ESM.
func InterpretJsImport(captures shared.CaptureMatch) *shared.ParsedImport {
	return typescript.InterpretTsImport(captures)
}

// InterpretJsTypeBinding handles the JS-only @type-binding.class-field tag
// before delegating to InterpretTsTypeBinding. The class-field tag is emitted
// by synthesizeConstructorFieldBindings and should produce source = 'annotation'.
// Remapping it to @type-binding.annotation achieves this without adding a
// JS-specific branch to the shared TS interpreter (DoD.md §2.2).
func InterpretJsTypeBinding(captures shared.CaptureMatch) *shared.ParsedTypeBinding {
	if _, ok := captures["@type-binding.class-field"]; ok {
		// Remap class-field → annotation so InterpretTsTypeBinding assigns
		// source = 'annotation'.
		remapped := make(shared.CaptureMatch)
		for k, v := range captures {
			if k == "@type-binding.class-field" {
				remapped["@type-binding.annotation"] = v
			} else {
				remapped[k] = v
			}
		}
		return typescript.InterpretTsTypeBinding(remapped)
	}
	return typescript.InterpretTsTypeBinding(captures)
}