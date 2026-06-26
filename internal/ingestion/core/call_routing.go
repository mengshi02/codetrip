package core

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// RubyRoutingKind discriminates the four Ruby call routing outcomes.
type RubyRoutingKind string

const (
	RoutingKindImport     RubyRoutingKind = "import"
	RoutingKindProperties RubyRoutingKind = "properties"
	RoutingKindCall       RubyRoutingKind = "call"
	RoutingKindSkip       RubyRoutingKind = "skip"
)

// RubyAccessorType represents Ruby attribute accessor types.
type RubyAccessorType string

const (
	AccessorReader RubyAccessorType = "reader"
	AccessorWriter RubyAccessorType = "writer"
	AccessorAccess RubyAccessorType = "access" // attr_accessor = reader + writer
)

// RubyPropertyItem represents a property inferred from Ruby attr_* declarations.
// YARD type annotations (@type [Type]) are parsed from inline comments.
type RubyPropertyItem struct {
	Name       string
	AccessType RubyAccessorType
	Type       *string // YARD @type annotation, nil = absent
}

// RubyCallRouting represents one of four routing outcomes for a Ruby call site.
// TS used a discriminated union (tagged union); Go uses a Kind field + type switch.
//   - import:     require / require_relative → ImportPath filled
//   - properties: attr_accessor/reader/writer → AccessType + Properties filled
//   - call:       standard method call → only Kind = "call"
//   - skip:       include/extend/prepend → only Kind = "skip"
type RubyCallRouting struct {
	Kind       RubyRoutingKind
	ImportPath string            // filled only when Kind == "import"
	AccessType RubyAccessorType  // filled only when Kind == "properties"
	Properties []RubyPropertyItem // filled only when Kind == "properties"
}

// CallRoutingResult: nil means no routing applied (TS's null branch).
type CallRoutingResult = *RubyCallRouting

// CallRouter decides whether a call site should be routed to a special path.
// Returns nil for normal call sites that go through the generic pipeline.
type CallRouter func(node *gotreesitter.Node, name string, source []byte, lang *gotreesitter.Language) CallRoutingResult

// Pre-allocated singletons to avoid heap allocation on hot paths.
// TS used const CALL_RESULT / SKIP_RESULT; Go uses package-level vars.
var (
	CallResult = &RubyCallRouting{Kind: RoutingKindCall}
	SkipResult = &RubyCallRouting{Kind: RoutingKindSkip}
)

// RouteRubyCall implements the Ruby call routing logic.
//   - require / require_relative → import (ImportPath = node text)
//   - include / extend / prepend → skip (no graph node needed)
//   - attr_accessor / attr_reader / attr_writer → properties
//   - everything else → call (generic pipeline)
func RouteRubyCall(node *gotreesitter.Node, name string, source []byte, lang *gotreesitter.Language) CallRoutingResult {
	switch name {
	case "require", "require_relative":
		importPath := node.Text(source)
		return &RubyCallRouting{
			Kind:       RoutingKindImport,
			ImportPath: strings.TrimSpace(importPath),
		}

	case "include", "extend", "prepend":
		return SkipResult

	case "attr_accessor", "attr_reader", "attr_writer":
		var accessType RubyAccessorType
		switch name {
		case "attr_reader":
			accessType = AccessorReader
		case "attr_writer":
			accessType = AccessorWriter
		case "attr_accessor":
			accessType = AccessorAccess
		}

		properties := extractRubyProperties(node, accessType, source, lang)
		return &RubyCallRouting{
			Kind:       RoutingKindProperties,
			AccessType: accessType,
			Properties: properties,
		}

	default:
		return CallResult
	}
}

// extractRubyProperties extracts property items from an attr_* call node.
// Scans the node's children for symbol arguments, and checks sibling
// comments for YARD @type annotations.
func extractRubyProperties(node *gotreesitter.Node, accessType RubyAccessorType, source []byte, lang *gotreesitter.Language) []RubyPropertyItem {
	var properties []RubyPropertyItem

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "symbol" {
			propName := strings.TrimPrefix(child.Text(source), ":")
			// Check for YARD type annotation in preceding comment
			propType := extractYARDType(child, source, lang)
			properties = append(properties, RubyPropertyItem{
				Name:       propName,
				AccessType: accessType,
				Type:       propType,
			})
		}
	}

	return properties
}

// extractYARDType attempts to extract a type annotation from YARD comments
// preceding a symbol node. Looks for the pattern: # @type [TypeName]
func extractYARDType(symbolNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	parent := symbolNode.Parent()
	if parent == nil {
		return nil
	}

	// Walk siblings backwards from symbolNode to find preceding comment
	symbolIdx := -1
	for i := 0; i < parent.ChildCount(); i++ {
		if parent.Child(i) == symbolNode {
			symbolIdx = i
			break
		}
	}
	if symbolIdx < 0 {
		return nil
	}

	for i := symbolIdx - 1; i >= 0; i-- {
		sibling := parent.Child(i)
		if sibling == nil {
			continue
		}
		sibKind := sibling.Type(lang)
		if sibKind == "comment" {
			commentText := sibling.Text(source)
			// Parse YARD @type annotation: # @type [TypeName]
			if idx := strings.Index(commentText, "@type"); idx >= 0 {
				afterType := commentText[idx+5:]
				afterType = strings.TrimSpace(afterType)
				if strings.HasPrefix(afterType, "[") {
					endBracket := strings.Index(afterType, "]")
					if endBracket > 1 {
						typeName := strings.TrimSpace(afterType[1:endBracket])
						if typeName != "" {
							return &typeName
						}
					}
				}
			}
			break // Only check the immediately preceding comment
		}
		// Skip whitespace/newline nodes
		if sibKind != "\n" && sibKind != " " {
			break
		}
	}

	return nil
}