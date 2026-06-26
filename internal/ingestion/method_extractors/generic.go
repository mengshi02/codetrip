package methodextractors

import (
	"fmt"
	"os"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// staticImplyingOwnerTypes lists owner node types that imply static member semantics.
// When config's TypeDeclarationNodes includes these types, they must also be covered
// in StaticOwnerTypes, otherwise methods would incorrectly get isStatic=false.
// Opt out: set StaticOwnerTypes: map[string]bool{} (explicit empty map) to indicate
// "I handle staticity entirely through IsStatic()".
var staticImplyingOwnerTypes = map[string]bool{
	"companion_object":   true,
	"object_declaration": true,
	"singleton_class":    true,
}

// CreateMethodExtractor creates a MethodExtractor from a declarative config.
//
// If TypeDeclarationNodes includes static-implying owner types
// (companion_object / object_declaration / singleton_class)
// but does not cover them in StaticOwnerTypes, it panics.
// This is a runtime invariant check, triggered once per language provider construction,
// to prevent silent isStatic=false regressions.
func CreateMethodExtractor(config core.MethodExtractionConfig) core.MethodExtractor {
	// Runtime invariant: every static-implying container type must be covered in StaticOwnerTypes.
	// Explicit empty map is treated as intentional opt-out.
	if config.StaticOwnerTypes == nil {
		var missing []string
		for _, t := range config.TypeDeclarationNodes {
			if staticImplyingOwnerTypes[t] {
				missing = append(missing, t)
			}
		}
		if len(missing) > 0 {
			panic(fmt.Sprintf(
				"[MethodExtractionConfig:%s] typeDeclarationNodes includes static-implying owner type(s) %v but staticOwnerTypes is not set. "+
					"Add them to staticOwnerTypes, or set staticOwnerTypes to empty map to opt out explicitly.",
				config.Language, missing,
			))
		}
	} else {
		var missing []string
		for _, t := range config.TypeDeclarationNodes {
			if staticImplyingOwnerTypes[t] && !config.StaticOwnerTypes[t] {
				missing = append(missing, t)
			}
		}
		// Explicit empty map is an opt-out signal; don't second-guess.
		if len(missing) > 0 && len(config.StaticOwnerTypes) > 0 {
			panic(fmt.Sprintf(
				"[MethodExtractionConfig:%s] typeDeclarationNodes includes static-implying owner type(s) %v that are missing from staticOwnerTypes. "+
					"Either add them to staticOwnerTypes, or set staticOwnerTypes to empty map to opt out explicitly.",
				config.Language, missing,
			))
		}
	}

	typeDeclSet := makeStringSet(config.TypeDeclarationNodes)
	methodNodeSet := makeStringSet(config.MethodNodeTypes)
	bodyNodeSet := makeStringSet(config.BodyNodeTypes)

	return &methodExtractorImpl{
		language:      config.Language,
		config:        config,
		typeDeclSet:   typeDeclSet,
		methodNodeSet: methodNodeSet,
		bodyNodeSet:   bodyNodeSet,
	}
}

type methodExtractorImpl struct {
	language      core.SupportedLanguage
	config        core.MethodExtractionConfig
	typeDeclSet   map[string]bool
	methodNodeSet map[string]bool
	bodyNodeSet   map[string]bool
}

func (e *methodExtractorImpl) Language() core.SupportedLanguage { return e.language }

func (e *methodExtractorImpl) Extract(node *gotreesitter.Node, ctx *core.MethodExtractorContext, source []byte, lang *gotreesitter.Language) *core.ExtractedMethods {
	if !e.typeDeclSet[node.Type(lang)] {
		return nil
	}

	// Resolve owner name: config hook → name field → type_identifier/simple_identifier/identifier → "Companion"
	var ownerName *string
	if e.config.ExtractOwnerName != nil {
		ownerName = e.config.ExtractOwnerName(node, source, lang)
	}
	if ownerName == nil {
		if nameField := node.ChildByFieldName("name", lang); nameField != nil {
			t := nameField.Text(source)
			ownerName = &t
		} else {
			for i := 0; i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				if child != nil &&
					(child.Type(lang) == "type_identifier" ||
						child.Type(lang) == "simple_identifier" ||
						child.Type(lang) == "identifier") {
					t := child.Text(source)
					ownerName = &t
					break
				}
			}
		}
	}
	// Unnamed companion objects use "Companion" (Kotlin convention)
	if ownerName == nil && node.Type(lang) == "companion_object" {
		t := "Companion"
		ownerName = &t
	}
	if ownerName == nil {
		return nil
	}

	var methods []core.MethodInfo
	bodies := findMethodBodies(node, e.bodyNodeSet, lang)
	for _, body := range bodies {
		extractMethodsFromBody(body, node, ctx, source, lang, e.config, e.methodNodeSet, &methods)
	}

	// Extract primary constructor (e.g. C# 12)
	if e.config.ExtractPrimaryConstructor != nil {
		if primaryCtor := e.config.ExtractPrimaryConstructor(node, ctx, source, lang); primaryCtor != nil {
			methods = append(methods, *primaryCtor)
		}
	}

	return &core.ExtractedMethods{
		OwnerName: *ownerName,
		Methods:   methods,
	}
}

