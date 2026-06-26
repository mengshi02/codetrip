// macroRegistry: Macro resolution via lookupCore.
// Ported from gitnexus scope-resolution registries (macro-registry pattern).
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// MacroRegistry resolves macro references using the 7-step lookupCore
// algorithm with useReceiverTypeBinding=false.
// Macros (C/C++ preprocessor, Rust macro_rules!) resolve purely from
// lexical scope — they do not participate in MRO dispatch.
type MacroRegistry struct{}

// NewMacroRegistry creates a MacroRegistry.
func NewMacroRegistry() *MacroRegistry {
	return &MacroRegistry{}
}

// Lookup resolves a macro name starting from the given scope.
// Uses MACRO_KINDS as the accepted kinds filter and disables type-binding MRO walk.
func (r *MacroRegistry) Lookup(name string, scope shared.ScopeID, ctx *RegistryContext) []shared.Resolution {
	params := CoreLookupParams{
		AcceptedKinds:           MACRO_KINDS,
		UseReceiverTypeBinding:  false,
		OwnerScopedContributor:  nil,
		Callsite:                nil,
		ExplicitReceiver:        nil,
	}
	return lookupCore(name, scope, params, ctx)
}

// LookupQualified resolves a macro by its global qualified name.
func (r *MacroRegistry) LookupQualified(name string, ctx *RegistryContext) []shared.Resolution {
	return LookupQualified(name, LookupQualifiedParams{
		AcceptedKinds: MACRO_KINDS,
	}, ctx)
}