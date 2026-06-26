package scope_resolution

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// ids.go — ResolveDefGraphID + ResolveCallerGraphID
// Ported from TS scope-resolution/graph-bridge/ids.ts (270 lines).
// ---------------------------------------------------------------------------

// ResolveDefGraphID turns a scope-resolution SymbolDefinition into the
// graph's node ID for the corresponding legacy node.
//
// Lookup strategy (6+ levels, mirrors TS resolveDefGraphId exactly):
//  1. templateConstraints: qualifiedKey(filePath, type, qn+tag)
//  2. parameterShape:      qualifiedKey(filePath, type, qn+shapeTag)
//  3. parameterTypes:      qualifiedKey(filePath, type, qn~types.join(','))
//  4. arity:               qualifiedKey(filePath, type, qn#count)
//  5. templateArguments:   qualifiedKey(filePath, type, qn~args.join(','))
//  6. qualifiedName:       qualifiedKey(filePath, type, qn)
//  7. namespacePrefix:     qualifiedKey(filePath, type, nsPrefix.qn)
//  8. simpleName:          simpleKey(filePath, simpleName)
func ResolveDefGraphID(filePath string, def *shared.SymbolDefinition, lookup *GraphNodeLookup) string {
	if def == nil {
		return ""
	}
	label := def.Type

	// If no qualified name, fall through to simple lookup
	if def.QualifiedName == nil {
		return resolveSimpleFallback(filePath, def, lookup)
	}
	qn := *def.QualifiedName

	// Level 1: templateConstraints
	// If def has template constraints (stored in Extra), try qualified key with constraint tag
	if len(def.ParameterTypeClasses) > 0 {
		tag := constraintTag(def.ParameterTypeClasses)
		if tag != "" {
			candidate := QualifiedKeyWithFile(filePath, label, qn+tag)
			if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
				return nodes[0].ID
			}
		}
	}

	// Level 2: parameterShape — try each ParameterTypeClass as a shape discriminator
	for _, ptc := range def.ParameterTypeClasses {
		if ptc.Indirection != shared.IndirectionValue && ptc.Indirection != shared.IndirectionUnknown ||
			ptc.CV != shared.CVNone && ptc.CV != shared.CVUnknown {
			stag := shapeTag(ptc.CV, ptc.Indirection, ptc.PointerDepth)
			if stag != "" {
				candidate := QualifiedKeyWithFile(filePath, label, qn+stag)
				if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
					return nodes[0].ID
				}
			}
		}
	}

	// Level 3: parameterTypes — qualifiedKey with qn~types.join(',')
	if len(def.ParameterTypes) > 0 {
		typesTag := "~" + strings.Join(def.ParameterTypes, ",")
		candidate := QualifiedKeyWithFile(filePath, label, qn+typesTag)
		if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	// Level 4: arity — qualifiedKey with qn#count
	if def.ParameterCount != nil {
		arityTag := fmt.Sprintf("%s#%d", qn, *def.ParameterCount)
		candidate := QualifiedKeyWithFile(filePath, label, arityTag)
		if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	// Level 5: templateArguments — qualifiedKey with qn~args.join(',')
	if len(def.TemplateArguments) > 0 {
		argsTag := qn + "~" + strings.Join(def.TemplateArguments, ",")
		candidate := QualifiedKeyWithFile(filePath, label, argsTag)
		if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	// Level 6: plain qualified name
	candidate := QualifiedKeyWithFile(filePath, label, qn)
	if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
		return nodes[0].ID
	}
	// Also try without filePath prefix (some nodes may not have file-scoped QN)
	if nodes := lookup.LookupByQualified(label, qn); len(nodes) > 0 {
		return nodes[0].ID
	}

	// Level 7: namespacePrefix — try with namespace prefix if different from qn
	simple := SimpleQualifiedName(def)
	if simple != qn {
		// Try the simple part as a qualified name
		candidate = QualifiedKeyWithFile(filePath, label, simple)
		if nodes := lookup.LookupByQualified(label, candidate); len(nodes) > 0 {
			return nodes[0].ID
		}
		if nodes := lookup.LookupByQualified(label, simple); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	// Level 8: simple name fallback
	return resolveSimpleFallback(filePath, def, lookup)
}

// resolveSimpleFallback handles the final fallback when qualified name lookups fail.
func resolveSimpleFallback(filePath string, def *shared.SymbolDefinition, lookup *GraphNodeLookup) string {
	label := def.Type

	// Try node ID directly
	if node := lookup.LookupByID(def.NodeID); node != nil {
		return node.ID
	}

	// Try by simple name
	simple := SimpleQualifiedName(def)
	if simple != "" {
		// With filePath context
		keyWithFile := fmt.Sprintf("%s:%s", filePath, simple)
		if nodes := lookup.LookupBySimple(label, keyWithFile); len(nodes) > 0 {
			return nodes[0].ID
		}
		// Without filePath
		if nodes := lookup.LookupBySimple(label, simple); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	// Last resort: try node ID as simple key
	if def.NodeID != "" {
		if nodes := lookup.LookupBySimple(label, def.NodeID); len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	return ""
}

// SimpleQualifiedName extracts the simple (short) name from a definition.
// If qualifiedName exists, returns the last segment after the last dot.
// Otherwise returns the nodeID.
func SimpleQualifiedName(def *shared.SymbolDefinition) string {
	if def == nil {
		return ""
	}
	if def.QualifiedName != nil {
		qn := *def.QualifiedName
		if idx := strings.LastIndex(qn, "."); idx >= 0 {
			return qn[idx+1:]
		}
		return qn
	}
	return def.NodeID
}

// QualifiedKeyWithFile builds a qualified lookup key with filePath context.
// Format: filePath:label:qualifiedName (mirrors TS qualifiedKey with filePath param).
func QualifiedKeyWithFile(filePath string, label shared.NodeLabel, qualifiedName string) string {
	if filePath != "" {
		return filePath + ":" + string(label) + ":" + qualifiedName
	}
	return string(label) + ":" + qualifiedName
}

// ResolveCallerGraphID walks a scope chain from a starting scope upward
// to find the enclosing function/method/class and returns its graph-node ID.
// Falls back to the File node for module-level calls.
//
// Algorithm (mirrors TS resolveCallerGraphId):
//  1. Walk scope chain upward (visited set prevents cycles)
//  2. At each scope: pickCallerCallableDef finds Function/Method/Constructor
//     → ResolveDefGraphID
//  3. Fallback: scope.ownedDefs with isCallerAnchorLabel → ResolveDefGraphID
//  4. Final fallback: generateId('File', lastFilePath)
func ResolveCallerGraphID(
	startScope shared.ScopeID,
	indexes *model.ScopeResolutionIndexes,
	lookup *GraphNodeLookup,
	atRange *shared.Range,
) string {
	if indexes == nil || indexes.ScopeTree() == nil {
		return ""
	}

	scopeTree := indexes.ScopeTree()
	visited := make(map[shared.ScopeID]bool)
	current := startScope
	var lastFilePath string

	for current != "" && !visited[current] {
		visited[current] = true
		scope := scopeTree.GetScope(current)
		if scope == nil {
			break
		}
		lastFilePath = scope.FilePath

		// Primary: pickCallerCallableDef — find Function/Method/Constructor
		// in child scopes that contain the reference position
		if callableDef := pickCallerCallableDef(scope, scopeTree, atRange); callableDef != nil {
			if graphID := ResolveDefGraphID(callableDef.FilePath, callableDef, lookup); graphID != "" {
				return graphID
			}
		}

		// Fallback: any owned def with caller-anchor label
		for i := range scope.OwnedDefs {
			def := &scope.OwnedDefs[i]
			if IsCallerAnchorLabel(def.Type) {
				if graphID := ResolveDefGraphID(def.FilePath, def, lookup); graphID != "" {
					return graphID
				}
			}
		}

		// Walk up
		if scope.Parent == nil {
			break
		}
		current = *scope.Parent
	}

	// Final fallback: File node
	if lastFilePath != "" {
		return shared.GenerateID("File", lastFilePath)
	}
	return ""
}

// pickCallerCallableDef searches for a Function/Method/Constructor callable
// in child scopes that contain the reference position.
//
// Algorithm (mirrors TS pickCallerCallableDef):
//  1. Iterate child scopes (Function kind) whose range contains the reference point
//  2. Check their ownedDefs for Function/Method/Constructor
//  3. Fallback: check the current scope's own ownedDefs for callable labels
func pickCallerCallableDef(
	scope *shared.Scope,
	scopeTree *shared.ScopeTree,
	atRange *shared.Range,
) *shared.SymbolDefinition {
	if scope == nil {
		return nil
	}

	// Search child scopes for Function-kind scopes containing the reference
	children := scopeTree.Children(scope.ID)
	for _, child := range children {
		if child.Kind != shared.ScopeKindFunction {
			continue
		}
		// If we have an atRange, check containment
		if atRange != nil && !rangeContainsPoint(child.Range, *atRange) {
			continue
		}
		// Check child's owned defs for callable
		for i := range child.OwnedDefs {
			def := &child.OwnedDefs[i]
			if isCallableLabel(def.Type) {
				return def
			}
		}
	}

	// Fallback: check the scope's own owned defs for callable
	for i := range scope.OwnedDefs {
		def := &scope.OwnedDefs[i]
		if isCallableLabel(def.Type) {
			return def
		}
	}

	return nil
}

// IsCallerAnchorLabel returns true if the label can be a caller (source of a
// CALLS/ACCESSES edge). Variables/Properties cannot be callers.
func IsCallerAnchorLabel(label shared.NodeLabel) bool {
	switch label {
	case shared.LabelFunction, shared.LabelMethod, shared.LabelConstructor,
		shared.LabelClass, shared.LabelInterface, shared.LabelStruct,
		shared.LabelEnum:
		return true
	}
	return false
}

// isCallableLabel returns true for Function/Method/Constructor — the labels
// that represent callable definitions.
func isCallableLabel(label shared.NodeLabel) bool {
	switch label {
	case shared.LabelFunction, shared.LabelMethod, shared.LabelConstructor:
		return true
	}
	return false
}

// rangeContainsPoint checks whether a scope range contains a reference point.
func rangeContainsPoint(scopeRange shared.Range, point shared.Range) bool {
	if scopeRange.StartLine < point.StartLine {
		if scopeRange.EndLine > point.EndLine {
			return true
		}
		if scopeRange.EndLine == point.EndLine && scopeRange.EndCol >= point.EndCol {
			return true
		}
	}
	if scopeRange.StartLine == point.StartLine && scopeRange.StartCol <= point.StartCol {
		if scopeRange.EndLine > point.EndLine {
			return true
		}
		if scopeRange.EndLine == point.EndLine && scopeRange.EndCol >= point.EndCol {
			return true
		}
	}
	return false
}

// constraintTag builds a tag string from parameter type classes for lookup.
func constraintTag(classes []shared.ParameterTypeClass) string {
	if len(classes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		parts = append(parts, c.Base)
	}
	return "<" + strings.Join(parts, ",") + ">"
}

// shapeTag builds a tag from CV qualifiers and indirection kind.
func shapeTag(cv shared.CVQualifier, indirection shared.IndirectionKind, depth int) string {
	var b strings.Builder
	if cv != shared.CVNone {
		b.WriteString(string(cv))
	}
	switch indirection {
	case shared.IndirectionPointer:
		b.WriteString("*")
		if depth > 1 {
			b.WriteString(fmt.Sprintf("%d", depth))
		}
	case shared.IndirectionLValueRef:
		b.WriteString("&")
	case shared.IndirectionRValueRef:
		b.WriteString("&&")
	}
	return b.String()
}