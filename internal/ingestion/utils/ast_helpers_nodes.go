package utils

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/odvcencio/gotreesitter"
)

type EnclosingClassInfo struct {
	ClassID          string
	ClassName        string
	QualifiedClassID string
}

const maxEnclosingWalkIterations = 4096

type ResolveEnclosingOwnerFunc func(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) *gotreesitter.Node
type GetQualifiedOwnerNameFunc func(node *gotreesitter.Node, simpleName string, lang *gotreesitter.Language, source []byte) *string

func FindEnclosingClassInfo(node *gotreesitter.Node, filePath string, resolveEnclosingOwner ResolveEnclosingOwnerFunc, getQualifiedOwnerName GetQualifiedOwnerNameFunc, lang *gotreesitter.Language, source []byte) *EnclosingClassInfo {
	current := node.Parent()
	iterations := 0
	visitedContainers := make(map[*gotreesitter.Node]bool)
	for current != nil {
		if iterations++; iterations > maxEnclosingWalkIterations { return nil }
		if current.Type(lang) == "method_declaration" {
			if info := findImplItemInfo(current, lang, source, filePath); info != nil { return info }
		}
		if current.Type(lang) == "type_declaration" {
			var typeSpec *gotreesitter.Node
			for i := 0; i < current.ChildCount(); i++ {
				c := current.Child(i)
				if c != nil && c.Type(lang) == "type_spec" { typeSpec = c; break }
			}
			if typeSpec != nil {
				typeBody := typeSpec.ChildByFieldName("type", lang)
				if typeBody != nil && (typeBody.Type(lang) == "struct_type" || typeBody.Type(lang) == "interface_type") {
					nameNode := typeSpec.ChildByFieldName("name", lang)
					if nameNode != nil {
						label := "Struct"
						if typeBody.Type(lang) == "interface_type" { label = "Interface" }
						className := nameNode.Text(source)
						return &EnclosingClassInfo{ClassID: shared.GenerateID(label, filePath+":"+className), ClassName: className}
					}
				}
			}
		}
		if ClassContainerTypes[current.Type(lang)] {
			if resolveEnclosingOwner != nil {
				if visitedContainers[current] { current = current.Parent(); continue }
				visitedContainers[current] = true
				resolved := resolveEnclosingOwner(current, lang, source)
				if resolved == nil { current = current.Parent(); continue }
				if resolved != current { current = resolved; continue }
			}
			if current.Type(lang) == "impl_item" {
				if info := findImplItemInfo(current, lang, source, filePath); info != nil { return info }
			}
			nameNode := findContainerNameNode(current, lang)
			if nameNode != nil { return buildEnclosingClassInfo(current, nameNode, filePath, lang, source, getQualifiedOwnerName) }
		}
		current = current.Parent()
	}
	return nil
}

