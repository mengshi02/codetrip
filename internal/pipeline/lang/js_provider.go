package lang

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ============ JavaScript LanguageProvider ============

// JavaScriptProvider JavaScript language provider
type JavaScriptProvider struct{}

// NewJavaScriptProvider creates a JavaScript language provider
func NewJavaScriptProvider() *JavaScriptProvider {
	return &JavaScriptProvider{}
}

// Language returns the language label
func (p *JavaScriptProvider) Language() graph.Label {
	return graph.LabelJSFile
}

// Captures returns node capture rules
func (p *JavaScriptProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_declaration name: (identifier) @fn.name) @fn.def
			(method_definition name: (property_identifier) @method.name) @method.def
			(class_declaration name: (identifier) @class.name) @class.def
			(variable_declarator name: (identifier) @var.name value: (arrow_function)) @arrow.def
			(variable_declarator name: (identifier) @var.name) @var.def
			(import_statement source: (string (string_fragment) @import.path)) @import.def
			(export_statement) @export.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":     graph.LabelFunction,
			"method.def": graph.LabelMethod,
			"class.def":  graph.LabelClass,
			"arrow.def":  graph.LabelFunction,
			"var.def":    graph.LabelVariable,
			"import.def": graph.LabelImport,
			"export.def": graph.LabelCodeElement,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *JavaScriptProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call_expression
				function: (member_expression
					object: (identifier) @call.receiver
					property: (property_identifier) @call.method
				)
			) @call.site
			(call_expression
				function: (identifier) @call.fn
			) @call.fn.site
		`,
		CaptureMap: map[string]graph.Label{
			"call.site":    graph.LabelCallSite,
			"call.fn.site": graph.LabelCallSite,
		},
		ReceiverKey: "call.receiver",
		MethodKey:   "call.method",
		ArgsKey:     "call.args",
	}
}

// ClassExtractConfig returns class extraction config
func (p *JavaScriptProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_declaration name: (identifier) @class.name (class_heritage (extends_clause value: (identifier) @class.base))?) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelClass,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction config
func (p *JavaScriptProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(public_field_definition name: (property_identifier) @field.name) @field.def
			(field_definition name: (property_identifier) @field.name) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "",
	}
}

// ImportResolveConfig returns import resolution config
func (p *JavaScriptProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(import_statement
				source: (string (string_fragment) @import.path)
				(import_clause
					(identifier)? @import.default
					(named_imports (import_specifier name: (identifier) @import.item))?
					(namespace_import (identifier) @import.alias)?
				)?
			) @import.decl
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "import.alias",
		ItemsKey:    "import.item",
		IsDotImport: false,
		IsWildcard:  false,
	}
}

// Interpret interprets capture results
func (p *JavaScriptProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
	result := &InterpretResult{
		Symbols:   make([]pipeline.SymbolInfo, 0),
		Imports:   make([]ImportInfo, 0),
		CallSites: make([]CallSite, 0),
		Classes:   make([]ClassInfo, 0),
		Fields:    make([]FieldInfo, 0),
	}

	for _, cap := range captures.Captures {
		switch cap.NodeType {
		case "fn.def", "arrow.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelFunction,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "method.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelMethod,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "class.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelClass,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
			cls := ClassInfo{
				Name: cap.Name, FilePath: captures.Filepath,
				StartLine: cap.StartRow, EndLine: cap.EndRow,
			}
			for _, child := range cap.Children {
				if child.NodeType == "class.base" {
					cls.BaseTypes = append(cls.BaseTypes, child.Text)
				}
			}
			result.Classes = append(result.Classes, cls)
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "import.def", "import.decl":
			path := strings.Trim(cap.Text, "\"'")
			imp := ImportInfo{
				Path: path, FilePath: captures.Filepath, StartLine: cap.StartRow,
			}
			for _, child := range cap.Children {
				switch child.NodeType {
				case "import.alias":
					imp.Alias = child.Text
				case "import.item":
					imp.Items = append(imp.Items, child.Text)
				}
			}
			result.Imports = append(result.Imports, imp)
		case "call.site":
			recv := ""
			method := ""
			for _, child := range cap.Children {
				if child.NodeType == "call.receiver" {
					recv = child.Text
				}
				if child.NodeType == "call.method" {
					method = child.Text
				}
			}
			result.CallSites = append(result.CallSites, CallSite{
				Receiver: recv, MethodName: method,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "call.fn.site":
			result.CallSites = append(result.CallSites, CallSite{
				MethodName: cap.Name,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "field.def":
			name := ""
			for _, child := range cap.Children {
				if child.NodeType == "field.name" {
					name = child.Text
				}
			}
			result.Fields = append(result.Fields, FieldInfo{
				Name: name,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (JavaScript uses named)
func (p *JavaScriptProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamed
}

// ============ Scope-Based Pipeline Methods ============

// TreeSitterLanguage returns the JavaScript tree-sitter language.
func (p *JavaScriptProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.JavascriptLanguage()
}

// QuerySet returns all S-expression queries for JavaScript extraction.
// Follows capture namespace conventions:
//   - Scope: @scope.<kind> (no .name sub-captures; names extracted from cap.Text)
//   - Declaration: @declaration.<kind> outer + @declaration.name inner
//   - Import: @import.statement / @import.dynamic single anchor
//   - TypeBinding: @type-binding.<kind> outer + @type-binding.name/.type inner
//   - Reference: @reference.call.free/member/constructor, @reference.write.member,
//     @reference.read.member + @reference.name/@reference.receiver inner
func (p *JavaScriptProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(program) @scope.module
			(class_declaration) @scope.class
			(class) @scope.class
			(function_declaration) @scope.function
			(generator_function_declaration) @scope.function
			(function_expression) @scope.function
			(arrow_function) @scope.function
			(method_definition) @scope.function
		`,
		Declaration: `
			(class_declaration name: (identifier) @declaration.name) @declaration.class
			(method_definition name: (property_identifier) @declaration.name) @declaration.method
			(field_definition property: (property_identifier) @declaration.name) @declaration.property
			(function_declaration name: (identifier) @declaration.name) @declaration.function
			(generator_function_declaration name: (identifier) @declaration.name) @declaration.function

			(lexical_declaration (variable_declarator name: (identifier) @declaration.name value: (arrow_function) @declaration.function))
			(lexical_declaration (variable_declarator name: (identifier) @declaration.name value: (function_expression) @declaration.function))
			(variable_declaration (variable_declarator name: (identifier) @declaration.name value: (arrow_function) @declaration.function))
			(variable_declaration (variable_declarator name: (identifier) @declaration.name value: (function_expression) @declaration.function))

			(pair key: (property_identifier) @declaration.name value: (arrow_function) @declaration.function)
			(pair key: (property_identifier) @declaration.name value: (function_expression) @declaration.function)

			(lexical_declaration (variable_declarator name: (identifier) @declaration.name value: (call_expression function: (identifier) arguments: (arguments (arrow_function) @declaration.function))))
			(lexical_declaration (variable_declarator name: (identifier) @declaration.name value: (call_expression function: (identifier) arguments: (arguments (function_expression) @declaration.function))))

			(lexical_declaration (variable_declarator name: (identifier) @declaration.name)) @declaration.const
			(variable_declaration (variable_declarator name: (identifier) @declaration.name)) @declaration.variable
		`,
		Import: `
			(import_statement) @import.statement
			(export_statement source: (string)) @import.statement
			(call_expression function: (import)) @import.dynamic
		`,
		TypeBinding: `
			(variable_declarator name: (identifier) @type-binding.name value: (new_expression constructor: (identifier) @type-binding.type)) @type-binding.constructor
			(variable_declarator name: (identifier) @type-binding.name value: (call_expression function: (identifier) @type-binding.type)) @type-binding.alias
			(variable_declarator name: (identifier) @type-binding.name value: (call_expression function: (member_expression) @type-binding.type)) @type-binding.alias
			(variable_declarator name: (identifier) @type-binding.name value: (identifier) @type-binding.type) @type-binding.alias
		`,
		Reference: `
			(call_expression function: (identifier) @reference.name) @reference.call.free
			(call_expression function: (member_expression object: (_) @reference.receiver property: (property_identifier) @reference.name)) @reference.call.member
			(new_expression constructor: (identifier) @reference.name) @reference.call.constructor
			(assignment_expression left: (member_expression object: (_) @reference.receiver property: (property_identifier) @reference.name)) @reference.write.member
			(member_expression object: (_) @reference.receiver property: (property_identifier) @reference.name) @reference.read.member
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. Scope queries use @scope.<kind> without
// name sub-captures — names are extracted from cap.Text (first line of the
// captured node's source text) using jsExtractScopeName.
func (p *JavaScriptProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices to identify distinct scope nodes
	seen := make(map[int]struct{})
	for _, cap := range captures {
		seen[cap.MatchIndex] = struct{}{}
	}

	scopes := make([]*pipeline.ScopeInfo, 0, len(seen))
	scopeID := 0

	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)

		var kind, name string
		var startLine, endLine int
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "scope.module":
				kind = "module"
				name = jsModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = jsExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = jsExtractScopeName(cap.Text, "function")
				startLine = cap.StartRow
				endLine = cap.EndRow
			}
		}
		if kind == "" {
			continue
		}

		scopeID++
		scopes = append(scopes, &pipeline.ScopeInfo{
			ID:        fmt.Sprintf("%s:scope:%d", filePath, scopeID),
			Kind:      kind,
			Name:      name,
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
		})
	}

	// Build ParentID by line-range nesting
	buildScopeParentIDs(scopes)

	return scopes
}

// InterpretDeclaration extracts symbol declarations from declaration query captures.
// Each match produces one symbol. The outer @declaration.<kind> capture determines
// the label; the inner @declaration.name capture provides the symbol name.
// Patterns without an outer anchor (arrow/function-expression variable, pair,
// HOC-wrapped) are detected by checking for @declaration.function sub-capture
// alongside @declaration.name.
func (p *JavaScriptProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		seen[cap.MatchIndex] = struct{}{}
	}

	symbols := make([]*pipeline.SymbolInfo, 0, len(seen))

	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)

		// Find the outer (parent) capture in this match to determine symbol kind.
		// Also detect inline function patterns that lack an outer anchor.
		var outerNodeType string
		var startLine, endLine int
		hasDeclarationFn := false
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "declaration.class", "declaration.method", "declaration.property",
				"declaration.function", "declaration.const", "declaration.variable":
				outerNodeType = cap.NodeType
				startLine = cap.StartRow
				endLine = cap.EndRow
			}
			// Inline patterns: lexical/variable/pair/HOC declare a function
			// via sub-capture but have no outer @declaration anchor.
			if cap.Name == "declaration.function" {
				hasDeclarationFn = true
			}
		}

		// If no outer anchor but has @declaration.function sub-capture,
		// treat as a function declaration.
		if outerNodeType == "" && hasDeclarationFn {
			outerNodeType = "declaration.function"
		}
		if outerNodeType == "" {
			continue
		}

		sym := &pipeline.SymbolInfo{
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
		}

		// Extract name from @declaration.name sub-capture (unified across all patterns)
		sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		switch outerNodeType {
		case "declaration.class":
			sym.Label = graph.LabelClass
		case "declaration.method":
			sym.Label = graph.LabelMethod
		case "declaration.property":
			sym.Label = graph.LabelProperty
		case "declaration.function":
			sym.Label = graph.LabelFunction
		case "declaration.const":
			sym.Label = graph.LabelConst
		case "declaration.variable":
			sym.Label = graph.LabelVariable
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Each match (import.decl) produces one ImportInfo. Sub-captures are looked
// up within the same match via MatchIndex to avoid cross-match contamination.
// InterpretImport extracts import information from import query captures.
// Each @import.statement or @import.dynamic match produces one ImportInfo.
// The import path is extracted from the captured node's text using jsExtractImportPath,
// since import queries no longer use sub-captures for path details.
func (p *JavaScriptProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices that are outer import captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "import.statement" || cap.NodeType == "import.dynamic" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	imports := make([]*pipeline.ImportInfo, 0, len(seen))

	for matchIdx := range seen {
		// Find the outer capture to get line and text
		var line int
		var capText string
		var isDynamic bool
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx {
				if cap.NodeType == "import.statement" || cap.NodeType == "import.dynamic" {
					line = cap.StartRow
					capText = cap.Text
					isDynamic = cap.NodeType == "import.dynamic"
					break
				}
			}
		}

		// Extract import path from the captured node's text
		path := jsExtractImportPath(capText, isDynamic)

		imports = append(imports, &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			Line:       line,
		})
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Handles JavaScript type-binding patterns using conventions:
//   - @type-binding.constructor: variable with new_expression
//   - @type-binding.alias: variable with call_expression or identifier value
func (p *JavaScriptProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices for type-binding outer captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.constructor", "type-binding.alias":
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	bindings := make([]*pipeline.TypeBindingInfo, 0, len(seen))

	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)

		tb := &pipeline.TypeBindingInfo{
			FilePath: filePath,
		}

		// Determine kind from the outer capture
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.alias":
				tb.Kind = "alias"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			}
		}
		if tb.Kind == "" {
			continue
		}

		// Extract bound variable name from @type-binding.name sub-capture
		tb.BoundNode = findCaptureTextInMatch(captures, matchIdx, "type-binding.name")

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// Uses capture namespace:
//   - @reference.call.free: free function call, e.g. foo()
//   - @reference.call.member: method call with receiver, e.g. obj.method()
//   - @reference.call.constructor: new expression, e.g. new Foo()
//   - @reference.write.member: member write in assignment, e.g. obj.field = value
//   - @reference.read.member: member read access, e.g. obj.field
//
// Sub-captures use unified @reference.name and @reference.receiver.
// For @reference.read.member, duplicates are filtered: if the same matchIdx
// already has a call kind, the read is skipped (since the member_expression
// inside a call_expression is captured by both patterns).
func (p *JavaScriptProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices for reference outer captures.
	// Track which matchIdx has a call kind to filter duplicate reads.
	seen := make(map[int]struct{})
	callMatchIndices := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.call.constructor",
			"reference.write.member", "reference.read.member":
			seen[cap.MatchIndex] = struct{}{}
			if cap.NodeType == "reference.call.member" || cap.NodeType == "reference.call.free" || cap.NodeType == "reference.call.constructor" {
				callMatchIndices[cap.MatchIndex] = struct{}{}
			}
		}
	}

	refs := make([]*pipeline.ReferenceInfo, 0, len(seen))

	for matchIdx := range seen {
		ref := &pipeline.ReferenceInfo{
			FilePath: filePath,
		}

		// Determine kind from the outer capture and extract sub-captures
		for _, cap := range captures {
			if cap.MatchIndex != matchIdx {
				continue
			}
			switch cap.NodeType {
			case "reference.call.free":
				ref.Kind = "free_call"
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.call.member":
				ref.Kind = "member_call"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.call.constructor":
				ref.Kind = "constructor"
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.write.member":
				ref.Kind = "field_write"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.read.member":
				ref.Kind = "field_read"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			}
		}
		if ref.Kind == "" {
			continue
		}

		// Filter duplicate reads: if the same matchIdx already has a call kind,
		// skip the read member (the member_expression inside a call is already
		// captured as a call).
		if ref.Kind == "field_read" {
			if _, hasCall := callMatchIndices[matchIdx]; hasCall {
				continue
			}
		}

		refs = append(refs, ref)
	}
	return refs
}

// ============ JavaScript Helper Functions ============

// jsFirstLine returns the first line of a text string.
func jsFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// jsParseFuncName extracts the function name from a JS function declaration line.
// Handles: "function Name(", "function* Name(", "function Name(" (generator),
// arrow-function variables: "const Name = ", "let Name = ", "var Name = ",
// method definitions: "Name(", "async Name(", "*Name(", "get Name(", "set Name(",
// and object property: "Name: (" or "Name: function" or "Name: =>".
func jsParseFuncName(line string) string {
	line = strings.TrimSpace(line)

	// function declaration: "function Name(" or "function* Name("
	if idx := strings.Index(line, "function "); idx >= 0 {
		rest := line[idx+9:] // after "function "
		rest = strings.TrimPrefix(rest, "* ")
		rest = strings.TrimSpace(rest)
		for i, ch := range rest {
			if ch == '(' || ch == ' ' || ch == '{' || ch == '<' {
				if i == 0 {
					return "" // anonymous function
				}
				return rest[:i]
			}
		}
		return rest
	}

	// Arrow/function-expression variable: "const Name =", "let Name =", "var Name ="
	for _, kw := range []string{"const ", "let ", "var "} {
		if idx := strings.Index(line, kw); idx >= 0 {
			rest := line[idx+len(kw):]
			rest = strings.TrimSpace(rest)
			for i, ch := range rest {
				if ch == ' ' || ch == '=' || ch == ':' || ch == ',' {
					if i == 0 {
						return ""
					}
					return rest[:i]
				}
			}
			return rest
		}
	}

	// Method definition: "Name(" or "async Name(" or "*Name(" or "get Name(" or "set Name("
	// Strip leading modifiers
	rest := line
	for _, mod := range []string{"async ", "static ", "get ", "set "} {
		rest = strings.TrimPrefix(rest, mod)
		rest = strings.TrimSpace(rest)
	}
	rest = strings.TrimPrefix(rest, "*")
	rest = strings.TrimSpace(rest)

	for i, ch := range rest {
		if ch == '(' || ch == ' ' || ch == '{' || ch == '<' {
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// jsParseClassName extracts the class name from a JS class declaration line.
// Handles: "class Name" and "class Name extends".
func jsParseClassName(line string) string {
	line = strings.TrimSpace(line)
	idx := strings.Index(line, "class ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+6:] // after "class "
	rest = strings.TrimSpace(rest)
	for i, ch := range rest {
		if ch == ' ' || ch == '{' || ch == '<' || ch == 'e' {
			// "extends" starts with 'e'
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// jsExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the function/class name.
func jsExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := jsFirstLine(text)

	switch kind {
	case "function":
		return jsParseFuncName(firstLine)
	case "class":
		return jsParseClassName(firstLine)
	}
	return ""
}

// jsModuleName infers the JS module name from the file path.
// Uses the file basename without extension as the module name.
func jsModuleName(filePath string) string {
	base := filePath
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndexByte(base, '.'); idx > 0 {
		base = base[:idx]
	}
	if base == "index" {
		// Use parent directory name for index files
		dir := filePath
		if idx := strings.LastIndexAny(dir, "/\\"); idx >= 0 {
			dir = dir[:idx]
		}
		if idx := strings.LastIndexAny(dir, "/\\"); idx >= 0 {
			dir = dir[idx+1:]
		}
		if dir != "" {
			return dir
		}
	}
	return base
}

// jsExtractImportPath extracts the import path from a captured import statement's text.
// For static imports: parses "import ... from 'path'" or "import 'path'" patterns.
// For dynamic imports: parses "import('path')" pattern.
func jsExtractImportPath(text string, isDynamic bool) string {
	if text == "" {
		return ""
	}

	if isDynamic {
		// Dynamic import: import('path') or import("path")
		// Find the string between parentheses
		idx := strings.IndexByte(text, '(')
		if idx < 0 {
			return ""
		}
		rest := text[idx+1:]
		return strings.Trim(rest, "\"'` \t)")
	}

	// Static import: find the string literal (from 'path' or just 'path')
	// Look for the last quoted string in the statement
	// Patterns: import 'path', import X from 'path', export { X } from 'path'

	// Try to find "from 'path'" or "from \"path\""
	fromIdx := strings.Index(text, " from ")
	if fromIdx >= 0 {
		rest := strings.TrimSpace(text[fromIdx+6:])
		return strings.Trim(rest, "\"'` ;")
	}

	// Try bare import: import 'path'
	firstLine := jsFirstLine(text)
	rest := strings.TrimSpace(firstLine)
	rest = strings.TrimPrefix(rest, "import ")
	rest = strings.TrimSpace(rest)
	return strings.Trim(rest, "\"'` ;")
}

