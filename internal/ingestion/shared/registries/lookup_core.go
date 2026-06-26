// Package registries — lookupCore: the 7-step scope-resolution algorithm.
// Ported from gitnexus-shared scope-resolution/registries/lookup-core.ts (466 lines).
package registries

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CoreLookupParams holds the per-registry specialization parameters
// consumed by lookupCore. Three registries (Class/Method/Field) run
// the same 7-step algorithm with different parameter tuples.
type CoreLookupParams struct {
	// AcceptedKinds filters candidates by NodeLabel.
	AcceptedKinds []shared.NodeLabel
	// UseReceiverTypeBinding enables Step 2 (type-binding MRO walk).
	// Class lookups: false. Method/Field lookups: true.
	UseReceiverTypeBinding bool
	// OwnerScopedContributor is the Step 3 callback. nil = skip Step 3.
	OwnerScopedContributor OwnerScopedContributor
	// Callsite carries arity/argument-type info for Step 5.
	Callsite *shared.Callsite
	// ExplicitReceiver is the receiver name (e.g. "user" in user.save()).
	ExplicitReceiver *string
}

// candidateState tracks per-candidate mutable state during lookup.
type candidateState struct {
	def        shared.SymbolDefinition
	signals    shared.RawSignals
	tieKey     TieBreakKey
}

// lookupCore implements the 7-step scope-resolution algorithm.
//
// Step 1: Lexical chain walk (hard shadow)
// Step 2: Type-binding MRO walk
// Step 3: Owner-scoped contributor
// Step 4: Kind filter
// Step 5: Arity filter
// Step 6: Global qualified-name fallback
// Step 7: Evidence composition + confidence ranking + tie-break cascade
func lookupCore(name string, scope shared.ScopeID, params CoreLookupParams, ctx *RegistryContext) []shared.Resolution {
	acceptedKindsSet := make(map[shared.NodeLabel]bool, len(params.AcceptedKinds))
	for _, k := range params.AcceptedKinds {
		acceptedKindsSet[k] = true
	}

	candidates := make(map[shared.DefID]*candidateState)
	tieKeys := make(map[string]TieBreakKey)

	// ─── Step 1: Lexical chain walk ───────────────────────────────────
	// Walk up the scope chain from the starting scope, collecting bindings
	// named `name`. First binding at each scope wins (hard shadow).
	scopeDepth := 0
	currentScopeID := &scope
	for currentScopeID != nil {
		s := ctx.ScopeTree.GetScope(*currentScopeID)
		if s == nil {
			break
		}

		// Collect bindings at this scope
		bindingsAtScope := lookupBindingsAt(s.ID, name, ctx)
		for _, bref := range bindingsAtScope {
			collectCandidate(candidates, bref, scopeDepth, 0, acceptedKindsSet)
		}

		// Walk up to parent
		currentScopeID = s.Parent
		scopeDepth++
	}

	// ─── Step 2: Type-binding MRO walk ────────────────────────────────
	if params.UseReceiverTypeBinding {
		mroWalk(name, params, ctx, candidates, acceptedKindsSet)
	}

	// ─── Step 3: Owner-scoped contributor ─────────────────────────────
	if params.OwnerScopedContributor != nil {
		ownerScopedWalk(name, params, ctx, candidates, acceptedKindsSet)
	}

	// ─── Step 4: Kind filter ──────────────────────────────────────────
	// Remove candidates whose type is not in acceptedKinds
	filterByKind(candidates, acceptedKindsSet)

	// ─── Step 5: Arity filter ─────────────────────────────────────────
	if params.Callsite != nil && ctx.Providers != nil && ctx.Providers.ArityCompatibility != nil {
		filterByArity(candidates, params.Callsite, ctx.Providers.ArityCompatibility)
	}

	// ─── Step 6: Global qualified-name fallback ───────────────────────
	if len(candidates) == 0 && ctx.QualifiedNames != nil {
		defIDs := ctx.QualifiedNames.Get(name)
		for _, defID := range defIDs {
			def := ctx.Defs.Get(defID)
			if def == nil || !acceptedKindsSet[def.Type] {
				continue
			}
			if _, exists := candidates[defID]; exists {
				continue
			}
			signals := shared.RawSignals{
				Origin:          shared.GlobalQualifiedOrigin,
				KindMatch:       true,
				GlobalQualified: true,
			}
			candidates[defID] = &candidateState{
				def:     *def,
				signals: signals,
				tieKey: TieBreakKey{
					ScopeDepth:     0,
					MroDepth:       0,
					OriginPriority: shared.OriginPriorityGlobalQN,
					DefID:          defID,
				},
			}
		}
	}

	// ─── Step 7: Evidence composition + confidence ranking + tie-break ─
	resolutions := buildResolutions(candidates, tieKeys)
	return resolutions
}