func findImplItemInfo(current *gotreesitter.Node, lang *gotreesitter.Language, source []byte, filePath string) *EnclosingClassInfo {
	if current.Type(lang) == "method_declaration" {
		receiver := current.ChildByFieldName("receiver", lang)
		if receiver != nil {
			var paramDecl *gotreesitter.Node
			for i := 0; i < receiver.NamedChildCount(); i++ {
				c := receiver.NamedChild(i)
				if c != nil && c.Type(lang) == "parameter_declaration" { paramDecl = c; break }
			}
			if paramDecl != nil {
				typeNode := paramDecl.ChildByFieldName("type", lang)
				if typeNode != nil {
					inner := typeNode
					if typeNode.Type(lang) == "pointer_type" { inner = firstNamedChild(typeNode, lang) }
					if inner != nil && (inner.Type(lang) == "type_identifier" || inner.Type(lang) == "identifier") {
						className := inner.Text(source)
						return &EnclosingClassInfo{ClassID: shared.GenerateID("Struct", filePath+":"+className), ClassName: className}
					}
				}
			}
		}
		return nil
	}
	children := make([]*gotreesitter.Node, 0, current.ChildCount())
	for i := 0; i < current.ChildCount(); i++ {
		if c := current.Child(i); c != nil { children = append(children, c) }
	}
	forIdx := -1
	for i, c := range children { if c.Text(source) == "for" { forIdx = i; break } }
	if forIdx != -1 {
		var nameNode *gotreesitter.Node
		for i := forIdx + 1; i < len(children); i++ {
			c := children[i]
			if c.Type(lang) == "type_identifier" || c.Type(lang) == "scoped_type_identifier" || c.Type(lang) == "identifier" { nameNode = c; break }
		}
		if nameNode != nil {
			className := nameNode.Text(source)
			return &EnclosingClassInfo{ClassID: shared.GenerateID("Struct", filePath+":"+className), ClassName: className}
		}
	}
	var implTarget *gotreesitter.Node
	for _, c := range children {
		if c.Type(lang) == "type_identifier" || c.Type(lang) == "scoped_type_identifier" || c.Type(lang) == "generic_type" { implTarget = c; break }
	}
	if implTarget != nil {
		var baseType *gotreesitter.Node
		if implTarget.Type(lang) == "generic_type" {
			baseType = implTarget.ChildByFieldName("type", lang)
			if baseType == nil { baseType = implTarget }
		} else { baseType = implTarget }
		if baseType.Type(lang) == "type_identifier" {
			qualified := QualifyRustImplTargetByModScope(current, baseType.Text(source), lang)
			return &EnclosingClassInfo{ClassID: shared.GenerateID("Impl", filePath+":"+qualified), ClassName: qualified}
		}
		if baseType.Type(lang) == "scoped_type_identifier" && implTarget.Type(lang) != "generic_type" {
			className := baseType.Text(source)
			return &EnclosingClassInfo{ClassID: shared.GenerateID("Impl", filePath+":"+className), ClassName: className}
		}
	}
	return nil
}

func findContainerNameNode(current *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	nameNode := current.ChildByFieldName("name", lang)
	if nameNode == nil {
		for i := 0; i < current.ChildCount(); i++ {
			c := current.Child(i)
			if c != nil && (c.Type(lang) == "type_identifier" || c.Type(lang) == "identifier" || c.Type(lang) == "name" || c.Type(lang) == "constant") { nameNode = c; break }
		}
	}
	return nameNode
}

func buildEnclosingClassInfo(current *gotreesitter.Node, nameNode *gotreesitter.Node, filePath string, lang *gotreesitter.Language, source []byte, getQualifiedOwnerName GetQualifiedOwnerNameFunc) *EnclosingClassInfo {
	label := "Class"
	if mapped, ok := ContainerTypeToLabel[current.Type(lang)]; ok { label = mapped }
	if current.Type(lang) == "class_declaration" && label == "Class" {
		for i := 0; i < current.ChildCount(); i++ {
			c := current.Child(i)
			if c != nil && c.Type(lang) == "interface" { label = "Interface"; break }
		}
	}
	if current.Type(lang) == "class_declaration" && label == "Class" {
		declKindNode := current.ChildByFieldName("declaration_kind", lang)
		if declKindNode != nil {
			dk := declKindNode.Text(source)
			if dk == "struct" { label = "Struct" } else if dk == "enum" { label = "Enum" }
		}
	}
	nameText := nameNode.Text(source)
	templateArgs := ExtractTemplateArguments(nameText)
	var classIDName string
	if templateArgs != nil {
		classIDName = StripTemplateArguments(nameText) + TemplateArgumentsIdTag(templateArgs)
	} else { classIDName = nameText }
	var qualifiedClassID string
	if getQualifiedOwnerName != nil {
		qn := getQualifiedOwnerName(current, nameText, lang, source)
		if qn != nil && *qn != nameText {
			var qid string
			if templateArgs != nil { qid = StripTemplateArguments(*qn) + TemplateArgumentsIdTag(templateArgs) } else { qid = *qn }
			qualifiedClassID = shared.GenerateID(label, filePath+":"+qid)
		}
	}
	return &EnclosingClassInfo{ClassID: shared.GenerateID(label, filePath+":"+classIDName), ClassName: nameText, QualifiedClassID: qualifiedClassID}
}

