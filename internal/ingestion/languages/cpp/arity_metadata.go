package cpp

import (
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/odvcencio/gotreesitter"
)

// CppArityInfo holds computed arity metadata for a C++ function declaration.
// Ported from TS languages/cpp/arity-metadata.ts.
type CppArityInfo struct {
	ParameterCount         int                       // Total parameter count (0 if variadic)
	RequiredParameterCount int                       // Required (non-default, non-variadic) parameter count
	ParameterTypes         []string                  // Normalized parameter type names
	ParameterTypeClasses   []shared.ParameterTypeClass // Sidecar type shape info
	HasParameterCount      bool                      // Whether ParameterCount is valid
}

// CppArityMetadata holds parameter metadata for a C++ function definition.
// Used by the provider-level ComputeCppArityMetadata entry point.
type CppArityMetadata struct {
	TotalParams    int  // total number of parameters (including defaults)
	RequiredParams int  // number of required parameters (no default value)
	IsVariadic     bool // true if the function accepts variadic arguments (...) or parameter pack
}

// ComputeCppArityMetadata computes arity metadata from a C++ symbol definition.
// This is the provider-level entry point; captures.go uses computeCppDeclarationArity
// which works directly with tree-sitter nodes.
func ComputeCppArityMetadata(def shared.SymbolDefinition) CppArityMetadata {
	total := 0
	req := 0
	if def.ParameterCount != nil {
		total = *def.ParameterCount
	}
	if def.RequiredParameterCount != nil {
		req = *def.RequiredParameterCount
	}
	isVariadic := false
	for _, pt := range def.ParameterTypes {
		if pt == "..." {
			isVariadic = true
			break
		}
	}
	return CppArityMetadata{
		TotalParams:    total,
		RequiredParams: req,
		IsVariadic:     isVariadic,
	}
}

// computeCppDeclarationArity computes arity metadata from a C++ function
// definition or declaration node.
// Extends the C arity computation with support for:
//   - optional_parameter_declaration (default parameters)
//   - variadic_parameter_declaration / parameter packs
//   - (void) explicit zero-parameter form
//
// Ported from TS languages/cpp/arity-metadata.ts computeCppDeclarationArity.
func computeCppDeclarationArity(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) CppArityInfo {
	funcDecl := findCppFuncDeclarator(lang, node)
	if funcDecl == nil {
		return CppArityInfo{}
	}

	paramList := funcDecl.ChildByFieldName("parameters", lang)
	if paramList == nil {
		return CppArityInfo{}
	}

	var params []*gotreesitter.Node
	hasEllipsis := false

	for i := 0; i < int(paramList.ChildCount()); i++ {
		child := paramList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "parameter_declaration" || ct == "optional_parameter_declaration" || ct == "variadic_parameter_declaration" {
			params = append(params, child)
		} else if ct == "..." || child.Text(source) == "..." {
			hasEllipsis = true
		}
	}

	// Empty parameter list: C++ `void foo()` means zero params (unlike C)
	if len(params) == 0 && !hasEllipsis {
		return CppArityInfo{
			ParameterCount:         0,
			RequiredParameterCount: 0,
			ParameterTypes:         []string{},
			HasParameterCount:      true,
		}
	}

	// (void) means zero parameters
	if len(params) == 1 && params[0].Type(lang) == "parameter_declaration" {
		typeNode := params[0].ChildByFieldName("type", lang)
		declaratorNode := params[0].ChildByFieldName("declarator", lang)
		if typeNode != nil && typeNode.Text(source) == "void" && declaratorNode == nil {
			return CppArityInfo{
				ParameterCount:         0,
				RequiredParameterCount: 0,
				ParameterTypes:         []string{},
				HasParameterCount:      true,
			}
		}
	}

	isVariadic := hasEllipsis
	optionalCount := 0
	requiredCount := 0

	var types []string
	var typeClasses []shared.ParameterTypeClass

	for _, p := range params {
		pt := p.Type(lang)
		if pt == "variadic_parameter_declaration" {
			isVariadic = true
			types = append(types, "...")
			typeClasses = append(typeClasses, unknownTypeClass("..."))
		} else if pt == "optional_parameter_declaration" {
			optionalCount++
			typeNode := p.ChildByFieldName("type", lang)
			rawType := "unknown"
			if typeNode != nil {
				rawType = typeNode.Text(source)
			}
			types = append(types, NormalizeCppParamType(rawType))
			declaratorNode := p.ChildByFieldName("declarator", lang)
			declText := ""
			if declaratorNode != nil {
				declText = declaratorNode.Text(source)
			}
			typeClasses = append(typeClasses, ClassifyCppParameterTypeSidecar(rawType, declText, p.Text(source)))
		} else {
			requiredCount++
			typeNode := p.ChildByFieldName("type", lang)
			rawType := "unknown"
			if typeNode != nil {
				rawType = typeNode.Text(source)
			}
			types = append(types, NormalizeCppParamType(rawType))
			declaratorNode := p.ChildByFieldName("declarator", lang)
			declText := ""
			if declaratorNode != nil {
				declText = declaratorNode.Text(source)
			}
			typeClasses = append(typeClasses, ClassifyCppParameterTypeSidecar(rawType, declText, p.Text(source)))
		}
	}

	// Append '...' for C-style variadic if not already in types
	if hasEllipsis && !containsString(types, "...") {
		types = append(types, "...")
		typeClasses = append(typeClasses, unknownTypeClass("..."))
	}

	totalNonVariadic := requiredCount + optionalCount

	result := CppArityInfo{
		RequiredParameterCount: requiredCount,
		ParameterTypes:         types,
		ParameterTypeClasses:   typeClasses,
		HasParameterCount:      true,
	}

	if isVariadic {
		// Variadic: ParameterCount is 0 and HasParameterCount is false
		// to indicate we have valid info but the count is open-ended
		result.HasParameterCount = false
	} else {
		result.ParameterCount = totalNonVariadic
	}

	return result
}