// isJSFieldWrite checks whether a member expression (receiver.property) appears
// on the left side of an assignment in the given source line.
// This handles patterns like "obj.field = value", "obj.field=", and "obj.field += value".
func isJSFieldWrite(line, receiver, field string) bool {
	// Build the expected member expression pattern
	pattern := receiver + "." + field
	idx := strings.Index(line, pattern)
	if idx < 0 {
		return false
	}

	// Check the text after the pattern for assignment operators
	after := strings.TrimSpace(line[idx+len(pattern):])
	if strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "==") {
		return true // =, +=, -=, etc.
	}
	if strings.HasPrefix(after, "+=") || strings.HasPrefix(after, "-=") ||
		strings.HasPrefix(after, "*=") || strings.HasPrefix(after, "/=") ||
		strings.HasPrefix(after, "%=") || strings.HasPrefix(after, "**=") ||
		strings.HasPrefix(after, "<<=") || strings.HasPrefix(after, ">>=") ||
		strings.HasPrefix(after, ">>>=") || strings.HasPrefix(after, "&=") ||
		strings.HasPrefix(after, "|=") || strings.HasPrefix(after, "^=") {
		return true
	}

	return false
}

// ============ JavaScript ScopeResolver ============

// JavaScriptScopeResolver JavaScript scope resolver
type JavaScriptScopeResolver struct {
	provider *JavaScriptProvider
}