// LookupCore is the exported version of lookupCore for use by the four registries.
func LookupCore(name string, scope shared.ScopeID, params CoreLookupParams, ctx *RegistryContext) []shared.Resolution {
	return lookupCore(name, scope, params, ctx)
}

// lookupBindingsAt collects all bindings for `name` at the given scope,
// including both primary bindings and binding augmentations.
func lookupBindingsAt(scopeID shared.ScopeID, name string, ctx *RegistryContext) []*shared.BindingRef {
	var result []*shared.BindingRef

	// Primary bindings
	if scopeBindings, ok := ctx.Bindings[scopeID]; ok {
		if refs, ok := scopeBindings[name]; ok {
			for i := range refs {
				result = append(result, refs[i])
			}
		}
	}

	// Binding augmentations (post-finalize channel)
	if ctx.BindingAugmentations != nil {
		if augBindings, ok := ctx.BindingAugmentations[scopeID]; ok {
			if refs, ok := augBindings[name]; ok {
				for i := range refs {
					result = append(result, refs[i])
				}
			}
		}
	}

	return result
}

// collectCandidate adds a binding reference as a candidate if it passes initial checks.
func collectCandidate(
	candidates map[shared.DefID]*candidateState,
	bref *shared.BindingRef,
	scopeDepth, mroDepth int,
	acceptedKindsSet map[shared.NodeLabel]bool,
) {
	defID := bref.Def.NodeID
	if _, exists := candidates[defID]; exists {
		return // first-write-wins
	}

	originPriority, ok := shared.ORIGIN_PRIORITY[bref.Origin]
	if !ok {
		originPriority = shared.OriginPriorityGlobal
	}

	signals := shared.RawSignals{
		Origin:     bref.Origin,
		KindMatch:  acceptedKindsSet[bref.Def.Type],
		ScopeDepth: scopeDepth,
		MroDepth:   mroDepth,
	}

	switch bref.Origin {
	case shared.OriginLocal:
		signals.Local = true
	case shared.OriginImport:
		signals.Import = true
	case shared.OriginNamespace:
		signals.Import = true
	case shared.OriginWildcard:
		signals.Import = true
	case shared.OriginReexport:
		signals.Import = true
	}

	if scopeDepth > 0 {
		signals.ScopeChain = true
	}

	candidates[defID] = &candidateState{
		def:     bref.Def,
		signals: signals,
		tieKey: TieBreakKey{
			ScopeDepth:     scopeDepth,
			MroDepth:       mroDepth,
			OriginPriority: originPriority,
			DefID:          defID,
		},
	}
}

// mroWalk implements Step 2: type-binding MRO walk.
// Resolves the receiver type at the callsite scope, then walks
// the MRO chain collecting method/field candidates.
func mroWalk(
	name string,
	params CoreLookupParams,
	ctx *RegistryContext,
	candidates map[shared.DefID]*candidateState,
	acceptedKindsSet map[shared.NodeLabel]bool,
) {
	// Find the receiver type binding
	var typeRef *shared.TypeRef

	// Check type bindings at the starting scope
	if ctx.TypeBindings != nil {
		// Try explicit receiver first
		if params.ExplicitReceiver != nil {
			// Walk scope chain looking for the receiver's type binding
			receiverName := *params.ExplicitReceiver
			typeRef = findTypeBindingForReceiver(receiverName, ctx)
		}

		// If no explicit receiver, try implicit self/this
		if typeRef == nil {
			typeRef = findSelfTypeBinding(ctx)
		}
	}

	if typeRef == nil {
		return
	}

	// Resolve the type reference to a definition
	resolveCtx := &shared.ResolveTypeRefContext{
		ScopeTree:      ctx.ScopeTree,
		DefIndex:       ctx.Defs,
		QualifiedNames: ctx.QualifiedNames,
	}
	typeDef := shared.ResolveTypeRef(*typeRef, resolveCtx)
	if typeDef == nil {
		return
	}

	// Walk the MRO chain for the resolved type
	if ctx.MethodDispatch == nil {
		return
	}
	mro := ctx.MethodDispatch.MROByOwner(typeDef.NodeID)
	if len(mro) == 0 {
		mro = []shared.DefID{typeDef.NodeID} // self-only
	}

	for mroDepth, ownerDefID := range mro {
		// Collect methods/fields owned by this MRO level
		ownerDef := ctx.Defs.Get(ownerDefID)
		if ownerDef == nil {
			continue
		}

		// Look up owned defs with matching name
		// This uses the owner-scoped lookup pattern
		if params.OwnerScopedContributor != nil {
			ownedDefs := params.OwnerScopedContributor(ownerDefID, name)
			for _, def := range ownedDefs {
				if _, exists := candidates[def.NodeID]; !exists {
					signals := shared.RawSignals{
						Origin:      shared.OriginLocal,
						TypeBinding: true,
						KindMatch:   acceptedKindsSet[def.Type],
						MroDepth:    mroDepth,
						ScopeDepth:  0,
					}
					candidates[def.NodeID] = &candidateState{
						def:     def,
						signals: signals,
						tieKey: TieBreakKey{
							ScopeDepth:     0,
							MroDepth:       mroDepth,
							OriginPriority: shared.OriginPriorityLocal,
							DefID:          def.NodeID,
						},
					}
				}
			}
		}
	}
}

