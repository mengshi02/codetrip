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

// ============ Rust LanguageProvider ============

// RustProvider Rust language provider
type RustProvider struct{}

// NewRustProvider creates a Rust language provider
func NewRustProvider() *RustProvider {
	return &RustProvider{}
}

// Language returns the language label
func (p *RustProvider) Language() graph.Label {
	return graph.LabelRustFile
}

// Captures returns node capture rules
func (p *RustProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_item name: (identifier) @fn.name) @fn.def
			(impl_item type: (type_identifier) @impl.type) @impl.def
			(struct_item name: (type_identifier) @struct.name) @struct.def
			(enum_item name: (type_identifier) @enum.name) @enum.def
			(trait_item name: (type_identifier) @trait.name) @trait.def
			(type_item name: (type_identifier) @type.name) @type.def
			(declaration_list (let_declaration pattern: (identifier) @var.name)) @var.def
			(use_declaration (scoped_identifier) @use.path) @use.def
			(macro_invocation macro: (identifier) @macro.name) @macro.def
			(attribute_item) @attr.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":     graph.LabelFunction,
			"impl.def":   graph.LabelImpl,
			"struct.def": graph.LabelStruct,
			"enum.def":   graph.LabelEnum,
			"trait.def":  graph.LabelTrait,
			"type.def":   graph.LabelTypeAlias,
			"var.def":    graph.LabelVariable,
			"use.def":    graph.LabelImport,
			"macro.def":  graph.LabelMacro,
			"attr.def":   graph.LabelDecorator,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *RustProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call_expression function: (field_expression value: (identifier) @call.receiver field: (field_identifier) @call.method)) @call.site
			(call_expression function: (identifier) @call.fn) @call.fn.site
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

// ClassExtractConfig returns class extraction config (Rust uses struct/enum instead of classes)
func (p *RustProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(struct_item name: (type_identifier) @class.name) @class.def
			(enum_item name: (type_identifier) @class.name) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelStruct,
		},
		NameKey: "class.name",
		BaseKey: "",
	}
}

