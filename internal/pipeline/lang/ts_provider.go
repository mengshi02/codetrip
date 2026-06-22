package lang

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ============ TypeScript LanguageProvider ============

// TypeScriptProvider TypeScript language provider
type TypeScriptProvider struct{}

// NewTypeScriptProvider creates a TypeScript language provider
func NewTypeScriptProvider() *TypeScriptProvider {
	return &TypeScriptProvider{}
}

// Language returns the language label
func (p *TypeScriptProvider) Language() graph.Label {
	return graph.LabelTSFile
}

// Captures returns node capture rules
func (p *TypeScriptProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_declaration name: (identifier) @fn.name) @fn.def
			(method_definition name: (property_identifier) @method.name) @method.def
			(class_declaration name: (type_identifier) @class.name) @class.def
			(interface_declaration name: (type_identifier) @iface.name) @iface.def
			(type_alias_declaration name: (type_identifier) @type.name) @type.def
			(enum_declaration name: (identifier) @enum.name) @enum.def
			(variable_declarator name: (identifier) @var.name) @var.def
			(lexical_declaration name: (identifier) @const.name) @const.def
			(import_statement (import_clause (identifier)? @import.default (named_imports (import_specifier name: (identifier) @import.item))?)) @import.def
			(export_statement) @export.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":     graph.LabelFunction,
			"method.def": graph.LabelMethod,
			"class.def":  graph.LabelClass,
			"iface.def":  graph.LabelInterface,
			"type.def":   graph.LabelTypeAlias,
			"enum.def":   graph.LabelEnum,
			"var.def":    graph.LabelVariable,
			"const.def":  graph.LabelConst,
			"import.def": graph.LabelImport,
			"export.def": graph.LabelCodeElement,
		},
	}
}

// CallExtractConfig returns call extraction configuration
func (p *TypeScriptProvider) CallExtractConfig() *CallExtractConfig {
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

// ClassExtractConfig returns class extraction configuration
func (p *TypeScriptProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_declaration name: (type_identifier) @class.name (class_heritage (extends_clause (identifier) @class.base))?) @class.def
			(interface_declaration name: (type_identifier) @class.name) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelClass,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction configuration
func (p *TypeScriptProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(public_field_definition name: (property_identifier) @field.name type: (type_annotation (_) @field.type)?) @field.def
			(property_signature name: (property_identifier) @field.name type: (type_annotation (_) @field.type)?) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "field.type",
	}
}

