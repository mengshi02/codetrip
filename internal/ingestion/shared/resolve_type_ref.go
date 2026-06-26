// Package shared — resolveTypeRef: strict single-return type resolver.
// Ported from gitnexus-shared scope-resolution/resolve-type-ref.ts (149 lines).
package shared

// STRICT_ORIGINS are the binding origins accepted by resolveTypeRef.
// Wildcard and global-qualified origins are excluded because they are
// too ambiguous for strict type resolution.
var STRICT_ORIGINS = map[BindingOrigin]bool{
	OriginLocal:     true,
	OriginImport:    true,
	OriginNamespace: true,
	OriginReexport:  true,
}

// TYPE_KINDS are the NodeLabels considered "type-like" for resolveTypeRef.
var TYPE_KINDS = map[NodeLabel]bool{
	LabelClass:      true,
	LabelStruct:     true,
	LabelInterface:  true,
	LabelEnum:       true,
	LabelUnion:      true,
	LabelTrait:      true,
	LabelTypeAlias:  true,
	LabelTypedef:    true,
	LabelRecord:     true,
	LabelDelegate:   true,
	LabelAnnotation: true,
	LabelTemplate:   true,
}

// ResolveTypeRefContext provides the dependencies for resolveTypeRef.
type ResolveTypeRefContext struct {
	ScopeTree     *ScopeTree
	DefIndex      *DefIndex
	QualifiedNames *QualifiedNameIndex
}

// ResolveTypeRef resolves a TypeRef to its single target SymbolDefinition.
// This is a strict resolver: it walks the scope chain, filters by STRICT_ORIGINS
// and TYPE_KINDS, and falls back to qualified-name lookup.
//
// Returns nil if no unique type definition can be found.
// This is the Go equivalent of the TS resolveTypeRef function used by
// Step 2 of the 7-step lookup (type-binding MRO walk).
func ResolveTypeRef(typeRef TypeRef, ctx *ResolveTypeRefContext) *SymbolDefinition {
	rawName := typeRef.RawName
	scopeID := typeRef.DeclaredAtScope

	// Walk the scope chain looking for a type binding
	currentScopeID := &scopeID
	for currentScopeID != nil {
		scope := ctx.ScopeTree.GetScope(*currentScopeID)
		if scope == nil {
			break
		}

		// Check bindings at this scope
		if bindings, ok := scope.Bindings[rawName]; ok {
			for _, bref := range bindings {
				// Filter: must be from a strict origin
				if !STRICT_ORIGINS[bref.Origin] {
					continue
				}
				// Filter: must be a type-like kind
				if !TYPE_KINDS[bref.Def.Type] {
					continue
				}
				// First strict match wins
				return &bref.Def
			}
		}

		// Walk up to parent
		currentScopeID = scope.Parent
	}

	// Fallback: qualified-name lookup
	if ctx.QualifiedNames != nil {
		defIDs := ctx.QualifiedNames.Get(rawName)
		for _, defID := range defIDs {
			def := ctx.DefIndex.Get(defID)
			if def != nil && TYPE_KINDS[def.Type] {
				return def
			}
		}
	}

	return nil
}

// ResolveTypeRefWithEvidence resolves a TypeRef and returns both the definition
// and the evidence trail. Used when the caller needs to understand WHY a
// particular type was resolved.
func ResolveTypeRefWithEvidence(typeRef TypeRef, ctx *ResolveTypeRefContext) (*SymbolDefinition, []ResolutionEvidence) {
	rawName := typeRef.RawName
	scopeID := typeRef.DeclaredAtScope

	// Walk the scope chain
	scopeDepth := 0
	currentScopeID := &scopeID
	for currentScopeID != nil {
		scope := ctx.ScopeTree.GetScope(*currentScopeID)
		if scope == nil {
			break
		}

		if bindings, ok := scope.Bindings[rawName]; ok {
			for _, bref := range bindings {
				if !STRICT_ORIGINS[bref.Origin] {
					continue
				}
				if !TYPE_KINDS[bref.Def.Type] {
					continue
				}
				signals := RawSignals{
					Origin:      bref.Origin,
					KindMatch:   true,
					ScopeDepth:  scopeDepth,
					ScopeChain:  scopeDepth > 0,
					Local:       scopeDepth == 0 && bref.Origin == OriginLocal,
					Import:      bref.Origin == OriginImport || bref.Origin == OriginNamespace || bref.Origin == OriginReexport,
				}
				evidence := ComposeEvidence(signals)
				return &bref.Def, evidence
			}
		}

		currentScopeID = scope.Parent
		scopeDepth++
	}

	// Fallback: qualified-name lookup
	if ctx.QualifiedNames != nil {
		defIDs := ctx.QualifiedNames.Get(rawName)
		for _, defID := range defIDs {
			def := ctx.DefIndex.Get(defID)
			if def != nil && TYPE_KINDS[def.Type] {
				signals := RawSignals{
					Origin:          GlobalQualifiedOrigin,
					KindMatch:       true,
					GlobalQualified: true,
				}
				evidence := ComposeEvidence(signals)
				return def, evidence
			}
		}
	}

	return nil, nil
}