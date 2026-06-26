package scope_resolution

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// walkers.go — scope-chain walkers, binding lookups, and helpers.
// Ported from TS scope-resolution/scope/walkers.ts (1076 lines).
// ---------------------------------------------------------------------------

// LookupBindingsAt merges bindings from 4 channels for a name at a scope,
// deduplicating by def.NodeID. Priority order:
//  1. finalized bindings (from indexes.bindings)
//  2. augmented bindings (from indexes.bindingAugmentations)
//  3. namespace-fqn bindings (from indexes.namespaceFqnBindings via accessibleNamespaces)
//  4. workspace-fqn bindings (from indexes.workspaceFqnBindings)
//
// Mirrors TS lookupBindingsAt.
func LookupBindingsAt(
	scopeID shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) []*shared.BindingRef {
	if indexes == nil {
		return nil
	}
	seen := make(map[string]bool) // dedup by def.NodeID
	var result []*shared.BindingRef

	push := func(refs []*shared.BindingRef) {
		for _, b := range refs {
			if !seen[b.Def.NodeID] {
				seen[b.Def.NodeID] = true
				result = append(result, b)
			}
		}
	}

	// Channel 1: finalized bindings
	if bindings := indexes.Bindings(); bindings != nil {
		if scopeBindings, ok := bindings[scopeID]; ok {
			if refs, ok := scopeBindings[name]; ok {
				push(refs)
			}
		}
	}

	// Channel 2: binding augmentations
	if augs := indexes.BindingAugmentations(); augs != nil {
		if scopeAugs, ok := augs[scopeID]; ok {
			if refs, ok := scopeAugs[name]; ok {
				push(refs)
			}
		}
	}

	// Channel 3: namespace-fqn bindings (via accessibleNamespacesByScope)
	if nsFqns := indexes.NamespaceFqnBindings(); nsFqns != nil {
		if namespaces := indexes.AccessibleNamespacesByScope(); namespaces != nil {
			if nsList, ok := namespaces[scopeID]; ok {
				for _, ns := range nsList {
					if nsBindings, ok := nsFqns[ns]; ok {
						if refs, ok := nsBindings[name]; ok {
							push(refs)
						}
					}
				}
			}
		}
	}

	// Channel 4: workspace-fqn bindings
	if wsFqns := indexes.WorkspaceFqnBindings(); wsFqns != nil {
		if refs, ok := wsFqns[name]; ok {
			push(refs)
		}
	}

	return result
}

// CollectNamespaceFqnBindings collects per-namespace bindings visible at a scope.
// Returns map of namespace → []BindingRef for the given name.
// Mirrors TS collectNamespaceFqnBindings.
func CollectNamespaceFqnBindings(
	scopeID shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) map[string][]*shared.BindingRef {
	result := make(map[string][]*shared.BindingRef)
	if indexes == nil {
		return result
	}
	nsFqns := indexes.NamespaceFqnBindings()
	if nsFqns == nil {
		return result
	}
	namespaces := indexes.AccessibleNamespacesByScope()
	if namespaces == nil {
		return result
	}
	nsList, ok := namespaces[scopeID]
	if !ok {
		return result
	}
	for _, ns := range nsList {
		if nsBindings, ok := nsFqns[ns]; ok {
			if refs, ok := nsBindings[name]; ok {
				result[ns] = refs
			}
		}
	}
	return result
}

// NamesAtScope returns all binding names visible at a scope from local and
// imported channels (NOT workspace channel). Used for wildcard expansion.
// Mirrors TS namesAtScope.
func NamesAtScope(scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) []string {
	if indexes == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string

	// Local bindings (from scope.bindings)
	scopeTree := indexes.ScopeTree()
	if scopeTree != nil {
		scope := scopeTree.GetScope(scopeID)
		if scope != nil {
			for name := range scope.Bindings {
				if !seen[name] {
					seen[name] = true
					result = append(result, name)
				}
			}
		}
	}

	// Finalized imported bindings
	if bindings := indexes.Bindings(); bindings != nil {
		if scopeBindings, ok := bindings[scopeID]; ok {
			for name := range scopeBindings {
				if !seen[name] {
					seen[name] = true
					result = append(result, name)
				}
			}
		}
	}

	// Augmented bindings
	if augs := indexes.BindingAugmentations(); augs != nil {
		if scopeAugs, ok := augs[scopeID]; ok {
			for name := range scopeAugs {
				if !seen[name] {
					seen[name] = true
					result = append(result, name)
				}
			}
		}
	}

	// Namespace-fqn bindings
	if nsFqns := indexes.NamespaceFqnBindings(); nsFqns != nil {
		if namespaces := indexes.AccessibleNamespacesByScope(); namespaces != nil {
			if nsList, ok := namespaces[scopeID]; ok {
				for _, ns := range nsList {
					if nsBindings, ok := nsFqns[ns]; ok {
						for name := range nsBindings {
							if !seen[name] {
								seen[name] = true
								result = append(result, name)
							}
						}
					}
				}
			}
		}
	}

	return result
}

