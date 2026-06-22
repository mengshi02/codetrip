package lang

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ============ C# LanguageProvider ============

// CSharpProvider C# language provider
type CSharpProvider struct{}

// NewCSharpProvider creates a C# language provider
func NewCSharpProvider() *CSharpProvider {
	return &CSharpProvider{}
}

// Language returns the language label
func (p *CSharpProvider) Language() graph.Label {
	return graph.LabelCSharpFile
}

// Captures returns node capture rules
func (p *CSharpProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(method_declaration name: (identifier) @method.name) @method.def
			(class_declaration name: (identifier) @class.name) @class.def
			(interface_declaration name: (identifier) @iface.name) @iface.def
			(struct_declaration name: (identifier) @struct.name) @struct.def
			(enum_declaration name: (identifier) @enum.name) @enum.def
			(constructor_declaration name: (identifier) @ctor.name) @ctor.def
			(field_declaration variable_declaration variable_declarator (identifier) @field.name) @field.def
			(local_declaration_statement variable_declaration variable_declarator (identifier) @var.name) @var.def
			(using_statement) @using.def
		`,
		CaptureMap: map[string]graph.Label{
			"method.def": graph.LabelMethod,
			"class.def":  graph.LabelClass,
			"iface.def":  graph.LabelInterface,
			"struct.def": graph.LabelStruct,
			"enum.def":   graph.LabelEnum,
			"ctor.def":   graph.LabelConstructor,
			"field.def":  graph.LabelField,
			"var.def":    graph.LabelVariable,
			"using.def":  graph.LabelImport,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *CSharpProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(invocation_expression
				function: (member_access_expression
					expression: (identifier) @call.receiver
					name: (identifier) @call.method
				)
			) @call.site
			(invocation_expression
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
func (p *CSharpProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_declaration name: (identifier) @class.name (base_list (identifier) @class.base)?) @class.def
			(interface_declaration name: (identifier) @class.name) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelClass,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction config
func (p *CSharpProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(field_declaration variable_declaration type: (_) @field.type variable_declarator (identifier) @field.name) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "field.type",
	}
}

// ImportResolveConfig returns import resolution config
func (p *CSharpProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(using_directive (identifier) @import.path) @import.decl
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "",
		ItemsKey:    "",
		IsDotImport: false,
		IsWildcard:  false,
	}
}

// Interpret interprets capture results
func (p *CSharpProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
	result := &InterpretResult{
		Symbols:   make([]pipeline.SymbolInfo, 0),
		Imports:   make([]ImportInfo, 0),
		CallSites: make([]CallSite, 0),
		Classes:   make([]ClassInfo, 0),
		Fields:    make([]FieldInfo, 0),
	}

	for _, cap := range captures.Captures {
		switch cap.NodeType {
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
		case "struct.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelStruct,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "enum.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelEnum,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "ctor.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelConstructor,
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
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "using.def", "import.decl":
			path := cap.Text
			result.Imports = append(result.Imports, ImportInfo{
				Path: path, FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
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
				FilePath:   captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (C# uses named)
func (p *CSharpProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamed
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the C# tree-sitter language.
func (p *CSharpProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.CSharpLanguage()
}

// QuerySet returns all S-expression queries for C# extraction.
// Follows capture namespace conventions:
//   - Scope: @scope.<kind> (NO name sub-captures to avoid match doubling)
//   - Declaration: @declaration.<kind> + @declaration.name
//   - Import: @import.statement (single anchor)
//   - TypeBinding: @type-binding.<kind> + @type-binding.name + @type-binding.type
//   - Reference: @reference.call.<kind> + @reference.name + @reference.receiver
func (p *CSharpProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(compilation_unit) @scope.module
			(namespace_declaration) @scope.namespace
			(file_scoped_namespace_declaration) @scope.namespace
			(class_declaration) @scope.class
			(interface_declaration) @scope.class
			(struct_declaration) @scope.class
			(record_declaration) @scope.class
			(enum_declaration) @scope.class
			(method_declaration) @scope.function
			(constructor_declaration) @scope.function
			(destructor_declaration) @scope.function
			(local_function_statement) @scope.function
			(operator_declaration) @scope.function
			(conversion_operator_declaration) @scope.function
		`,
		Declaration: `
			(class_declaration name: (identifier) @declaration.name) @declaration.class
			(interface_declaration name: (identifier) @declaration.name) @declaration.interface
			(struct_declaration name: (identifier) @declaration.name) @declaration.struct
			(record_declaration name: (identifier) @declaration.name) @declaration.record
			(enum_declaration name: (identifier) @declaration.name) @declaration.enum
			(method_declaration name: (identifier) @declaration.name) @declaration.method
			(constructor_declaration name: (identifier) @declaration.name) @declaration.constructor
			(local_function_statement name: (identifier) @declaration.name) @declaration.function
			(property_declaration name: (identifier) @declaration.name) @declaration.property
			(field_declaration (variable_declaration (variable_declarator name: (identifier) @declaration.name))) @declaration.variable
			(local_declaration_statement (variable_declaration (variable_declarator name: (identifier) @declaration.name))) @declaration.variable
			(namespace_declaration name: (_) @declaration.name) @declaration.namespace
			(file_scoped_namespace_declaration name: (_) @declaration.name) @declaration.namespace
		`,
		Import: `
			(using_directive) @import.statement
		`,
		TypeBinding: `
			(parameter type: (identifier) @type-binding.type name: (identifier) @type-binding.name) @type-binding.parameter
			(parameter type: (generic_name) @type-binding.type name: (identifier) @type-binding.name) @type-binding.parameter
			(local_declaration_statement (variable_declaration type: (identifier) @type-binding.type (variable_declarator name: (identifier) @type-binding.name))) @type-binding.annotation
			(local_declaration_statement (variable_declaration (variable_declarator name: (identifier) @type-binding.name (object_creation_expression type: (identifier) @type-binding.type)))) @type-binding.constructor
			(method_declaration returns: (identifier) @type-binding.type name: (identifier) @type-binding.name) @type-binding.return
		`,
		Reference: `
			(invocation_expression function: (identifier) @reference.name) @reference.call.free
			(invocation_expression function: (member_access_expression expression: (_) @reference.receiver name: (identifier) @reference.name)) @reference.call.member
			(object_creation_expression type: (identifier) @reference.name) @reference.call.constructor
			(assignment_expression left: (member_access_expression expression: (_) @reference.receiver name: (identifier) @reference.name)) @reference.write.member
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. Names are extracted from cap.Text
// using helper functions (NOT from name sub-captures, which cause match
// count doubling with tree-sitter's outer/inner capture semantics).
// After all scopes are created, ParentID is computed by nesting.
func (p *CSharpProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = csharpModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.namespace":
				kind = "namespace"
				name = csharpExtractScopeName(cap.Text, "namespace")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = csharpExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = csharpExtractScopeName(cap.Text, "function")
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
// Uses conventions: @declaration.<kind> as outer capture + @declaration.name
// as uniform name sub-capture. Each match produces one symbol.
func (p *CSharpProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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

		// Find the outer (parent) capture in this match to determine symbol kind
		var outerNodeType string
		var startLine, endLine int
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "declaration.class", "declaration.interface", "declaration.struct",
				"declaration.record", "declaration.enum", "declaration.method",
				"declaration.constructor", "declaration.function", "declaration.property",
				"declaration.variable", "declaration.namespace":
				outerNodeType = cap.NodeType
				startLine = cap.StartRow
				endLine = cap.EndRow
			}
		}
		if outerNodeType == "" {
			continue
		}

		sym := &pipeline.SymbolInfo{
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
		}

		// Use uniform @declaration.name for all kinds
		sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		switch outerNodeType {
		case "declaration.class":
			sym.Label = graph.LabelClass
		case "declaration.interface":
			sym.Label = graph.LabelInterface
		case "declaration.struct":
			sym.Label = graph.LabelStruct
		case "declaration.record":
			sym.Label = graph.LabelClass // record is a reference type like class
		case "declaration.enum":
			sym.Label = graph.LabelEnum
		case "declaration.method":
			sym.Label = graph.LabelMethod
		case "declaration.constructor":
			sym.Label = graph.LabelConstructor
		case "declaration.function":
			sym.Label = graph.LabelFunction
		case "declaration.property":
			sym.Label = graph.LabelProperty
		case "declaration.variable":
			sym.Label = graph.LabelVariable
		case "declaration.namespace":
			sym.Label = graph.LabelPackage
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Uses convention: @import.statement as single anchor capture.
// Path is extracted from cap.Text using csharpExtractImportPath.
func (p *CSharpProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices that are import statement captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "import.statement" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	imports := make([]*pipeline.ImportInfo, 0, len(seen))

	for matchIdx := range seen {
		// Find the outer capture to get text and line
		var line int
		var text string
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && cap.NodeType == "import.statement" {
				line = cap.StartRow
				text = cap.Text
				break
			}
		}

		path := csharpExtractImportPath(text)

		imports = append(imports, &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			Line:       line,
		})
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Uses conventions: @type-binding.<kind> as outer capture +
// @type-binding.name and @type-binding.type as uniform sub-captures.
func (p *CSharpProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.annotation",
			"type-binding.constructor", "type-binding.return":
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	bindings := make([]*pipeline.TypeBindingInfo, 0, len(seen))

	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)

		tb := &pipeline.TypeBindingInfo{
			FilePath: filePath,
		}

		// Determine kind from the outer capture and extract uniform sub-captures
		for _, cap := range matchCaps {
			switch cap.NodeType {
			case "type-binding.parameter":
				tb.Kind = "parameter"
				tb.StartLine = cap.StartRow
			case "type-binding.annotation":
				tb.Kind = "annotation"
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.StartLine = cap.StartRow
			case "type-binding.return":
				tb.Kind = "return"
				tb.StartLine = cap.StartRow
			}
		}
		if tb.Kind == "" {
			continue
		}

		// Use uniform @type-binding.name and @type-binding.type
		tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// Uses conventions: @reference.call.<kind> as outer capture +
// uniform @reference.name and @reference.receiver sub-captures.
func (p *CSharpProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member",
			"reference.call.constructor", "reference.write.member":
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	refs := make([]*pipeline.ReferenceInfo, 0, len(seen))

	for matchIdx := range seen {
		ref := &pipeline.ReferenceInfo{
			FilePath: filePath,
		}

		// Determine kind from the outer capture and extract uniform sub-captures
		for _, cap := range captures {
			if cap.MatchIndex != matchIdx {
				continue
			}
			switch cap.NodeType {
			case "reference.call.free":
				ref.Kind = "free_call"
				ref.StartLine = cap.StartRow
			case "reference.call.member":
				ref.Kind = "member_call"
				ref.StartLine = cap.StartRow
			case "reference.call.constructor":
				ref.Kind = "constructor"
				ref.StartLine = cap.StartRow
			case "reference.write.member":
				ref.Kind = "field_write"
				ref.StartLine = cap.StartRow
			}
		}
		if ref.Kind == "" {
			continue
		}

		// Use uniform @reference.name and @reference.receiver
		ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
		ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")

		refs = append(refs, ref)
	}
	return refs
}

// ============ C# Helper Functions ============

// csharpModuleName extracts the module name from a file path.
// For C#, the module name is derived from the top-level directory name.
func csharpModuleName(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(dir)
	if base == "." || base == "" {
		return "global"
	}
	return base
}

// csharpExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the namespace/class/function name.
func csharpExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := firstLineOfCaptureText(text)

	switch kind {
	case "namespace":
		return csharpParseNamespaceName(firstLine)
	case "class":
		return csharpParseClassName(firstLine)
	case "function":
		return csharpParseMethodName(firstLine)
	}
	return ""
}

// firstLineOfCaptureText returns the first line of a text string.
func firstLineOfCaptureText(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// csharpParseNamespaceName extracts the namespace name from a namespace declaration line.
// Handles: "namespace Foo.Bar" or "namespace Foo {"
func csharpParseNamespaceName(line string) string {
	line = strings.TrimSpace(line)
	idx := strings.Index(line, "namespace ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+10:] // after "namespace "
	rest = strings.TrimSpace(rest)

	// Remove trailing { if present
	if braceIdx := strings.Index(rest, "{"); braceIdx >= 0 {
		rest = rest[:braceIdx]
	}
	rest = strings.TrimSpace(rest)
	return rest
}

// csharpParseClassName extracts the class/interface/struct/record/enum name from a declaration line.
// Handles: "class Foo", "class Foo<T>", "class Foo : Bar", etc.
func csharpParseClassName(line string) string {
	line = strings.TrimSpace(line)

	// Find the keyword and skip modifiers
	// Common patterns: "class Name", "interface Name", "struct Name", "record Name", "enum Name"
	keywords := []string{"class ", "interface ", "struct ", "record ", "enum "}
	for _, kw := range keywords {
		idx := strings.Index(line, kw)
		if idx >= 0 {
			rest := line[idx+len(kw):]
			return csharpExtractIdentifier(rest)
		}
	}
	return ""
}

// csharpParseMethodName extracts the method/constructor name from a declaration line.
// Handles: "void Foo(", "Foo(", "static void Foo(", "public int Bar("
func csharpParseMethodName(line string) string {
	line = strings.TrimSpace(line)

	// Find opening parenthesis — the name is just before it
	parenIdx := strings.LastIndex(line, "(")
	if parenIdx < 0 {
		return ""
	}
	beforeParen := line[:parenIdx]

	// The name is the last identifier before the parenthesis
	// Walk backwards to find the identifier
	end := len(beforeParen) - 1
	for end >= 0 && beforeParen[end] == ' ' {
		end--
	}
	if end < 0 {
		return ""
	}

	start := end
	for start >= 0 {
		ch := beforeParen[start]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			start--
		} else {
			break
		}
	}
	name := beforeParen[start+1 : end+1]

	// Filter out C# keywords that might appear before the name
	csharpKeywords := map[string]bool{
		"void": true, "int": true, "string": true, "bool": true, "double": true,
		"float": true, "long": true, "byte": true, "char": true, "decimal": true,
		"short": true, "object": true, "var": true, "dynamic": true,
		"async": true, "static": true, "public": true, "private": true,
		"protected": true, "internal": true, "override": true, "virtual": true,
		"abstract": true, "sealed": true, "new": true, "extern": true,
		"partial": true, "unsafe": true,
	}
	if csharpKeywords[name] {
		// This was a return type, not the name — try to find the actual name
		// Look for the pattern: "TypeName MethodName(" where TypeName might be generic
		// Scan for the identifier after the type keyword
		rest := beforeParen[:start+1]
		return csharpParseMethodName(rest)
	}

	return name
}

// csharpExtractIdentifier extracts the first identifier from a string.
// Handles generic types like "Foo<T>" by stopping at '<'.
func csharpExtractIdentifier(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}

	// Find end of identifier: stop at space, '<', ':', '{', ',', ';', '('
	end := 0
	for _, ch := range rest {
		if ch == ' ' || ch == '<' || ch == ':' || ch == '{' || ch == ',' || ch == ';' || ch == '(' {
			break
		}
		end++
	}
	if end == 0 {
		return ""
	}
	return rest[:end]
}

// csharpExtractImportPath extracts the import path from a using directive text.
// Handles: "using System;", "using System.Collections.Generic;",
// "using static System.Math;", "using Foo = Bar.Baz;"
func csharpExtractImportPath(text string) string {
	if text == "" {
		return ""
	}
	firstLine := firstLineOfCaptureText(text)
	firstLine = strings.TrimSpace(firstLine)

	// Remove "using " prefix
	if !strings.HasPrefix(firstLine, "using ") {
		return ""
	}
	rest := firstLine[6:] // after "using "
	rest = strings.TrimSpace(rest)

	// Handle "using static" — extract "static Namespace.Path"
	if strings.HasPrefix(rest, "static ") {
		rest = rest[7:] // after "static "
		rest = strings.TrimSpace(rest)
	}

	// Handle "using Alias = Namespace.Path;" — extract the RHS
	if eqIdx := strings.Index(rest, "="); eqIdx >= 0 {
		rest = rest[eqIdx+1:]
		rest = strings.TrimSpace(rest)
	}

	// Remove trailing semicolon
	rest = strings.TrimSuffix(rest, ";")
	rest = strings.TrimSpace(rest)

	return rest
}

// ============ C# ScopeResolver ============

// CSharpScopeResolver C# scope resolver
type CSharpScopeResolver struct {
	provider *CSharpProvider
}

// NewCSharpScopeResolver creates a C# scope resolver
func NewCSharpScopeResolver(provider *CSharpProvider) *CSharpScopeResolver {
	return &CSharpScopeResolver{provider: provider}
}

func (r *CSharpScopeResolver) Language() graph.Label {
	return graph.LabelCSharpFile
}

func (r *CSharpScopeResolver) LanguageProvider() *CSharpProvider {
	return r.provider
}

func (r *CSharpScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	return nil
}

func (r *CSharpScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	return nil
}

func (r *CSharpScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	return nil, nil
}

// ============ Boolean switches ============

func (r *CSharpScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }
func (r *CSharpScopeResolver) FieldFallbackOnMethodLookup() bool        { return false }
func (r *CSharpScopeResolver) UnwrapCollectionAccessor() bool           { return false }
func (r *CSharpScopeResolver) CollapseMemberCallsByCallerTarget() bool  { return true }
func (r *CSharpScopeResolver) PopulateNamespaceSiblings() bool          { return false }
func (r *CSharpScopeResolver) HoistTypeBindingsToModule() bool          { return true }

// ============ 4 core methods ============

// MergeBindings merges binding sets (C# named import takes precedence)
func (r *CSharpScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	for name, ids := range imported.Bindings {
		result.Bindings[name] = ids
	}
	for name, ids := range local.Bindings {
		if _, exists := result.Bindings[name]; !exists {
			result.Bindings[name] = ids
		}
	}
	return result
}

func (r *CSharpScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true
	}
	return caller.Args == targetArity
}

func (r *CSharpScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "wildcard-import"
	}
	if len(imp.Symbols) > 0 {
		return "named-import"
	}
	return "namespace-import"
}

func (r *CSharpScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super" || recv == "base"
}

// ============ 4 functional hooks ============

func (r *CSharpScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
}

func (r *CSharpScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

func (r *CSharpScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
}

func (r *CSharpScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	return 0
}