func (e *methodExtractorImpl) IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return e.typeDeclSet[node.Type(lang)]
}

func (e *methodExtractorImpl) ExtractFromNode(node *gotreesitter.Node, ctx *core.MethodExtractorContext, source []byte, lang *gotreesitter.Language) *core.MethodInfo {
	if !e.methodNodeSet[node.Type(lang)] {
		return nil
	}
	return buildMethod(node, node, ctx, source, lang, e.config)
}

func (e *methodExtractorImpl) ExtractFunctionName(node *gotreesitter.Node, filePath *string, lang *gotreesitter.Language) *core.FunctionNameResult {
	if e.config.ExtractFunctionName != nil {
		return e.config.ExtractFunctionName(node, filePath, lang)
	}
	return nil
}

// findMethodBodies finds body containers for a type declaration node.
func findMethodBodies(node *gotreesitter.Node, bodyNodeSet map[string]bool, lang *gotreesitter.Language) []*gotreesitter.Node {
	var result []*gotreesitter.Node

	if bodyField := node.ChildByFieldName("body", lang); bodyField != nil && bodyNodeSet[bodyField.Type(lang)] {
		result = append(result, bodyField)
		addNestedBodies(bodyField, bodyNodeSet, &result, nil, lang)
		return result
	}

	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && bodyNodeSet[child.Type(lang)] {
			result = append(result, child)
		}
	}

	if len(result) == 0 {
		if bodyField := node.ChildByFieldName("body", lang); bodyField != nil {
			// Fallback: body field exists but its type is not in bodyNodeTypes.
			// Likely a config typo — warn in development.
			if os.Getenv("NODE_ENV") == "development" {
				fmt.Fprintf(os.Stderr,
					"[MethodExtractor] body field type '%s' not in bodyNodeTypes for node '%s'\n",
					bodyField.Type(lang), node.Type(lang),
				)
			}
			result = append(result, bodyField)
			addNestedBodies(bodyField, bodyNodeSet, &result, nil, lang)
		}
	}

	return result
}

// addNestedBodies recursively adds nested body nodes.
func addNestedBodies(
	parent *gotreesitter.Node,
	bodyNodeSet map[string]bool,
	out *[]*gotreesitter.Node,
	seen map[*gotreesitter.Node]bool,
	lang *gotreesitter.Language,
) {
	if seen == nil {
		seen = make(map[*gotreesitter.Node]bool)
		for _, n := range *out {
			seen[n] = true
		}
	}
	for i := 0; i < parent.NamedChildCount(); i++ {
		child := parent.NamedChild(i)
		if child != nil && bodyNodeSet[child.Type(lang)] && !seen[child] {
			seen[child] = true
			*out = append(*out, child)
		}
	}
}