// IsClassLike returns true for class-like node labels (Class/Interface/Struct/
// Record/Enum/Trait).
// Mirrors TS isClassLike.
func IsClassLike(label shared.NodeLabel) bool {
	switch label {
	case shared.LabelClass, shared.LabelInterface, shared.LabelStruct,
		shared.LabelRecord, shared.LabelEnum, shared.LabelTrait:
		return true
	}
	return false
}

// IsOwnableValueLabel returns true for labels that represent ownable value
// definitions (Const/Variable/Property/Static).
// Mirrors TS isOwnableValueLabel.
func IsOwnableValueLabel(label shared.NodeLabel) bool {
	switch label {
	case shared.LabelConst, shared.LabelVariable,
		shared.LabelProperty, shared.LabelStatic:
		return true
	}
	return false
}

// WalkScopeChain walks a scope chain upward from startScope, calling predicate
// at each step with both local bindings and imported bindings.
// Returns the first def for which predicate returns true.
// Mirrors TS walkScopeChain.
func WalkScopeChain(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
	predicate func(def *shared.SymbolDefinition) bool,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	visited := make(map[shared.ScopeID]bool)
	current := startScope

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}

		// Local bindings first
		if refs, ok := scope.Bindings[name]; ok {
			for i := range refs {
				if predicate(&refs[i].Def) {
					return &refs[i].Def
				}
			}
		}

		// Imported/augmented bindings
		imported := LookupBindingsAt(current, name, indexes)
		for _, b := range imported {
			if predicate(&b.Def) {
				return &b.Def
			}
		}

		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}
	return nil
}

// FindClassBindingInScope walks the scope chain looking for a class-like binding
// by name, with qualifiedNames single-match fallback and dotted-name simple-name fallback.
// Mirrors TS findClassBindingInScope.
func FindClassBindingInScope(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}

	// Primary: walkScopeChain(isClassLike)
	def := WalkScopeChain(startScope, name, indexes, func(d *shared.SymbolDefinition) bool {
		return IsClassLike(d.Type)
	})
	if def != nil {
		return def
	}

	// Fallback 1: qualifiedNames single-match — search all workspace bindings
	// for class-like defs whose qualifiedName ends with .name
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	qualifiedMatch := findQualifiedClassMatch(name, indexes)
	if qualifiedMatch != nil {
		return qualifiedMatch
	}

	// Fallback 2: dotted name — if name contains dots, try simple name (last segment)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		simpleName := name[idx+1:]
		def := WalkScopeChain(startScope, simpleName, indexes, func(d *shared.SymbolDefinition) bool {
			return IsClassLike(d.Type)
		})
		if def != nil {
			return def
		}
	}

	return nil
}