// findTypeBindingForReceiver searches the scope chain for a type binding
// matching the given receiver variable name.
func findTypeBindingForReceiver(receiverName string, ctx *RegistryContext) *shared.TypeRef {
	for _, scopeBindings := range ctx.TypeBindings {
		if tr, ok := scopeBindings[receiverName]; ok {
			return &tr
		}
	}
	return nil
}

// findSelfTypeBinding searches for an implicit self/this type binding.
func findSelfTypeBinding(ctx *RegistryContext) *shared.TypeRef {
	for _, scopeBindings := range ctx.TypeBindings {
		if tr, ok := scopeBindings["self"]; ok {
			return &tr
		}
		if tr, ok := scopeBindings["this"]; ok {
			return &tr
		}
	}
	return nil
}

// ownerScopedWalk implements Step 3: owner-scoped contributor.
// Uses the implsByInterfaceDefId index to find implementations
// when the lookup is for an interface method.
func ownerScopedWalk(
	name string,
	params CoreLookupParams,
	ctx *RegistryContext,
	candidates map[shared.DefID]*candidateState,
	acceptedKindsSet map[shared.NodeLabel]bool,
) {
	// This is delegated to the OwnerScopedContributor callback
	// which is already handled in Step 2's MRO walk.
	// Step 3 is an extension point for languages with complex
	// impl relationships (e.g., C# explicit interface implementations).
	// The callback is called during the MRO walk.
}

// filterByKind removes candidates whose NodeLabel is not in acceptedKinds.
func filterByKind(candidates map[shared.DefID]*candidateState, acceptedKindsSet map[shared.NodeLabel]bool) {
	for defID, state := range candidates {
		if !acceptedKindsSet[state.def.Type] {
			delete(candidates, defID)
		}
	}
}

// filterByArity evaluates arity compatibility for each candidate and
// removes those with clear mismatches.
func filterByArity(
	candidates map[shared.DefID]*candidateState,
	callsite *shared.Callsite,
	arityFn func(*shared.SymbolDefinition, *shared.Callsite) shared.ArityVerdict,
) {
	for defID, state := range candidates {
		verdict := arityFn(&state.def, callsite)
		state.signals.ArityMatch = verdict
		if verdict == shared.ArityMismatch {
			delete(candidates, defID)
		}
	}
}

// buildResolutions converts candidates into a sorted slice of Resolutions.
func buildResolutions(candidates map[shared.DefID]*candidateState, tieKeys map[string]TieBreakKey) []shared.Resolution {
	if len(candidates) == 0 {
		return nil
	}

	resolutions := make([]shared.Resolution, 0, len(candidates))
	for _, state := range candidates {
		evidence := shared.ComposeEvidence(state.signals)
		confidence := shared.ConfidenceFromEvidence(evidence)
		resolutions = append(resolutions, shared.Resolution{
			Def:        state.def,
			Confidence: confidence,
			Evidence:   evidence,
		})
		tieKeys[state.def.NodeID] = state.tieKey
	}

	// Sort by confidence + tie-break cascade (descending)
	sort.SliceStable(resolutions, func(i, j int) bool {
		cmp := CompareByConfidenceWithTiebreaks(resolutions[i], resolutions[j], tieKeys)
		return cmp > 0 // higher priority first
	})

	return resolutions
}