type ObjectLiteralBindingInfo struct{ OwnerID string }

var BlockScopeBoundaryTypes = map[string]bool{
	"statement_block": true, "if_statement": true, "else_clause": true,
	"for_statement": true, "for_in_statement": true, "for_of_statement": true,
	"while_statement": true, "do_statement": true, "try_statement": true,
	"catch_clause": true, "finally_clause": true, "switch_statement": true,
	"switch_case": true, "switch_default": true, "with_statement": true,
}

func FindObjectLiteralBindingInfo(node *gotreesitter.Node, filePath string, lang *gotreesitter.Language, source []byte) *ObjectLiteralBindingInfo {
	current := node
	objectDepth := 0
	var declarator *gotreesitter.Node
	for current != nil {
		if current.Type(lang) == "object" { objectDepth++ }
		if current.Type(lang) == "variable_declarator" && objectDepth >= 1 {
			if objectDepth > 1 { return nil }
			declarator = current; break
		}
		if current != node && (FunctionNodeTypes[current.Type(lang)] || ClassContainerTypes[current.Type(lang)]) { return nil }
		current = current.Parent()
	}
	if declarator == nil { return nil }
	anc := declarator.Parent()
	for anc != nil {
		if anc.Type(lang) == "program" || anc.Type(lang) == "export_statement" { break }
		if FunctionNodeTypes[anc.Type(lang)] || ClassContainerTypes[anc.Type(lang)] { return nil }
		if BlockScopeBoundaryTypes[anc.Type(lang)] { return nil }
		anc = anc.Parent()
	}
	nameNode := declarator.ChildByFieldName("name", lang)
	if nameNode == nil || nameNode.Type(lang) != "identifier" { return nil }
	declaration := declarator.Parent()
	ownerLabel := "Variable"
	if declaration != nil && declaration.Type(lang) == "variable_declaration" { ownerLabel = "Const" }
	return &ObjectLiteralBindingInfo{OwnerID: shared.GenerateID(ownerLabel, filePath+":"+nameNode.Text(source))}
}

func FindEnclosingClassID(node *gotreesitter.Node, filePath string, lang *gotreesitter.Language, source []byte) *string {
	info := FindEnclosingClassInfo(node, filePath, nil, nil, lang, source)
	if info == nil { return nil }
	return &info.ClassID
}

func FindSiblingChild(parent *gotreesitter.Node, siblingType string, childType string, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < parent.ChildCount(); i++ {
		sibling := parent.Child(i)
		if sibling != nil && sibling.Type(lang) == siblingType {
			for j := 0; j < sibling.ChildCount(); j++ {
				child := sibling.Child(j)
				if child != nil && child.Type(lang) == childType { return child }
			}
		}
	}
	return nil
}

func GenericFuncName(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) *string {
	nameField := node.ChildByFieldName("name", lang)
	if nameField != nil { text := nameField.Text(source); return &text }
	if node.Type(lang) == "arrow_function" || node.Type(lang) == "function_expression" { return nil }
	for i := 0; i < node.ChildCount(); i++ {
		c := node.Child(i)
		if c != nil && (c.Type(lang) == "identifier" || c.Type(lang) == "property_identifier" || c.Type(lang) == "simple_identifier") {
			text := c.Text(source); return &text
		}
	}
	return nil
}

var MethodLabelNodeTypes = map[string]bool{
	"method_definition": true, "method_declaration": true, "method": true, "singleton_method": true,
}

var ConstructorLabelNodeTypes = map[string]bool{
	"constructor_declaration": true, "compact_constructor_declaration": true,
}

func InferFunctionLabel(nodeType string) string {
	if MethodLabelNodeTypes[nodeType] { return "Method" }
	if ConstructorLabelNodeTypes[nodeType] { return "Constructor" }
	return "Function"
}

