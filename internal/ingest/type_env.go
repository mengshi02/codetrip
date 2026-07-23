package ingest

import (
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ─────────────────────────────────────────────────────────────────────────────
// TypeEnv: per-file scoped type environment mapping (scope, variableName) → typeName.
//
// Design constraints:
//   - Explicit-only: only type annotations, never inferred types
//   - Scope-aware: function-local variables don't collide across functions
//   - Conservative: complex/generic types extract the base name only
//   - Per-file: built once, used for receiver resolution, then discarded
// ─────────────────────────────────────────────────────────────────────────────

// TypeEnv maps scope key → (variableName → typeName).
// File-level scope uses "" (empty string) key.
type TypeEnv map[string]map[string]string

const fileScope = ""

// LookupTypeEnv looks up a variable's type in the TypeEnv.
// Tries the call's enclosing function scope first, then falls back to file-level scope.
func LookupTypeEnv(env TypeEnv, varName string, callNode *sitter.Node, source []byte) string {
	scopeKey := findEnclosingScopeKey(callNode, source)
	if scopeKey != "" {
		if scopeEnv, ok := env[scopeKey]; ok {
			if result, ok2 := scopeEnv[varName]; ok2 {
				return result
			}
		}
	}
	// Fall back to file-level scope
	if fileEnv, ok := env[fileScope]; ok {
		if result := fileEnv[varName]; result != "" {
			return result
		}
	}
	// Dynamic-language instance fields are commonly initialized in a constructor
	// and consumed by sibling methods. Keep propagation class-local instead of
	// treating every same-named field in the file as interchangeable.
	pythonField := len(varName) > len("self.") && varName[:len("self.")] == "self."
	jsField := len(varName) > len("this.") && varName[:len("this.")] == "this."
	phpField := len(varName) > len("$this->") && varName[:len("$this->")] == "$this->"
	if pythonField || jsField || phpField {
		for current := callNode.Parent(); current != nil; current = current.Parent() {
			if current.Kind() != "class_definition" && current.Kind() != "class_declaration" {
				continue
			}
			var result string
			var visit func(*sitter.Node)
			visit = func(node *sitter.Node) {
				if node == nil || result != "" {
					return
				}
				if node != current && (node.Kind() == "class_definition" || node.Kind() == "class_declaration") {
					return
				}
				if FunctionNodeTypes[node.Kind()] {
					name, _ := ExtractFunctionName(node, source)
					if name == "__init__" || name == "constructor" || name == "__construct" {
						result = env[name+"@"+uintToStr(node.StartByte())][varName]
					}
					return
				}
				for i := uint(0); i < node.NamedChildCount(); i++ {
					visit(node.NamedChild(i))
				}
			}
			visit(current)
			return result
		}
	}
	return ""
}

// InferSwiftSelfType resolves Swift's contextual Self pseudo-type to the
// enclosing nominal type or protocol.
func InferSwiftSelfType(callNode *sitter.Node, source []byte) string {
	for current := callNode.Parent(); current != nil; current = current.Parent() {
		if current.Kind() != "class_declaration" && current.Kind() != "protocol_declaration" {
			continue
		}
		name := current.ChildByFieldName("name")
		if name == nil {
			continue
		}
		if typeName := extractSimpleTypeName(name, source); typeName != "" {
			return typeName
		}
	}
	return ""
}

// InferSwiftGenericReceiverType returns the leading protocol/type constraint
// for a generic receiver declared by the enclosing function. Swift permits
// static dispatch through that generic name (for example T.log where
// T: Real & FixedWidthFloatingPoint).
func InferSwiftGenericReceiverType(callNode *sitter.Node, receiverName string, source []byte) string {
	if receiverName == "" || receiverName == "Self" {
		return ""
	}
	for current := callNode.Parent(); current != nil; current = current.Parent() {
		if !FunctionNodeTypes[current.Kind()] {
			continue
		}
		declaration := current.Utf8Text(source)
		if body := strings.Index(declaration, "{"); body >= 0 {
			declaration = declaration[:body]
		}
		name := regexp.QuoteMeta(receiverName)
		patterns := []string{
			`(?:<|,)\s*` + name + `\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`,
			`\bwhere\s+` + name + `\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`,
		}
		for _, pattern := range patterns {
			match := regexp.MustCompile(pattern).FindStringSubmatch(declaration)
			if len(match) == 2 {
				return match[1]
			}
		}
		return ""
	}
	return ""
}

// findEnclosingScopeKey finds the enclosing function name for scope lookup.
// Returns "funcName@startIndex" or "" for file-level scope.
func findEnclosingScopeKey(node *sitter.Node, source []byte) string {
	current := node.Parent()
	for current != nil {
		if FunctionNodeTypes[current.Kind()] {
			funcName, _ := ExtractFunctionName(current, source)
			if funcName != "" {
				return funcName + "@" + uintToStr(current.StartByte())
			}
			return "anonymous@" + uintToStr(current.StartByte())
		}
		current = current.Parent()
	}
	return ""
}

// uintToStr converts a uint to string without importing strconv (tiny helper).
func uintToStr(v uint) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// BuildTypeEnv builds a scoped TypeEnv from a tree-sitter AST for a given language.
// Walks the tree tracking enclosing function scopes.
func BuildTypeEnv(rootNode *sitter.Node, language string, source []byte) TypeEnv {
	env := make(TypeEnv)
	walkForTypes(rootNode, language, env, fileScope, source)
	return env
}

// walkForTypes recursively walks the AST, tracking scope boundaries.
func walkForTypes(node *sitter.Node, language string, env TypeEnv, currentScope string, source []byte) {
	// Detect scope boundaries (function/method definitions)
	scope := currentScope
	if FunctionNodeTypes[node.Kind()] {
		funcName, _ := ExtractFunctionName(node, source)
		if funcName != "" {
			scope = funcName + "@" + uintToStr(node.StartByte())
		} else {
			scope = "anonymous@" + uintToStr(node.StartByte())
		}
	}

	// Get or create the sub-map for this scope
	if _, ok := env[scope]; !ok {
		env[scope] = make(map[string]string)
	}
	scopeEnv := env[scope]

	// Check if this node provides type information
	extractTypeBinding(node, language, scopeEnv, source)

	// Recurse into children
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil {
			walkForTypes(child, language, env, scope, source)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// extractTypeBinding: dispatch to per-language type extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractTypeBinding(node *sitter.Node, language string, env map[string]string, source []byte) {
	// === PARAMETERS (most languages) ===
	// This guard eliminates 90%+ of calls before any language dispatch.
	if TypedParameterTypes[node.Kind()] {
		config := getTypeConfig(language)
		config.extractParameter(node, env, source)
		return
	}

	// === Per-language declaration extraction ===
	config := getTypeConfig(language)
	if config.declarationNodeTypes[node.Kind()] {
		config.extractDeclaration(node, env, source)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TYPED_PARAMETER_TYPES: node types for function/method parameters with type annotations.
// ─────────────────────────────────────────────────────────────────────────────

var TypedParameterTypes = map[string]bool{
	"required_parameter":           true, // TS: (x: Foo)
	"optional_parameter":           true, // TS: (x?: Foo)
	"formal_parameter":             true, // Java/Kotlin
	"parameter":                    true, // C#/Rust/Go/Python/Swift
	"parameter_declaration":        true, // C/C++ void f(Type name)
	"simple_parameter":             true, // PHP function(Foo $x)
	"typed_parameter":              true, // Python def f(x: Foo)
	"property_promotion_parameter": true, // PHP constructor property promotion
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared type extraction helpers.
// ─────────────────────────────────────────────────────────────────────────────

// extractSimpleTypeName extracts the simple type name from a type AST node.
// Handles generic types, qualified names, nullable types, pointers, etc.
// Returns "" for complex types (unions, intersections, function types).
func extractSimpleTypeName(typeNode *sitter.Node, source []byte) string {
	if typeNode == nil {
		return ""
	}
	kind := typeNode.Kind()

	// Direct type identifier
	if kind == "type_identifier" || kind == "identifier" || kind == "simple_identifier" {
		return typeNode.Utf8Text(source)
	}

	// Qualified/scoped names: take the last segment (e.g., models.User → User)
	if kind == "scoped_identifier" || kind == "qualified_identifier" ||
		kind == "scoped_type_identifier" || kind == "qualified_name" ||
		kind == "qualified_type" || kind == "member_expression" || kind == "attribute" {
		last := typeNode.NamedChild(typeNode.NamedChildCount() - 1)
		if last != nil {
			return extractSimpleTypeName(last, source)
		}
	}

	// Generic types: extract the base type (e.g., List<User> → List)
	if kind == "generic_type" || kind == "generic_name" || kind == "parameterized_type" || kind == "template_type" {
		base := typeNode.ChildByFieldName("name")
		if base == nil {
			base = typeNode.ChildByFieldName("type")
		}
		if base == nil && typeNode.NamedChildCount() > 0 {
			base = typeNode.NamedChild(0)
		}
		if base != nil {
			return extractSimpleTypeName(base, source)
		}
	}

	// Nullable types (Kotlin User?, C# User?)
	if kind == "nullable_type" {
		if typeNode.NamedChildCount() > 0 {
			return extractSimpleTypeName(typeNode.NamedChild(0), source)
		}
	}

	// Type annotations that wrap the actual type
	if kind == "type_annotation" || kind == "type" || kind == "user_type" || kind == "type_descriptor" {
		if typeNode.NamedChildCount() > 0 {
			return extractSimpleTypeName(typeNode.NamedChild(0), source)
		}
	}

	// Pointer/reference types (C++, Rust)
	if kind == "pointer_type" || kind == "reference_type" {
		if typeNode.NamedChildCount() > 0 {
			return extractSimpleTypeName(typeNode.NamedChild(0), source)
		}
	}

	// PHP named_type / optional_type
	if kind == "named_type" || kind == "optional_type" {
		inner := typeNode.ChildByFieldName("name")
		if inner == nil && typeNode.NamedChildCount() > 0 {
			inner = typeNode.NamedChild(0)
		}
		if inner != nil {
			return extractSimpleTypeName(inner, source)
		}
	}

	// Name node (PHP)
	if kind == "name" {
		return typeNode.Utf8Text(source)
	}

	return ""
}

// extractVarName extracts the variable name from a declarator or pattern node.
// Returns "" for destructuring/complex patterns.
func extractVarName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	kind := node.Kind()
	if kind == "identifier" || kind == "field_identifier" || kind == "property_identifier" || kind == "simple_identifier" || kind == "variable_name" || kind == "name" {
		return node.Utf8Text(source)
	}
	// variable_declarator (Java/C#): has a 'name' field
	if kind == "variable_declarator" {
		nameChild := node.ChildByFieldName("name")
		if nameChild != nil {
			return extractVarName(nameChild, source)
		}
	}
	return ""
}

// findChildByTypeIn finds the first named child with the given node type.
func findChildByTypeIn(node *sitter.Node, targetType string) *sitter.Node {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Kind() == targetType {
			return child
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-language type extraction configurations.
// ─────────────────────────────────────────────────────────────────────────────

// LanguageTypeConfig holds per-language type extraction configuration.
type LanguageTypeConfig struct {
	declarationNodeTypes map[string]bool
	extractDeclaration   func(node *sitter.Node, env map[string]string, source []byte)
	extractParameter     func(node *sitter.Node, env map[string]string, source []byte)
}

// getTypeConfig returns the LanguageTypeConfig for the given language.
func getTypeConfig(language string) *LanguageTypeConfig {
	config, ok := typeConfigs[language]
	if !ok {
		return &noopTypeConfig
	}
	return config
}

// noopTypeConfig is a no-op config used for unsupported languages.
var noopTypeConfig = LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{},
	extractDeclaration:   func(_ *sitter.Node, _ map[string]string, _ []byte) {},
	extractParameter:     func(_ *sitter.Node, _ map[string]string, _ []byte) {},
}

// typeConfigs maps language ID → LanguageTypeConfig.
var typeConfigs = map[string]*LanguageTypeConfig{
	"javascript": tsTypeConfig,
	"typescript": tsTypeConfig,
	"tsx":        tsTypeConfig,
	"java":       javaTypeConfig,
	"kotlin":     kotlinTypeConfig,
	"csharp":     csharpTypeConfig,
	"go":         goTypeConfig,
	"rust":       rustTypeConfig,
	"python":     pythonTypeConfig,
	"swift":      swiftTypeConfig,
	"c":          cCppTypeConfig,
	"cpp":        cCppTypeConfig,
	"php":        phpTypeConfig,
}

// ─────────────────────────────────────────────────────────────────────────────
// TypeScript/JavaScript type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var tsTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"lexical_declaration":     true,
		"variable_declaration":    true,
		"public_field_definition": true,
		"assignment_expression":   true,
	},
	extractDeclaration: tsExtractDeclaration,
	extractParameter:   tsExtractParameter,
}

func tsExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	if node.Kind() == "assignment_expression" {
		left := node.ChildByFieldName("left")
		right := node.ChildByFieldName("right")
		name := extractReceiverBinding(left, source)
		var typeName string
		if right != nil && right.Kind() == "new_expression" {
			typeName = extractSimpleTypeName(right.ChildByFieldName("constructor"), source)
		} else if right != nil && right.Kind() == "identifier" {
			typeName = env[right.Utf8Text(source)]
		}
		if name != "" && typeName != "" {
			env[name] = typeName
		}
		return
	}
	if node.Kind() == "public_field_definition" {
		nameNode := node.ChildByFieldName("name")
		typeNode := node.ChildByFieldName("type")
		name := extractVarName(nameNode, source)
		typeName := extractSimpleTypeName(typeNode, source)
		if name != "" && typeName != "" {
			env["this."+name] = typeName
		}
		return
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		declarator := node.NamedChild(i)
		if declarator == nil || declarator.Kind() != "variable_declarator" {
			continue
		}
		nameNode := declarator.ChildByFieldName("name")
		typeAnnotation := declarator.ChildByFieldName("type")
		if nameNode == nil {
			continue
		}
		varName := extractVarName(nameNode, source)
		typeName := extractSimpleTypeName(typeAnnotation, source)
		if typeName == "" {
			value := declarator.ChildByFieldName("value")
			if value != nil && value.Kind() == "new_expression" {
				typeName = extractSimpleTypeName(value.ChildByFieldName("constructor"), source)
			}
		}
		if varName != "" && typeName != "" {
			env[varName] = typeName
		}
	}
}

func tsExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	kind := node.Kind()

	if kind == "required_parameter" || kind == "optional_parameter" {
		nameNode = node.ChildByFieldName("pattern")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("name")
		}
		typeNode = node.ChildByFieldName("type")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}

	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Java type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var javaTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"local_variable_declaration": true,
		"field_declaration":          true,
	},
	extractDeclaration: javaExtractDeclaration,
	extractParameter:   javaExtractParameter,
}

func javaExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return
	}
	typeName := extractSimpleTypeName(typeNode, source)
	if typeName == "" {
		return
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Kind() != "variable_declarator" {
			continue
		}
		nameNode := child.ChildByFieldName("name")
		if nameNode != nil {
			varName := extractVarName(nameNode, source)
			if varName != "" {
				env[varName] = typeName
			}
		}
	}
}

func javaExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "formal_parameter" {
		typeNode = node.ChildByFieldName("type")
		nameNode = node.ChildByFieldName("name")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Kotlin type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var kotlinTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"property_declaration": true,
		"variable_declaration": true,
	},
	extractDeclaration: kotlinExtractDeclaration,
	extractParameter:   kotlinExtractParameter,
}

func kotlinExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	if node.Kind() == "property_declaration" {
		varDecl := findChildByTypeIn(node, "variable_declaration")
		if varDecl != nil {
			nameNode := findChildByTypeIn(varDecl, "simple_identifier")
			typeNode := findChildByTypeIn(varDecl, "user_type")
			if nameNode == nil || typeNode == nil {
				return
			}
			varName := extractVarName(nameNode, source)
			typeName := extractSimpleTypeName(typeNode, source)
			if varName != "" && typeName != "" {
				env[varName] = typeName
			}
			return
		}
		// Fallback: try direct fields
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = findChildByTypeIn(node, "simple_identifier")
		}
		typeNode := node.ChildByFieldName("type")
		if typeNode == nil {
			typeNode = findChildByTypeIn(node, "user_type")
		}
		if nameNode == nil || typeNode == nil {
			return
		}
		varName := extractVarName(nameNode, source)
		typeName := extractSimpleTypeName(typeNode, source)
		if varName != "" && typeName != "" {
			env[varName] = typeName
		}
	} else if node.Kind() == "variable_declaration" {
		nameNode := findChildByTypeIn(node, "simple_identifier")
		typeNode := findChildByTypeIn(node, "user_type")
		if nameNode != nil && typeNode != nil {
			varName := extractVarName(nameNode, source)
			typeName := extractSimpleTypeName(typeNode, source)
			if varName != "" && typeName != "" {
				env[varName] = typeName
			}
		}
	}
}

func kotlinExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "formal_parameter" {
		typeNode = node.ChildByFieldName("type")
		nameNode = node.ChildByFieldName("name")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	// tree-sitter-kotlin exposes parameter children positionally rather than
	// through name/type fields in current grammar versions.
	if nameNode == nil {
		nameNode = findChildByTypeIn(node, "simple_identifier")
	}
	if typeNode == nil {
		typeNode = findChildByTypeIn(node, "user_type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// C# type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var csharpTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"local_declaration_statement": true,
		"variable_declaration":        true,
		"field_declaration":           true,
	},
	extractDeclaration: csharpExtractDeclaration,
	extractParameter:   csharpExtractParameter,
}

func csharpExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	// C# tree-sitter: local_declaration_statement > variable_declaration > ...
	// Recursively descend through wrapper nodes
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Kind() == "variable_declaration" || child.Kind() == "local_declaration_statement" {
			csharpExtractDeclaration(child, env, source)
			return
		}
	}

	// At variable_declaration level: first child is type, rest are variable_declarators
	var typeNode *sitter.Node
	var declarators []*sitter.Node

	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if typeNode == nil && child.Kind() != "variable_declarator" && child.Kind() != "equals_value_clause" {
			typeNode = child
		}
		if child.Kind() == "variable_declarator" {
			declarators = append(declarators, child)
		}
	}

	if typeNode == nil || len(declarators) == 0 {
		return
	}

	// Handle 'var x = new Foo()' — infer from object_creation_expression
	var typeName string
	if typeNode.Kind() == "implicit_type" && typeNode.Utf8Text(source) == "var" {
		if len(declarators) == 1 {
			initializer := findChildByTypeIn(declarators[0], "object_creation_expression")
			if initializer == nil {
				eqClause := findChildByTypeIn(declarators[0], "equals_value_clause")
				if eqClause != nil && eqClause.NamedChildCount() > 0 {
					firstChild := eqClause.NamedChild(0)
					if firstChild != nil && firstChild.Kind() == "object_creation_expression" {
						initializer = firstChild
					}
				}
			}
			if initializer != nil && initializer.Kind() == "object_creation_expression" {
				ctorType := initializer.ChildByFieldName("type")
				if ctorType != nil {
					typeName = extractSimpleTypeName(ctorType, source)
				}
			}
		}
	} else {
		typeName = extractSimpleTypeName(typeNode, source)
	}

	if typeName == "" {
		return
	}
	for _, decl := range declarators {
		nameNode := decl.ChildByFieldName("name")
		if nameNode == nil && decl.NamedChildCount() > 0 {
			nameNode = decl.NamedChild(0)
		}
		if nameNode != nil {
			varName := extractVarName(nameNode, source)
			if varName != "" {
				env[varName] = typeName
			}
		}
	}
}

func csharpExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter" {
		typeNode = node.ChildByFieldName("type")
		nameNode = node.ChildByFieldName("name")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Go type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var goTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"var_declaration":       true,
		"var_spec":              true,
		"short_var_declaration": true,
	},
	extractDeclaration: goExtractDeclaration,
	extractParameter:   goExtractParameter,
}

func goExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	if node.Kind() == "var_declaration" {
		for i := uint(0); i < node.NamedChildCount(); i++ {
			spec := node.NamedChild(i)
			if spec != nil && spec.Kind() == "var_spec" {
				goExtractDeclaration(spec, env, source)
			}
		}
		return
	}

	if node.Kind() == "var_spec" {
		nameNode := node.ChildByFieldName("name")
		typeNode := node.ChildByFieldName("type")
		if nameNode == nil || typeNode == nil {
			return
		}
		varName := extractVarName(nameNode, source)
		typeName := extractSimpleTypeName(typeNode, source)
		if varName != "" && typeName != "" {
			env[varName] = typeName
		}
		return
	}

	if node.Kind() == "short_var_declaration" {
		left := node.ChildByFieldName("left")
		right := node.ChildByFieldName("right")
		if left == nil || right == nil {
			return
		}

		// Collect LHS names and RHS values
		var lhsNodes, rhsNodes []*sitter.Node
		if left.Kind() == "expression_list" {
			for i := uint(0); i < left.NamedChildCount(); i++ {
				c := left.NamedChild(i)
				if c != nil {
					lhsNodes = append(lhsNodes, c)
				}
			}
		} else {
			lhsNodes = append(lhsNodes, left)
		}
		if right.Kind() == "expression_list" {
			for i := uint(0); i < right.NamedChildCount(); i++ {
				c := right.NamedChild(i)
				if c != nil {
					rhsNodes = append(rhsNodes, c)
				}
			}
		} else {
			rhsNodes = append(rhsNodes, right)
		}

		// Pair each LHS name with its corresponding RHS value
		count := len(lhsNodes)
		if len(rhsNodes) < count {
			count = len(rhsNodes)
		}
		for i := 0; i < count; i++ {
			valueNode := rhsNodes[i]
			if valueNode.Kind() != "composite_literal" {
				continue
			}
			typeNode := valueNode.ChildByFieldName("type")
			if typeNode == nil {
				continue
			}
			typeName := extractSimpleTypeName(typeNode, source)
			if typeName == "" {
				continue
			}
			varName := extractVarName(lhsNodes[i], source)
			if varName != "" {
				env[varName] = typeName
			}
		}
	}
}

func goExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter" {
		nameNode = node.ChildByFieldName("name")
		typeNode = node.ChildByFieldName("type")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rust type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var rustTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"let_declaration":   true,
		"field_declaration": true,
	},
	extractDeclaration: rustExtractDeclaration,
	extractParameter:   rustExtractParameter,
}

func rustExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	if node.Kind() == "field_declaration" {
		nameNode := node.ChildByFieldName("name")
		typeNode := node.ChildByFieldName("type")
		name := extractVarName(nameNode, source)
		typeName := extractSimpleTypeName(typeNode, source)
		if name != "" && typeName != "" {
			env["self."+name] = typeName
		}
		return
	}
	pattern := node.ChildByFieldName("pattern")
	typeNode := node.ChildByFieldName("type")
	if pattern == nil {
		return
	}
	varName := extractVarName(pattern, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if typeName == "" {
		value := node.ChildByFieldName("value")
		if value != nil && value.Kind() == "struct_expression" {
			typeName = extractSimpleTypeName(value.ChildByFieldName("name"), source)
		}
	}
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

func rustExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter" {
		nameNode = node.ChildByFieldName("pattern")
		typeNode = node.ChildByFieldName("type")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Python type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var pythonTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"assignment": true,
	},
	extractDeclaration: pythonExtractDeclaration,
	extractParameter:   pythonExtractParameter,
}

func pythonExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	left := node.ChildByFieldName("left")
	typeNode := node.ChildByFieldName("type")
	if left == nil {
		return
	}
	varName := extractVarName(left, source)
	if varName == "" && left.Kind() == "attribute" {
		varName = extractReceiverBinding(left, source)
	}
	var typeName string
	if typeNode != nil {
		typeName = extractSimpleTypeName(typeNode, source)
	}
	if typeName == "" {
		right := node.ChildByFieldName("right")
		if right != nil && right.Kind() == "call" {
			function := right.ChildByFieldName("function")
			if function != nil {
				typeName = extractSimpleTypeName(function, source)
			}
		} else if right != nil && right.Kind() == "identifier" {
			typeName = env[right.Utf8Text(source)]
		}
	}
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

func pythonExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter" {
		nameNode = node.ChildByFieldName("name")
		typeNode = node.ChildByFieldName("type")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		if nameNode == nil && node.Kind() == "typed_parameter" && node.NamedChildCount() > 0 {
			nameNode = node.NamedChild(0)
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Swift type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var swiftTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"property_declaration": true,
	},
	extractDeclaration: swiftExtractDeclaration,
	extractParameter:   swiftExtractParameter,
}

func swiftExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	pattern := node.ChildByFieldName("pattern")
	if pattern == nil {
		pattern = findChildByTypeIn(node, "pattern")
	}
	typeAnnotation := node.ChildByFieldName("type")
	if typeAnnotation == nil {
		typeAnnotation = findChildByTypeIn(node, "type_annotation")
	}
	if pattern == nil || typeAnnotation == nil {
		return
	}
	varName := extractVarName(pattern, source)
	if varName == "" {
		varName = pattern.Utf8Text(source)
	}
	typeName := extractSimpleTypeName(typeAnnotation, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

func swiftExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter" {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("internal_name")
		}
		typeNode = node.ChildByFieldName("type")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// C/C++ type extraction.
