// Package variableextractors provides table-driven variable extraction from AST nodes.
//
// generic.go — Generic variable extractor factory.
//
// Follows the same config+factory pattern as field-extractors/generic.go and
// call-extractors/generic.go. Define a VariableExtractionConfig per language
// and generate extractors from configs. The factory converts node type arrays
// to maps at construction time for O(1) lookups.
//
// Ported from TS variable-extractors/generic.ts.
package variableextractors

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// classLikeContainers are type-declaration node types whose body can be a bare
// `block`. tree-sitter-python models a class body as a `block` node — the same
// node type used for function and control-flow bodies — so a class attribute
// would otherwise look block-scoped. Most other grammars give class bodies
// dedicated node types (class_body, declaration_list, body_statement), which
// are not in the block-scope list, so this guard is a no-op for them but keeps
// the rule language-agnostic.
var classLikeContainers = map[string]bool{
	"class_definition":    true,
	"class_declaration":   true,
	"class_specifier":     true,
	"struct_item":         true,
	"impl_item":           true,
	"trait_item":          true,
	"interface_declaration": true,
	"enum_declaration":    true,
	"object_declaration":  true,
}

// top-level program/module nodes indicate module/file scope.
var moduleContainerTypes = map[string]bool{
	"program":            true,
	"source_file":        true,
	"module":             true,
	"translation_unit":   true,
	"compilation_unit":   true,
}

// function/method boundary nodes indicate block scope.
var functionContainerTypes = map[string]bool{
	"function_declaration":  true,
	"function_definition":   true,
	"function_item":         true,
	"method_declaration":    true,
	"method_definition":     true,
	"arrow_function":        true,
	"function_expression":   true,
	"lambda":                true,
	"function_body":         true,
	"compound_statement":    true,
}

// CreateVariableExtractor creates a VariableExtractor from a declarative config.
func CreateVariableExtractor(config core.VariableExtractionConfig) core.VariableExtractor {
	staticNodeSet := makeStringSet(config.StaticNodeTypes)
	// Combined set for fast isVariableDeclaration checks
	allNodeTypes := makeStringSet(config.VariableNodeTypes)
	for _, t := range config.ConstNodeTypes {
		allNodeTypes[t] = true
	}
	for _, t := range config.StaticNodeTypes {
		allNodeTypes[t] = true
	}

	return &variableExtractorImpl{
		language:       config.Language,
		config:         config,
		staticNodeSet:  staticNodeSet,
		allNodeTypes:   allNodeTypes,
	}
}

type variableExtractorImpl struct {
	language      core.SupportedLanguage
	config        core.VariableExtractionConfig
	staticNodeSet map[string]bool
	allNodeTypes  map[string]bool
}

func (e *variableExtractorImpl) Language() core.SupportedLanguage { return e.language }

func (e *variableExtractorImpl) IsVariableDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return e.allNodeTypes[node.Type(lang)]
}

func (e *variableExtractorImpl) Extract(node *gotreesitter.Node, ctx *core.VariableExtractorContext, source []byte, lang *gotreesitter.Language) []core.VariableInfo {
	all := e.ExtractAll(node, ctx, source, lang)
	if len(all) == 0 {
		return nil
	}
	return all[:1]
}

func (e *variableExtractorImpl) ExtractAll(node *gotreesitter.Node, ctx *core.VariableExtractorContext, source []byte, lang *gotreesitter.Language) []core.VariableInfo {
	if !e.allNodeTypes[node.Type(lang)] {
		return nil
	}

	var names []string
	if e.config.ExtractNames != nil {
		names = e.config.ExtractNames(node, source, lang)
	} else if e.config.ExtractName != nil {
		if name := e.config.ExtractName(node, source, lang); name != "" {
			names = []string{name}
		}
	}
	if len(names) == 0 {
		return nil
	}

	// isConst/isStatic: node type membership is a hint, but config.IsConst/IsStatic
	// has final say. For languages where const and non-const share a node type
	// (e.g., TS lexical_declaration for both const and let), config.IsConst disambiguates.
	isConst := false
	if e.config.IsConst != nil {
		isConst = e.config.IsConst(node, source, lang)
	}
	isStatic := e.staticNodeSet[node.Type(lang)]
	if e.config.IsStatic != nil {
		isStatic = isStatic || e.config.IsStatic(node, source, lang)
	}
	isMutable := false
	if e.config.IsMutable != nil {
		isMutable = e.config.IsMutable(node, source, lang)
	}
	scope := e.determineScope(node, lang)

	result := make([]core.VariableInfo, 0, len(names))
	for _, name := range names {
		var typ *string
		if e.config.ExtractTypeForName != nil {
			typ = e.config.ExtractTypeForName(node, name, source, lang)
		}
		if typ == nil && e.config.ExtractType != nil {
			typ = e.config.ExtractType(node, source, lang)
		}

		visibility := core.VisibilityPrivate // default
		if e.config.ExtractVisibilityForName != nil {
			visibility = e.config.ExtractVisibilityForName(node, name, source, lang)
		} else if e.config.ExtractVisibility != nil {
			visibility = e.config.ExtractVisibility(node, source, lang)
		}

		sourceFile := ""
		line := 0
		if ctx != nil {
			sourceFile = ctx.FilePath
		}
		if node != nil {
			line = int(node.StartPoint().Row) + 1
		}

		result = append(result, core.VariableInfo{
			Name:       name,
			Type:       typ,
			Visibility: visibility,
			IsConst:    isConst,
			IsStatic:   isStatic,
			IsMutable:  isMutable,
			Scope:      scope,
			SourceFile: sourceFile,
			Line:       line,
		})
	}
	return result
}

// determineScope walks up the AST to determine the scope level:
//   - 'module': node is inside a top-level program/module/source_file container
//   - 'block': node is inside a function, method, or block scope
//   - 'file': fallback when no recognizable container is found
func (e *variableExtractorImpl) determineScope(node *gotreesitter.Node, lang *gotreesitter.Language) core.VariableScope {
	current := node.Parent()
	for current != nil {
		t := current.Type(lang)
		if moduleContainerTypes[t] {
			return core.ScopeModule
		}
		if functionContainerTypes[t] {
			return core.ScopeBlock
		}
		// A bare `block` is block scope UNLESS it is a class body. A class member
		// (e.g. Python `class C: MAX = 100`) is not an inert function-local — keep
		// walking so it resolves to its true enclosing scope ('module' for a
		// top-level class) instead of being misclassified and pruned.
		if t == "block" {
			parent := current.Parent()
			if parent == nil || !classLikeContainers[parent.Type(lang)] {
				return core.ScopeBlock
			}
		}
		current = current.Parent()
	}
	return core.ScopeFile
}

// --- helpers ---

func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}