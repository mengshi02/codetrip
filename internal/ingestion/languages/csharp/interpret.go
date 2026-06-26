// Package csharp — C# import/type-binding interpretation hooks.
// These functions translate raw tree-sitter captures into ParsedImport
// and ParsedTypeBinding records that the scope-resolution pipeline consumes.
// Ported from TS languages/csharp/interpret.ts.
package csharp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterpretCsharpImport converts a C# using directive capture into a
// ParsedImport. Handles namespace imports ("using System.IO"),
// static imports ("using static System.Math"), and alias imports
// ("using X = System.Collections.Generic.List<int>").
//
// Mirrors TS interpretCsharpImport(captures).
// TODO: full implementation — currently returns zero-value ParsedImport.
func InterpretCsharpImport(captures shared.CaptureMatch) shared.ParsedImport {
	// TODO: extract @using-namespace, @using-static, @using-alias captures,
	// determine Kind (namespace/named/alias), fill fields.
	return shared.ParsedImport{}
}

// InterpretCsharpTypeBinding converts a C# type-binding capture into a
// ParsedTypeBinding. Handles variable-type declarations, return-type
// annotations, and self bindings (this/base receiver types).
//
// Mirrors TS interpretCsharpTypeBinding(captures).
// TODO: full implementation — currently returns zero-value ParsedTypeBinding.
func InterpretCsharpTypeBinding(captures shared.CaptureMatch) shared.ParsedTypeBinding {
	// TODO: extract @type-binding-name and @type-binding-type captures,
	// determine Source (annotation/self/return-annotation), fill BoundName and RawTypeName.
	return shared.ParsedTypeBinding{}
}

// NormalizeCsharpTypeName strips C# type syntax wrappers (nullable ?,
// generic type arguments <...>, array brackets []) from a raw type name
// to produce the simple type name used for binding lookups.
//
// Examples:
//   "List<int>" → "List"
//   "Dictionary<string, int>" → "Dictionary"
//   "int?" → "int"
//   "string[]" → "string"
//
// Mirrors TS normalizeCsharpTypeName(raw).
// TODO: full implementation — currently does basic trimming.
func NormalizeCsharpTypeName(raw string) string {
	s := strings.TrimSpace(raw)
	// Strip nullable suffix
	s = strings.TrimSuffix(s, "?")
	// Strip generic type arguments
	if idx := strings.Index(s, "<"); idx >= 0 {
		s = s[:idx]
	}
	// Strip array brackets
	s = strings.TrimSuffix(s, "[]")
	return s
}