// findQualifiedClassMatch searches workspace-wide for a class-like def whose
// qualifiedName has `name` as its tail, returning it only if there's exactly one match.
func findQualifiedClassMatch(name string, indexes *model.ScopeResolutionIndexes) *shared.SymbolDefinition {
	var matches []*shared.SymbolDefinition

	// Search through all scopes for class-like defs matching the qualified name tail
	if indexes.ScopeTree() == nil {
		return nil
	}

	for _, scope := range indexes.ScopeTree().AllScopes() {
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			if !IsClassLike(def.Type) {
				continue
			}
			if def.QualifiedName != nil {
				qn := *def.QualifiedName
				// Exact match or tail match
				if qn == name {
					matches = append(matches, def)
					continue
				}
				if strings.HasSuffix(qn, "."+name) {
					matches = append(matches, def)
				}
			}
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

// FindReceiverTypeBinding finds the type binding for a receiver name by walking
// the scope chain, then falling back to namespace and workspace type bindings.
// Mirrors TS findReceiverTypeBinding.
func FindReceiverTypeBinding(
	startScope shared.ScopeID,
	receiverName string,
	indexes *model.ScopeResolutionIndexes,
) *shared.TypeRef {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	visited := make(map[shared.ScopeID]bool)
	current := startScope

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}

		// Local type bindings
		if tr, ok := scope.TypeBindings[receiverName]; ok {
			result := tr
			return &result
		}

		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}

	// Fallback: namespaceTypeBindingFor
	if tr := NamespaceTypeBindingFor(startScope, receiverName, indexes); tr != nil {
		return tr
	}

	// Fallback: workspaceTypeBindings
	if wsTypes := indexes.WorkspaceTypeBindings(); wsTypes != nil {
		if tr, ok := wsTypes[receiverName]; ok {
			return tr
		}
	}

	return nil
}

// NamespaceTypeBindingFor finds a type binding in the accessible namespaces for a scope.
// Mirrors TS namespaceTypeBindingFor.
func NamespaceTypeBindingFor(
	scopeID shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.TypeRef {
	if indexes == nil {
		return nil
	}
	nsTypes := indexes.NamespaceTypeBindings()
	if nsTypes == nil {
		return nil
	}
	namespaces := indexes.AccessibleNamespacesByScope()
	if namespaces == nil {
		return nil
	}
	nsList, ok := namespaces[scopeID]
	if !ok {
		return nil
	}
	for _, ns := range nsList {
		if nsTypeBindings, ok := nsTypes[ns]; ok {
			if tr, ok := nsTypeBindings[name]; ok {
				return tr // already *TypeRef
			}
		}
	}
	return nil
}

// ModuleScopeIdOf walks up the scope tree to find the enclosing Module scope ID.
// Mirrors TS moduleScopeIdOf.
func ModuleScopeIdOf(scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) shared.ScopeID {
	if indexes == nil {
		return ""
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return ""
	}

	visited := make(map[shared.ScopeID]bool)
	current := scopeID
	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}
		if scope.Kind == shared.ScopeKindModule {
			return scope.ID
		}
		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}
	return ""
}

// FindCallableBindingInScope walks the scope chain looking for a callable
// (Function/Method/Constructor) binding by name.
// Mirrors TS findCallableBindingInScope.
func FindCallableBindingInScope(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	return WalkScopeChain(startScope, name, indexes, func(d *shared.SymbolDefinition) bool {
		return isCallableLabel(d.Type)
	})
}

// FindAllCallableBindingsInScope collects all callable bindings for a name at
// the nearest scope that has any binding, preserving scope-walk boundary.
// Mirrors TS findAllCallableBindingsInScope.
func FindAllCallableBindingsInScope(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) []*shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	visited := make(map[shared.ScopeID]bool)
	current := startScope

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}

		var out []*shared.SymbolDefinition
		seen := make(map[string]bool)

		pushCallable := func(def *shared.SymbolDefinition) {
			if isCallableLabel(def.Type) && !seen[def.NodeID] {
				seen[def.NodeID] = true
				out = append(out, def)
			}
		}

		// Local bindings
		if refs, ok := scope.Bindings[name]; ok {
			for i := range refs {
				pushCallable(&refs[i].Def)
			}
		}

		// Imported bindings
		imported := LookupBindingsAt(current, name, indexes)
		for _, b := range imported {
			pushCallable(&b.Def)
		}

		if len(out) > 0 {
			return out
		}

		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}
	return nil
}

// FindCallableBindingsAndAdlBlocker finds callable bindings and detects ADL
// suppression (C++ argument-dependent lookup). Returns callables found at the
// nearest scope, whether a non-callable was present, and whether a block-scope
// declaration was found.
// Mirrors TS findCallableBindingsAndAdlBlocker.
type AdlBlockerResult struct {
	Callables           []*shared.SymbolDefinition
	NonCallableFound    bool
	BlockScopeDeclFound bool
}