// NewJavaScriptScopeResolver creates a JavaScript scope resolver
func NewJavaScriptScopeResolver(provider *JavaScriptProvider) *JavaScriptScopeResolver {
	return &JavaScriptScopeResolver{provider: provider}
}

// Language returns the language label
func (r *JavaScriptScopeResolver) Language() graph.Label {
	return graph.LabelJSFile
}

// LanguageProvider returns the language provider
func (r *JavaScriptScopeResolver) LanguageProvider() *JavaScriptProvider {
	return r.provider
}

// PopulateOwners populates owner relationships (function/method -> file, class -> file, etc.)
// JavaScript symbols belong to file nodes via ES module semantics
func (r *JavaScriptScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	if model == nil {
		return nil
	}
	repo := gs.Repo()
	return model.ForEachSymbol(func(key string, entry *pipeline.SymbolEntry) error {
		// Find file node
		fileNodes, err := gs.GetNodesByFile(repo, entry.FilePath)
		if err != nil || len(fileNodes) == 0 {
			return nil // continue iteration
		}

		var fileNode *graph.Node
		for _, n := range fileNodes {
			if n.Label == graph.LabelFile {
				fileNode = n
				break
			}
		}
		if fileNode == nil {
			return nil // continue iteration
		}

		// Create symbol node
		symNode := &graph.Node{
			Name:     entry.Name,
			FilePath: entry.FilePath,
		}
		// Map kind string to graph label
		switch entry.Kind {
		case "function":
			symNode.Label = graph.LabelFunction
		case "method":
			symNode.Label = graph.LabelMethod
		case "class":
			symNode.Label = graph.LabelClass
		case "variable":
			symNode.Label = graph.LabelVariable
		default:
			symNode.Label = graph.LabelCodeElement
		}
		if err := gs.BufferNode(symNode); err != nil {
			return fmt.Errorf("add symbol node %s: %w", entry.Name, err)
		}

		// CONTAINS edge: file -> symbol
		if err := gs.BufferEdge(&graph.Edge{
			Type:   graph.RelContains,
			Source: fileNode.ID,
			Target: symNode.ID,
		}); err != nil {
			return fmt.Errorf("add contains edge: %w", err)
		}
		return nil
	})
}

