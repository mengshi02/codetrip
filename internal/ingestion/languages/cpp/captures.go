package cpp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// CaptureMatch represents a grouped set of captures for one query match.
// Maps capture tag (e.g., "@scope.function") → shared.Capture.
type CaptureMatch map[string]shared.Capture

// EmitCppScopeCaptures executes the unified C++ scope query against the AST and
// synthesizes additional captures based on node structure.
// Ported from GitNexus cpp/captures.ts (emitCppScopeCaptures).
func EmitCppScopeCaptures(source []byte, filePath string) []CaptureMatch {
	lang := grammars.CppLanguage()
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

	query := CppScopeQueryCompiled()
	if query == nil {
		return nil
	}

	matches := query.ExecuteNode(root, lang, source)
	if len(matches) == 0 {
		return nil
	}

	var out []CaptureMatch

	// Track ranges where typedef-struct/enum was captured as its concrete type
	// so we can suppress the duplicate @declaration.typedef match.
	concreteTypedefRanges := make(map[string]bool)

	for _, m := range matches {
		grouped := make(CaptureMatch)
		// Parallel tag -> captured Node map. The tree-sitter query already
		// hands us each matched node as c.node, so anchors resolve via a
		// type-guarded lookup (NodeIfType) instead of re-deriving them with
		// findNodeAtRange per match.
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

		// ── Handle #include statements ──
		if _, ok := grouped["@import.statement"]; ok {
			includeNode := utils.NodeIfType(nodeMap["@import.statement"], lang, "preproc_include")
			if includeNode != nil {
				split := splitCppInclude(lang, includeNode, source)
				if split != nil {
					out = append(out, cppIncludeCaptureToMatch(split))
					continue
				}
			}
		}

		// ── Handle using declarations (using namespace / using name) ──
		if _, ok := grouped["@import.using-decl"]; ok {
			usingNode := utils.NodeIfType(nodeMap["@import.using-decl"], lang, "using_declaration")
			if usingNode != nil {
				split := splitCppUsingDecl(lang, usingNode, source)
				if split != nil {
					out = append(out, cppIncludeCaptureToMatch(split))
					continue
				}
			}
		}

		// ── Track concrete typedef ranges ──
		concreteTypeAnchor := grouped["@declaration.struct"]
		if concreteTypeAnchor.Name == "" {
			concreteTypeAnchor = grouped["@declaration.class"]
		}
		if concreteTypeAnchor.Name == "" {
			concreteTypeAnchor = grouped["@declaration.enum"]
		}
		if concreteTypeAnchor.Name != "" {
			r := concreteTypeAnchor.Range
			concreteTypedefRanges[fmt.Sprintf("%d:%d:%d:%d", r.StartLine, r.StartCol, r.EndLine, r.EndCol)] = true
		}

		// Suppress @declaration.typedef if the same range was already captured as a concrete type
		if typedefAnchor, ok := grouped["@declaration.typedef"]; ok {
			r := typedefAnchor.Range
			key := fmt.Sprintf("%d:%d:%d:%d", r.StartLine, r.StartCol, r.EndLine, r.EndCol)
			if concreteTypedefRanges[key] {
				continue
			}
		}

		// ── Enrich function/method declarations with arity metadata ──
		declAnchorNode := nodeMap["@declaration.function"]
		if declAnchorNode == nil {
			declAnchorNode = nodeMap["@declaration.method"]
		}
		if declAnchorNode != nil {
			fnNode := utils.NodeIfType(declAnchorNode, lang, "function_definition", "declaration", "field_declaration")
			if fnNode != nil {
				enrichCppFunctionDecl(grouped, fnNode, lang, source, filePath)
			}
		}

		// ── Detect static variables (file-local linkage) ──
		if _, ok := grouped["@declaration.variable"]; ok {
			varNode := utils.NodeIfType(nodeMap["@declaration.variable"], lang, "declaration")
			if varNode != nil {
				if hasCppStaticStorageClass(lang, varNode, source) || isInsideAnonymousNamespace(lang, varNode) {
					if nameCap, ok := grouped["@declaration.name"]; ok {
						MarkFileLocal(filePath, nameCap.Text)
					}
				}
			}
		}

		// ── Enrich call references with arity ──
		callAnchorNode := nodeMap["@reference.call.free"]
		if callAnchorNode == nil {
			callAnchorNode = nodeMap["@reference.call.member"]
		}
		if callAnchorNode == nil {
			callAnchorNode = nodeMap["@reference.call.qualified"]
		}
		operatorAnchor := grouped["@reference.operator"]
		if operatorAnchor.Name != "" {
			// When @reference.operator fires, the co-captured call anchor is the
			// enclosing binary_expression itself.
			operatorNode := utils.NodeIfType(callAnchorNode, lang, "binary_expression")
			if operatorNode != nil && isPrimitiveOnlyBinaryOperator(lang, operatorNode, source) {
				continue
			}
		}
		if callAnchorNode != nil {
			if _, hasArity := grouped["@reference.arity"]; !hasArity {
				callNode := utils.NodeIfType(callAnchorNode, lang, "call_expression", "binary_expression")
				if callNode != nil {
					ct := callNode.Type(lang)
					if ct == "call_expression" {
						grouped["@reference.arity"] = utils.SyntheticCapture(
							"@reference.arity", callNode,
							fmt.Sprintf("%d", computeCppCallArity(lang, callNode, source)),
						)
					} else if ct == "binary_expression" {
						arityText := "2"
						if _, isMember := grouped["@reference.call.member"]; isMember {
							arityText = "1"
						}
						grouped["@reference.arity"] = utils.SyntheticCapture(
							"@reference.arity", callNode, arityText,
						)
					}
				}
			}
		}

		if operatorAnchor.Name != "" {
			if _, hasName := grouped["@reference.name"]; !hasName {
				grouped["@reference.name"] = utils.SyntheticCapture(
					"@reference.name", root,
					"operator"+operatorAnchor.Text,
				)
			}
		}

		// ── Enrich constructor calls (new Foo()) with arity ──
		ctorCallAnchorNode := nodeMap["@reference.call.constructor"]
		if _, hasCtorArity := grouped["@reference.arity"]; !hasCtorArity {
			if ctorCallAnchorNode != nil {
				newNode := utils.NodeIfType(ctorCallAnchorNode, lang, "new_expression")
				if newNode != nil {
					grouped["@reference.arity"] = utils.SyntheticCapture(
						"@reference.arity", newNode,
						fmt.Sprintf("%d", computeCppCallArity(lang, newNode, source)),
					)
				}
			}
		}

		// ── Synthesize argument types for overload narrowing ──
		anyCallAnchorNode := callAnchorNode
		if anyCallAnchorNode == nil {
			anyCallAnchorNode = ctorCallAnchorNode
		}
		if anyCallAnchorNode != nil {
			if _, hasParamTypes := grouped["@reference.parameter-types"]; !hasParamTypes {
				cNode := utils.NodeIfType(anyCallAnchorNode, lang, "call_expression", "new_expression", "binary_expression")
				if cNode != nil {
					ct := cNode.Type(lang)
					var argTypes []string
					var argTypeClasses []shared.ParameterTypeClass
					if ct == "binary_expression" {
						_, isFreeCall := grouped["@reference.call.free"]
						argTypes = inferCppBinaryOperatorArgTypes(lang, cNode, source, isFreeCall)
						argTypeClasses = inferCppBinaryOperatorArgTypeClasses(lang, cNode, source, isFreeCall)
					} else {
						argTypes = inferCppCallArgTypes(lang, cNode, source)
						argTypeClasses = inferCppCallArgTypeClasses(lang, cNode, source)
					}
					if len(argTypes) > 0 {
						aj, _ := json.Marshal(argTypes)
						grouped["@reference.parameter-types"] = utils.SyntheticCapture(
							"@reference.parameter-types", cNode, string(aj),
						)
					}
					if len(argTypeClasses) > 0 {
						acj, _ := json.Marshal(argTypeClasses)
						grouped["@reference.parameter-type-classes"] = utils.SyntheticCapture(
							"@reference.parameter-type-classes", cNode, string(acj),
						)
					}
				}
			}
		}

		// ── Inline namespace detection ──
		namespaceScopeAnchorNode := nodeMap["@declaration.namespace"]
		if namespaceScopeAnchorNode == nil {
			namespaceScopeAnchorNode = nodeMap["@scope.namespace"]
		}
		if namespaceScopeAnchorNode != nil {
			nsNode := utils.NodeIfType(namespaceScopeAnchorNode, lang, "namespace_definition")
			if nsNode != nil {
				nsRange := shared.Range{
					StartLine: int(nsNode.StartPoint().Row) + 1,
					StartCol:  int(nsNode.StartPoint().Column),
					EndLine:   int(nsNode.EndPoint().Row) + 1,
					EndCol:    int(nsNode.EndPoint().Column),
				}
				if isInlineNamespace(lang, nsNode) {
					MarkCppInlineNamespaceRange(filePath, nsRange)
				}
				// Anonymous namespace: namespace_definition with no name field
				nameField := nsNode.ChildByFieldName("name", lang)
				if nameField == nil {
					MarkCppAnonymousNamespaceRange(filePath, nsRange)
				}
			}
		}

		// ── ADL (Koenig lookup) per-site recording ──
		if _, ok := grouped["@reference.call.free"]; ok {
			freeCallNode := utils.NodeIfType(nodeMap["@reference.call.free"], lang, "call_expression")
			if freeCallNode != nil {
				adlAnchorRange := grouped["@reference.call.free"].Range
				if isParenthesizedFunctionCall(lang, freeCallNode) {
					MarkCppAdlSiteNoAdl(filePath, adlAnchorRange.StartLine, adlAnchorRange.StartCol)
				}
				adlArgs := inferCppCallAdlArgs(lang, freeCallNode, source)
				if len(adlArgs) > 0 {
					MarkCppAdlSiteArgs(filePath, adlAnchorRange.StartLine, adlAnchorRange.StartCol, adlArgs)
				}
			}
		}

		// ── Post-process @type-binding.assignment for auto declarations ──
		if _, hasAssignment := grouped["@type-binding.assignment"]; hasAssignment {
			if typeCap, hasType := grouped["@type-binding.type"]; hasType && typeCap.Text == "auto" {
				anchor := grouped["@type-binding.assignment"]
				declNode := utils.NodeIfType(nodeMap["@type-binding.assignment"], lang, "declaration")
				if declNode != nil {
					declarator := declNode.ChildByFieldName("declarator", lang)
					if declarator != nil && declarator.Type(lang) == "init_declarator" {
						valueNode := declarator.ChildByFieldName("value", lang)
						if valueNode != nil {
							vt := valueNode.Type(lang)
							if vt == "identifier" {
								// auto alias = existingVar → promote to @type-binding.alias
								grouped["@type-binding.alias"] = anchor
								grouped["@type-binding.type"] = utils.NodeToCapture("@type-binding.type", valueNode, source)
								delete(grouped, "@type-binding.assignment")
							} else if vt == "field_expression" {
								// auto addr = user.address → promote to @type-binding.member-access
								argNode := valueNode.ChildByFieldName("argument", lang)
								fieldNode := valueNode.ChildByFieldName("field", lang)
								if argNode != nil && fieldNode != nil {
									grouped["@type-binding.member-access"] = anchor
									grouped["@type-binding.member-access-receiver"] = utils.NodeToCapture("@type-binding.member-access-receiver", argNode, source)
									grouped["@type-binding.type"] = utils.NodeToCapture("@type-binding.type", fieldNode, source)
									delete(grouped, "@type-binding.assignment")
								}
							} else if vt == "call_expression" {
								fnNode := valueNode.ChildByFieldName("function", lang)
								if fnNode != nil && fnNode.Type(lang) == "field_expression" {
									argNode := fnNode.ChildByFieldName("argument", lang)
									fieldNode := fnNode.ChildByFieldName("field", lang)
									if argNode != nil && fieldNode != nil {
										grouped["@type-binding.member-access"] = anchor
										grouped["@type-binding.member-access-receiver"] = utils.NodeToCapture("@type-binding.member-access-receiver", argNode, source)
										grouped["@type-binding.type"] = utils.NodeToCapture("@type-binding.type", fieldNode, source)
										delete(grouped, "@type-binding.assignment")
									}
								}
							}
						}
					}
				}
			}
		}

		out = append(out, grouped)
	}

	// ── Emit inheritance references for scope-resolution MRO / EXTENDS ──
	emitCppInheritanceCaptures(lang, root, source, &out, filePath)

	// ── Detect dependent-base relationships for two-phase template lookup ──
	detectCppDependentBases(lang, root, source, filePath)

	return out
}

