package scope_resolution

import (
	"math"
	"regexp"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ConversionRankFn scores the cost of converting argType to paramType.
//   - 0 = exact match (no conversion)
//   - 1 = promotion (e.g. char→int, bool→int in C++)
//   - 2 = standard conversion (e.g. int→double)
//   - +Inf = incompatible types
//
// Each language provides its own implementation.
type ConversionRankFn func(argType, paramType string, argTypeClass, paramTypeClass *shared.ParameterTypeClass) float64

// OverloadNarrowingHookCtx bundles optional extension points for narrowing.
// Threaded in from pickOverload / pickFirstNonStaticOnly so per-language
// narrowing can layer in conversion-rank scoring and constraint filtering
// without changing the call signature at every site.
type OverloadNarrowingHookCtx struct {
	// ArgumentTypeClasses is aligned with argTypes.
	ArgumentTypeClasses []shared.ParameterTypeClass
	// ConversionRankFn is the conversion-rank scoring fallback (step 4b).
	// Engages when the exact-type filter rejects every candidate.
	ConversionRankFn ConversionRankFn
	// ConstraintCompatibility drops candidates whose template constraints
	// (SFINAE enable_if_t, C++20 requires, etc.) provably fail at the call
	// site. Three-valued — 'unknown' keeps the candidate (monotonicity).
	ConstraintCompatibility func(callsite shared.Callsite, def shared.SymbolDefinition, argTypes []string, argTypeClasses []shared.ParameterTypeClass) ArityVerdict
}

// templatePlaceholderRegex matches C++ template parameter placeholders
// (uppercase first letter, e.g. T, InputIterator).
var templatePlaceholderRegex = regexp.MustCompile(`^[A-Z]\w*$`)

// NarrowOverloadCandidates picks candidates from a list of same-named
// method/function overloads using the call-site's arity and argument-type
// signals.
//
// Semantics (first-wins; callers take result[0]):
//  1. If argCount is undefined (-1), arity is a pass-through.
//  2. Exact-required-match wins over variadic. Variadic is detected
//     via a parameterTypes entry equal to "params" or starting with
//     "params " (C# params / variadic marker).
//  3. If the arity filter empties the set AND any candidate had unknown
//     bounds (both parameterCount and requiredParameterCount undefined),
//     fall back to the full overload list. If EVERY rejected candidate
//     had definite arity bounds, trust the filter and return empty.
//  4. If argTypes is present, filter further by per-slot type equality.
//     An empty string in argTypes[i] means "unknown" and counts as a
//     match. Mismatches disqualify. A non-empty typed result wins;
//     otherwise return the arity-filtered candidates.
//  4b. When exact-type filter returns empty AND conversionRankFn is
//     provided, rank candidates via pairwise dominance comparison
//     (ISO C++ [over.ics.rank]).
//  4c. Per-candidate constraint filter (SFINAE / requires).
//  4d. Conservative C++ template partial-order approximation.
//  5. Empty input returns empty output.
func NarrowOverloadCandidates(
	overloads []shared.SymbolDefinition,
	argCount int, // -1 = undefined
	argTypes []string,
	hookCtx *OverloadNarrowingHookCtx,
) []shared.SymbolDefinition {
	if len(overloads) == 0 {
		return nil
	}

	// Step 1–3: arity filter
	var arityMatches []shared.SymbolDefinition
	if argCount < 0 {
		arityMatches = overloads
	} else {
		for _, d := range overloads {
			if !arityFilterPass(d, argCount) {
				continue
			}
			arityMatches = append(arityMatches, d)
		}
	}

	// Fallback: only use full overload list if some candidate had unknown bounds
	anyUnknownBounds := false
	for _, d := range overloads {
		if d.ParameterCount == nil && d.RequiredParameterCount == nil {
			anyUnknownBounds = true
			break
		}
	}

	candidates := arityMatches
	if len(candidates) == 0 {
		if anyUnknownBounds {
			candidates = overloads
		} else {
			return nil
		}
	}

	result := candidates

	// Step 4: exact-type filter
	if len(argTypes) > 0 {
		var typed []shared.SymbolDefinition
		for _, d := range candidates {
			if !exactTypeFilterPass(d, argTypes, hookCtx) {
				continue
			}
			typed = append(typed, d)
		}
		if len(typed) > 0 {
			result = typed
		} else if hookCtx != nil && hookCtx.ConversionRankFn != nil {
			// Step 4b: conversion-rank scoring
			ranked := rankByConversion(candidates, argTypes, hookCtx)
			if len(ranked) > 0 {
				result = ranked
			}
		}
	}

	// Step 4c: constraint filter
	if hookCtx != nil && hookCtx.ConstraintCompatibility != nil && argCount >= 0 {
		callsite := shared.Callsite{Arity: &argCount}
		argTypeClasses := []shared.ParameterTypeClass{}
		if hookCtx.ArgumentTypeClasses != nil {
			argTypeClasses = hookCtx.ArgumentTypeClasses
		}
		var filtered []shared.SymbolDefinition
		for _, def := range result {
			if len(def.TemplateArguments) > 0 || hasTemplateConstraints(def) {
				verdict := hookCtx.ConstraintCompatibility(callsite, def, argTypes, argTypeClasses)
				if verdict == ArityIncompatible {
					continue
				}
			}
			filtered = append(filtered, def)
		}
		result = filtered
	}

	// Step 4d: template partial-order approximation
	if len(result) > 1 && len(argTypes) > 0 {
		partiallyOrdered := rankByTemplatePartialOrdering(result, argTypes, hookCtx)
		if partiallyOrdered != nil {
			result = partiallyOrdered
		}
	}

	return result
}

// arityFilterPass checks whether a definition can accept the given argCount.
func arityFilterPass(d shared.SymbolDefinition, argCount int) bool {
	if d.ParameterCount != nil && argCount > *d.ParameterCount {
		// Check variadic marker — C# 'params' keyword
		variadic := false
		for _, t := range d.ParameterTypes {
			if t == "params" || len(t) > 6 && t[:7] == "params " {
				variadic = true
				break
			}
		}
		if !variadic {
			return false
		}
	}
	if d.RequiredParameterCount != nil && argCount < *d.RequiredParameterCount {
		return false
	}
	return true
}

// exactTypeFilterPass checks per-slot type equality.
func exactTypeFilterPass(d shared.SymbolDefinition, argTypes []string, hookCtx *OverloadNarrowingHookCtx) bool {
	params := d.ParameterTypes
	if params == nil {
		return false
	}
	for i := 0; i < len(argTypes) && i < len(params); i++ {
		if argTypes[i] == "" {
			continue // unknown arg → match
		}
		var argTC, paramTC *shared.ParameterTypeClass
		if hookCtx != nil && i < len(hookCtx.ArgumentTypeClasses) {
			tc := hookCtx.ArgumentTypeClasses[i]
			argTC = &tc
		}
		if i < len(d.ParameterTypeClasses) {
			tc := d.ParameterTypeClasses[i]
			paramTC = &tc
		}
		if !exactTypeSlotMatches(argTypes[i], params[i], argTC, paramTC) {
			return false
		}
	}
	return true
}

// exactTypeSlotMatches checks whether an argument type matches a parameter type slot.
func exactTypeSlotMatches(argType, paramType string, argTypeClass, paramTypeClass *shared.ParameterTypeClass) bool {
	if argType != paramType {
		return false
	}
	// C++ normalizes away pointer markers (int* → int). When both sides
	// provide shape sidecars, don't let that collapse make int exactly
	// match int*. Unknown sidecar evidence preserves the string-only path.
	if argTypeClass == nil || paramTypeClass == nil {
		return true
	}
	if argTypeClass.Indirection == shared.IndirectionUnknown || paramTypeClass.Indirection == shared.IndirectionUnknown {
		return true
	}
	return isPointerShape(*argTypeClass) == isPointerShape(*paramTypeClass)
}

// isPointerShape returns true when the type class describes a pointer.
func isPointerShape(tc shared.ParameterTypeClass) bool {
	return tc.Indirection == shared.IndirectionPointer && tc.PointerDepth > 0
}

// hasTemplateConstraints checks if a definition has template constraints.
func hasTemplateConstraints(def shared.SymbolDefinition) bool {
	// In Go port, templateConstraints are encoded in TemplateArguments
	// with a "requires:" prefix, or we check if there are any
	// non-type template arguments with constraint markers.
	// For now, conservative: assume any definition with template
	// arguments might have constraints.
	return len(def.TemplateArguments) > 0
}

// rankByConversion performs pairwise dominance comparison (ISO C++ [over.ics.rank]).
func rankByConversion(
	candidates []shared.SymbolDefinition,
	argTypes []string,
	hookCtx *OverloadNarrowingHookCtx,
) []shared.SymbolDefinition {
	type viable struct {
		def   shared.SymbolDefinition
		ranks []float64
	}

	var viableList []viable
	for _, d := range candidates {
		params := d.ParameterTypes
		if params == nil {
			continue
		}
		var ranks []float64
		ok := true
		for i := 0; i < len(argTypes); i++ {
			paramType := parameterTypeAt(params, i)
			if paramType == "" {
				ok = false
				break
			}
			if argTypes[i] == "" {
				ranks = append(ranks, 0) // unknown arg → any-match
				continue
			}
			var argTC, paramTC *shared.ParameterTypeClass
			if hookCtx != nil && i < len(hookCtx.ArgumentTypeClasses) {
				tc := hookCtx.ArgumentTypeClasses[i]
				argTC = &tc
			}
			if i < len(d.ParameterTypeClasses) {
				tc := d.ParameterTypeClasses[i]
				paramTC = &tc
			}
			r := hookCtx.ConversionRankFn(argTypes[i], paramType, argTC, paramTC)
			if math.IsInf(r, 1) { // +Infinity = incompatible
				ok = false
				break
			}
			ranks = append(ranks, r)
		}
		if !ok {
			continue
		}
		viableList = append(viableList, viable{def: d, ranks: ranks})
	}

	if len(viableList) <= 1 {
		result := make([]shared.SymbolDefinition, len(viableList))
		for i, v := range viableList {
			result[i] = v.def
		}
		return result
	}

	// Pairwise dominance
	dominated := make(map[int]bool)
	for i := 0; i < len(viableList); i++ {
		if dominated[i] {
			continue
		}
		for j := i + 1; j < len(viableList); j++ {
			if dominated[j] {
				continue
			}
			cmp := pairwiseCompare(viableList[i].ranks, viableList[j].ranks)
			if cmp < 0 {
				dominated[j] = true // i dominates j
			} else if cmp > 0 {
				dominated[i] = true // j dominates i
			}
		}
	}

	var result []shared.SymbolDefinition
	for i, v := range viableList {
		if !dominated[i] {
			result = append(result, v.def)
		}
	}
	return result
}

// parameterTypeAt returns the parameter type at the given argument index,
// accounting for variadic markers.
func parameterTypeAt(params []string, argIndex int) string {
	if argIndex < len(params) {
		return params[argIndex]
	}
	last := params[len(params)-1]
	if last == "..." {
		return "..."
	}
	return ""
}

// parameterTypeClassAt returns the parameter type class at the given argument index.
func parameterTypeClassAt(params []shared.ParameterTypeClass, argIndex int) *shared.ParameterTypeClass {
	if params == nil {
		return nil
	}
	if argIndex < len(params) {
		return &params[argIndex]
	}
	last := params[len(params)-1]
	if last.Base == "..." {
		return &last
	}
	return nil
}

// pairwiseCompare compares two per-slot rank vectors.
// Returns -1 if a dominates b, +1 if b dominates a, 0 otherwise.
func pairwiseCompare(a, b []float64) int {
	aBetter := false
	bBetter := false
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	for i := 0; i < length; i++ {
		if a[i] < b[i] {
			aBetter = true
		} else if b[i] < a[i] {
			bBetter = true
		}
		if aBetter && bBetter {
			return 0 // incomparable
		}
	}
	if aBetter && !bBetter {
		return -1
	}
	if bBetter && !aBetter {
		return 1
	}
	return 0
}

// rankByTemplatePartialOrdering performs closed-table approximation of C++
// function-template partial ordering.
func rankByTemplatePartialOrdering(
	candidates []shared.SymbolDefinition,
	argTypes []string,
	hookCtx *OverloadNarrowingHookCtx,
) []shared.SymbolDefinition {
	if hookCtx == nil || hookCtx.ArgumentTypeClasses == nil {
		return nil
	}

	type viable struct {
		def   shared.SymbolDefinition
		ranks []int
	}

	var viableList []viable
	for _, def := range candidates {
		params := def.ParameterTypes
		paramClasses := def.ParameterTypeClasses
		if params == nil || paramClasses == nil {
			continue
		}

		var ranks []int
		sawTemplateSlot := false
		ok := true
		for i := 0; i < len(argTypes); i++ {
			paramType := parameterTypeAt(params, i)
			paramClass := parameterTypeClassAt(paramClasses, i)
			var argClass *shared.ParameterTypeClass
			if i < len(hookCtx.ArgumentTypeClasses) {
				tc := hookCtx.ArgumentTypeClasses[i]
				argClass = &tc
			}
			if paramType == "" || paramClass == nil || argClass == nil {
				ok = false
				break
			}

			rank := templatePartialOrderSlotRank(paramType, *paramClass, *argClass)
			if rank < 0 { // -1 = undefined
				ok = false
				break
			}
			if isTemplatePlaceholder(paramType) {
				sawTemplateSlot = true
			}
			ranks = append(ranks, rank)
		}
		if ok && sawTemplateSlot {
			viableList = append(viableList, viable{def: def, ranks: ranks})
		}
	}

	if len(viableList) == 0 {
		return nil
	}
	if len(viableList) != len(candidates) {
		return []shared.SymbolDefinition{}
	}
	if len(viableList) <= 1 {
		result := make([]shared.SymbolDefinition, len(viableList))
		for i, v := range viableList {
			result[i] = v.def
		}
		return result
	}

	dominated := make(map[int]bool)
	for i := 0; i < len(viableList); i++ {
		if dominated[i] {
			continue
		}
		for j := i + 1; j < len(viableList); j++ {
			if dominated[j] {
				continue
			}
			cmp := compareSpecializationRanks(viableList[i].ranks, viableList[j].ranks)
			if cmp < 0 {
				dominated[j] = true
			} else if cmp > 0 {
				dominated[i] = true
			}
		}
	}

	var result []shared.SymbolDefinition
	for i, v := range viableList {
		if !dominated[i] {
			result = append(result, v.def)
		}
	}
	return result
}

// templatePartialOrderSlotRank returns a specialization rank for a single slot.
// Returns -1 if the slot rank is undefined.
func templatePartialOrderSlotRank(paramType string, paramClass, argClass shared.ParameterTypeClass) int {
	if !isTemplatePlaceholder(paramType) {
		return -1
	}
	if argClass.Indirection == shared.IndirectionUnknown || paramClass.Indirection == shared.IndirectionUnknown {
		return -1
	}
	if isPointerShape(paramClass) {
		if isPointerShape(argClass) {
			return 3
		}
		return -1
	}
	if paramClass.Indirection == shared.IndirectionValue {
		return 1
	}
	return -1
}

// isTemplatePlaceholder checks if a type name is a C++ template parameter placeholder.
func isTemplatePlaceholder(typeName string) bool {
	return templatePlaceholderRegex.MatchString(typeName)
}

// compareSpecializationRanks compares two specialization rank vectors.
// Higher rank is better. Returns -1 if a dominates b, +1 if b dominates a, 0 otherwise.
func compareSpecializationRanks(a, b []int) int {
	aBetter := false
	bBetter := false
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	for i := 0; i < length; i++ {
		if a[i] > b[i] {
			aBetter = true
		} else if b[i] > a[i] {
			bBetter = true
		}
		if aBetter && bBetter {
			return 0
		}
	}
	if aBetter && !bBetter {
		return -1
	}
	if bBetter && !aBetter {
		return 1
	}
	return 0
}

// IsOverloadAmbiguousAfterNormalization detects when >1 candidate share
// identical parameterTypes after the per-language normalizer has collapsed
// distinct underlying types.
//
// Returns false when:
//   - 0 or 1 candidates (no ambiguity)
//   - any candidate has undefined parameterTypes
//   - candidates differ in arity or in any parameter-type slot
func IsOverloadAmbiguousAfterNormalization(candidates []shared.SymbolDefinition, argCount int) bool {
	if len(candidates) < 2 {
		return false
	}
	first := candidates[0].ParameterTypes
	if first == nil {
		return false
	}

	// When argCount >= 0, compare only the first argCount slots.
	compareUpTo := len(first)
	if argCount >= 0 && argCount < compareUpTo {
		compareUpTo = argCount
	}
	if compareUpTo == 0 {
		return false
	}
	if len(first) < compareUpTo {
		return false
	}

	for i := 1; i < len(candidates); i++ {
		p := candidates[i].ParameterTypes
		if p == nil {
			return false
		}
		if len(p) < compareUpTo {
			return false
		}
		for j := 0; j < compareUpTo; j++ {
			if p[j] != first[j] {
				return false
			}
		}
		// When argCount < 0, also require length equality so distinct-arity
		// candidates that share a prefix don't collapse to ambiguous.
		if argCount < 0 && len(p) != len(first) {
			return false
		}
	}
	return true
}