// BuildMRO builds method resolution order
// JavaScript uses class extends single inheritance (prototype chain)
func (r *JavaScriptScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	// JavaScript MRO strategy: prototype chain single inheritance
	for _, clsNode := range classes {
		baseNames := clsNode.Props.BaseTypes
		if len(baseNames) == 0 {
			continue
		}

		for _, baseName := range baseNames {
			baseNodes, err := gs.GetNodesByName("", baseName)
			if err != nil || len(baseNodes) == 0 {
				continue
			}

			// EMBRACES edge: subclass -> base class
			for _, baseNode := range baseNodes {
				_ = gs.BufferEdge((&graph.Edge{
					Type:   graph.RelEmbraces,
					Source: clsNode.ID,
					Target: baseNode.ID,
				}).WithProp("confidence", 0.85))
			}
		}
	}
	return nil
}

// ResolveImportTarget resolves import target
// JavaScript ES module: import path -> module file -> export symbols
func (r *JavaScriptScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	path := imp.Path
	if !strings.Contains(path, "/") && !strings.HasPrefix(path, ".") {
		// npm package, cannot resolve in local repo
		return nil, nil
	}

	// Find matching file node in repo
	var targets []*graph.Node
	allNodes := gs.GetAllNodes("", 10000)
	for _, node := range allNodes {
		if node.Label == graph.LabelFile && strings.Contains(node.FilePath, path) {
			targets = append(targets, node)
		}
	}

	return targets, nil
}