func FindCallableBindingsAndAdlBlocker(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) AdlBlockerResult {
	if indexes == nil {
		return AdlBlockerResult{}
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return AdlBlockerResult{}
	}

	visited := make(map[shared.ScopeID]bool)
	current := startScope

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			return AdlBlockerResult{}
		}

		callables := []*shared.SymbolDefinition{}
		seen := make(map[string]bool)
		nonCallableFound := false
		anyBinding := false

		process := func(def *shared.SymbolDefinition) {
			anyBinding = true
			if isCallableLabel(def.Type) {
				if !seen[def.NodeID] {
					seen[def.NodeID] = true
					callables = append(callables, def)
				}
			} else {
				nonCallableFound = true
			}
		}

		// Local bindings
		if refs, ok := scope.Bindings[name]; ok {
			for i := range refs {
				process(&refs[i].Def)
			}
		}

		// Imported bindings
		imported := LookupBindingsAt(current, name, indexes)
		for _, b := range imported {
			process(&b.Def)
		}

		if anyBinding {
			blockScopeDeclFound := len(callables) > 0 &&
				(scope.Kind == shared.ScopeKindFunction || scope.Kind == shared.ScopeKindBlock)
			return AdlBlockerResult{
				Callables:           callables,
				NonCallableFound:    nonCallableFound,
				BlockScopeDeclFound: blockScopeDeclFound,
			}
		}

		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}
	return AdlBlockerResult{}
}

// PopulateClassOwnedMembers assigns OwnerID on every def structurally owned by
// a Class scope — methods (defs in Function scopes whose parent is Class) and
// class-body fields (defs directly in Class scopes). Also promotes qualifiedName
// from `methodName` to `ClassName.methodName`.
// Mirrors TS populateClassOwnedMembers.
func PopulateClassOwnedMembers(parsed *shared.ParsedFile) {
	if parsed == nil {
		return
	}

	scopesByID := make(map[shared.ScopeID]*shared.Scope)
	for _, scope := range parsed.Scopes {
		scopesByID[scope.ID] = scope
	}

	qualify := func(def *shared.SymbolDefinition, classDef *shared.SymbolDefinition) {
		q := def.QualifiedName
		if q == nil || *q == "" {
			return
		}
		if strings.Contains(*q, ".") {
			return // already qualified
		}
		classQ := classDef.QualifiedName
		if classQ == nil || *classQ == "" {
			return
		}
		newQ := *classQ + "." + *q
		def.QualifiedName = &newQ
	}

	for _, scope := range parsed.Scopes {
		// Methods: function scope whose parent is a Class scope
		if scope.Parent != nil {
			parentScope, ok := scopesByID[*scope.Parent]
			if ok && parentScope.Kind == shared.ScopeKindClass {
				classDef := findClassLikeDef(parentScope)
				if classDef != nil {
					for i := range scope.OwnedDefs {
						def := &scope.OwnedDefs[i]
						def.OwnerID = &classDef.NodeID
						qualify(def, classDef)
					}
				}
			}
		}
		// Class-body fields: defs directly owned by a Class scope
		if scope.Kind == shared.ScopeKindClass {
			classDef := findClassLikeDef(scope)
			if classDef != nil {
				for i := range scope.OwnedDefs {
					def := &scope.OwnedDefs[i]
					if def.NodeID == classDef.NodeID {
						continue
					}
					def.OwnerID = &classDef.NodeID
					qualify(def, classDef)
				}
			}
		}
	}
}

// findClassLikeDef finds the first class-like owned def in a scope.
func findClassLikeDef(scope *shared.Scope) *shared.SymbolDefinition {
	for i := range scope.OwnedDefs {
		if IsClassLike(scope.OwnedDefs[i].Type) {
			return &scope.OwnedDefs[i]
		}
	}
	return nil
}