// FieldExtractConfig returns field extraction config
func (p *RustProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(field_identifier) @field.name
		`,
		CaptureMap: map[string]graph.Label{
			"field.name": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "",
	}
}

// ImportResolveConfig returns import resolution config
func (p *RustProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(use_declaration (scoped_identifier) @import.path) @import.decl
			(use_declaration (identifier) @import.path) @import.simple
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl":   graph.LabelImport,
			"import.simple": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "",
		ItemsKey:    "",
		IsDotImport: false,
		IsWildcard:  true,
	}
}

// Interpret interprets capture results
func (p *RustProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
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
		case "impl.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelImpl,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "struct.def", "class.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelStruct,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
			result.Classes = append(result.Classes, ClassInfo{
				Name: cap.Name, FilePath: captures.Filepath,
				StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "enum.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelEnum,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "trait.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTrait,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "type.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTypeAlias,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "macro.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelMacro,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "attr.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelDecorator,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "use.def", "import.decl", "import.simple":
			path := cap.Text
			// Remove trailing ::{...} or ::*
			if idx := strings.Index(path, "::"); idx >= 0 {
				// Keep full path, including ::
			}
			if strings.HasSuffix(path, "::*") {
				path = strings.TrimSuffix(path, "::*")
			}
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
		case "field.name":
			result.Fields = append(result.Fields, FieldInfo{
				Name:     cap.Text,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (Rust uses named)
func (p *RustProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamed
}

// ============ Scope-Based Pipeline Methods ============

// TreeSitterLanguage returns the Rust tree-sitter language.
func (p *RustProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.RustLanguage()
}

// QuerySet returns all S-expression queries for Rust extraction.
func (p *RustProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		// Scope query: captures scope-defining nodes without inner name
		// sub-captures. The scope name is extracted from the captured node's
		// text in InterpretScope (like the Go provider's goExtractScopeName).
		Scope: `
			(source_file) @scope.module
			(struct_item) @scope.class
			(trait_item) @scope.class
			(impl_item) @scope.class
			(enum_item) @scope.class
			(function_item) @scope.function
			(closure_expression) @scope.function
			(block) @scope.block
		`,
		// Declaration query: the outer capture determines the symbol kind;
		// inner @declaration.name extracts the declared name.
		Declaration: `
			(struct_item name: (type_identifier) @declaration.name) @declaration.struct
			(trait_item name: (type_identifier) @declaration.name) @declaration.trait
			(enum_item name: (type_identifier) @declaration.name) @declaration.enum
			(impl_item type: (type_identifier) @declaration.name) @declaration.impl
			(function_item name: (identifier) @declaration.name) @declaration.function
			(field_declaration name: (field_identifier) @declaration.name type: (_) @declaration.field-type) @declaration.property
			(let_declaration pattern: (identifier) @declaration.name) @declaration.variable
			(const_item name: (identifier) @declaration.name) @declaration.const
			(static_item name: (identifier) @declaration.name) @declaration.static
			(macro_definition name: (identifier) @declaration.name) @declaration.macro
		`,
		// Import query: a single outer capture per use declaration.
		Import: `
			(use_declaration) @import.statement
		`,
		// TypeBinding query: binds identifiers to type annotations.
		// Covers parameters, let-declarations, constructors, call-return
		// inference, identifier aliases, and function return types.
		TypeBinding: `
			(parameter pattern: (identifier) @type-binding.param.name type: (type_identifier) @type-binding.param.type) @type-binding.parameter
			(parameter pattern: (identifier) @type-binding.param.name type: (primitive_type) @type-binding.param.type) @type-binding.parameter
			(let_declaration pattern: (identifier) @type-binding.var.name type: (type_identifier) @type-binding.var.type) @type-binding.variable
			(let_declaration pattern: (identifier) @type-binding.var.name type: (primitive_type) @type-binding.var.type) @type-binding.variable
			(let_declaration pattern: (identifier) @type-binding.var.name value: (struct_expression name: (type_identifier) @type-binding.var.type)) @type-binding.constructor
			(let_declaration pattern: (identifier) @type-binding.var.name value: (call_expression function: (identifier) @type-binding.var.type)) @type-binding.call-return
			(let_declaration pattern: (identifier) @type-binding.alias.name value: (identifier) @type-binding.alias.target) @type-binding.alias
			(function_item return_type: (type_identifier) @type-binding.return.type) @type-binding.return
			(function_item return_type: (primitive_type) @type-binding.return.type) @type-binding.return
		`,
		// Reference query: classified references. To avoid duplicate matches,
		// member_call (call_expression with field_expression) is matched before
		// free_call (plain identifier call). struct_expression captures
		// constructor calls; field_write captures assignment to fields.
		Reference: `
			(call_expression function: (identifier) @reference.name) @reference.call.free
			(call_expression function: (field_expression value: (identifier) @reference.receiver field: (field_identifier) @reference.name)) @reference.call.member
			(struct_expression name: (type_identifier) @reference.name) @reference.call.constructor
			(field_expression value: (identifier) @reference.receiver field: (field_identifier) @reference.name) @reference.read
			(assignment_expression left: (field_expression value: (identifier) @reference.receiver field: (field_identifier) @reference.name)) @reference.write
			(macro_invocation macro: (identifier) @reference.name) @reference.macro
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. After all scopes are created,
// ParentID is computed by nesting (child scope's line range is within
// parent scope's line range). Scope names are extracted from the
// captured node's text (no inner @name sub-captures) to avoid
// tree-sitter emitting separate matches for inner captures.
func (p *RustProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = rustModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = rustExtractScopeName(cap.Text, "function")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = rustExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.block":
				kind = "block"
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
// Each match produces one symbol. The outer capture (e.g. "declaration.struct")
// determines the symbol kind; inner @declaration.name extracts the declared name.
func (p *RustProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "declaration.struct", "declaration.trait", "declaration.enum",
				"declaration.impl", "declaration.function", "declaration.property", "declaration.variable",
				"declaration.const", "declaration.static", "declaration.macro":
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

		switch outerNodeType {
		case "declaration.struct":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelStruct
		case "declaration.trait":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelTrait
		case "declaration.enum":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelEnum
		case "declaration.impl":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelImpl
		case "declaration.function":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelFunction
		case "declaration.property":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelProperty
		case "declaration.variable":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelVariable
		case "declaration.const":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelConst
		case "declaration.static":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelStatic
		case "declaration.macro":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelMacro
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Each match (import.statement) produces one ImportInfo. The full path
// text is extracted from the captured use_declaration node.
func (p *RustProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices that are outer import captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "import.statement" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	imports := make([]*pipeline.ImportInfo, 0, len(seen))

	for matchIdx := range seen {
		// Extract path from the outer capture's text (the full use_declaration)
		var path string
		var line int
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && cap.NodeType == "import.statement" {
				path = rustExtractUsePath(cap.Text)
				line = cap.StartRow
				break
			}
		}

		// Remove trailing ::* wildcard
		isWildcard := strings.HasSuffix(path, "::*")
		if isWildcard {
			path = strings.TrimSuffix(path, "::*")
		}

		imports = append(imports, &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			IsWildcard: isWildcard,
			Line:       line,
		})
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Each match produces one TypeBindingInfo.
func (p *RustProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.variable", "type-binding.constructor",
			"type-binding.call-return", "type-binding.alias", "type-binding.return":
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
			case "type-binding.parameter":
				tb.Kind = "parameter"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.param.type")
				tb.StartLine = cap.StartRow
			case "type-binding.variable":
				tb.Kind = "variable"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.var.type")
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.var.type")
				tb.StartLine = cap.StartRow
			case "type-binding.call-return":
				tb.Kind = "call-return"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.var.type")
				tb.StartLine = cap.StartRow
			case "type-binding.alias":
				tb.Kind = "alias"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.alias.target")
				tb.StartLine = cap.StartRow
			case "type-binding.return":
				tb.Kind = "return"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.return.type")
				tb.StartLine = cap.StartRow
			}
		}
		if tb.Kind == "" {
			continue
		}

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// Each match produces one ReferenceInfo.
func (p *RustProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.call.constructor",
			"reference.read", "reference.write", "reference.macro":
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
			case "reference.read":
				ref.Kind = "field_read"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.write":
				ref.Kind = "field_write"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.macro":
				ref.Kind = "macro"
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

// ============ Rust Scope Extraction Helpers ============

// rustFirstLine returns the first line of a text string.
func rustFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// rustParseFuncName extracts the function name from a Rust declaration line.
// Handles: "fn name(" or "pub fn name("
func rustParseFuncName(line string) string {
	idx := strings.Index(line, "fn ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+3:] // after "fn "

	// Skip whitespace
	rest = strings.TrimLeft(rest, " \t")

	// Extract name before ( or < or space or {
	for i, ch := range rest {
		if ch == '(' || ch == '<' || ch == ' ' || ch == '{' {
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// rustParseTypeName extracts the type name from a Rust struct/enum/trait/impl declaration line.
// Handles: "struct Name", "enum Name", "trait Name", "impl Name", "impl Trait for Type"
func rustParseTypeName(line string) string {
	for _, keyword := range []string{"struct ", "enum ", "trait ", "impl "} {
		idx := strings.Index(line, keyword)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(keyword):]

		// For impl, handle "impl Trait for Type" — extract the impl name
		// (which could be a trait or a type depending on context)
		if keyword == "impl " {
			// "impl Trait for Type" — we want the type after "for"
			// "impl Type" — we want Type directly
			forIdx := strings.Index(rest, " for ")
			if forIdx >= 0 {
				// This is "impl Trait for Type" — return the implementing type
				typePart := rest[forIdx+5:]
				typePart = strings.TrimLeft(typePart, " \t")
				for i, ch := range typePart {
					if ch == '{' || ch == '<' || ch == ' ' || ch == '(' || ch == '&' {
						if i == 0 {
							return ""
						}
						return typePart[:i]
					}
				}
				return typePart
			}
		}

		rest = strings.TrimLeft(rest, " \t")
		for i, ch := range rest {
			if ch == '{' || ch == '<' || ch == ' ' || ch == '(' || ch == ':' {
				if i == 0 {
					return ""
				}
				return rest[:i]
			}
		}
		return rest
	}
	return ""
}

// rustExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the function/type name.
func rustExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := rustFirstLine(text)

	switch kind {
	case "function":
		return rustParseFuncName(firstLine)
	case "class":
		return rustParseTypeName(firstLine)
	}
	return ""
}

// rustModuleName infers a Rust module name from file path.
// Rust modules are named by directory structure (mod.rs) or file name.
func rustModuleName(filePath string) string {
	// mod.rs -> parent directory name
	if strings.HasSuffix(filePath, "/mod.rs") || filePath == "mod.rs" {
		dir := filepath.Dir(filePath)
		base := filepath.Base(dir)
		if base == "" || base == "." {
			return "root"
		}
		return base
	}
	// Strip .rs extension
	name := filepath.Base(filePath)
	name = strings.TrimSuffix(name, ".rs")
	return name
}

// rustExtractUsePath extracts the import path from a use declaration text.
// Handles: "use std::collections::HashMap;", "use std::io::*;",
// "use crate::module::Item;"
func rustExtractUsePath(text string) string {
	text = strings.TrimSpace(text)
	// Remove "use " prefix
	if !strings.HasPrefix(text, "use ") {
		return text
	}
	text = text[4:] // after "use "

	// Remove trailing semicolon
	text = strings.TrimRight(text, ";")

	// Remove trailing curly-brace group: "std::collections::{HashMap, BTreeMap}"
	if idx := strings.Index(text, "::{"); idx >= 0 {
		text = text[:idx]
	}

	// Remove trailing wildcard: "std::io::*"
	if strings.HasSuffix(text, "::*") {
		text = strings.TrimSuffix(text, "::*")
	}

	return strings.TrimSpace(text)
}

// ============ Rust ScopeResolver ============

// RustScopeResolver Rust scope resolver
type RustScopeResolver struct {
	provider *RustProvider
}

// NewRustScopeResolver creates a Rust scope resolver
func NewRustScopeResolver(provider *RustProvider) *RustScopeResolver {
	return &RustScopeResolver{provider: provider}
}

func (r *RustScopeResolver) Language() graph.Label {
	return graph.LabelRustFile
}

func (r *RustScopeResolver) LanguageProvider() *RustProvider {
	return r.provider
}

func (r *RustScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	return nil
}

// BuildMRO Rust has no traditional inheritance, trait has interface semantics
func (r *RustScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	return nil
}

func (r *RustScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	return nil, nil
}

// ============ Boolean switches ============

func (r *RustScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }
func (r *RustScopeResolver) FieldFallbackOnMethodLookup() bool        { return false }
func (r *RustScopeResolver) UnwrapCollectionAccessor() bool           { return false }
func (r *RustScopeResolver) CollapseMemberCallsByCallerTarget() bool  { return true }
func (r *RustScopeResolver) PopulateNamespaceSiblings() bool          { return true } // Rust same-module pub visible
func (r *RustScopeResolver) HoistTypeBindingsToModule() bool          { return false }

// ============ 4 core methods ============

// MergeBindings merges binding sets (Rust use statements explicit import, import takes precedence)
func (r *RustScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
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

// ArityCompatibility Rust static typing, strict checking
func (r *RustScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true
	}
	return caller.Args == targetArity
}

func (r *RustScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "wildcard-use"
	}
	if len(imp.Symbols) > 0 {
		return "named-use"
	}
	return "extern-crate"
}

func (r *RustScopeResolver) IsSuperReceiver(recv string) bool {
	// Rust has no super keyword, trait objects use dyn
	return recv == "Self"
}

// ============ 4 functional hooks ============

func (r *RustScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
}

func (r *RustScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

func (r *RustScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
}

func (r *RustScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	return 0
}