// ============ Boolean switches ============

// PropagatesReturnTypesAcrossImports JavaScript supports propagating return types across imports (JSDoc/TS annotations)
func (r *JavaScriptScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }

// FieldFallbackOnMethodLookup JavaScript dynamic typing, fallback to method when field lookup fails
func (r *JavaScriptScopeResolver) FieldFallbackOnMethodLookup() bool { return true }

// UnwrapCollectionAccessor JavaScript unwrap collection accessor (array destructuring, etc.)
func (r *JavaScriptScopeResolver) UnwrapCollectionAccessor() bool { return true }

// CollapseMemberCallsByCallerTarget JavaScript one edge per caller-target
func (r *JavaScriptScopeResolver) CollapseMemberCallsByCallerTarget() bool { return true }

// PopulateNamespaceSiblings JavaScript same-module files implicitly visible
func (r *JavaScriptScopeResolver) PopulateNamespaceSiblings() bool { return true }

// HoistTypeBindingsToModule JavaScript hoist type bindings to module
func (r *JavaScriptScopeResolver) HoistTypeBindingsToModule() bool { return true }

// ============ Core methods ============

// MergeBindings merges local binding set and imported binding set
// JavaScript strategy: import takes precedence (ES module semantics, import declaration shadows local declaration)
func (r *JavaScriptScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	result.IsImported = false

	// Put local bindings first
	for name, ids := range local.Bindings {
		result.Bindings[name] = ids
	}

	// Import bindings take precedence (ES module semantics: import declaration shadows local variable)
	for name, ids := range imported.Bindings {
		result.Bindings[name] = ids
	}

	return result
}

