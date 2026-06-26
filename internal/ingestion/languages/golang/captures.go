// Package golang — Go scope capture emission.
// Runs tree-sitter queries against Go source to extract scope boundaries,
// definitions, imports, and type bindings.
// Ported from TS languages/go/captures.ts.
package golang

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// CaptureMatch represents a grouped set of captures for one query match.
// Maps capture tag (e.g., "@scope.function") → shared.Capture.
// Alias to shared.CaptureMatch for type compatibility across the package.
type CaptureMatch = shared.CaptureMatch

// EmitGoScopeCaptures runs the Go tree-sitter query against source and
// returns capture matches representing scopes, definitions, imports,
// and type bindings found in the file.
//
// Mirrors TS emitGoScopeCaptures(source, filePath).
func EmitGoScopeCaptures(source []byte, filePath string) []CaptureMatch {
	lang := grammars.GoLanguage()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil || root.NamedChildCount() == 0 {
		return nil
	}

	query := GoScopeQueryCompiled()
	if query == nil {
		return nil
	}

	matches := query.ExecuteNode(root, lang, source)
	if len(matches) == 0 {
		return nil
	}

	var out []CaptureMatch

	// We need a nodeMap per match to access AST nodes for synthesis.
	// nodeMap maps capture tag → *gotreesitter.Node for local walks.
	type nodeMapEntry struct {
		node *gotreesitter.Node
	}

	for _, m := range matches {
		grouped := make(CaptureMatch)
		nodeMap := make(map[string]*gotreesitter.Node)

		for _, qc := range m.Captures {
			tag := "@" + qc.Name
			if strings.HasPrefix(qc.Name, "_") {
				continue // skip anonymous captures
			}
			grouped[tag] = utils.NodeToCapture(tag, qc.Node, source)
			nodeMap[tag] = qc.Node
		}

		if len(grouped) == 0 {
			continue
		}

		// Handle import statements: split grouped imports into individual ParsedImports
		if _, hasImport := grouped["@import.statement"]; hasImport {
			importNode := nodeMap["@import.statement"]
			if importNode != nil {
				// Walk parent chain for import_declaration at same range
				resolvedNode := resolveImportNode(lang, importNode)
				splitCaptures := SplitGoImportStatementFromNode(lang, resolvedNode, source)
				if len(splitCaptures) > 0 {
					out = append(out, splitCaptures...)
					continue
				}
			}
		}

		// Synthesize receiver binding for method declarations
		if scopeNode, hasScopeFn := nodeMap["@scope.function"]; hasScopeFn {
			if scopeNode.Type(lang) == "method_declaration" {
				receiver := SynthesizeGoReceiverBindingFromNode(lang, scopeNode, source)
				if receiver != nil {
					out = append(out, receiver)
				}
			}
		}

		// Skip raw multi-assign type bindings (they are synthesized separately)
		if isRawMultiAssignTypeBinding(lang, nodeMap) {
			continue
		}

		// Normalize generic constructor captures
		normalizeGenericConstructorCapture(lang, nodeMap, grouped, source)

		// Enrich function/method declarations with arity metadata
		declAnchorNode := nodeMap["@declaration.function"]
		if declAnchorNode == nil {
			declAnchorNode = nodeMap["@declaration.method"]
		}
		if declAnchorNode != nil {
			nt := declAnchorNode.Type(lang)
			if nt == "function_declaration" || nt == "method_declaration" || nt == "method_elem" {
				arity := ComputeGoDeclarationArityFromNode(lang, declAnchorNode, source)
				if arity.ParameterCount > 0 {
					grouped["@declaration.parameter-count"] = utils.SyntheticCapture(
						"@declaration.parameter-count", declAnchorNode,
						fmt.Sprintf("%d", arity.ParameterCount),
					)
				}
				if arity.RequiredParameterCount > 0 {
					grouped["@declaration.required-parameter-count"] = utils.SyntheticCapture(
						"@declaration.required-parameter-count", declAnchorNode,
						fmt.Sprintf("%d", arity.RequiredParameterCount),
					)
				}
				if len(arity.ParameterTypes) > 0 {
					grouped["@declaration.parameter-types"] = utils.SyntheticCapture(
						"@declaration.parameter-types", declAnchorNode,
						fmt.Sprintf("[%s]", strings.Join(arity.ParameterTypes, ",")),
					)
				}
				if arity.ReturnType != "" {
					grouped["@declaration.return-type"] = utils.SyntheticCapture(
						"@declaration.return-type", declAnchorNode,
						arity.ReturnType,
					)
				}
			}
			out = append(out, grouped)
			continue
		}

		// Enrich call references with arity
		if _, hasArity := grouped["@reference.arity"]; !hasArity {
			callNode := nodeMap["@reference.call.free"]
			if callNode == nil {
				callNode = nodeMap["@reference.call.member"]
			}
			if callNode == nil {
				callNode = nodeMap["@reference.call.constructor"]
			}
			if callNode != nil && (callNode.Type(lang) == "call_expression" || callNode.Type(lang) == "composite_literal") {
				callArity := ComputeGoCallArityFromNode(lang, callNode)
				grouped["@reference.arity"] = utils.SyntheticCapture(
					"@reference.arity", callNode,
					fmt.Sprintf("%d", callArity),
				)
			}
		}

		out = append(out, grouped)
	}

	// Layer on type-binding synthesis (new/make/qualified composite literal, etc.)
	synthesized := SynthesizeGoTypeBindingsFromRoot(lang, root, source)
	out = append(out, synthesized...)

	// Synthesize typeBindings for struct fields
	for _, match := range out {
		if _, hasField := match["@declaration.field"]; !hasField {
			continue
		}
		nameCap, hasName := match["@declaration.name"]
		typeCap, hasType := match["@declaration.field-type"]
		if !hasName || !hasType {
			continue
		}
		out = append(out, CaptureMatch{
			"@type-binding.field": typeCap,
			"@type-binding.name":  nameCap,
			"@type-binding.type": shared.Capture{
				Name:  "@type-binding.type",
				Text:  typeCap.Text,
				Range: typeCap.Range,
			},
		})
	}

	// Synthesize inheritance references (struct embedding / interface embedding)
	inheritCaptures := synthesizeGoInheritanceReferences(lang, root, source)
	out = append(out, inheritCaptures...)

	return out
}