// extractMethodsFromBody extracts methods from a body node.
func extractMethodsFromBody(
	body *gotreesitter.Node,
	ownerNode *gotreesitter.Node,
	ctx *core.MethodExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
	config core.MethodExtractionConfig,
	methodNodeSet map[string]bool,
	out *[]core.MethodInfo,
) {
	for i := 0; i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}

		// C++ template methods are wrapped in template_declaration — unwrap to inner node
		if child.Type(lang) == "template_declaration" {
			for j := 0; j < child.NamedChildCount(); j++ {
				inner := child.NamedChild(j)
				if inner != nil && methodNodeSet[inner.Type(lang)] {
					child = inner
					break
				}
			}
		}

		if methodNodeSet[child.Type(lang)] {
			if method := buildMethod(child, ownerNode, ctx, source, lang, config); method != nil {
				*out = append(*out, *method)
			}
		}

		// Recurse into enum constant anonymous class body
		if child.Type(lang) == "enum_constant" {
			for j := 0; j < child.NamedChildCount(); j++ {
				innerBody := child.NamedChild(j)
				if innerBody != nil && innerBody.Type(lang) == "class_body" {
					extractMethodsFromBody(innerBody, ownerNode, ctx, source, lang, config, methodNodeSet, out)
				}
			}
		}
	}
}

// buildMethod constructs a single MethodInfo.
func buildMethod(
	node *gotreesitter.Node,
	ownerNode *gotreesitter.Node,
	ctx *core.MethodExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
	config core.MethodExtractionConfig,
) *core.MethodInfo {
	name := config.ExtractName(node, source, lang)
	if name == nil {
		return nil
	}

	isAbstract := config.IsAbstract(node, ownerNode, source, lang)
	isFinal := config.IsFinal(node, source, lang)
	// Domain invariant: abstract methods cannot be final
	if isAbstract {
		isFinal = false
	}

	// Static owner detection: driven by config, each language declares which container node types imply static
	isStatic := (config.StaticOwnerTypes != nil && config.StaticOwnerTypes[ownerNode.Type(lang)]) || config.IsStatic(node, source, lang)

	result := &core.MethodInfo{
		Name:        *name,
		ReturnType:  config.ExtractReturnType(node, source, lang),
		Parameters:  config.ExtractParameters(node, source, lang),
		Visibility:  config.ExtractVisibility(node, source, lang),
		IsStatic:    isStatic,
		IsAbstract:  isAbstract,
		IsFinal:     isFinal,
		Annotations: nil,
		SourceFile:  ctx.FilePath,
		Line:        int(node.StartPoint().Row) + 1,
	}

	// Optional fields
	if config.ExtractReceiverType != nil {
		result.ReceiverType = config.ExtractReceiverType(node, source, lang)
	}
	if config.ExtractAnnotations != nil {
		result.Annotations = config.ExtractAnnotations(node, source, lang)
	} else {
		result.Annotations = []string{}
	}
	if config.IsVirtual != nil && config.IsVirtual(node, source, lang) {
		result.IsVirtual = true
	}
	if config.IsOverride != nil && config.IsOverride(node, source, lang) {
		result.IsOverride = true
	}
	if config.IsAsync != nil && config.IsAsync(node, source, lang) {
		result.IsAsync = true
	}
	if config.IsPartial != nil && config.IsPartial(node, source, lang) {
		result.IsPartial = true
	}
	if config.IsConst != nil && config.IsConst(node, source, lang) {
		result.IsConst = true
	}
	if config.IsDeleted != nil && config.IsDeleted(node, source, lang) {
		result.IsDeleted = true
	}

	return result
}

// makeStringSet creates a map[string]bool from a string slice.
func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
