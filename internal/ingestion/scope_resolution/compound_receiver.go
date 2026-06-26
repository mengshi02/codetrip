package scope_resolution

import (
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// Max depth for compound-receiver chain resolution (a().b().c().d()).
const compoundReceiverMaxDepth = 8

// mapTupleSentinelRegex matches __MAP_TUPLE_i__:rhs sentinel strings.
var mapTupleSentinelRegex = regexp.MustCompile(`^__MAP_TUPLE_(\d+)__:(.+)$`)

// ResolveCompoundReceiverOptions controls the compound-receiver resolution.
type ResolveCompoundReceiverOptions struct {
	FieldFallback             bool
	UnwrapCollectionAccessor  func(receiverType string, accessor string) string
	HoistTypeBindingsToModule bool
}

// mapTupleParseResult captures the tuple index and RHS name from a
// __MAP_TUPLE_i__:rhs sentinel.
type mapTupleParseResult struct {
	tupleIdx int
	rhs      string
}

// typeBindingAt returns a pointer to the TypeRef for name in scope's TypeBindings,
// or nil if not found.  Needed because Go maps return value types, not pointers.
func typeBindingAt(scope *shared.Scope, name string) *shared.TypeRef {
	if scope == nil || scope.TypeBindings == nil {
		return nil
	}
	tr, ok := scope.TypeBindings[name]
	if !ok {
		return nil
	}
	return &tr
}

// parseMapTupleSentinel attempts to decompose a __MAP_TUPLE_i__:rhs string.
func parseMapTupleSentinel(text string) *mapTupleParseResult {
	match := mapTupleSentinelRegex.FindStringSubmatch(text)
	if match == nil || len(match) < 3 {
		return nil
	}
	idx := 0
	for _, c := range match[1] {
		idx = idx*10 + int(c-'0')
	}
	return &mapTupleParseResult{tupleIdx: idx, rhs: match[2]}
}

// ResolveCompoundReceiverClass resolves a compound-receiver expression's TYPE
// to the class def of the value the receiver expression produces.
func ResolveCompoundReceiverClass(
	receiverText string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
	options ResolveCompoundReceiverOptions,
	depth int,
) *shared.SymbolDefinition {
	if depth > compoundReceiverMaxDepth {
		return nil
	}
	text := strings.TrimSpace(receiverText)
	if len(text) == 0 {
		return nil
	}
	fieldFallback := options.FieldFallback
	if !fieldFallback {
		fieldFallback = true
	}

	if !strings.Contains(text, ".") && !strings.Contains(text, "(") {
		return resolveBareIdent(text, inScope, scopes, index, options, depth, fieldFallback)
	}
	if strings.HasSuffix(text, ")") {
		return resolveCallExpr(text, inScope, scopes, index, options, depth, fieldFallback)
	}
	return resolveMixedChain(text, inScope, scopes, index, options, depth, fieldFallback)
}

// resolveBareIdent handles the bare identifier case for compound receiver resolution.
func resolveBareIdent(
	text string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
	options ResolveCompoundReceiverOptions,
	depth int,
	fieldFallback bool,
) *shared.SymbolDefinition {
	mapTuple := parseMapTupleSentinel(text)
	if mapTuple != nil {
		rhsTB := FindReceiverTypeBinding(inScope, mapTuple.rhs, scopes)
		if rhsTB == nil {
			return nil
		}
		arg := extractShallowMapTypeArgByIndex(rhsTB.RawName, mapTuple.tupleIdx)
		if arg == "" {
			return nil
		}
		return FindClassBindingInScope(rhsTB.DeclaredAtScope, arg, scopes)
	}

	tb := FindReceiverTypeBinding(inScope, text, scopes)
	if tb != nil {
		boundMapTuple := parseMapTupleSentinel(tb.RawName)
		if boundMapTuple != nil {
			rhsTB := FindReceiverTypeBinding(inScope, boundMapTuple.rhs, scopes)
			if rhsTB == nil {
				return nil
			}
			arg := extractShallowMapTypeArgByIndex(rhsTB.RawName, boundMapTuple.tupleIdx)
			if arg == "" {
				return nil
			}
			return FindClassBindingInScope(rhsTB.DeclaredAtScope, arg, scopes)
		}

		viaTB := FindClassBindingInScope(tb.DeclaredAtScope, tb.RawName, scopes)
		if viaTB != nil {
			return viaTB
		}

		// Member-alias / call-result shapes store RHS path on rawName.
		if strings.Contains(tb.RawName, ".") && !strings.Contains(tb.RawName, "(") {
			if dotted := ResolveCompoundReceiverClass(tb.RawName, inScope, scopes, index, options, depth+1); dotted != nil {
				return dotted
			}
			if dottedCall := ResolveCompoundReceiverClass(tb.RawName+"()", inScope, scopes, index, options, depth+1); dottedCall != nil {
				return dottedCall
			}
		}

		// Callable alias (const user = getUser() → rawName getUser).
		if !strings.Contains(tb.RawName, ".") && !strings.Contains(tb.RawName, "(") {
			if callAlias := ResolveCompoundReceiverClass(tb.RawName+"()", inScope, scopes, index, options, depth+1); callAlias != nil {
				return callAlias
			}
		}

		// Compound member-call alias with both . and ().
		if strings.Contains(tb.RawName, ".") && strings.Contains(tb.RawName, "(") {
			if compound := ResolveCompoundReceiverClass(tb.RawName, inScope, scopes, index, options, depth+1); compound != nil {
				return compound
			}
		}
	}
	return FindClassBindingInScope(inScope, text, scopes)
}

// resolveCallExpr handles trailing `()` call expressions.
func resolveCallExpr(
	text string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
	options ResolveCompoundReceiverOptions,
	depth int,
	fieldFallback bool,
) *shared.SymbolDefinition {
	classScopeByDefId := index.ClassScopeByDefId

	openIdx := matchingOpenParen(text)
	if openIdx < 0 {
		return nil
	}
	fnExpr := strings.TrimSpace(text[:openIdx])
	if len(fnExpr) == 0 {
		return nil
	}

	lastDot := strings.LastIndex(fnExpr, ".")
	if lastDot < 0 {
		// Free call `name()`.
		_ = findExportedDefByName(fnExpr, inScope, scopes, index)
		retType := FindReceiverTypeBinding(inScope, fnExpr, scopes)
		if retType == nil {
			return nil
		}
		return FindClassBindingInScope(retType.DeclaredAtScope, retType.RawName, scopes)
	}

	// `obj.method()` — resolve obj's class, look up method return type via MRO.
	objExpr := fnExpr[:lastDot]
	methodName := fnExpr[lastDot+1:]
	objClass := ResolveCompoundReceiverClass(objExpr, inScope, scopes, index, options, depth+1)
	if objClass == nil {
		return nil
	}

	var retType *shared.TypeRef
	ownerChain := []shared.DefID{shared.DefID(objClass.NodeID)}
	if mro := scopes.MethodDispatch().MROByOwner(shared.DefID(objClass.NodeID)); mro != nil {
		for _, id := range mro {
			ownerChain = append(ownerChain, id)
		}
	}

	for _, ownerID := range ownerChain {
		cs := classScopeByDefId[string(ownerID)]
		if cs == nil {
			continue
		}
		if candidate := typeBindingAt(cs, methodName); candidate != nil {
			retType = candidate
			break
		}

		// Fallback: walk up to ancestor (Module) scopes for hoisted return-type bindings.
		if options.HoistTypeBindingsToModule && cs.Parent != nil {
			curID := cs.Parent
			for curID != nil {
				curScope := scopes.ScopeTree().GetScope(*curID)
				if curScope == nil {
					break
				}
				if cand := typeBindingAt(curScope, methodName); cand != nil {
					retType = cand
					break
				}
				curID = curScope.Parent
			}
			if retType != nil {
				break
			}
		}
	}

	// Field-fallback heuristic (Python-shaped).
	if retType == nil && fieldFallback {
		objCS := classScopeByDefId[objClass.NodeID]
		if objCS != nil {
			for _, fieldType := range objCS.TypeBindings {
				fieldClass := FindClassBindingInScope(fieldType.DeclaredAtScope, fieldType.RawName, scopes)
				if fieldClass == nil {
					continue
				}
				fcs := classScopeByDefId[fieldClass.NodeID]
				if fcs == nil {
					continue
				}
				if candidate := typeBindingAt(fcs, methodName); candidate != nil {
					retType = candidate
					break
				}
			}
		}
	}

	// Map<K,V>.values() heuristic.
	if retType == nil && methodName == "values" {
		mapVal := resolveMapValueTypeNameFromPrefix(objExpr, inScope, scopes, index, options)
		if mapVal != "" {
			retType = &shared.TypeRef{
				RawName:         mapVal,
				DeclaredAtScope: inScope,
				Source:          "return-annotation",
			}
		}
	}

	if retType == nil {
		return nil
	}
	return FindClassBindingInScope(retType.DeclaredAtScope, retType.RawName, scopes)
}

// resolveMixedChain handles mixed dotted + call chain expressions.
func resolveMixedChain(
	text string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
	options ResolveCompoundReceiverOptions,
	depth int,
	fieldFallback bool,
) *shared.SymbolDefinition {
	classScopeByDefId := index.ClassScopeByDefId
	parts := splitChainAtTopLevel(text)

	// Language-specific collection-accessor suffix (C# Dictionary<K,V>.Values, etc.)
	if options.UnwrapCollectionAccessor != nil && len(parts) >= 2 {
		last := parts[len(parts)-1]
		headInner := parts[0]
		if last != "" && headInner != "" {
			prefix := strings.Join(parts[:len(parts)-1], ".")
			var prefixType *shared.TypeRef

			if len(parts) == 2 {
				prefixType = FindReceiverTypeBinding(inScope, prefix, scopes)
			} else {
				cur := FindReceiverTypeBinding(inScope, headInner, scopes)
				for i := 1; i < len(parts)-1 && cur != nil; i++ {
					cls := FindClassBindingInScope(cur.DeclaredAtScope, cur.RawName, scopes)
					if cls == nil {
						cur = nil
						break
					}
					cs := classScopeByDefId[cls.NodeID]
					if cs == nil {
						cur = nil
						break
					}
					cur = typeBindingAt(cs, parts[i])
				}
				prefixType = cur
			}

			if prefixType != nil {
				elemName := options.UnwrapCollectionAccessor(prefixType.RawName, last)
				if elemName != "" {
					return FindClassBindingInScope(prefixType.DeclaredAtScope, elemName, scopes)
				}
			}
		}
	}

	head := parts[0]
	if head == "" {
		return nil
	}
	headMemberName := stripCallParens(head)
	headType := FindReceiverTypeBinding(inScope, headMemberName, scopes)

	var currentClass *shared.SymbolDefinition
	if headType != nil {
		currentClass = FindClassBindingInScope(headType.DeclaredAtScope, headType.RawName, scopes)
	} else {
		currentClass = FindClassBindingInScope(inScope, headMemberName, scopes)
	}

	// Callable alias fallback.
	if currentClass == nil && headType != nil &&
		!strings.Contains(headType.RawName, ".") &&
		!strings.Contains(headType.RawName, "(") {
		currentClass = ResolveCompoundReceiverClass(
			headType.RawName+"()", inScope, scopes, index, options, depth+1,
		)
	}

	for i := 1; i < len(parts) && currentClass != nil; i++ {
		segment := parts[i]
		if segment == "" {
			break
		}
		memberName := stripCallParens(segment)
		cs := classScopeByDefId[currentClass.NodeID]
		var memberType *shared.TypeRef
		if cs != nil {
			memberType = typeBindingAt(cs, memberName)
		}

		// Walk up to ancestor (Module) scopes for hoisted return-type bindings.
		if memberType == nil && options.HoistTypeBindingsToModule && cs != nil && cs.Parent != nil {
			curID := cs.Parent
			for curID != nil {
				curScope := scopes.ScopeTree().GetScope(*curID)
				if curScope == nil {
					break
				}
				if cand := typeBindingAt(curScope, memberName); cand != nil {
					memberType = cand
					break
				}
				curID = curScope.Parent
			}
		}

		if memberType == nil {
			if !strings.Contains(segment, "(") {
				prefix := strings.Join(parts[:i], ".")
				if asCall := ResolveCompoundReceiverClass(
					prefix+"."+memberName+"()", inScope, scopes, index, options, depth+1,
				); asCall != nil {
					return asCall
				}
			}
			return nil
		}

		var nextClass *shared.SymbolDefinition
		nextClass = FindClassBindingInScope(memberType.DeclaredAtScope, memberType.RawName, scopes)
		if nextClass == nil {
			fromMap := unwrapMapValueToClass(memberType, scopes)
			if fromMap != nil {
				nextClass = fromMap
			}
		}
		currentClass = nextClass
	}
	return currentClass
}

// splitChainAtTopLevel splits a chain expression at top-level `.` separators
// — i.e. `.` characters NOT nested inside balanced `(...)`, `[...]`, or `<...>`.
func splitChainAtTopLevel(text string) []string {
	var out []string
	depth := 0
	last := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '(' || ch == '[' || ch == '<' {
			depth++
		} else if ch == ')' || ch == ']' || ch == '>' {
			if depth > 0 {
				depth--
			}
		} else if ch == '.' && depth == 0 {
			out = append(out, text[last:i])
			last = i + 1
		}
	}
	out = append(out, text[last:])
	var filtered []string
	for _, s := range out {
		if len(s) > 0 {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// stripCallParens strips trailing `(...)` from a chain segment.
func stripCallParens(segment string) string {
	if !strings.HasSuffix(segment, ")") {
		return segment
	}
	open := strings.Index(segment, "(")
	if open < 0 {
		return segment
	}
	return segment[:open]
}

// matchingOpenParen finds the index of the `(` matching the trailing `)`.
func matchingOpenParen(text string) int {
	if !strings.HasSuffix(text, ")") {
		return -1
	}
	depth := 0
	for i := len(text) - 1; i >= 0; i-- {
		ch := text[i]
		if ch == ')' {
			depth++
		} else if ch == '(' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractShallowMapTypeArgByIndex returns the type argument at wantIndex from
// a shallow Map<K,V> or ReadonlyMap<K,V> type string.
func extractShallowMapTypeArgByIndex(mapText string, wantIndex int) string {
	t := strings.TrimSpace(mapText)
	var prefix string
	if strings.HasPrefix(t, "ReadonlyMap") {
		prefix = "ReadonlyMap"
	} else if strings.HasPrefix(t, "Map") {
		prefix = "Map"
	} else {
		return ""
	}

	rest := t[len(prefix):]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 || rest[0] != '<' {
		return ""
	}

	depth := 1
	var args []string
	segStart := 1

	for i := 1; i < len(rest); i++ {
		ch := rest[i]
		if ch == '<' {
			depth++
		} else if ch == '>' {
			depth--
			if depth == 0 {
				tail := strings.TrimSpace(rest[segStart:i])
				if len(tail) > 0 {
					args = append(args, tail)
				}
				break
			}
		} else if ch == ',' && depth == 1 {
			args = append(args, strings.TrimSpace(rest[segStart:i]))
			segStart = i + 1
		}
	}

	if wantIndex < len(args) {
		picked := strings.TrimSpace(args[wantIndex])
		if len(picked) > 0 {
			return picked
		}
	}
	return ""
}

// unwrapMapValueToClass resolves Map<K,V> rawName to the V class definition.
func unwrapMapValueToClass(memberType *shared.TypeRef, scopes *model.ScopeResolutionIndexes) *shared.SymbolDefinition {
	v := extractShallowMapTypeArgByIndex(memberType.RawName, 1)
	if v == "" {
		return nil
	}
	return FindClassBindingInScope(memberType.DeclaredAtScope, v, scopes)
}

// resolveMapValueTypeNameFromPrefix walks objExpr as a field chain and returns
// the V type name from a terminal Map<K,V> field binding.
func resolveMapValueTypeNameFromPrefix(
	objExpr string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
	options ResolveCompoundReceiverOptions,
) string {
	classScopeByDefId := index.ClassScopeByDefId
	parts := splitChainAtTopLevel(objExpr)
	if len(parts) == 0 {
		return ""
	}
	head := parts[0]
	headMemberName := stripCallParens(head)
	headType := FindReceiverTypeBinding(inScope, headMemberName, scopes)

	var currentClass *shared.SymbolDefinition
	if headType != nil {
		currentClass = FindClassBindingInScope(headType.DeclaredAtScope, headType.RawName, scopes)
	} else {
		currentClass = FindClassBindingInScope(inScope, headMemberName, scopes)
	}

	if currentClass == nil && headType != nil &&
		!strings.Contains(headType.RawName, ".") &&
		!strings.Contains(headType.RawName, "(") {
		currentClass = ResolveCompoundReceiverClass(
			headType.RawName+"()", inScope, scopes, index, options, 1,
		)
	}

	var lastMemberType *shared.TypeRef
	for i := 1; i < len(parts) && currentClass != nil; i++ {
		segment := parts[i]
		if segment == "" {
			break
		}
		memberName := stripCallParens(segment)
		cs := classScopeByDefId[currentClass.NodeID]
		if cs == nil {
			return ""
		}
		memberType := typeBindingAt(cs, memberName)

		if memberType == nil && options.HoistTypeBindingsToModule && cs.Parent != nil {
			curID := cs.Parent
			for curID != nil {
				curScope := scopes.ScopeTree().GetScope(*curID)
				if curScope == nil {
					break
				}
				if cand := typeBindingAt(curScope, memberName); cand != nil {
					memberType = cand
					break
				}
				curID = curScope.Parent
			}
		}

		if memberType == nil {
			return ""
		}
		lastMemberType = memberType
		var nextClass *shared.SymbolDefinition
		nextClass = FindClassBindingInScope(memberType.DeclaredAtScope, memberType.RawName, scopes)
		if nextClass == nil {
			fromMap := unwrapMapValueToClass(memberType, scopes)
			if fromMap != nil {
				nextClass = fromMap
			}
		}
		currentClass = nextClass
	}

	if lastMemberType == nil {
		return ""
	}
	return extractShallowMapTypeArgByIndex(lastMemberType.RawName, 1)
}

// findExportedDefByName looks up a function def by name accessible from scope.
func findExportedDefByName(
	name string,
	inScope shared.ScopeID,
	scopes *model.ScopeResolutionIndexes,
	index *WorkspaceResolutionIndex,
) *shared.SymbolDefinition {
	refs := LookupBindingsAt(inScope, name, scopes)
	for _, ref := range refs {
		if ref.Def.Type == shared.LabelFunction || ref.Def.Type == shared.LabelMethod {
			return &ref.Def
		}
	}
	def := index.ExportedCallableByName[name]
	if def != nil {
		return def
	}
	return nil
}

// EmitCompoundReceiverCalls resolves call sites with compound receivers
// (receivers containing `.` or `()`). This is the standalone version of
// Case 0 from EmitReceiverBoundCallsFull, useful when the caller only
// wants compound-receiver handling without the full 7-case dispatch.
// Returns the number of edges emitted.
//
// Mirrors TS receiver-bound-calls.ts Case 0 (compound receiver).
func EmitCompoundReceiverCalls(
	g shared.KnowledgeGraph,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
	handledSites map[string]bool,
	mro map[string][]string,
	sites []shared.ReferenceSite,
) int {
	if indexes == nil || handledSites == nil || len(sites) == 0 {
		return 0
	}

	emitted := 0
	seen := make(map[string]bool) // I5: never pre-seed

	compoundOpts := ResolveCompoundReceiverOptions{
		FieldFallback:             provider.FieldFallbackOnMethodLookup(),
		HoistTypeBindingsToModule: provider.HoistTypeBindingsToModule(),
	}
	if unwrapFn := provider.UnwrapCollectionAccessor(); unwrapFn != nil {
		compoundOpts.UnwrapCollectionAccessor = func(receiverType, accessor string) string {
			syntheticDef := &shared.SymbolDefinition{NodeID: accessor, Type: shared.LabelMethod}
			qn := receiverType + "." + accessor
			syntheticDef.QualifiedName = &qn
			result := unwrapFn(syntheticDef)
			if result == nil {
				return ""
			}
			if result.QualifiedName != nil {
				return *result.QualifiedName
			}
			return result.NodeID
		}
	}

	// Build workspace index for compound receiver resolution using
	// ScopeTree.AllScopes() + findClassLikeDef (same pattern as BuildWorkspaceResolutionIndex).
	wsIndex := &WorkspaceResolutionIndex{
		ClassScopeByDefId: make(map[string]*shared.Scope),
	}
	for _, scope := range indexes.ScopeTree().AllScopes() {
		if scope.Kind != shared.ScopeKindClass {
			continue
		}
		cd := findClassLikeDef(scope)
		if cd != nil {
			wsIndex.ClassScopeByDefId[cd.NodeID] = scope
		}
	}

	for i := range sites {
		site := &sites[i]
		if site.Kind != shared.ReferenceCall && site.Kind != shared.ReferenceRead && site.Kind != shared.ReferenceWrite {
			continue
		}
		if site.ExplicitReceiver == nil {
			continue
		}
		receiverName := *site.ExplicitReceiver
		if !strings.Contains(receiverName, ".") && !strings.Contains(receiverName, "(") {
			continue
		}

		memberName := site.SymbolName
		siteKey := site.FilePath + ":" + itoa(site.Range.StartLine) + ":" + itoa(site.Range.StartCol)
		if handledSites[siteKey] {
			continue
		}

		currentClass := ResolveCompoundReceiverClass(receiverName, site.InScope, indexes, wsIndex, compoundOpts, 0)
		if currentClass == nil {
			continue
		}

		chain := buildMROChain(currentClass.NodeID, indexes, mro)
		for _, ownerID := range chain {
			var picked pickResult
			if site.Kind == shared.ReferenceCall {
				picked = pickFirstNonStaticOnly(ownerID, memberName, *site, nil, provider)
			} else {
				picked = pickResult{Def: findOwnedMemberViaModel(ownerID, memberName, nil)}
			}
			if picked.Ambiguous {
				handledSites[siteKey] = true
				break
			}
			if picked.StaticOnlyFiltered || picked.Def == nil {
				continue
			}
			if suppressDeletedCallTarget(site, picked.Def) {
				handledSites[siteKey] = true
				break
			}
			reason := "global"
			if picked.Def.FilePath != site.FilePath {
				reason = "import-resolved"
			}
			si := SiteInfo{InScope: site.InScope, AtRange: site.Range, Kind: string(site.Kind)}
			if TryEmitEdge(g, indexes, lookup, si, picked.Def, reason, seen, 0.85, provider.CollapseMemberCallsByCallerTarget()) {
				handledSites[siteKey] = true
				emitted++
				break
			}
			handledSites[siteKey] = true
			break
		}
	}

	return emitted
}

// ResolveCompoundReceiver is the public wrapper that resolves a compound
// receiver expression (e.g. `user.address.save()`) to its target member
// definition via MRO walk. Returns a ReceiverMemberResolution with
// Kind "resolved" or "ambiguous", or nil if unresolvable.
//
// Mirrors TS resolveCompoundReceiver (used by receiver-bound-calls.ts Case 0
// when only the receiver resolution result, not the edge emission, is needed).
func ResolveCompoundReceiver(
	receiverName string,
	scopeID shared.ScopeID,
	provider ScopeResolver,
	lookup *GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
	mro map[string][]string,
) *ReceiverMemberResolution {
	if indexes == nil || lookup == nil {
		return nil
	}

	compoundOpts := ResolveCompoundReceiverOptions{
		FieldFallback:             provider.FieldFallbackOnMethodLookup(),
		HoistTypeBindingsToModule: provider.HoistTypeBindingsToModule(),
	}

	// Build a lightweight WorkspaceResolutionIndex with just ClassScopeByDefId
	// using ScopeTree.AllScopes() + findClassLikeDef (same pattern as BuildWorkspaceResolutionIndex).
	wsIndex := &WorkspaceResolutionIndex{
		ClassScopeByDefId: make(map[string]*shared.Scope),
	}
	for _, scope := range indexes.ScopeTree().AllScopes() {
		if scope.Kind != shared.ScopeKindClass {
			continue
		}
		cd := findClassLikeDef(scope)
		if cd != nil {
			wsIndex.ClassScopeByDefId[cd.NodeID] = scope
		}
	}

	// Split the receiver expression to extract the member name.
	// For `obj.method()`, the receiver is `obj` and member is `method`.
	// For compound `a.b.c()`, resolve the chain to find the class,
	// then look up the final member.
	parts := splitChainAtTopLevel(receiverName)
	if len(parts) == 0 {
		return nil
	}

	// The last part contains the member access; strip call parens to get the name
	lastPart := parts[len(parts)-1]
	memberName := stripCallParens(lastPart)

	// Resolve the receiver class — everything before the last segment
	var currentClass *shared.SymbolDefinition
	if len(parts) == 1 {
		// Single segment: `method()` or bare `name`
		// Resolve via type binding chain
		currentClass = ResolveCompoundReceiverClass(receiverName, scopeID, indexes, wsIndex, compoundOpts, 0)
		if currentClass == nil {
			return nil
		}
	} else {
		// Multi-segment: resolve prefix to get the receiver class
		prefix := strings.Join(parts[:len(parts)-1], ".")
		// If the last segment has call parens, also try resolving the full expression
		// to get the return type class
		if strings.Contains(lastPart, "(") {
			currentClass = ResolveCompoundReceiverClass(prefix, scopeID, indexes, wsIndex, compoundOpts, 0)
		} else {
			currentClass = ResolveCompoundReceiverClass(prefix, scopeID, indexes, wsIndex, compoundOpts, 0)
		}
		if currentClass == nil {
			return nil
		}
	}

	// Walk the MRO to find the member on the receiver class or its ancestors
	chain := buildMROChain(currentClass.NodeID, indexes, mro)
	for _, ownerID := range chain {
		cs := wsIndex.ClassScopeByDefId[ownerID]
		if cs == nil {
			continue
		}
		if tb := typeBindingAt(cs, memberName); tb != nil {
			def := FindClassBindingInScope(tb.DeclaredAtScope, tb.RawName, indexes)
			if def != nil {
				return &ReceiverMemberResolution{
					Kind:       "resolved",
					Definition: def,
				}
			}
		}
	}

	// Try the full compound receiver class resolution as a fallback —
	// resolves the entire expression including the member call to its
	// return-type class. Useful when the caller wants the type that
	// the compound expression produces.
	fullClass := ResolveCompoundReceiverClass(receiverName, scopeID, indexes, wsIndex, compoundOpts, 0)
	if fullClass != nil {
		return &ReceiverMemberResolution{
			Kind:       "resolved",
			Definition: fullClass,
		}
	}

	return nil
}
