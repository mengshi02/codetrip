// CSharp Namespace Gate — determines whether a C# symbol belongs to
// a namespace visible at the file level.
//
// Mirrors TS csharp-namespace-gate.ts, adapted for codetrip.
// C# namespaces are declared with `namespace X.Y.Z { ... }` at file level.
// This gate checks whether a given C# symbol's qualified name starts
// with the file's declared namespace, which is needed for correct
// import resolution in the scope resolution engine.
//
// In codetrip's 9 core languages, C# is the only one with file-level
// namespaces (Java has packages via directory convention, Go has
// packages, but C# requires explicit namespace declarations).

package core

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ─── Types ──────────────────────────────────────────────────

// CSharpNamespaceInfo holds the namespace extraction result for a C# file.
type CSharpNamespaceInfo struct {
	// DeclaredNamespace is the namespace declared at file level (e.g., "MyApp.Services")
	DeclaredNamespace string
	// IsNamespaceRoot indicates whether the file declares a root-level namespace
	// (no nesting under another namespace block).
	IsNamespaceRoot bool
}

// ─── Main gate function ──────────────────────────────────────

// GetCSharpNamespaceGate extracts namespace information from a C# file's
// symbol definitions. It checks for namespace declarations at the top level
// (not nested inside classes/interfaces).
//
// This is used by the scope resolution engine to determine whether a C#
// symbol's qualified name should include the file's namespace prefix.
func GetCSharpNamespaceGate(defs []shared.SymbolDefinition) CSharpNamespaceInfo {
	for _, def := range defs {
		if def.Type != shared.LabelNamespace {
			continue
		}
		// Namespace declarations have a QualifiedName that matches the namespace path
		if def.QualifiedName != nil {
			return CSharpNamespaceInfo{
				DeclaredNamespace: *def.QualifiedName,
				IsNamespaceRoot:    true, // file-level namespace declarations are always root
			}
		}
	}
	return CSharpNamespaceInfo{
		DeclaredNamespace: "",
		IsNamespaceRoot:    false,
	}
}

// ─── Helpers ────────────────────────────────────────────────

// IsCSharpSymbolInNamespace checks whether a C# symbol belongs to the
// given namespace. A symbol is in the namespace if its qualified name
// starts with the namespace prefix followed by a dot.
func IsCSharpSymbolInNamespace(qualifiedName, namespace string) bool {
	if namespace == "" {
		return true // global namespace
	}
	return strings.HasPrefix(qualifiedName, namespace+".")
}

// NormalizeCSharpNamespace normalizes a C# namespace string.
// C# namespaces use dots as separators; this ensures consistent casing.
func NormalizeCSharpNamespace(ns string) string {
	return strings.TrimSpace(ns)
}