// TagNamespacePrefixes tags every def declared inside one or more Namespace
// scopes with its enclosing-namespace path on a sidecar namespacePrefix field,
// WITHOUT touching qualifiedName.
// Mirrors TS tagNamespacePrefixes.
func TagNamespacePrefixes(parsed *shared.ParsedFile) {
	if parsed == nil {
		return
	}

	scopesByID := make(map[shared.ScopeID]*shared.Scope)
	for _, scope := range parsed.Scopes {
		scopesByID[scope.ID] = scope
	}

	// Compute the enclosing-namespace prefix for a scope
	namespacePrefixOf := func(scope *shared.Scope) string {
		var segments []string
		parentID := scope.Parent
		for parentID != nil {
			parent, ok := scopesByID[*parentID]
			if !ok {
				break
			}
			if parent.Kind == shared.ScopeKindNamespace {
				nsDef := findNamespaceDef(parent)
				if nsDef != nil && nsDef.QualifiedName != nil && *nsDef.QualifiedName != "" {
					nsQ := *nsDef.QualifiedName
					dot := strings.LastIndex(nsQ, ".")
					if dot == -1 {
						segments = append([]string{nsQ}, segments...)
					} else {
						segments = append([]string{nsQ[dot+1:]}, segments...)
					}
				}
			}
			parentID = parent.Parent
		}
		return strings.Join(segments, ".")
	}

	// Tag non-Namespace scope defs with enclosing-namespace prefix
	for _, scope := range parsed.Scopes {
		if scope.Kind == shared.ScopeKindNamespace {
			continue
		}
		prefix := namespacePrefixOf(scope)
		if prefix == "" {
			continue
		}
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			q := def.QualifiedName
			if q == nil || *q == "" {
				continue
			}
			if *q == prefix || strings.HasPrefix(*q, prefix+".") {
				continue // already namespaced
			}
			def.NamespacePrefix = &prefix
		}
	}

	// Also tag defs declared DIRECTLY in a Namespace scope with that
	// namespace's OWN full path.
	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindNamespace {
			continue
		}
		ownNsDef := findNamespaceDef(scope)
		if ownNsDef == nil || ownNsDef.QualifiedName == nil || *ownNsDef.QualifiedName == "" {
			continue
		}
		ownQ := *ownNsDef.QualifiedName
		ownTail := ownQ
		if idx := strings.LastIndex(ownQ, "."); idx >= 0 {
			ownTail = ownQ[idx+1:]
		}
		parentPrefix := namespacePrefixOf(scope)
		var fullPrefix string
		if parentPrefix != "" {
			fullPrefix = parentPrefix + "." + ownTail
		} else {
			fullPrefix = ownTail
		}
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			if def.Type == shared.LabelNamespace {
				continue
			}
			q := def.QualifiedName
			if q == nil || *q == "" {
				continue
			}
			if *q == fullPrefix || strings.HasPrefix(*q, fullPrefix+".") {
				continue
			}
			if def.NamespacePrefix != nil {
				continue
			}
			def.NamespacePrefix = &fullPrefix
		}
	}
}

// findNamespaceDef finds the first Namespace-type owned def in a scope.
func findNamespaceDef(scope *shared.Scope) *shared.SymbolDefinition {
	for i := range scope.OwnedDefs {
		if scope.OwnedDefs[i].Type == shared.LabelNamespace {
			return &scope.OwnedDefs[i]
		}
	}
	return nil
}

// FindEnclosingClassDef walks up the scope chain to find the innermost
// enclosing Class scope and returns that class's def.
// Mirrors TS findEnclosingClassDef.
func FindEnclosingClassDef(
	startScope shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	visited := make(map[shared.ScopeID]bool)
	current := startScope
	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			return nil
		}
		if scope.Kind == shared.ScopeKindClass {
			cd := findClassLikeDef(scope)
			if cd != nil {
				return cd
			}
		}
		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}
	return nil
}

// FindExportedDefByName finds a free-function def by simple name, preferring
// scope-chain-visible bindings before falling back to workspace-wide scan.
// Mirrors TS findExportedDefByName.
func FindExportedDefByName(
	name string,
	inScope shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	wsIndex *WorkspaceResolutionIndex,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	visited := make(map[shared.ScopeID]bool)
	current := inScope

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}

		// Local bindings
		if refs, ok := scope.Bindings[name]; ok {
			for i := range refs {
				if refs[i].Def.Type == shared.LabelFunction || refs[i].Def.Type == shared.LabelMethod {
					return &refs[i].Def
				}
			}
		}

		// Imported/augmented bindings
		finalized := LookupBindingsAt(current, name, indexes)
		for _, b := range finalized {
			if b.Def.Type == shared.LabelFunction || b.Def.Type == shared.LabelMethod {
				return &b.Def
			}
		}

		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}

	// Workspace-wide fallback
	if wsIndex != nil {
		if def, ok := wsIndex.ExportedCallableByName[name]; ok {
			return def
		}
	}

	return nil
}