// resolveImportNode walks the parent chain for an import_declaration whose
// range equals the import_spec's; otherwise returns the spec itself.
// Mirrors TS resolveImportNode.
func resolveImportNode(lang *gotreesitter.Language, importSpec *gotreesitter.Node) *gotreesitter.Node {
	current := importSpec.Parent()
	for current != nil {
		if current.Type(lang) == "import_declaration" {
			if nodeRangeEquals(current, importSpec) {
				return current
			}
			break
		}
		// import_spec is nested at most under import_declaration ->
		// import_spec_list -> import_spec
		if current.Type(lang) != "import_spec_list" {
			break
		}
		current = current.Parent()
	}
	return importSpec
}

// nodeRangeEquals returns true iff two nodes occupy the exact same source range.
func nodeRangeEquals(a, b *gotreesitter.Node) bool {
	return a.StartPoint().Row == b.StartPoint().Row &&
		a.StartPoint().Column == b.StartPoint().Column &&
		a.EndPoint().Row == b.EndPoint().Row &&
		a.EndPoint().Column == b.EndPoint().Column
}

// isRawMultiAssignTypeBinding detects multi-assignment short_var_declarations
// that should be filtered out (they are synthesized separately by type-binding.ts).
// Mirrors TS isRawMultiAssignTypeBinding.
func isRawMultiAssignTypeBinding(lang *gotreesitter.Language, nodeMap map[string]*gotreesitter.Node) bool {
	anchor := nodeMap["@type-binding.constructor"]
	if anchor == nil {
		anchor = nodeMap["@type-binding.call-return"]
	}
	if anchor == nil {
		anchor = nodeMap["@type-binding.assertion"]
	}
	if anchor == nil {
		return false
	}
	if anchor.Type(lang) != "short_var_declaration" {
		return false
	}
	lhs := anchor.ChildByFieldName("left", lang)
	rhs := anchor.ChildByFieldName("right", lang)
	if lhs == nil || rhs == nil {
		return false
	}
	return countNamedChildrenOfType(lang, lhs, "identifier") >= 2 &&
		rhs.NamedChildCount() >= 2
}

// normalizeGenericConstructorCapture normalizes generic_type nodes in
// constructor and reference captures to their base type.
// Mirrors TS normalizeGenericConstructorCapture.
func normalizeGenericConstructorCapture(
	lang *gotreesitter.Language,
	nodeMap map[string]*gotreesitter.Node,
	grouped CaptureMatch,
	source []byte,
) {
	if _, ok := grouped["@type-binding.constructor"]; ok {
		typeNode := nodeMap["@type-binding.type"]
		if typeNode != nil && typeNode.Type(lang) == "generic_type" {
			base := typeNode.ChildByFieldName("type", lang)
			if base != nil {
				grouped["@type-binding.type"] = utils.SyntheticCapture(
					"@type-binding.type", base,
					ExtractSimpleTypeNameTextFromNode(lang, base, source),
				)
			}
		}
	}

	if _, ok := grouped["@reference.call.constructor"]; ok {
		refNode := nodeMap["@reference.name"]
		if refNode != nil && refNode.Type(lang) == "generic_type" {
			base := refNode.ChildByFieldName("type", lang)
			if base != nil {
				grouped["@reference.name"] = utils.SyntheticCapture(
					"@reference.name", base,
					ExtractSimpleTypeNameTextFromNode(lang, base, source),
				)
			}
		}
	}
}