// computeCppCallArity computes the number of arguments in a call_expression node.
// Ported from TS languages/cpp/arity-metadata.ts computeCppCallArity.
func computeCppCallArity(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) int {
	argList := node.ChildByFieldName("arguments", lang)
	if argList == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		// Skip punctuation (commas, parens)
		if ct != "," && ct != "(" && ct != ")" {
			count++
		}
	}
	return count
}

// NormalizeCppParamType normalizes a C++ parameter type for overload disambiguation.
// Maps common qualified/aliased types to their canonical short forms.
// Ported from TS languages/cpp/arity-metadata.ts normalizeCppParamType.
func NormalizeCppParamType(raw string) string {
	t := strings.TrimSpace(raw)
	// Strip const, volatile, etc.
	stripCV := regexp.MustCompile(`\b(const|volatile|restrict|mutable|constexpr)\b`)
	t = stripCV.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	// Strip reference/pointer markers at the end
	stripRefPtr := regexp.MustCompile(`[&*]+\s*$`)
	t = stripRefPtr.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	// Strip template parameters (loop handles nested: Map<List<int>> → Map)
	for strings.Contains(t, "<") {
		stripped := regexp.MustCompile(`<[^<>]*>`).ReplaceAllString(t, "")
		if stripped == t {
			break // avoid infinite loop on malformed input
		}
		t = strings.TrimSpace(stripped)
	}
	t = strings.TrimSpace(t)
	// Map std:: types to canonical short forms
	stdMap := map[string]string{
		"std::string":      "string",
		"std::wstring":     "string",
		"std::string_view": "string",
		"string":           "string",
		"char":             "char",
		"int":              "int",
		"long":             "int",
		"short":            "int",
		"unsigned":         "int",
		"unsigned int":     "int",
		"long long":        "int",
		"size_t":           "int",
		"std::size_t":      "int",
		"float":            "double",
		"double":           "double",
		"bool":             "bool",
		"nullptr_t":        "null",
		"std::nullptr_t":   "null",
	}
	if mapped, ok := stdMap[t]; ok {
		return mapped
	}
	return t
}

// ClassifyCppParameterTypeSidecar classifies a C++ parameter type for the
// sidecar ParameterTypeClass shape, using the shared.ParameterTypeClass struct.
// Ported from TS languages/cpp/arity-metadata.ts classifyCppParameterType.
func ClassifyCppParameterTypeSidecar(rawType, declaratorText, fullParameterText string) shared.ParameterTypeClass {
	source := fullParameterText
	if source == "" {
		source = rawType + " " + declaratorText
		source = strings.TrimSpace(source)
	}
	if rawType == "unknown" {
		return unknownTypeClass("unknown")
	}

	hasConst := strings.Contains(source, "const")
	hasVolatile := strings.Contains(source, "volatile")

	var cv shared.CVQualifier
	if hasConst && hasVolatile {
		cv = shared.CVConstVolatile
	} else if hasConst {
		cv = shared.CVConst
	} else if hasVolatile {
		cv = shared.CVVolatile
	} else {
		cv = shared.CVNone
	}

	pointerDepth := strings.Count(source, "*")
	var indirection shared.IndirectionKind
	if pointerDepth > 0 {
		indirection = shared.IndirectionPointer
	} else if strings.Contains(source, "&&") {
		indirection = shared.IndirectionRValueRef
	} else if strings.Contains(source, "&") {
		indirection = shared.IndirectionLValueRef
	} else {
		indirection = shared.IndirectionValue
	}

	return shared.ParameterTypeClass{
		Base:         NormalizeCppParamType(rawType),
		CV:           cv,
		Indirection:  indirection,
		PointerDepth: pointerDepth,
	}
}

// unknownTypeClass returns a ParameterTypeClass with unknown CV and indirection.
func unknownTypeClass(base string) shared.ParameterTypeClass {
	return shared.ParameterTypeClass{
		Base:         base,
		CV:           shared.CVUnknown,
		Indirection:  shared.IndirectionUnknown,
		PointerDepth: 0,
	}
}

// findCppFuncDeclarator finds the function_declarator node, unwrapping
// pointer_declarator, reference_declarator, and init_declarator wrappers.
// Ported from TS languages/cpp/arity-metadata.ts findFuncDeclarator.
func findCppFuncDeclarator(lang *gotreesitter.Language, node *gotreesitter.Node) *gotreesitter.Node {
	decl := node.ChildByFieldName("declarator", lang)
	if decl == nil {
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && c.Type(lang) == "function_declarator" {
				return c
			}
		}
		return nil
	}

	// Unwrap declarator wrappers. Deleted free functions are represented as
	// init_declarator(function_declarator, delete_expression) by tree-sitter-cpp.
	for decl.Type(lang) == "pointer_declarator" || decl.Type(lang) == "reference_declarator" || decl.Type(lang) == "init_declarator" {
		next := decl.ChildByFieldName("declarator", lang)
		if next == nil {
			// reference_declarator may not use field name
			for i := 0; i < int(decl.ChildCount()); i++ {
				c := decl.Child(i)
				if c != nil && c.Type(lang) == "function_declarator" {
					return c
				}
			}
			break
		}
		decl = next
	}
	if decl.Type(lang) == "function_declarator" {
		return decl
	}
	return nil
}

// containsString checks if a string slice contains a given string.
func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}