var ParameterListNodeTypes = map[string]bool{
	"formal_parameters": true, "parameters": true, "parameter_list": true,
	"function_value_parameters": true, "class_parameters": true,
}

var LocalScopeBodyNodeTypes map[string]bool

func init() {
	LocalScopeBodyNodeTypes = make(map[string]bool)
	for k := range FunctionNodeTypes {
		if k == "function_signature" || k == "method_signature" { continue }
		LocalScopeBodyNodeTypes[k] = true
	}
	LocalScopeBodyNodeTypes["anonymous_initializer"] = true
	LocalScopeBodyNodeTypes["getter"] = true
	LocalScopeBodyNodeTypes["setter"] = true
	LocalScopeBodyNodeTypes["computed_property"] = true
	LocalScopeBodyNodeTypes["computed_getter"] = true
	LocalScopeBodyNodeTypes["computed_setter"] = true
	LocalScopeBodyNodeTypes["computed_modify"] = true
}

func FindDescendant(root *gotreesitter.Node, nodeType string, lang *gotreesitter.Language) *gotreesitter.Node {
	stack := []*gotreesitter.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]; stack = stack[:len(stack)-1]
		if n.Type(lang) == nodeType { return n }
		for i := n.ChildCount() - 1; i >= 0; i-- {
			if child := n.Child(i); child != nil { stack = append(stack, child) }
		}
	}
	return nil
}

func ExtractStringContent(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte) *string {
	if node == nil { return nil }
	for i := 0; i < node.ChildCount(); i++ {
		c := node.Child(i)
		if c != nil && c.Type(lang) == "string_content" { text := c.Text(source); return &text }
	}
	if node.Type(lang) == "string_content" { text := node.Text(source); return &text }
	return nil
}

func FindChild(node *gotreesitter.Node, childType string, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == childType { return child }
	}
	return nil
}

func NodeToCapture(name string, node *gotreesitter.Node, source []byte) shared.Capture {
	return shared.Capture{
		Name: name,
		Range: shared.Range{
			StartLine: int(node.StartPoint().Row) + 1,
			StartCol:  int(node.StartPoint().Column),
			EndLine:   int(node.EndPoint().Row) + 1,
			EndCol:    int(node.EndPoint().Column),
		},
		Text: node.Text(source),
	}
}

func SyntheticCapture(name string, atNode *gotreesitter.Node, text string) shared.Capture {
	return shared.Capture{
		Name: name,
		Range: shared.Range{
			StartLine: int(atNode.StartPoint().Row) + 1,
			StartCol:  int(atNode.StartPoint().Column),
			EndLine:   int(atNode.EndPoint().Row) + 1,
			EndCol:    int(atNode.EndPoint().Column),
		},
		Text: text,
	}
}

func rangeMatches(node *gotreesitter.Node, r shared.Range) bool {
	return int(node.StartPoint().Row)+1 == r.StartLine &&
		int(node.StartPoint().Column) == r.StartCol &&
		int(node.EndPoint().Row)+1 == r.EndLine &&
		int(node.EndPoint().Column) == r.EndCol
}

func FindNodeAtRange(root *gotreesitter.Node, r shared.Range, expectedType *string, lang *gotreesitter.Language) *gotreesitter.Node {
	startRow := int(r.StartLine) - 1
	endRow := int(r.EndLine) - 1
	stack := []*gotreesitter.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]; stack = stack[:len(stack)-1]
		if rangeMatches(n, r) {
			if expectedType == nil || n.Type(lang) == *expectedType { return n }
		}
		for i := n.NamedChildCount() - 1; i >= 0; i-- {
			child := n.NamedChild(i)
			if child == nil { continue }
			if int(child.EndPoint().Row) < startRow { continue }
			if int(child.StartPoint().Row) > endRow { continue }
			stack = append(stack, child)
		}
	}
	return nil
}

func NodeIfType(node *gotreesitter.Node, lang *gotreesitter.Language, types ...string) *gotreesitter.Node {
	if node == nil { return nil }
	for _, t := range types { if node.Type(lang) == t { return node } }
	return nil
}