// ImportResolveConfig returns import resolution configuration
func (p *TypeScriptProvider) ImportResolveConfig() *ImportResolveConfig {
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
func (p *TypeScriptProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
	result := &InterpretResult{
		Symbols:   make([]pipeline.SymbolInfo, 0),
		Imports:   make([]ImportInfo, 0),
		CallSites: make([]CallSite, 0),
		Classes:   make([]ClassInfo, 0),
		Fields:    make([]FieldInfo, 0),
	}

	for _, cap := range captures.Captures {
		switch cap.NodeType {
		case "fn.def":
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
		case "iface.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelInterface,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "type.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTypeAlias,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "enum.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelEnum,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "const.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelConst,
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
			typeName := ""
			for _, child := range cap.Children {
				if child.NodeType == "field.name" {
					name = child.Text
				}
				if child.NodeType == "field.type" {
					typeName = child.Text
				}
			}
			result.Fields = append(result.Fields, FieldInfo{
				Name: name, TypeName: typeName,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (TypeScript uses named)
func (p *TypeScriptProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamed
}

// ============ Scope-Based Pipeline Methods ============

// TreeSitterLanguage returns the TypeScript tree-sitter language.
func (p *TypeScriptProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.TypescriptLanguage()
}

// QuerySet returns all S-expression queries for TypeScript extraction.
// Uses unified capture namespace convention:
//   - Scope: @scope.<kind> (NO name sub-captures — names extracted from cap.Text)
//   - Declaration: @declaration.<kind> outer + @declaration.name inner
//   - Import: @import.statement / @import.dynamic single anchor (NO sub-captures)
//   - TypeBinding: @type-binding.<kind> outer + @type-binding.name/type inner
//   - Reference: @reference.call.free/member/constructor + @reference.name/receiver inner
func (p *TypeScriptProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(program) @scope.module
			(internal_module) @scope.namespace
			(class_declaration) @scope.class
			(abstract_class_declaration) @scope.class
			(interface_declaration) @scope.class
			(enum_declaration) @scope.class
			(class) @scope.class
			(function_declaration) @scope.function
			(generator_function_declaration) @scope.function
			(function_signature) @scope.function
			(method_definition) @scope.function
			(method_signature) @scope.function
			(abstract_method_signature) @scope.function
			(arrow_function) @scope.function
			(function_expression) @scope.function
		`,
		Declaration: `
			(class_declaration name: (type_identifier) @declaration.name) @declaration.class
			(abstract_class_declaration name: (type_identifier) @declaration.name) @declaration.class
			(interface_declaration name: (type_identifier) @declaration.name) @declaration.interface
			(enum_declaration name: (identifier) @declaration.name) @declaration.enum
			(type_alias_declaration name: (type_identifier) @declaration.name) @declaration.type
			(internal_module name: (identifier) @declaration.name) @declaration.namespace
			(function_declaration name: (identifier) @declaration.name) @declaration.function
			(generator_function_declaration name: (identifier) @declaration.name) @declaration.function
			(method_definition name: (property_identifier) @declaration.name) @declaration.method
			(method_signature name: (property_identifier) @declaration.name) @declaration.method
			(public_field_definition name: (property_identifier) @declaration.name) @declaration.property
			(lexical_declaration (variable_declarator name: (identifier) @declaration.name)) @declaration.variable
			(variable_declaration (variable_declarator name: (identifier) @declaration.name)) @declaration.variable
		`,
		Import: `
			(import_statement) @import.statement
			(export_statement source: (string)) @import.statement
			(call_expression function: (import)) @import.dynamic
		`,
		TypeBinding: `
			(required_parameter pattern: (identifier) @type-binding.name type: (type_annotation (_) @type-binding.type)) @type-binding.parameter
			(optional_parameter pattern: (identifier) @type-binding.name type: (type_annotation (_) @type-binding.type)) @type-binding.parameter
			(variable_declarator name: (identifier) @type-binding.name type: (type_annotation (_) @type-binding.type)) @type-binding.annotation
			(variable_declarator name: (identifier) @type-binding.name value: (new_expression constructor: (identifier) @type-binding.type)) @type-binding.constructor
			(variable_declarator name: (identifier) @type-binding.name value: (call_expression function: (identifier) @type-binding.type)) @type-binding.alias
			(variable_declarator name: (identifier) @type-binding.name value: (identifier) @type-binding.type) @type-binding.alias
			(function_declaration name: (identifier) @type-binding.name return_type: (type_annotation (_) @type-binding.type)) @type-binding.return
			(method_definition name: (property_identifier) @type-binding.name return_type: (type_annotation (_) @type-binding.type)) @type-binding.return
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
// convention: scope queries have NO name sub-captures.
// Names are extracted from cap.Text using helper functions.
func (p *TypeScriptProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = tsModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.namespace":
				kind = "namespace"
				name = tsExtractScopeName(cap.Text, "namespace")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = tsExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = tsExtractScopeName(cap.Text, "function")
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
// convention: @declaration.<kind> outer + @declaration.name inner.
// Each match produces one symbol. Name is extracted from @declaration.name sub-capture.
func (p *TypeScriptProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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

		// Find the outer @declaration.<kind> capture to determine symbol kind
		var outerNodeType string
		var startLine, endLine int
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "declaration.class", "declaration.interface", "declaration.enum",
				"declaration.type", "declaration.namespace", "declaration.function",
				"declaration.method", "declaration.property", "declaration.variable":
				outerNodeType = cap.NodeType
				startLine = cap.StartRow
				endLine = cap.EndRow
			}
		}
		if outerNodeType == "" {
			continue
		}

		// Extract name from @declaration.name sub-capture
		name := findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		sym := &pipeline.SymbolInfo{
			Name:      name,
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
		}

		switch outerNodeType {
		case "declaration.class":
			sym.Label = graph.LabelClass
			sym.Visibility = tsVisibility(name)
		case "declaration.interface":
			sym.Label = graph.LabelInterface
			sym.Visibility = tsVisibility(name)
		case "declaration.enum":
			sym.Label = graph.LabelEnum
			sym.Visibility = tsVisibility(name)
		case "declaration.type":
			sym.Label = graph.LabelTypeAlias
			sym.Visibility = tsVisibility(name)
		case "declaration.namespace":
			sym.Label = graph.LabelNamespace
			sym.Visibility = tsVisibility(name)
		case "declaration.function":
			sym.Label = graph.LabelFunction
			sym.Visibility = tsVisibility(name)
		case "declaration.method":
			sym.Label = graph.LabelMethod
			sym.Visibility = tsVisibility(name)
		case "declaration.property":
			sym.Label = graph.LabelProperty
			sym.Visibility = tsVisibility(name)
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
// convention: @import.statement and @import.dynamic are single anchors.
// Path and other info are parsed from cap.Text.
func (p *TypeScriptProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
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
		// Find the outer capture for this match
		var cap pipeline.LangCapture
		for _, c := range captures {
			if c.MatchIndex == matchIdx && (c.NodeType == "import.statement" || c.NodeType == "import.dynamic") {
				cap = c
				break
			}
		}

		path, isDefault, isDynamic := tsExtractImportPath(cap.Text)

		imp := &pipeline.ImportInfo{
			Path:       path,
			SourceFile:  filePath,
			Line:       cap.StartRow,
		}

		if isDynamic {
			imp.Alias = ""
		} else if isDefault {
			// Default import — the imported name is the path basename
			imp.Symbols = []string{}
		}

		// For static imports, parse symbols from text
		if cap.NodeType == "import.statement" {
			symbols := tsExtractImportSymbols(cap.Text)
			if len(symbols) > 0 {
				imp.Symbols = symbols
			}
		}

		imports = append(imports, imp)
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// convention: @type-binding.<kind> outer + @type-binding.name/type inner.
// Each match produces one TypeBindingInfo.
func (p *TypeScriptProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices for known outer capture types
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.annotation",
			"type-binding.constructor", "type-binding.alias",
			"type-binding.return":
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	bindings := make([]*pipeline.TypeBindingInfo, 0, len(seen))

	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)

		tb := &pipeline.TypeBindingInfo{
			FilePath: filePath,
		}

		// Determine kind from the outer capture and extract sub-captures
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "type-binding.parameter":
				tb.Kind = "parameter"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
				// Check if optional by looking at whether the text contains "?"
				if strings.Contains(cap.Text, "?:") {
					tb.IsOptional = true
				}
			case "type-binding.annotation":
				tb.Kind = "annotation"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.alias":
				tb.Kind = "alias"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.return":
				tb.Kind = "return"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			}
		}
		if tb.Kind == "" {
			continue
		}

		// Extract the bound name from @type-binding.name sub-capture
		name := findCaptureTextInMatch(captures, matchIdx, "type-binding.name")
		_ = name // available for future use (e.g., binding to symbol)

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// convention: @reference.call.free/member/constructor + @reference.name/receiver inner,
// plus @reference.write.member and @reference.read.member.
// For @reference.read.member, skip if the matchIdx already has a @reference.call.member match
// (because member_expression for read will also match calls).
func (p *TypeScriptProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices and track which ones have call.member
	callMemberMatchIdxs := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "reference.call.member" {
			callMemberMatchIdxs[cap.MatchIndex] = struct{}{}
		}
	}

	// Collect unique match indices for reference captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.call.constructor",
			"reference.write.member", "reference.read.member":
			seen[cap.MatchIndex] = struct{}{}
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
				// Deduplication: skip read.member if same matchIdx already has call.member
				if _, hasCall := callMemberMatchIdxs[matchIdx]; hasCall {
					continue
				}
				ref.Kind = "field_read"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			}
		}
		if ref.Kind == "" {
			continue
		}

		refs = append(refs, ref)
	}
	return refs
}

// ============ TypeScript Helper Functions ============
// These helpers extract names from captured node text
// convention that scope queries have NO name sub-captures.

// tsFirstLine returns the first line of text, trimmed of whitespace.
func tsFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

// tsParseFuncName extracts the function name from a line like
// "function Name(" or "function* Name(" or "async function Name(".
func tsParseFuncName(line string) string {
	line = strings.TrimSpace(line)
	// Remove leading modifiers
	for _, prefix := range []string{"async ", "export ", "declare "} {
		line = strings.TrimPrefix(line, prefix)
	}
	// Handle generator functions: "function* Name("
	if strings.HasPrefix(line, "function*") {
		rest := strings.TrimPrefix(line, "function*")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, '(')
	}
	if strings.HasPrefix(line, "function") {
		rest := strings.TrimPrefix(line, "function")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, '(')
	}
	// Arrow or method: try to get name before "(" or "=>"
	return tsParseIdentifierBefore(line, '(')
}

// tsParseClassName extracts the class name from a line like
// "class Name" or "abstract class Name" or "interface Name".
func tsParseClassName(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "abstract ")
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "declare ")
	if strings.HasPrefix(line, "class ") {
		rest := strings.TrimPrefix(line, "class ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	if strings.HasPrefix(line, "interface ") {
		rest := strings.TrimPrefix(line, "interface ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	if strings.HasPrefix(line, "enum ") {
		rest := strings.TrimPrefix(line, "enum ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	return tsParseIdentifierBefore(line, ' ')
}

// tsParseInterfaceName extracts the interface name from "interface Name".
func tsParseInterfaceName(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "declare ")
	if strings.HasPrefix(line, "interface ") {
		rest := strings.TrimPrefix(line, "interface ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	return ""
}

// tsParseNamespaceName extracts the namespace name from "namespace Name".
func tsParseNamespaceName(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "declare ")
	if strings.HasPrefix(line, "namespace ") {
		rest := strings.TrimPrefix(line, "namespace ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	if strings.HasPrefix(line, "module ") {
		rest := strings.TrimPrefix(line, "module ")
		rest = strings.TrimSpace(rest)
		return tsParseIdentifierBefore(rest, ' ')
	}
	return ""
}

// tsExtractScopeName dispatches to the appropriate name parser based on kind.
func tsExtractScopeName(text, kind string) string {
	line := tsFirstLine(text)
	switch kind {
	case "function":
		return tsParseFuncName(line)
	case "class":
		return tsParseClassName(line)
	case "namespace":
		return tsParseNamespaceName(line)
	default:
		return tsParseIdentifierBefore(line, ' ')
	}
}

// tsModuleName derives a module name from the file path.
// For "src/utils/helper.ts" returns "helper".
func tsModuleName(filePath string) string {
	base := filePath
	if idx := strings.LastIndexByte(base, '/'); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndexByte(base, '\\'); idx >= 0 {
		base = base[idx+1:]
	}
	// Remove extension
	if idx := strings.LastIndexByte(base, '.'); idx >= 0 {
		base = base[:idx]
	}
	if base == "index" {
		// Use parent directory name for index files
		dir := filePath
		if idx := strings.LastIndexByte(dir, '/'); idx >= 0 {
			dir = dir[:idx]
		}
		if idx := strings.LastIndexByte(dir, '/'); idx >= 0 {
			dir = dir[idx+1:]
		}
		if dir != "" && dir != "." {
			return dir
		}
	}
	return base
}

// tsExtractImportPath parses the import path from an import statement text.
// Returns (path, isDefault, isDynamic).
// For "import foo from 'bar'" returns ("bar", true, false).
// For "import { foo } from 'bar'" returns ("bar", false, false).
// For "import('bar')" returns ("bar", false, true).
func tsExtractImportPath(text string) (path string, isDefault bool, isDynamic bool) {
	text = strings.TrimSpace(text)

	// Dynamic import: import('path')
	if strings.HasPrefix(text, "import(") || strings.HasPrefix(text, "import (") {
		isDynamic = true
		rest := text[strings.IndexByte(text, '(')+1:]
		rest = strings.TrimSpace(rest)
		path = tsExtractStringLiteral(rest)
		return
	}

	// Static import: find "from 'path'" or "from \"path\""
	fromIdx := strings.Index(text, " from ")
	if fromIdx >= 0 {
		afterFrom := strings.TrimSpace(text[fromIdx+6:])
		path = tsExtractStringLiteral(afterFrom)

		// Determine if default import
		// "import Name from" — single identifier before "from"
		beforeFrom := strings.TrimSpace(text[len("import"):fromIdx])
		beforeFrom = strings.TrimSpace(beforeFrom)
		if beforeFrom != "" && !strings.HasPrefix(beforeFrom, "{") && !strings.HasPrefix(beforeFrom, "*") {
			isDefault = true
		}
		return
	}

	// Side-effect import: "import 'path'"
	rest := strings.TrimPrefix(text, "import ")
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "'") || strings.HasPrefix(rest, "\"") || strings.HasPrefix(rest, "`") {
		path = tsExtractStringLiteral(rest)
		return
	}

	return
}

// tsExtractImportSymbols extracts named import symbols from import statement text.
// For "import { Foo, Bar as Baz } from 'x'" returns ["Foo", "Baz"].
func tsExtractImportSymbols(text string) []string {
	// Find the { } block
	start := strings.IndexByte(text, '{')
	end := strings.IndexByte(text, '}')
	if start < 0 || end < 0 || end <= start {
		return nil
	}

	content := text[start+1 : end]
	var symbols []string
	for _, part := range strings.Split(content, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Handle "Foo as Bar" — take the alias "Bar"
		if asIdx := strings.Index(part, " as "); asIdx >= 0 {
			part = strings.TrimSpace(part[asIdx+4:])
		}
		part = strings.TrimSpace(part)
		if part != "" {
			symbols = append(symbols, part)
		}
	}
	return symbols
}

// tsExtractStringLiteral extracts the content of a string literal.
// For "'path'" or "\"path\"" or "`path`" returns "path".
func tsExtractStringLiteral(text string) string {
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		return text
	}
	quote := text[0]
	if (quote == '\'' || quote == '"' || quote == '`') && text[len(text)-1] == quote {
		return text[1 : len(text)-1]
	}
	// Try to find the closing quote
	for i := 1; i < len(text); i++ {
		if text[i] == quote {
			return text[1:i]
		}
	}
	return text
}

// tsParseIdentifierBefore extracts an identifier from the beginning of s
// stopping before the first occurrence of any character in the stop set.
func tsParseIdentifierBefore(s string, stop byte) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == stop || ch == '{' || ch == '<' || ch == ':' || ch == '=' {
			break
		}
		if ch == ' ' || ch == '\t' {
			if b.Len() > 0 {
				break
			}
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// tsVisibility returns the visibility string for a TypeScript identifier.
// TypeScript exports are determined by the export keyword, not name casing.
// Since tree-sitter captures do not include export context here, we default
// to "private" and rely on the export.def capture in the full pipeline.
func tsVisibility(name string) string {
	return "private"
}

// ============ TypeScript ScopeResolver ============

// TypeScriptScopeResolver TypeScript scope resolver
type TypeScriptScopeResolver struct {
	provider *TypeScriptProvider
}

// NewTypeScriptScopeResolver creates a TypeScript scope resolver
func NewTypeScriptScopeResolver(provider *TypeScriptProvider) *TypeScriptScopeResolver {
	return &TypeScriptScopeResolver{provider: provider}
}

// Language returns the language label
func (r *TypeScriptScopeResolver) Language() graph.Label {
	return graph.LabelTSFile
}

// LanguageProvider returns the language provider
func (r *TypeScriptScopeResolver) LanguageProvider() *TypeScriptProvider {
	return r.provider
}

// PopulateOwners populates owner relationships (function/method -> file, class -> file, etc.)
// TypeScript symbols are owned by file nodes through ES module semantics
func (r *TypeScriptScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	if model == nil {
		return nil
	}
	// Iterate over the mutable model's symbol table entries
	return model.ForEachSymbol(func(_ string, entry *pipeline.SymbolEntry) error {
		// Find file node
		fileNodes, err := gs.GetNodesByFile("", entry.FilePath)
		if err != nil || len(fileNodes) == 0 {
			return nil // skip, continue iterating
		}

		var fileNode *graph.Node
		for _, n := range fileNodes {
			if n.Label == graph.LabelFile {
				fileNode = n
				break
			}
		}
		if fileNode == nil {
			return nil // skip, continue iterating
		}

		// Create symbol node
		symNode := &graph.Node{
			Name:     entry.Name,
			Label:    graph.Label(entry.Kind),
			FilePath: entry.FilePath,
		}
		symNode.Props.SetProp("startLine", 0)
		symNode.Props.SetProp("endLine", 0)
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
// TypeScript uses class extends single inheritance + implements interfaces
func (r *TypeScriptScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	// TypeScript MRO strategy: single inheritance chain, interfaces do not participate in MRO
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
				}).WithProp("confidence", 0.9))
			}
		}
	}
	return nil
}

