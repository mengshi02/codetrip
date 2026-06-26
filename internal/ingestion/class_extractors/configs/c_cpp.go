package configs

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// ---------------------------------------------------------------------------
// C
// ---------------------------------------------------------------------------

// CClassConfig extracts struct/enum declarations from C source code.
// C has no namespaces or classes; only struct_specifier and enum_specifier.
var CClassConfig = core.ClassExtractionConfig{
	Language:             core.LangC,
	TypeDeclarationNodes: []string{"struct_specifier", "enum_specifier"},
}

// ---------------------------------------------------------------------------
// C++
// ---------------------------------------------------------------------------

// CppClassConfig extracts class-like declarations from C++ source code.
// Supports class_specifier, struct_specifier, enum_specifier.
// Includes ancestor scopewalking through namespaces, classes, structs, and
// named unions. Anonymous namespaces get a deterministic discriminator
// based on their start byte offset (#1995).
var CppClassConfig = core.ClassExtractionConfig{
	Language: core.LangCpp,
	TypeDeclarationNodes: []string{
		"class_specifier",
		"struct_specifier",
		"enum_specifier",
	},
	// #1995: union_specifier is included so a type nested in a NAMED union
	// qualifies as U1.Inner. Anonymous unions have no name child →
	// extractScopeSegments returns nil → they contribute nothing (members
	// inject into the enclosing scope). C uses the separate CClassConfig
	// (no qualifiedNodeId), so it is intentionally untouched.
	AncestorScopeNodeTypes: []string{
		"namespace_definition",
		"class_specifier",
		"struct_specifier",
		"union_specifier",
	},
	// #1978: key nested-type nodes by their fully-qualified path so same-tail
	// nested types in one TU stay distinct instead of silently merging.
	QualifiedNodeId: true,
	// #1995: anonymous namespaces have no name child, so the generic scope
	// walker drops them (empty segment) and two `namespace { struct Inner {} }`
	// blocks in one TU collapse onto a single Inner node. Give each anonymous
	// namespace_definition a deterministic per-block discriminator (its start
	// byte offset) so the nested types stay distinct. Returning nil for every
	// other scope — named namespaces (incl. inline namespace), classes, structs,
	// named unions — falls through to the default name-based extraction.
	ExtractScopeSegments: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
		if node.Type(lang) == "namespace_definition" && node.ChildByFieldName("name", lang) == nil {
			return []string{formatAnonNamespace(node)}
		}
		return nil
	},
	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		if nameNode.Type(lang) != "template_type" {
			return nil
		}
		// Strip template arguments from the name (e.g. "List<T>" → "List").
		t := utils.StripTemplateArguments(nameNode.Text(source))
		return &t
	},
	ExtractTemplateArguments: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil || nameNode.Type(lang) != "template_type" {
			return nil
		}
		return utils.ExtractTemplateArguments(nameNode.Text(source))
	},
	ShouldSkipClassCapture: func(ctx *core.ClassCaptureContext, nodeLabel core.ClassLikeNodeLabel, lang *gotreesitter.Language) bool {
		return shouldSkipCppTemplateDuplicateCapture(ctx, lang)
	},
	ExtractTemplateArgumentsFromCapture: func(ctx *core.ClassCaptureContext, source []byte, lang *gotreesitter.Language) []string {
		return extractCppTemplateArgumentsWithFallback(ctx, source, lang)
	},
}

// formatAnonNamespace generates a deterministic discriminator for anonymous
// namespaces using their start byte offset.
func formatAnonNamespace(node *gotreesitter.Node) string {
	return "@anon" + uint32ToString(node.StartByte())
}

// shouldSkipCppTemplateDuplicateCapture returns true when the current capture
// is a generic class capture that would duplicate a specialization-aware capture.
// If template-arguments capture exists, we're in the specialization-aware path → keep.
// Otherwise, check if the definition name itself is templated; if so, skip the
// generic capture to avoid duplicate class defs.
func shouldSkipCppTemplateDuplicateCapture(ctx *core.ClassCaptureContext, lang *gotreesitter.Language) bool {
	if _, ok := ctx.CaptureMap["template-arguments"]; ok {
		return false
	}
	if ctx.DefinitionNode == nil {
		return false
	}
	defNameNode := ctx.DefinitionNode.ChildByFieldName("name", lang)
	if defNameNode == nil {
		return false
	}
	defNameText := defNameNode.Text(nil)
	argsFromDefinitionName := utils.ExtractTemplateArguments(defNameText)
	if argsFromDefinitionName == nil {
		return false
	}
	// Check captured name
	var capturedNameText string
	if ctx.NameNode != nil {
		capturedNameText = ctx.NameNode.Text(nil)
	}
	argsFromCaptureName := utils.ExtractTemplateArguments(capturedNameText)
	// Generic class capture emits only "List", while the specialization-aware
	// capture emits "List" + @declaration.template-arguments. Skip the former
	// when the declaration name itself is templated.
	return argsFromCaptureName == nil
}

// extractCppTemplateArgumentsWithFallback tries multiple sources to extract
// template arguments: capture map → definition node name → captured name.
func extractCppTemplateArgumentsWithFallback(ctx *core.ClassCaptureContext, source []byte, lang *gotreesitter.Language) []string {
	// 1. From capture map
	if taNode, ok := ctx.CaptureMap["template-arguments"]; ok {
		if args := utils.ExtractTemplateArguments(taNode.Text(source)); args != nil {
			return args
		}
	}
	// 2. From definition node name
	if ctx.DefinitionNode != nil {
		if defNameNode := ctx.DefinitionNode.ChildByFieldName("name", lang); defNameNode != nil {
			if args := utils.ExtractTemplateArguments(defNameNode.Text(source)); args != nil {
				return args
			}
		}
	}
	// 3. From captured name node
	if ctx.NameNode != nil {
		if args := utils.ExtractTemplateArguments(ctx.NameNode.Text(source)); args != nil {
			return args
		}
	}
	return nil
}

// uint32ToString converts a uint32 to its decimal string representation.
func uint32ToString(v uint32) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	// Reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}