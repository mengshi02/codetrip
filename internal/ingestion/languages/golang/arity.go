// Package go — Go arity compatibility check.
// Ported from TS languages/go/arity.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// GoArityCompatibility checks whether a callsite's arity matches a definition's
// parameter signature. Go variadic parameters (type starting with "...") are
// handled specially — callsites with more args than the def's max are still
// compatible when the def is variadic.
//
// Mirrors TS goArityCompatibility(def, callsite).
func GoArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	max := def.ParameterCount
	min := def.RequiredParameterCount
	if max == nil && min == nil {
		return scope_resolution.ArityUnknown
	}
	if callsite.Arity != nil && *callsite.Arity < 0 {
		return scope_resolution.ArityUnknown
	}

	variadic := false
	for _, t := range def.ParameterTypes {
		if strings.HasPrefix(t, "...") {
			variadic = true
			break
		}
	}
	callArity := 0
	if callsite.Arity != nil {
		callArity = *callsite.Arity
	}
	if min != nil && callArity < *min {
		return scope_resolution.ArityIncompatible
	}
	if max != nil && callArity > *max && !variadic {
		return scope_resolution.ArityIncompatible
	}
	return scope_resolution.ArityCompatible
}