// ResolveImportTarget resolves import target
// TypeScript ES module: import path -> module file -> export symbols
func (r *TypeScriptScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	path := imp.Path
	if !strings.Contains(path, "/") && !strings.HasPrefix(path, ".") {
		// npm package, cannot resolve in local repository
		return nil, nil
	}

	// Find matching file nodes in repository
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

// PropagatesReturnTypesAcrossImports TypeScript supports propagating return types across imports
func (r *TypeScriptScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }

// FieldFallbackOnMethodLookup TypeScript dynamic type fallback, falls back to method when field lookup fails
func (r *TypeScriptScopeResolver) FieldFallbackOnMethodLookup() bool { return true }

// UnwrapCollectionAccessor TypeScript unwraps collection accessors (array destructuring, etc.)
func (r *TypeScriptScopeResolver) UnwrapCollectionAccessor() bool { return true }

// CollapseMemberCallsByCallerTarget TypeScript one edge per caller-target
func (r *TypeScriptScopeResolver) CollapseMemberCallsByCallerTarget() bool { return true }

// PopulateNamespaceSiblings TypeScript namespace is implicitly visible across files
func (r *TypeScriptScopeResolver) PopulateNamespaceSiblings() bool { return true }

// HoistTypeBindingsToModule TypeScript hoists type bindings to module
func (r *TypeScriptScopeResolver) HoistTypeBindingsToModule() bool { return true }

// ============ Core methods ============

// MergeBindings merges local binding set and imported binding set
// TypeScript strategy: import takes priority (ES module semantics, import overrides local declaration)
func (r *TypeScriptScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	result.IsImported = false

	// Add local bindings first
	for name, ids := range local.Bindings {
		result.Bindings[name] = ids
	}

	// Import bindings take priority (ES module semantics: import declarations shadow local variables)
	for name, ids := range imported.Bindings {
		result.Bindings[name] = ids
	}

	return result
}

// ArityCompatibility checks parameter compatibility between call site and target
// TypeScript/JavaScript loose check: supports variadic parameters, parameter count does not need to match strictly
func (r *TypeScriptScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true // No arity info, default compatible
	}
	// JS/TS loose check: call parameters >= target required parameters is considered compatible
	return caller.Args >= targetArity || caller.Args+1 >= targetArity
}

