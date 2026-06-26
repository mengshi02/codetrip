package fieldextractors

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// TypeScriptFieldExtractor is a hand-written field extractor for TypeScript.
// This exists alongside the config-based TypeScriptFieldConfig in configs/typescript_javascript.go
// (used for JavaScript) because TypeScript has unique requirements:
// 1. type_alias_declaration with object type literals (e.g., type Config = { key: string })
// 2. Optional property detection appending '| undefined' to types
// 3. Nested type discovery within class/interface bodies
// The config-based extractor cannot express these TS-specific capabilities.
// JavaScript uses the config-based version since it lacks type syntax.
// Ported from TS field-extractors/typescript.ts.

var tsTypeDeclarationNodes = map[string]bool{
	"class_declaration":            true,
	"interface_declaration":       true,
	"abstract_class_declaration":  true,
	"type_alias_declaration":      true, // for object type literals
}

var tsFieldNodeTypes = map[string]bool{
	"public_field_definition": true, // class field: private users: User[]
	"property_signature":      true, // interface property: name: string
	"field_definition":        true, // fallback field type
}

var tsVisibilityModifiers = map[core.FieldVisibility]bool{
	core.VisibilityPublic:    true,
	core.VisibilityPrivate:   true,
	core.VisibilityProtected: true,
}

// TypeScriptFieldExtractor implements FieldExtractor for TypeScript.
type TypeScriptFieldExtractor struct {
	core.BaseFieldExtractor
}

// IsTypeDeclaration checks whether the node is a type declaration with fields.
func (e *TypeScriptFieldExtractor) IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return tsTypeDeclarationNodes[node.Type(lang)]
}

// extractVisibility extracts the visibility modifier from a field node.
func (e *TypeScriptFieldExtractor) extractVisibility(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
	// Check for accessibility_modifier named child (tree-sitter typescript)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "accessibility_modifier" {
			text := strings.TrimSpace(child.Type(lang))
			v := core.FieldVisibility(text)
			if tsVisibilityModifiers[v] {
				return v
			}
		}
	}

	// Check for modifiers in the field's unnamed children (fallback)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() {
			text := strings.TrimSpace(child.Type(lang))
			v := core.FieldVisibility(text)
			if tsVisibilityModifiers[v] {
				return v
			}
		}
	}

	// TypeScript class members are public by default
	return core.VisibilityPublic
}

// isStatic checks whether a field has the static modifier.
func (e *TypeScriptFieldExtractor) isStatic(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() && strings.TrimSpace(child.Type(lang)) == "static" {
			return true
		}
	}
	return false
}

// isReadonly checks whether a field has the readonly modifier.
func (e *TypeScriptFieldExtractor) isReadonly(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() && strings.TrimSpace(child.Type(lang)) == "readonly" {
			return true
		}
	}
	return false
}

// isOptional checks whether a property is optional (has ?: syntax).
func (e *TypeScriptFieldExtractor) isOptional(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	// Look for the optional marker '?' in unnamed children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() && child.Type(lang) == "?" {
			return true
		}
	}

	// Also check for optional kind marker
	kind := node.ChildByFieldName("kind", lang)
	if kind != nil && kind.Type(lang) == "?" {
		return true
	}

	return false
}

// extractFullType handles type_annotation unwrapping and type normalization.
func (e *TypeScriptFieldExtractor) extractFullType(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	if typeNode == nil {
		return ""
	}
	if typeNode.Type(lang) == "type_annotation" {
		inner := typeNode.NamedChild(0)
		if inner != nil {
			return e.NormalizeType(inner.Text(source))
		}
		return ""
	}
	return e.NormalizeType(typeNode.Text(source))
}

// extractField extracts a single FieldInfo from a field definition node.
func (e *TypeScriptFieldExtractor) extractField(
	node *gotreesitter.Node,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
) *core.FieldInfo {
	// Get the field name
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil {
		nameNode = node.ChildByFieldName("property", lang)
	}
	if nameNode == nil {
		return nil
	}

	name := nameNode.Text(source)
	if name == "" {
		return nil
	}

	// Get the type annotation
	typeNode := node.ChildByFieldName("type", lang)
	var fieldType string
	if typeNode != nil {
		fieldType = e.extractFullType(typeNode, source, lang)
	}

	// Resolve the type using context
	if fieldType != "" {
		resolved := e.ResolveType(fieldType, ctx)
		if resolved != "" {
			fieldType = resolved
		}
	}

	return &core.FieldInfo{
		Name:       name,
		Type:       &fieldType,
		Visibility: e.extractVisibility(node, lang),
		IsStatic:   e.isStatic(node, lang),
		IsReadonly: e.isReadonly(node, lang),
		SourceFile: ctx.FilePath,
		Line:       int(node.StartPoint().Row) + 1,
	}
}