// FindExportedDef finds a file-level def by simple name in the target file's
// Module scope's finalized bindings. Only defs bound at module-scope with
// origin=local qualify.
// Mirrors TS findExportedDef.
func FindExportedDef(
	targetFile string,
	memberName string,
	wsIndex *WorkspaceResolutionIndex,
) *shared.SymbolDefinition {
	if wsIndex == nil {
		return nil
	}
	moduleScope, ok := wsIndex.ModuleScopeByFile[targetFile]
	if !ok {
		return nil
	}
	refs, ok := moduleScope.Bindings[memberName]
	if !ok {
		return nil
	}
	for i := range refs {
		if refs[i].Origin == shared.OriginLocal {
			return &refs[i].Def
		}
	}
	return nil
}

// FindValueBindingInScope walks the scope chain looking for an ownable-value
// binding (Const/Variable/Property/Static).
// Mirrors TS findValueBindingInScope.
func FindValueBindingInScope(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	return WalkScopeChain(startScope, name, indexes, func(d *shared.SymbolDefinition) bool {
		return IsOwnableValueLabel(d.Type)
	})
}

// ResolveInheritanceBaseInScope resolves a base class name by combining
// qualified lookup, class binding search, and import-based disambiguation.
// Mirrors TS resolveInheritanceBaseInScope.
func ResolveInheritanceBaseInScope(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}

	// Try qualified inheritance base first
	def := resolveQualifiedInheritanceBase(startScope, name, indexes)
	if def != nil {
		return def
	}

	// Try class binding in scope
	def = FindClassBindingInScope(startScope, name, indexes)
	if def != nil {
		return def
	}

	// Try ambiguous resolution via imports
	def = resolveAmbiguousInheritanceBaseViaImports(startScope, name, indexes)
	if def != nil {
		return def
	}

	return nil
}

// resolveQualifiedInheritanceBase tries progressive prefix matching with
// namespace prefix disambiguation.
// Mirrors TS resolveQualifiedInheritanceBase.
func resolveQualifiedInheritanceBase(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	// Get the deriving class's enclosing scope segments for prefix matching
	segments := enclosingScopeSegments(startScope, indexes)

	// Try progressively longer prefixes
	for i := len(segments); i >= 1; i-- {
		prefix := strings.Join(segments[:i], ".")
		candidate := prefix + "." + name
		def := WalkScopeChain(startScope, candidate, indexes, func(d *shared.SymbolDefinition) bool {
			return IsClassLike(d.Type)
		})
		if def != nil {
			return def
		}
	}

	// Try with namespacePrefix from scope defs
	visited := make(map[shared.ScopeID]bool)
	current := startScope
	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			if def.NamespacePrefix != nil && *def.NamespacePrefix != "" {
				candidate := *def.NamespacePrefix + "." + name
				found := WalkScopeChain(startScope, candidate, indexes, func(d *shared.SymbolDefinition) bool {
					return IsClassLike(d.Type)
				})
				if found != nil {
					return found
				}
			}
		}
		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}

	return nil
}

// enclosingScopeSegments returns the qualified-name segments of the deriving
// class's enclosing scope (e.g. ["Outer", "Inner"] from "Outer.Inner").
// Mirrors TS enclosingScopeSegments.
func enclosingScopeSegments(scopeID shared.ScopeID, indexes *model.ScopeResolutionIndexes) []string {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	classDef := FindEnclosingClassDef(scopeID, indexes)
	if classDef == nil || classDef.QualifiedName == nil {
		return nil
	}
	qn := *classDef.QualifiedName
	parts := strings.Split(qn, ".")
	if len(parts) <= 1 {
		return nil
	}
	// Return all segments except the last (the class's own name)
	return parts[:len(parts)-1]
}