// ImportEdgeReason returns the reason description for import edge
func (r *TypeScriptScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.Alias != "" {
		return "namespace-import"
	}
	if len(imp.Symbols) > 0 {
		return "named-import"
	}
	return "re-export"
}

// IsSuperReceiver determines if receiver is a superclass receiver
// In TypeScript, super call in class extends
func (r *TypeScriptScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super"
}

// ============ 4 function-type Hooks ============

// PopulateRangeBindings for-of / for-in variable bindings
// TypeScript destructuring assignments are captured as local variables at tree-sitter parsing stage
func (r *TypeScriptScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
	// TypeScript for-of variables are captured as local variables at tree-sitter parsing stage
	// This is a no-op, TS does not need extra range binding logic
}

// CollectScopeContextPaths collects scope context file paths
// TypeScript module scope, current file path
func (r *TypeScriptScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

// EmitPostResolutionEdges post-processing edge emission
// TypeScript does not need extra post-processing edges
func (r *TypeScriptScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
	// TypeScript has no post-processing edge requirements
}

// EmitUnresolvedReceiverEdges untyped receiver fallback
// TypeScript has a type system but may still encounter dynamic scenarios, currently returns 0
func (r *TypeScriptScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	// TypeScript has type annotations to assist resolution, currently returns 0 fallback edges
	return 0
}