// extractFieldsFromBody extracts fields from a class/interface body node.
func (e *TypeScriptFieldExtractor) extractFieldsFromBody(
	bodyNode *gotreesitter.Node,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
) []core.FieldInfo {
	var fields []core.FieldInfo

	for i := 0; i < int(bodyNode.NamedChildCount()); i++ {
		child := bodyNode.NamedChild(i)
		if child == nil {
			continue
		}
		if tsFieldNodeTypes[child.Type(lang)] {
			field := e.extractField(child, ctx, source, lang)
			if field != nil {
				// Mark optional properties by appending "| undefined"
				if e.isOptional(child, lang) && field.Type != nil && *field.Type != "" {
					*field.Type = *field.Type + " | undefined"
				}
				fields = append(fields, *field)
			}
		}
	}

	return fields
}

// extractFieldsFromObjectType extracts fields from an object_type node
// (used in type aliases like: type Config = { key: string }).
func (e *TypeScriptFieldExtractor) extractFieldsFromObjectType(
	objectTypeNode *gotreesitter.Node,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
) []core.FieldInfo {
	var fields []core.FieldInfo

	// Walk named children for property_signature nodes
	for i := 0; i < int(objectTypeNode.NamedChildCount()); i++ {
		propNode := objectTypeNode.NamedChild(i)
		if propNode == nil || propNode.Type(lang) != "property_signature" {
			continue
		}
		field := e.extractField(propNode, ctx, source, lang)
		if field != nil {
			// Mark optional properties
			if e.isOptional(propNode, lang) && field.Type != nil && *field.Type != "" {
				*field.Type = *field.Type + " | undefined"
			}
			fields = append(fields, *field)
		}
	}

	return fields
}

// Extract extracts fields from a TypeScript type declaration node.
func (e *TypeScriptFieldExtractor) Extract(
	node *gotreesitter.Node,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
) *core.ExtractedFields {
	if !tsTypeDeclarationNodes[node.Type(lang)] {
		return nil
	}

	// Get the type name
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil {
		return nil
	}

	typeName := nameNode.Text(source)
	ownerFQN := typeName

	var fields []core.FieldInfo
	var nestedTypes []string

	nodeType := node.Type(lang)

	// Handle different declaration types
	if nodeType == "class_declaration" || nodeType == "abstract_class_declaration" || nodeType == "interface_declaration" {
		bodyNode := node.ChildByFieldName("body", lang)
		if bodyNode != nil {
			extracted := e.extractFieldsFromBody(bodyNode, ctx, source, lang)
			fields = append(fields, extracted...)
		}
	} else if nodeType == "type_alias_declaration" {
		// Handle type aliases with object types
		valueNode := node.ChildByFieldName("value", lang)
		if valueNode != nil && valueNode.Type(lang) == "object_type" {
			extracted := e.extractFieldsFromObjectType(valueNode, ctx, source, lang)
			fields = append(fields, extracted...)
		}
	}

	// Find nested type declarations (walk named children of the body)
	bodyNode := node.ChildByFieldName("body", lang)
	if bodyNode != nil {
		for i := 0; i < int(bodyNode.NamedChildCount()); i++ {
			child := bodyNode.NamedChild(i)
			if child == nil {
				continue
			}
			childType := child.Type(lang)
			if childType == "class_declaration" || childType == "interface_declaration" {
				nestedName := child.ChildByFieldName("name", lang)
				if nestedName != nil {
					nestedTypes = append(nestedTypes, nestedName.Text(source))
				}
			}
		}
	}

	return &core.ExtractedFields{
		OwnerFQN:    ownerFQN,
		Fields:      fields,
		NestedTypes: nestedTypes,
	}
}