// ─────────────────────────────────────────────────────────────────────────────

var cCppTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{
		"declaration":       true,
		"field_declaration": true,
		"for_range_loop":    true,
	},
	extractDeclaration: cCppExtractDeclaration,
	extractParameter:   cCppExtractParameter,
}

func cCppExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return
	}
	typeName := extractCppReceiverTypeName(typeNode, source)
	if typeName == "" {
		return
	}

	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return
	}

	// init_declarator: Type x = value
	var nameNode *sitter.Node
	if declarator.Kind() == "init_declarator" {
		nameNode = declarator.ChildByFieldName("declarator")
	} else {
		nameNode = declarator
	}
	if nameNode == nil {
		return
	}

	// Handle pointer/reference declarators
	finalName := nameNode
	for finalName != nil && (finalName.Kind() == "pointer_declarator" || finalName.Kind() == "reference_declarator") {
		next := finalName.ChildByFieldName("declarator")
		if next == nil && finalName.NamedChildCount() > 0 {
			next = finalName.NamedChild(finalName.NamedChildCount() - 1)
		}
		if next == nil {
			break
		}
		finalName = next
	}
	if finalName != nil && finalName.Kind() == "function_declarator" && hasAncestorKind(node, "compound_statement") {
		finalName = finalName.ChildByFieldName("declarator")
	}
	if finalName == nil {
		return
	}

	varName := extractVarName(finalName, source)
	if varName != "" {
		env[varName] = typeName
	}
}

