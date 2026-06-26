package typeextractors

import (
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// FirstNamedChild returns the first named child of a node.
// gotreesitter does not expose FirstNamedChild() directly.
func FirstNamedChild(n *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if n == nil {
		return nil
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child != nil && child.IsNamed() {
			return child
		}
	}
	return nil
}

// LastNamedChild returns the last named child of a node.
// gotreesitter does not expose LastNamedChild() directly.
func LastNamedChild(n *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if n == nil {
		return nil
	}
	count := n.NamedChildCount()
	if count == 0 {
		return nil
	}
	return n.NamedChild(count - 1)
}

// ---------------------------------------------------------------------------
// Container type descriptors
// ---------------------------------------------------------------------------

// ContainerDescriptor describes which type parameter position each access method yields.
type ContainerDescriptor struct {
	Arity        int             // 1 = single-element, 2 = key-value
	KeyMethods   map[string]bool // methods that yield the first type parameter (key type)
	ValueMethods map[string]bool // methods that yield the last type parameter (value type)
}

// Pre-defined method sets shared across containers.
var (
	noKeys           = map[string]bool{}
	stdKeyMethods    = map[string]bool{"keys": true}
	javaKeyMethods   = map[string]bool{"keySet": true}
	csharpKeyMethods = map[string]bool{"Keys": true}

	stdValueMethods      = map[string]bool{"values": true, "get": true, "pop": true, "remove": true}
	csharpValueMethods   = map[string]bool{"Values": true, "TryGetValue": true}
	singleElementMethods = map[string]bool{
		"iter": true, "into_iter": true, "iterator": true, "get": true,
		"first": true, "last": true, "pop": true, "peek": true,
		"poll": true, "find": true, "filter": true, "map": true,
	}
)

// ContainerDescriptors maps container base names to their type parameter semantics.
var ContainerDescriptors = map[string]*ContainerDescriptor{
	// --- Map / Dict types (arity 2: key + value) ---
	"Map":                  {2, stdKeyMethods, stdValueMethods},
	"WeakMap":              {2, stdKeyMethods, stdValueMethods},
	"HashMap":              {2, stdKeyMethods, stdValueMethods},
	"BTreeMap":             {2, stdKeyMethods, stdValueMethods},
	"LinkedHashMap":        {2, javaKeyMethods, stdValueMethods},
	"TreeMap":              {2, javaKeyMethods, stdValueMethods},
	"dict":                 {2, stdKeyMethods, stdValueMethods},
	"Dict":                 {2, stdKeyMethods, stdValueMethods},
	"Dictionary":           {2, csharpKeyMethods, csharpValueMethods},
	"SortedDictionary":     {2, csharpKeyMethods, csharpValueMethods},
	"Record":               {2, stdKeyMethods, stdValueMethods},
	"OrderedDict":          {2, stdKeyMethods, stdValueMethods},
	"ConcurrentHashMap":    {2, javaKeyMethods, stdValueMethods},
	"ConcurrentDictionary": {2, csharpKeyMethods, csharpValueMethods},
	"MutableMap":           {2, stdKeyMethods, stdValueMethods},

	// --- Single-element containers (arity 1) ---
	"Array":                {1, noKeys, singleElementMethods},
	"List":                 {1, noKeys, singleElementMethods},
	"ArrayList":            {1, noKeys, singleElementMethods},
	"LinkedList":           {1, noKeys, singleElementMethods},
	"Vec":                  {1, noKeys, singleElementMethods},
	"VecDeque":             {1, noKeys, singleElementMethods},
	"Set":                  {1, noKeys, singleElementMethods},
	"HashSet":              {1, noKeys, singleElementMethods},
	"BTreeSet":             {1, noKeys, singleElementMethods},
	"TreeSet":              {1, noKeys, singleElementMethods},
	"Queue":                {1, noKeys, singleElementMethods},
	"Deque":                {1, noKeys, singleElementMethods},
	"Stack":                {1, noKeys, singleElementMethods},
	"Sequence":             {1, noKeys, singleElementMethods},
	"Iterable":             {1, noKeys, singleElementMethods},
	"Iterator":             {1, noKeys, singleElementMethods},
	"IEnumerable":          {1, noKeys, singleElementMethods},
	"IList":                {1, noKeys, singleElementMethods},
	"ICollection":          {1, noKeys, singleElementMethods},
	"Collection":           {1, noKeys, singleElementMethods},
	"ObservableCollection": {1, noKeys, singleElementMethods},
	"IEnumerator":          {1, noKeys, singleElementMethods},
	"SortedSet":            {1, noKeys, singleElementMethods},
	"Stream":               {1, noKeys, singleElementMethods},
	"MutableList":          {1, noKeys, singleElementMethods},
	"MutableSet":           {1, noKeys, singleElementMethods},
	"LinkedHashSet":        {1, noKeys, singleElementMethods},
	"ArrayDeque":           {1, noKeys, singleElementMethods},
	"PriorityQueue":        {1, noKeys, singleElementMethods},
	"list":                 {1, noKeys, singleElementMethods},
	"set":                  {1, noKeys, singleElementMethods},
	"tuple":                {1, noKeys, singleElementMethods},
	"frozenset":            {1, noKeys, singleElementMethods},
}

// MethodToTypeArgPosition determines which type arg to extract based on container
// type name and access method.
func MethodToTypeArgPosition(methodName string, containerTypeName string) TypeArgPosition {
	if containerTypeName != "" {
		desc := ContainerDescriptors[containerTypeName]
		if desc != nil {
			if desc.Arity == 1 {
				return TypeArgLast
			}
			if methodName != "" && desc.KeyMethods[methodName] {
				return TypeArgFirst
			}
			return TypeArgLast
		}
	}
	// Fallback heuristic
	if methodName == "keys" || methodName == "keySet" || methodName == "Keys" {
		return TypeArgFirst
	}
	return TypeArgLast
}

// GetContainerDescriptor looks up the container descriptor for a type name.
func GetContainerDescriptor(typeName string) *ContainerDescriptor {
	return ContainerDescriptors[typeName]
}

// ---------------------------------------------------------------------------
// resolveIterableElementType — 3-strategy fallback
// ---------------------------------------------------------------------------

// ExtractFromTypeNodeFunc is a language-specific function to extract element type from AST node.
type ExtractFromTypeNodeFunc func(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos TypeArgPosition) *string

// FindParamElementTypeFunc is a language-specific AST walk to find parameter type.
type FindParamElementTypeFunc func(name string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos TypeArgPosition) *string

// ResolveIterableElementType resolves the element type of a container variable
// using a 3-strategy fallback:
// 1. declarationTypeNodes — raw AST type annotation node
// 2. scopeEnv string — ExtractElementTypeFromString on the stored type string
// 3. AST walk — language-specific upward walk to enclosing function parameters
func ResolveIterableElementType(
	iterableName string,
	node *gotreesitter.Node,
	source []byte,
	lang *gotreesitter.Language,
	scopeEnv map[string]string,
	declarationTypeNodes map[string]*gotreesitter.Node,
	scope string,
	extractFromTypeNode ExtractFromTypeNodeFunc,
	findParamElementType FindParamElementTypeFunc,
	typeArgPos TypeArgPosition,
) *string {
	// Strategy 1: declarationTypeNodes AST node
	key := scope + "\x00" + iterableName
	typeNode := declarationTypeNodes[key]
	if typeNode == nil && scope != "" {
		typeNode = declarationTypeNodes["\x00"+iterableName]
	}
	if typeNode != nil {
		if t := extractFromTypeNode(typeNode, source, lang, typeArgPos); t != nil {
			return t
		}
	}
	// Strategy 2: scopeEnv string → ExtractElementTypeFromString
	if iterableType, ok := scopeEnv[iterableName]; ok {
		if el := ExtractElementTypeFromString(iterableType, typeArgPos); el != nil {
			return el
		}
	}
	// Strategy 3: AST walk to function parameters
	if findParamElementType != nil {
		return findParamElementType(iterableName, node, source, lang, typeArgPos)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Nullable wrapper types
// ---------------------------------------------------------------------------

// NullableWrapperTypes are single-arg nullable wrapper types that unwrap to their inner type.
var NullableWrapperTypes = map[string]bool{
	"Optional": true, // Java
	"Option":   true, // Rust, Scala
	"Maybe":    true, // Haskell-style, Kotlin Arrow
}

// ---------------------------------------------------------------------------
// extractSimpleTypeName (enhanced version)
// ---------------------------------------------------------------------------

// ExtractSimpleTypeNameFromNode extracts the simple (unqualified) type name from an
// AST node representing a type reference. This is the full implementation ported from
// TS shared.ts extractSimpleTypeName, replacing the basic fallback in the
// typeextractors package.
func ExtractSimpleTypeNameFromNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, depth int) *string {
	if typeNode == nil || depth > 50 {
		return nil
	}
	text := typeNode.Text(source)
	if len(text) == 0 || len(text) > 2048 {
		return nil
	}
	nodeType := typeNode.Type(lang)

	// Direct type identifier
	switch nodeType {
	case "type_identifier", "identifier", "simple_identifier", "constant":
		t := strings.TrimSpace(text)
		return &t

	// Qualified/scoped names: take the last segment
	case "scoped_identifier", "qualified_identifier", "scoped_type_identifier",
		"qualified_name", "qualified_type", "member_expression",
		"member_access_expression", "attribute", "scope_resolution",
		"selector_expression":
		last := LastNamedChild(typeNode, lang)
		if last != nil {
			lt := last.Type(lang)
			switch lt {
			case "type_identifier", "identifier", "simple_identifier",
				"name", "constant", "property_identifier", "field_identifier":
				t := strings.TrimSpace(last.Text(source))
				return &t
			}
		}

	// C++ template_type
	case "template_type":
		base := typeNode.ChildByFieldName("name", lang)
		if base == nil {
			base = FirstNamedChild(typeNode, lang)
		}
		if base != nil {
			return ExtractSimpleTypeNameFromNode(base, source, lang, depth+1)
		}

	// Generic types
	case "generic_type", "generic_name":
		base := typeNode.ChildByFieldName("name", lang)
		if base == nil {
			base = typeNode.ChildByFieldName("type", lang)
		}
		if base == nil {
			base = FirstNamedChild(typeNode, lang)
		}
		if base == nil {
			return nil
		}
		baseName := ExtractSimpleTypeNameFromNode(base, source, lang, depth+1)
		// Unwrap known nullable wrappers: Optional<User> → User
		if baseName != nil && NullableWrapperTypes[*baseName] {
			args := ExtractGenericTypeArgs(typeNode, source, lang, 0)
			if len(args) >= 1 {
				return &args[0]
			}
		}
		return baseName

	// Nullable type (Kotlin User?, C# User?)
	case "nullable_type":
		inner := FirstNamedChild(typeNode, lang)
		if inner != nil {
			return ExtractSimpleTypeNameFromNode(inner, source, lang, depth+1)
		}

	// Union type (TS/JS: User | null)
	case "union_type":
		var nonNullTypes []*gotreesitter.Node
		for i := 0; i < int(typeNode.NamedChildCount()); i++ {
			child := typeNode.NamedChild(i)
			if child == nil {
				continue
			}
			ct := strings.TrimSpace(child.Text(source))
			if ct == "null" || ct == "undefined" || ct == "void" {
				continue
			}
			nonNullTypes = append(nonNullTypes, child)
		}
		if len(nonNullTypes) == 1 {
			return ExtractSimpleTypeNameFromNode(nonNullTypes[0], source, lang, depth+1)
		}

	// Type annotation wrappers
	case "type_annotation", "type", "user_type":
		inner := FirstNamedChild(typeNode, lang)
		if inner != nil {
			return ExtractSimpleTypeNameFromNode(inner, source, lang, depth+1)
		}

	// Pointer/reference types
	case "pointer_type", "reference_type":
		for i := 0; i < int(typeNode.NamedChildCount()); i++ {
			child := typeNode.NamedChild(i)
			if child != nil && child.Type(lang) != "mutable_specifier" {
				return ExtractSimpleTypeNameFromNode(child, source, lang, depth+1)
			}
		}

	// Primitive/predefined types
	case "primitive_type", "predefined_type", "integral_type",
		"floating_point_type", "boolean_type", "void_type":
		t := strings.TrimSpace(text)
		return &t

	// PHP named_type / optional_type
	case "named_type", "optional_type":
		inner := typeNode.ChildByFieldName("name", lang)
		if inner == nil {
			inner = FirstNamedChild(typeNode, lang)
		}
		if inner != nil {
			return ExtractSimpleTypeNameFromNode(inner, source, lang, depth+1)
		}

	// Name node
	case "name":
		t := strings.TrimSpace(text)
		return &t

	// Array type (Go: []User, etc.)
	case "array_type":
		for i := 0; i < int(typeNode.NamedChildCount()); i++ {
			child := typeNode.NamedChild(i)
			if child != nil {
				return ExtractSimpleTypeNameFromNode(child, source, lang, depth+1)
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// extractVarName
// ---------------------------------------------------------------------------

// ExtractVarName extracts a variable name from a declarator or pattern node.
func ExtractVarName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	if node == nil {
		return nil
	}
	nodeType := node.Type(lang)
	switch nodeType {
	case "identifier", "simple_identifier", "variable_name",
		"name", "constant", "property_identifier":
		t := strings.TrimSpace(node.Text(source))
		return &t
	case "variable_declarator":
		nameChild := node.ChildByFieldName("name", lang)
		if nameChild != nil {
			return ExtractVarName(nameChild, source, lang)
		}
	case "mut_pattern":
		inner := FirstNamedChild(node, lang)
		if inner != nil {
			return ExtractVarName(inner, source, lang)
		}
	case "pattern":
		inner := FirstNamedChild(node, lang)
		if inner != nil {
			return ExtractVarName(inner, source, lang)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// TYPED_PARAMETER_TYPES
// ---------------------------------------------------------------------------

// TypedParameterTypes lists AST node types for function/method parameters with type annotations.
var TypedParameterTypes = map[string]bool{
	"required_parameter":           true, // TS
	"optional_parameter":           true, // TS
	"formal_parameter":             true, // Java
	"parameter":                    true, // C#/Rust/Go/Python
	"typed_parameter":              true, // Python
	"parameter_declaration":        true, // C/C++
	"simple_parameter":             true, // PHP
	"property_promotion_parameter": true, // PHP 8.0+
	"closure_parameter":            true, // Rust
}

// ---------------------------------------------------------------------------
// extractGenericTypeArgs
// ---------------------------------------------------------------------------

// ExtractGenericTypeArgs extracts type arguments from a generic type node.
func ExtractGenericTypeArgs(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, depth int) []string {
	if depth > 50 || typeNode == nil {
		return nil
	}
	nodeType := typeNode.Type(lang)

	// Unwrap pure wrapper nodes
	switch nodeType {
	case "type_annotation", "type", "nullable_type", "optional_type":
		inner := FirstNamedChild(typeNode, lang)
		if inner != nil {
			return ExtractGenericTypeArgs(inner, source, lang, depth+1)
		}
		return nil
	}

	// Only generic-bearing nodes
	if nodeType != "generic_type" && nodeType != "generic_name" && nodeType != "user_type" {
		return nil
	}

	// Find type_arguments / type_argument_list child
	var argsNode *gotreesitter.Node
	for i := 0; i < int(typeNode.NamedChildCount()); i++ {
		child := typeNode.NamedChild(i)
		if child != nil {
			ct := child.Type(lang)
			if ct == "type_arguments" || ct == "type_argument_list" {
				argsNode = child
				break
			}
		}
	}
	if argsNode == nil {
		if nodeType == "user_type" {
			inner := FirstNamedChild(typeNode, lang)
			if inner != nil {
				return ExtractGenericTypeArgs(inner, source, lang, depth+1)
			}
		}
		return nil
	}

	var result []string
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		argNode := argsNode.NamedChild(i)
		if argNode == nil {
			continue
		}
		// Kotlin: type_arguments > type_projection > user_type > type_identifier
		if argNode.Type(lang) == "type_projection" {
			argNode = FirstNamedChild(argNode, lang)
			if argNode == nil {
				continue
			}
		}
		if name := ExtractSimpleTypeNameFromNode(argNode, source, lang, 0); name != nil {
			result = append(result, *name)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// hasTypeAnnotation
// ---------------------------------------------------------------------------

// HasTypeAnnotation checks if an AST node has an explicit type annotation.
func HasTypeAnnotation(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	if node.ChildByFieldName("type", lang) != nil {
		return true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type(lang) == "type_annotation" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// stripNullable
// ---------------------------------------------------------------------------

// NullableKeywords are bare nullable keywords that should not produce a receiver binding.
var NullableKeywords = map[string]bool{
	"null": true, "undefined": true, "void": true, "None": true, "nil": true,
}

// StripNullable strips nullable wrappers from a type name string.
func StripNullable(typeName string) *string {
	text := strings.TrimSpace(typeName)
	if text == "" {
		return nil
	}
	if NullableKeywords[text] {
		return nil
	}
	// Strip nullable suffix: User? → User
	if strings.HasSuffix(text, "?") {
		text = strings.TrimSpace(text[:len(text)-1])
	}
	// Strip union with null/undefined
	if strings.Contains(text, "|") {
		var parts []string
		for _, p := range strings.Split(text, "|") {
			p = strings.TrimSpace(p)
			if p != "" && !NullableKeywords[p] {
				parts = append(parts, p)
			}
		}
		if len(parts) == 1 {
			return &parts[0]
		}
		return nil // genuine union
	}
	if text == "" {
		return nil
	}
	return &text
}

// ---------------------------------------------------------------------------
// unwrapAwait
// ---------------------------------------------------------------------------

// UnwrapAwait unwraps an await_expression to get the inner value.
func UnwrapAwait(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if node.Type(lang) == "await_expression" {
		return FirstNamedChild(node, lang)
	}
	return node
}

// ---------------------------------------------------------------------------
// extractCalleeName
// ---------------------------------------------------------------------------

// ExtractCalleeName extracts the callee name from a call_expression node.
func ExtractCalleeName(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	fn := callNode.ChildByFieldName("function", lang)
	if fn == nil {
		fn = FirstNamedChild(callNode, lang)
	}
	if fn == nil {
		return nil
	}
	return ExtractSimpleTypeNameFromNode(fn, source, lang, 0)
}

// ---------------------------------------------------------------------------
// extractElementTypeFromString — bracket-balanced parsing
// ---------------------------------------------------------------------------

var wordOnlyRe = regexp.MustCompile(`^\w+$`)

// ExtractElementTypeFromString extracts element type from a container type string
// using bracket-balanced parsing.
func ExtractElementTypeFromString(typeStr string, pos TypeArgPosition) *string {
	if typeStr == "" || len(typeStr) > 2048 {
		return nil
	}

	// 1. Array suffix: User[] → User
	if strings.HasSuffix(typeStr, "[]") {
		base := strings.TrimSpace(typeStr[:len(typeStr)-2])
		if base != "" && wordOnlyRe.MatchString(base) {
			return &base
		}
		return nil
	}

	// 2. Go slice prefix: []User → User
	if strings.HasPrefix(typeStr, "[]") {
		element := strings.TrimSpace(typeStr[2:])
		if element != "" && wordOnlyRe.MatchString(element) {
			return &element
		}
		return nil
	}

	// 3. Swift array sugar: [User] → User
	if strings.HasPrefix(typeStr, "[") && strings.HasSuffix(typeStr, "]") && !strings.Contains(typeStr, "<") {
		element := strings.TrimSpace(typeStr[1 : len(typeStr)-1])
		if element != "" && wordOnlyRe.MatchString(element) {
			return &element
		}
		return nil
	}

	// 4. Generic bracket-balanced extraction
	openAngle := strings.Index(typeStr, "<")
	openSquare := strings.Index(typeStr, "[")

	var openIdx int
	var closeChar byte

	if openAngle >= 0 && (openSquare < 0 || openAngle < openSquare) {
		openIdx = openAngle
		closeChar = '>'
	} else if openSquare >= 0 {
		openIdx = openSquare
		closeChar = ']'
	} else {
		return nil
	}

	depth := 0
	start := openIdx + 1
	lastCommaIdx := -1

	for i := start; i < len(typeStr); i++ {
		ch := typeStr[i]
		if ch == '<' || ch == '[' {
			depth++
		} else if ch == '>' || ch == ']' {
			if depth == 0 {
				if ch != closeChar {
					return nil
				}
				if pos == TypeArgLast && lastCommaIdx >= 0 {
					lastArg := strings.TrimSpace(typeStr[lastCommaIdx+1 : i])
					if lastArg != "" && wordOnlyRe.MatchString(lastArg) {
						return &lastArg
					}
					return nil
				}
				inner := strings.TrimSpace(typeStr[start:i])
				firstArg := extractFirstArg(inner)
				if firstArg != "" && wordOnlyRe.MatchString(firstArg) {
					return &firstArg
				}
				return nil
			}
			depth--
		} else if ch == ',' && depth == 0 {
			if pos == TypeArgFirst {
				arg := strings.TrimSpace(typeStr[start:i])
				if arg != "" && wordOnlyRe.MatchString(arg) {
					return &arg
				}
				return nil
			}
			lastCommaIdx = i
		}
	}

	return nil
}

// extractFirstArg extracts the first comma-separated argument from a string,
// respecting nested angle-bracket and square-bracket depth.
func extractFirstArg(args string) string {
	depth := 0
	for i := 0; i < len(args); i++ {
		ch := args[i]
		if ch == '<' || ch == '[' {
			depth++
		} else if ch == '>' || ch == ']' {
			depth--
		} else if ch == ',' && depth == 0 {
			return strings.TrimSpace(args[:i])
		}
	}
	return strings.TrimSpace(args)
}

// ---------------------------------------------------------------------------
// extractReturnTypeName
// ---------------------------------------------------------------------------

// PrimitiveTypes lists primitive/built-in types that should NOT produce a receiver binding.
var PrimitiveTypes = map[string]bool{
	"string": true, "number": true, "boolean": true, "void": true,
	"int": true, "float": true, "double": true, "long": true,
	"short": true, "byte": true, "char": true, "bool": true, "str": true,
	"i8": true, "i16": true, "i32": true, "i64": true,
	"u8": true, "u16": true, "u32": true, "u64": true,
	"f32": true, "f64": true, "usize": true, "isize": true,
	"undefined": true, "null": true, "None": true, "nil": true,
}

// WrapperGenerics are generic types whose first type argument should be unwrapped
// for return-type inference.
var WrapperGenerics = map[string]bool{
	// async wrappers
	"Promise": true, "Observable": true, "Future": true,
	"CompletableFuture": true, "Task": true, "ValueTask": true,
	// nullable wrappers
	"Option": true, "Some": true, "Optional": true, "Maybe": true,
	// result wrappers
	"Result": true, "Either": true,
	// Rust smart pointers
	"Rc": true, "Arc": true, "Weak": true,
	"MutexGuard": true, "RwLockReadGuard": true, "RwLockWriteGuard": true,
	"Ref": true, "RefMut": true, "Cow": true,
}

var (
	ptrRefPrefixRe   = regexp.MustCompile(`^[&*]+\s*(mut\s+)?`)
	nullableSuffixRe = regexp.MustCompile(`\?$`)
	genericMatchRe   = regexp.MustCompile(`^(\w+)\s*<(.+)>$`)
	qualifiedSplitRe = regexp.MustCompile(`::|[.\\]`)
	upperIdentRe     = regexp.MustCompile(`^[A-Z_]\w*$`)
)

const (
	maxReturnTypeInputLength = 2048
	maxReturnTypeLength      = 512
	maxReturnTypeDepth       = 10
)

// ExtractReturnTypeName extracts a simple type name from raw return-type text.
func ExtractReturnTypeName(raw string, depth int) *string {
	if depth > maxReturnTypeDepth || len(raw) > maxReturnTypeInputLength {
		return nil
	}
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	// Strip pointer/reference prefixes
	text = ptrRefPrefixRe.ReplaceAllString(text, "")

	// Strip nullable suffix
	text = nullableSuffixRe.ReplaceAllString(text, "")

	// Handle union types
	if strings.Contains(text, "|") {
		var parts []string
		for _, p := range strings.Split(text, "|") {
			p = strings.TrimSpace(p)
			if p != "" && p != "null" && p != "undefined" && p != "void" && p != "None" && p != "nil" {
				parts = append(parts, p)
			}
		}
		if len(parts) == 1 {
			text = parts[0]
		} else {
			return nil
		}
	}

	// Handle generics
	if m := genericMatchRe.FindStringSubmatch(text); m != nil {
		base := m[1]
		args := m[2]
		if WrapperGenerics[base] {
			firstArg := extractFirstTypeArg(args)
			return ExtractReturnTypeName(firstArg, depth+1)
		}
		if PrimitiveTypes[strings.ToLower(base)] {
			return nil
		}
		return &base
	}

	// Bare wrapper type without generic argument
	if WrapperGenerics[text] {
		return nil
	}

	// Qualified names
	if strings.Contains(text, "::") || strings.Contains(text, ".") || strings.Contains(text, "\\") {
		parts := qualifiedSplitRe.Split(text, -1)
		text = parts[len(parts)-1]
	}

	// Skip primitives
	if PrimitiveTypes[text] || PrimitiveTypes[strings.ToLower(text)] {
		return nil
	}

	// Must start with uppercase
	if !upperIdentRe.MatchString(text) {
		return nil
	}

	if len(text) > maxReturnTypeLength {
		return nil
	}

	return &text
}

// extractFirstTypeArg extracts the first non-lifetime type argument from a generic
// argument string, skipping Rust lifetime parameters.
func extractFirstTypeArg(args string) string {
	remaining := args
	for remaining != "" {
		first := extractFirstGenericArg(remaining)
		if !strings.HasPrefix(first, "'") {
			return first
		}
		commaIdx := strings.Index(remaining[len(first):], ",")
		if commaIdx < 0 {
			return first
		}
		remaining = strings.TrimSpace(remaining[len(first)+1+commaIdx:])
	}
	return strings.TrimSpace(args)
}

// extractFirstGenericArg extracts the first comma-separated generic argument,
// respecting nested angle brackets.
func extractFirstGenericArg(args string) string {
	depth := 0
	for i := 0; i < len(args); i++ {
		if args[i] == '<' {
			depth++
		} else if args[i] == '>' {
			depth--
		} else if args[i] == ',' && depth == 0 {
			return strings.TrimSpace(args[:i])
		}
	}
	return strings.TrimSpace(args)
}

// ---------------------------------------------------------------------------
// findChild — local alias for utils.FindChild
// ---------------------------------------------------------------------------

// FindChildByType returns the first named child of node with the given type, or nil.
func FindChildByType(node *gotreesitter.Node, childType string, lang *gotreesitter.Language) *gotreesitter.Node {
	return utils.FindChild(node, childType, lang)
}