// synthesizeGoInheritanceReferences creates @reference.inherits captures for
// Go struct embedding and interface embedding.
// Mirrors TS synthesizeGoInheritanceReferences.
func synthesizeGoInheritanceReferences(lang *gotreesitter.Language, root *gotreesitter.Node, source []byte) []CaptureMatch {
	var out []CaptureMatch
	walkNamedTree(lang, root, func(node *gotreesitter.Node) {
		if node.Type(lang) != "type_declaration" {
			return
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			spec := node.NamedChild(i)
			if spec == nil || spec.Type(lang) != "type_spec" {
				continue
			}
			typeNode := spec.ChildByFieldName("type", lang)
			if typeNode == nil {
				continue
			}
			if typeNode.Type(lang) == "struct_type" {
				fieldList := findNamedChildOfType(lang, typeNode, "field_declaration_list")
				if fieldList == nil {
					continue
				}
				for j := 0; j < int(fieldList.NamedChildCount()); j++ {
					field := fieldList.NamedChild(j)
					if field == nil || field.Type(lang) != "field_declaration" {
						continue
					}
					// Embedded (anonymous) field: no `name` field
					if field.ChildByFieldName("name", lang) != nil {
						continue
					}
					emitGoEmbedInheritance(lang, field.ChildByFieldName("type", lang), &out, source)
				}
			} else if typeNode.Type(lang) == "interface_type" {
				for j := 0; j < int(typeNode.NamedChildCount()); j++ {
					elem := typeNode.NamedChild(j)
					if elem == nil || elem.Type(lang) != "type_elem" || elem.NamedChildCount() != 1 {
						continue
					}
					emitGoEmbedInheritance(lang, elem.NamedChild(0), &out, source)
				}
			}
		}
	})
	return out
}

// emitGoEmbedInheritance emits one @reference.inherits match for a Go embed base node.
func emitGoEmbedInheritance(lang *gotreesitter.Language, baseNode *gotreesitter.Node, out *[]CaptureMatch, source []byte) {
	if baseNode == nil {
		return
	}
	nameNode := goEmbedBaseNameNode(lang, baseNode)
	if nameNode == nil {
		return
	}
	*out = append(*out, CaptureMatch{
		"@reference.inherits": utils.NodeToCapture("@reference.inherits", baseNode, source),
		"@reference.name":    utils.NodeToCapture("@reference.name", nameNode, source),
	})
}

// goEmbedBaseNameNode reduces a Go embed base node to its trailing bare type_identifier.
// Mirrors TS goEmbedBaseNameNode.
func goEmbedBaseNameNode(lang *gotreesitter.Language, node *gotreesitter.Node) *gotreesitter.Node {
	if node.Type(lang) == "type_identifier" {
		return node
	}
	if node.Type(lang) == "qualified_type" {
		name := node.ChildByFieldName("name", lang)
		if name != nil {
			return goEmbedBaseNameNode(lang, name)
		}
		return nil
	}
	if node.Type(lang) == "generic_type" {
		inner := node.ChildByFieldName("type", lang)
		if inner != nil {
			return goEmbedBaseNameNode(lang, inner)
		}
		return nil
	}
	return nil
}

// --- AST helper functions ---

// walkNamedTree walks all named nodes in the tree in DFS order.
func walkNamedTree(lang *gotreesitter.Language, node *gotreesitter.Node, visit func(*gotreesitter.Node)) {
	visit(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil {
			walkNamedTree(lang, child, visit)
		}
	}
}

// findNamedChildOfType returns the first named child of node matching the given type.
func findNamedChildOfType(lang *gotreesitter.Language, node *gotreesitter.Node, childType string) *gotreesitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == childType {
			return child
		}
	}
	return nil
}

// countNamedChildrenOfType counts named children of a given type.
func countNamedChildrenOfType(lang *gotreesitter.Language, node *gotreesitter.Node, childType string) int {
	count := 0
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == childType {
			count++
		}
	}
	return count
}