var cppTransparentReceiverWrappers = map[string]bool{
	"unique_ptr":        true,
	"shared_ptr":        true,
	"weak_ptr":          true,
	"optional":          true,
	"reference_wrapper": true,
}

// extractCppReceiverTypeName unwraps standard ownership/value wrappers whose
// member access is semantically directed at their first template argument.
func extractCppReceiverTypeName(typeNode *sitter.Node, source []byte) string {
	typeName := extractSimpleTypeName(typeNode, source)
	if !cppTransparentReceiverWrappers[typeName] {
		return typeName
	}
	var findArguments func(*sitter.Node) *sitter.Node
	findArguments = func(node *sitter.Node) *sitter.Node {
		if node == nil {
			return nil
		}
		if node.Kind() == "template_argument_list" {
			return node
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			if found := findArguments(node.NamedChild(i)); found != nil {
				return found
			}
		}
		return nil
	}
	arguments := findArguments(typeNode)
	if arguments == nil || arguments.NamedChildCount() == 0 {
		return typeName
	}
	return extractSimpleTypeName(arguments.NamedChild(0), source)
}

func hasAncestorKind(node *sitter.Node, kind string) bool {
	for current := node.Parent(); current != nil; current = current.Parent() {
		if current.Kind() == kind {
			return true
		}
	}
	return false
}

func cCppExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "parameter_declaration" {
		typeNode = node.ChildByFieldName("type")
		declarator := node.ChildByFieldName("declarator")
		if declarator != nil {
			nameNode = declarator
			for nameNode != nil && (nameNode.Kind() == "pointer_declarator" || nameNode.Kind() == "reference_declarator") {
				next := nameNode.ChildByFieldName("declarator")
				if next == nil && nameNode.NamedChildCount() > 0 {
					next = nameNode.NamedChild(nameNode.NamedChildCount() - 1)
				}
				if next == nil {
					break
				}
				nameNode = next
			}
		}
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PHP type extraction.
// PHP has no local variable type annotations; only params carry types.
// ─────────────────────────────────────────────────────────────────────────────

var phpTypeConfig = &LanguageTypeConfig{
	declarationNodeTypes: map[string]bool{"assignment_expression": true},
	extractDeclaration:   phpExtractDeclaration,
	extractParameter:     phpExtractParameter,
}

func phpExtractDeclaration(node *sitter.Node, env map[string]string, source []byte) {
	left := node.ChildByFieldName("left")
	right := node.ChildByFieldName("right")
	name := extractReceiverBinding(left, source)
	if name == "" {
		name = extractVarName(left, source)
	}
	var typeName string
	if right != nil && right.Kind() == "object_creation_expression" {
		typeName = extractSimpleTypeName(right.ChildByFieldName("type"), source)
		if typeName == "" && right.NamedChildCount() > 0 {
			typeName = extractSimpleTypeName(right.NamedChild(0), source)
		}
	} else if right != nil && right.Kind() == "variable_name" {
		typeName = env[right.Utf8Text(source)]
	}
	if name != "" && typeName != "" {
		env[name] = typeName
	}
}

func phpExtractParameter(node *sitter.Node, env map[string]string, source []byte) {
	var nameNode, typeNode *sitter.Node
	if node.Kind() == "simple_parameter" {
		typeNode = node.ChildByFieldName("type")
		nameNode = node.ChildByFieldName("name")
	} else {
		nameNode = node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern")
		}
		typeNode = node.ChildByFieldName("type")
	}
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := extractVarName(nameNode, source)
	typeName := extractSimpleTypeName(typeNode, source)
	if varName != "" && typeName != "" {
		env[varName] = typeName
		if (node.Kind() == "simple_parameter" || node.Kind() == "property_promotion_parameter") && findChildByTypeIn(node, "visibility_modifier") != nil {
			env["$this->"+strings.TrimPrefix(varName, "$")] = typeName
		}
	}
}