// ArityCompatibility checks call site and target parameter compatibility
// JavaScript loose checking: supports variadic params (rest params), parameter count need not match strictly
func (r *JavaScriptScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true // No arity info, default compatible
	}
	// JS loose checking: call args >= target required args is considered compatible
	return caller.Args >= targetArity || caller.Args+1 >= targetArity
}

// ImportEdgeReason returns import edge reason description
func (r *JavaScriptScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.Alias != "" {
		return "namespace-import"
	}
	if len(imp.Symbols) > 0 {
		return "named-import"
	}
	return "re-export"
}

// IsSuperReceiver checks if receiver is a superclass receiver
// JavaScript class extends super call
func (r *JavaScriptScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super"
}

// ============ 4 functional hooks ============

// PopulateRangeBindings for-of / for-in variable bindings
// JavaScript destructuring assignment is captured as local variables during tree-sitter parsing
func (r *JavaScriptScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
	// JavaScript for-of variables are captured as local variables during tree-sitter parsing
	// This is a no-op, JS does not need additional range binding logic
}

// CollectScopeContextPaths collects scope context file paths
// JavaScript module scope, current file path
func (r *JavaScriptScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

// EmitPostResolutionEdges post-processing edge emission
// JavaScript does not need additional post-processing edges
func (r *JavaScriptScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
	// JavaScript has no post-processing edge requirements
}

// EmitUnresolvedReceiverEdges untyped receiver fallback
// JavaScript dynamic typing language, currently returns 0
func (r *JavaScriptScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	// JavaScript dynamic typing, currently returns 0 fallback edges
	return 0
}