// resolveAmbiguousInheritanceBaseViaImports resolves an ambiguous base class
// name by checking import edges for exact-file and same-directory matches.
// Mirrors TS resolveAmbiguousInheritanceBaseViaImports.
func resolveAmbiguousInheritanceBaseViaImports(
	startScope shared.ScopeID,
	name string,
	indexes *model.ScopeResolutionIndexes,
) *shared.SymbolDefinition {
	if indexes == nil {
		return nil
	}
	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return nil
	}

	// Find the module scope for the starting scope
	moduleID := ModuleScopeIdOf(startScope, indexes)
	if moduleID == "" {
		return nil
	}
	moduleScope := scopeTree.GetScope(moduleID)
	if moduleScope == nil {
		return nil
	}

	// Collect all class-like defs matching name across workspace
	var candidates []*shared.SymbolDefinition
	for _, scope := range scopeTree.AllScopes() {
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			if !IsClassLike(def.Type) {
				continue
			}
			if def.QualifiedName != nil {
				qn := *def.QualifiedName
				simple := qn
				if idx := strings.LastIndex(qn, "."); idx >= 0 {
					simple = qn[idx+1:]
				}
				if simple == name {
					candidates = append(candidates, def)
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Layer 1: exact file match via imports
	imports := indexes.Imports()
	if imports != nil {
		if importEdges, ok := imports[moduleID]; ok {
			for _, edge := range importEdges {
				if edge.TargetFile == nil {
					continue
				}
				targetFile := *edge.TargetFile
				for _, c := range candidates {
					if c.FilePath == targetFile {
						return c
					}
				}
			}
		}
	}

	// Layer 2: same directory match
	moduleFilePath := moduleScope.FilePath
	moduleDir := moduleFilePath
	if idx := strings.LastIndex(moduleFilePath, "/"); idx >= 0 {
		moduleDir = moduleFilePath[:idx]
	}

	for _, c := range candidates {
		cDir := c.FilePath
		if idx := strings.LastIndex(c.FilePath, "/"); idx >= 0 {
			cDir = c.FilePath[:idx]
		}
		if cDir == moduleDir {
			return c
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Walk helpers — iterate over scopes, definitions, callsites, references.
// ---------------------------------------------------------------------------

// WalkScopes walks the scope tree depth-first and calls the visitor function
// for each scope. The visitor receives the scope and its depth in the tree.
// Mirrors TS walkScopes.
func WalkScopes(
	indexes *model.ScopeResolutionIndexes,
	visitor func(scopeID shared.ScopeID, depth int),
) {
	if indexes == nil || indexes.ScopeTree() == nil {
		return
	}
	scopeTree := indexes.ScopeTree()
	var walk func(scopeID shared.ScopeID, depth int)
	walk = func(scopeID shared.ScopeID, depth int) {
		visitor(scopeID, depth)
		for _, child := range scopeTree.Children(scopeID) {
			walk(child.ID, depth+1)
		}
	}
	for _, root := range scopeTree.Roots() {
		walk(root.ID, 0)
	}
}

// WalkDefinitions walks all symbol definitions across all scopes and calls
// the visitor for each one.
// Mirrors TS walkDefinitions.
func WalkDefinitions(
	indexes *model.ScopeResolutionIndexes,
	visitor func(def *shared.SymbolDefinition, scopeID shared.ScopeID),
) {
	if indexes == nil || indexes.ScopeTree() == nil {
		return
	}
	for _, scope := range indexes.ScopeTree().AllScopes() {
		for i := range scope.OwnedDefs {
			visitor(&scope.OwnedDefs[i], scope.ID)
		}
	}
}

// WalkCallsites walks all callsites in reference sites and calls the visitor.
// Mirrors TS walkCallsites.
func WalkCallsites(
	indexes *model.ScopeResolutionIndexes,
	visitor func(ref *shared.ReferenceSite, scopeID shared.ScopeID),
) {
	if indexes == nil {
		return
	}
	for _, ref := range indexes.ReferenceSites() {
		visitor(ref, "")
	}
}

// WalkReferences walks all reference sites and calls the visitor for each one.
// Mirrors TS walkReferences.
func WalkReferences(
	indexes *model.ScopeResolutionIndexes,
	visitor func(ref *shared.ReferenceSite, scopeID shared.ScopeID),
) {
	if indexes == nil {
		return
	}
	for _, ref := range indexes.ReferenceSites() {
		visitor(ref, "")
	}
}