// enrichCppFunctionDecl enriches a function/method declaration with arity,
// return type, explicit/deleted markers, template constraints, static/anonymous-namespace linkage.
func enrichCppFunctionDecl(grouped CaptureMatch, fnNode *gotreesitter.Node, lang *gotreesitter.Language, source []byte, filePath string) {
	arity := computeCppDeclarationArity(lang, fnNode, source)
	if arity.HasParameterCount {
		grouped["@declaration.parameter-count"] = utils.SyntheticCapture(
			"@declaration.parameter-count", fnNode,
			fmt.Sprintf("%d", arity.ParameterCount),
		)
	}
	if arity.RequiredParameterCount > 0 || arity.HasParameterCount {
		grouped["@declaration.required-parameter-count"] = utils.SyntheticCapture(
			"@declaration.required-parameter-count", fnNode,
			fmt.Sprintf("%d", arity.RequiredParameterCount),
		)
	}
	if len(arity.ParameterTypes) > 0 {
		ptJSON, _ := json.Marshal(arity.ParameterTypes)
		grouped["@declaration.parameter-types"] = utils.SyntheticCapture(
			"@declaration.parameter-types", fnNode,
			string(ptJSON),
		)
	}
	if len(arity.ParameterTypeClasses) > 0 {
		ptcJSON, _ := json.Marshal(arity.ParameterTypeClasses)
		grouped["@declaration.parameter-type-classes"] = utils.SyntheticCapture(
			"@declaration.parameter-type-classes", fnNode,
			string(ptcJSON),
		)
	}

	// Return type
	returnType := extractCppDeclarationReturnType(lang, fnNode, source)
	if returnType != "" {
		grouped["@declaration.return-type"] = utils.SyntheticCapture(
			"@declaration.return-type", fnNode,
			returnType,
		)
	}

	// Detect explicit specifier
	if hasExplicitSpecifier(lang, fnNode) {
		grouped["@declaration.is-explicit"] = utils.SyntheticCapture(
			"@declaration.is-explicit", fnNode,
			"true",
		)
	}

	// Detect deleted method
	nameText := ""
	if nameCap, ok := grouped["@declaration.name"]; ok {
		nameText = nameCap.Text
	}
	if hasDeletedMethodClause(lang, fnNode, nameText, source) {
		grouped["@declaration.is-deleted"] = utils.SyntheticCapture(
			"@declaration.is-deleted", fnNode,
			"true",
		)
	}

	// Detect static storage class (file-local linkage)
	if hasCppStaticStorageClass(lang, fnNode, source) {
		if nameText != "" {
			MarkFileLocal(filePath, nameText)
		}
	}

	// Detect anonymous namespace (file-local linkage)
	if isInsideAnonymousNamespace(lang, fnNode) {
		if nameText != "" {
			MarkFileLocal(filePath, nameText)
		}
	}

	// SFINAE / requires-clause aware constraints for overload narrowing.
	// Walk from the enclosing template_declaration — not the inner function_definition —
	// so inline method templates pick up the correct outer constraint scope.
	templateDecl := findEnclosingTemplateDeclaration(lang, fnNode)
	if templateDecl != nil {
		funcDeclarator := findCppFunctionDeclarator(lang, fnNode)
		constraints := ExtractCppTemplateConstraints(lang, templateDecl, funcDeclarator, source)
		if constraints != nil {
			cJSON, _ := json.Marshal(constraints)
			grouped["@declaration.template-constraints"] = utils.SyntheticCapture(
				"@declaration.template-constraints", fnNode,
				string(cJSON),
			)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Helper functions ported from TS cpp/captures.ts
// ────────────────────────────────────────────────────────────────────────────

// extractCppDeclarationReturnType extracts the return type text from a function node.
// Returns empty string if no return type can be determined.
func extractCppDeclarationReturnType(lang *gotreesitter.Language, fnNode *gotreesitter.Node, source []byte) string {
	typeNode := fnNode.ChildByFieldName("type", lang)
	if typeNode == nil {
		return ""
	}
	funcDeclarator := findCppFunctionDeclarator(lang, fnNode)
	if funcDeclarator != nil && isCppUnsupportedReturnTypeDeclarator(lang, funcDeclarator, source) {
		return ""
	}
	typeText := strings.TrimSpace(typeNode.Text(source))
	if typeText != "auto" {
		if len(typeText) > 0 {
			return typeText
		}
		return ""
	}
	// Auto return type — look for trailing return type
	if funcDeclarator == nil {
		return typeText
	}
	for i := 0; i < int(funcDeclarator.NamedChildCount()); i++ {
		child := funcDeclarator.NamedChild(i)
		if child != nil && child.Type(lang) == "trailing_return_type" {
			typeDesc := firstNamedChild(lang, child)
			if typeDesc != nil {
				t := strings.TrimSpace(typeDesc.Text(source))
				if t != "" {
					return t
				}
			}
		}
	}
	return typeText
}

// isCppUnsupportedReturnTypeDeclarator returns true for operator overloads and destructors
// where the return type should not be captured.
func isCppUnsupportedReturnTypeDeclarator(lang *gotreesitter.Language, funcDeclarator *gotreesitter.Node, source []byte) bool {
	text := funcDeclarator.Text(source)
	return strings.Contains(text, "operator") || strings.Contains(text, "~")
}

// hasExplicitSpecifier checks if a function has the `explicit` specifier.
func hasExplicitSpecifier(lang *gotreesitter.Language, fnNode *gotreesitter.Node) bool {
	for i := 0; i < int(fnNode.ChildCount()); i++ {
		child := fnNode.Child(i)
		if child != nil && child.Type(lang) == "explicit_function_specifier" {
			return true
		}
	}
	return false
}

// hasDeletedMethodClause checks if a function declaration has a `= delete` clause.
func hasDeletedMethodClause(lang *gotreesitter.Language, fnNode *gotreesitter.Node, nameText string, source []byte) bool {
	for i := 0; i < int(fnNode.ChildCount()); i++ {
		child := fnNode.Child(i)
		if child != nil && child.Type(lang) == "delete_expression" {
			return true
		}
	}
	// Also check init_declarator wrapping (tree-sitter-cpp 0.23+)
	for i := 0; i < int(fnNode.ChildCount()); i++ {
		child := fnNode.Child(i)
		if child != nil && child.Type(lang) == "init_declarator" {
			for j := 0; j < int(child.ChildCount()); j++ {
				c := child.Child(j)
				if c != nil && c.Type(lang) == "delete_expression" {
					return true
				}
			}
		}
	}
	return false
}

// hasCppStaticStorageClass checks if a node has `static` storage class.
// Walks direct children for a storage_class_specifier node with text "static".
func hasCppStaticStorageClass(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type(lang) == "storage_class_specifier" && child.Text(source) == "static" {
			return true
		}
	}
	return false
}

// isInsideAnonymousNamespace walks the parent chain to detect whether a node
// is inside a `namespace { ... }` (anonymous namespace).
func isInsideAnonymousNamespace(lang *gotreesitter.Language, node *gotreesitter.Node) bool {
	cur := node.Parent()
	for cur != nil {
		if cur.Type(lang) == "namespace_definition" {
			nameField := cur.ChildByFieldName("name", lang)
			if nameField == nil {
				return true
			}
		}
		cur = cur.Parent()
	}
	return false
}

// isInlineNamespace checks whether a namespace_definition AST node is inline.
// Tree-sitter-cpp exposes the `inline` keyword as a child node.
func isInlineNamespace(lang *gotreesitter.Language, nsNode *gotreesitter.Node) bool {
	for i := 0; i < int(nsNode.ChildCount()); i++ {
		c := nsNode.Child(i)
		if c == nil {
			continue
		}
		ct := c.Type(lang)
		if ct == "inline" {
			return true
		}
		// Some grammar variants surface keywords by their text rather than
		// by a dedicated node type; check both for resilience.
		if c.Text(nil) == "inline" && (ct == "storage_class_specifier" || ct == "inline") {
			return true
		}
	}
	return false
}

// isParenthesizedFunctionCall detects `(f)(args)` shape — the call-expression's
// function field is a parenthesized_expression. ISO C++ specifies that this form
// suppresses ADL.
func isParenthesizedFunctionCall(lang *gotreesitter.Language, callNode *gotreesitter.Node) bool {
	fn := callNode.ChildByFieldName("function", lang)
	return fn != nil && fn.Type(lang) == "parenthesized_expression"
}

// findEnclosingTemplateDeclaration walks the parent chain from a function node
// to find the enclosing template_declaration. Returns nil when the function
// isn't templated.
func findEnclosingTemplateDeclaration(lang *gotreesitter.Language, fnNode *gotreesitter.Node) *gotreesitter.Node {
	cur := fnNode.Parent()
	hops := 8
	for cur != nil && hops > 0 {
		if cur.Type(lang) == "template_declaration" {
			return cur
		}
		if cur.Type(lang) == "translation_unit" {
			return nil
		}
		cur = cur.Parent()
		hops--
	}
	return nil
}

// findCppFunctionDeclarator locates the function_declarator AST node within a
// function definition or declaration. Unwraps pointer/reference declarator wrappers.
func findCppFunctionDeclarator(lang *gotreesitter.Language, fnNode *gotreesitter.Node) *gotreesitter.Node {
	direct := fnNode.ChildByFieldName("declarator", lang)
	cur := direct
	hops := 8
	for cur != nil && hops > 0 {
		if cur.Type(lang) == "function_declarator" {
			return cur
		}
		if cur.Type(lang) == "pointer_declarator" || cur.Type(lang) == "reference_declarator" {
			next := cur.ChildByFieldName("declarator", lang)
			if next == nil {
				break
			}
			cur = next
			continue
		}
		break
	}
	// Fallback: recursive search
	return findFirstDescendantOfType(lang, fnNode, "function_declarator")
}

// cppIncludeCaptureToMatch converts a CppIncludeCapture to a CaptureMatch.
func cppIncludeCaptureToMatch(cap *CppIncludeCapture) CaptureMatch {
	cm := CaptureMatch{
		"@import.statement": cap.Statement,
		"@import.kind":      cap.Kind,
		"@import.source":    cap.Source,
	}
	if cap.System.Text != "" {
		cm["@import.system"] = cap.System
	}
	if cap.Name.Text != "" {
		cm["@import.name"] = cap.Name
	}
	if cap.UsingNS.Text != "" {
		cm["@import.using-namespace"] = cap.UsingNS
	}
	return cm
}