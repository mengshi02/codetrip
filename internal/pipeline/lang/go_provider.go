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

// ============ Go LanguageProvider ============

// GoProvider Go language provider
type GoProvider struct{}

// NewGoProvider creates a Go language provider
func NewGoProvider() *GoProvider {
	return &GoProvider{}
}

// Language returns language label
func (g *GoProvider) Language() graph.Label {
	return graph.LabelGoFile
}

// Captures returns node capture rules
func (g *GoProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_declaration name: (identifier) @fn.name) @fn.def
			(method_declaration receiver: (parameter_list) @method.recv name: (field_identifier) @method.name) @method.def
			(type_declaration (type_spec name: (type_identifier) @type.name type: (struct_type) @struct.body)) @struct.def
			(type_declaration (type_spec name: (type_identifier) @type.name type: (interface_type) @iface.body)) @iface.def
			(var_spec name: (identifier) @var.name) @var.def
			(const_spec name: (identifier) @const.name) @const.def
			(import_declaration (import_spec path: (interpreted_string_literal) @import.path)) @import.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":     graph.LabelFunction,
			"method.def": graph.LabelMethod,
			"struct.def": graph.LabelStruct,
			"iface.def":  graph.LabelInterface,
			"var.def":    graph.LabelVariable,
			"const.def":  graph.LabelConst,
			"import.def": graph.LabelImport,
		},
	}
}

// CallExtractConfig returns call extraction configuration
func (g *GoProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call_expression
				function: (selector_expression
					operand: (identifier) @call.receiver
					field: (field_identifier) @call.method
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
func (g *GoProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(type_declaration (type_spec name: (type_identifier) @class.name type: (struct_type)) @class.def)
			(type_declaration (type_spec name: (type_identifier) @class.name type: (interface_type)) @class.def)
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelStruct,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction configuration
func (g *GoProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(field_declaration_list
				(field_declaration name: (field_identifier) @field.name type: (_) @field.type)
			) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "field.type",
	}
}

// ImportResolveConfig returns import resolution configuration
func (g *GoProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(import_declaration
				(import_spec
					path: (interpreted_string_literal) @import.path
					name: (identifier)? @import.alias
				)
			) @import.decl
			(import_declaration
				(import_spec_list
					(import_spec
						path: (interpreted_string_literal) @import.path
						name: (identifier)? @import.alias
					)
				)
			) @import.multi
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl":  graph.LabelImport,
			"import.multi": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "import.alias",
		ItemsKey:    "",
		IsDotImport: true,
		IsWildcard:  true,
	}
}

// Interpret interprets capture results into structured symbol information
func (g *GoProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
	cr := captures
	result := &InterpretResult{
		Symbols:   make([]pipeline.SymbolInfo, 0),
		Imports:   make([]ImportInfo, 0),
		CallSites: make([]CallSite, 0),
		Classes:   make([]ClassInfo, 0),
		Fields:    make([]FieldInfo, 0),
	}

	for _, cap := range cr.Captures {
		switch cap.NodeType {
		case "fn.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelFunction,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
		case "method.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelMethod,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
		case "struct.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelStruct,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
			result.Classes = append(result.Classes, ClassInfo{
				Name:      cap.Name,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
		case "iface.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelInterface,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
			result.Classes = append(result.Classes, ClassInfo{
				Name:      cap.Name,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelVariable,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
			})
		case "const.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      cap.Name,
				Label:     graph.LabelConst,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
			})
		case "import.def", "import.decl", "import.multi":
			path := strings.Trim(cap.Text, "\"")
			alias := ""
			isDot := false
			for _, child := range cap.Children {
				if child.NodeType == "import.alias" {
					alias = child.Text
					if alias == "." {
						isDot = true
					}
				}
			}
			result.Imports = append(result.Imports, ImportInfo{
				Path:      path,
				Alias:     alias,
				IsDot:     isDot,
				FilePath:  cr.Filepath,
				StartLine: cap.StartRow,
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
				Receiver:   recv,
				MethodName: method,
				FilePath:   cr.Filepath,
				StartLine:  cap.StartRow,
				EndLine:    cap.EndRow,
			})
		case "call.fn.site":
			result.CallSites = append(result.CallSites, CallSite{
				MethodName: cap.Name,
				FilePath:   cr.Filepath,
				StartLine:  cap.StartRow,
				EndLine:    cap.EndRow,
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
				Name:       name,
				TypeName:   typeName,
				FilePath:   cr.Filepath,
				StartLine:  cap.StartRow,
				IsExported: len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z',
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (Go uses wildcard-leaf)
func (g *GoProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsWildcardLeaf
}

// ============ Scope-Based Pipeline Methods ============

// TreeSitterLanguage returns the Go tree-sitter language.
func (g *GoProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.GoLanguage()
}

// QuerySet returns all S-expression queries for Go extraction.
func (g *GoProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(source_file) @scope.module
			(function_declaration) @scope.function
			(func_literal) @scope.function
			(method_declaration) @scope.method
			(type_declaration (type_spec type: (struct_type))) @scope.class
			(type_declaration (type_spec type: (interface_type))) @scope.class
		`,
		Declaration: `
			(function_declaration name: (identifier) @fn.name) @fn.def
			(method_declaration receiver: (parameter_list) @method.recv name: (field_identifier) @method.name) @method.def
			(type_declaration (type_spec name: (type_identifier) @type.name type: (struct_type) @struct.body)) @struct.def
			(type_declaration (type_spec name: (type_identifier) @type.name type: (interface_type) @iface.body)) @iface.def
			(type_declaration (type_spec name: (type_identifier) @typealias.name type: (type_identifier))) @typealias.def
			(var_spec name: (identifier) @var.name) @var.def
			(const_spec name: (identifier) @const.name) @const.def
		`,
		Import: `
			(import_declaration
				(import_spec path: (interpreted_string_literal) @import.path name: (identifier)? @import.alias)
			) @import.decl
			(import_declaration
				(import_spec_list
					(import_spec path: (interpreted_string_literal) @import.path name: (identifier)? @import.alias)
				)
			) @import.multi
		`,
		TypeBinding: `
			(parameter_declaration name: (identifier) @type-binding.param.name type: (type_identifier) @type-binding.param.type) @type-binding.parameter
			(method_declaration receiver: (parameter_list (parameter_declaration name: (identifier) @type-binding.receiver.name type: (pointer_type (type_identifier) @type-binding.receiver.type)))) @type-binding.receiver
			(method_declaration receiver: (parameter_list (parameter_declaration name: (identifier) @type-binding.receiver.name type: (type_identifier) @type-binding.receiver.type))) @type-binding.receiver.value
			(short_var_declaration left: (expression_list (identifier) @type-binding.var.name) right: (expression_list (composite_literal type: (type_identifier) @type-binding.var.type))) @type-binding.constructor
			(function_declaration name: (identifier) @type-binding.return.name result: (_) @type-binding.return.type) @type-binding.return
			(method_declaration name: (field_identifier) @type-binding.return.name result: (_) @type-binding.return.type) @type-binding.return.method
			(var_declaration (var_spec name: (identifier) @type-binding.var.name type: (_) @type-binding.var.type)) @type-binding.assignment
			(short_var_declaration left: (expression_list (identifier) @type-binding.alias.name) right: (expression_list (identifier) @type-binding.alias.target)) @type-binding.alias
		`,
		Reference: `
			(call_expression function: (selector_expression operand: (identifier) @reference.member_call.receiver field: (field_identifier) @reference.member_call.method)) @reference.member_call
			(call_expression function: (identifier) @reference.free_call.name) @reference.free_call
			(composite_literal type: (type_identifier) @reference.constructor.type) @reference.constructor
			(assignment_statement left: (selector_expression operand: (identifier) @reference.field_write.receiver field: (field_identifier) @reference.field_write.name)) @reference.field_write
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. After all scopes are created,
// ParentID is computed by nesting (child scope's line range is within
// parent scope's line range).
func (g *GoProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = GoPackageName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = goExtractScopeName(cap.Text, "function")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.method":
				kind = "method"
				name = goExtractScopeName(cap.Text, "method")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = goExtractScopeName(cap.Text, "class")
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
// Each match produces one symbol. Sub-captures (name, receiver, body) are
// looked up within the same match to avoid cross-match contamination.
func (g *GoProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "fn.def", "method.def", "struct.def", "iface.def", "typealias.def", "var.def", "const.def":
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
		case "fn.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "fn.name")
			sym.Label = graph.LabelFunction
			sym.Visibility = goVisibility(sym.Name)
		case "method.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "method.name")
			sym.Label = graph.LabelMethod
			sym.Visibility = goVisibility(sym.Name)
			recv := findCaptureTextInMatch(captures, matchIdx, "method.recv")
			if recv != "" {
				sym.Props = map[string]any{"receiver": GoReceiverType(recv)}
			}
		case "struct.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "type.name")
			sym.Label = graph.LabelStruct
			sym.Visibility = goVisibility(sym.Name)
		case "iface.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "type.name")
			sym.Label = graph.LabelInterface
			sym.Visibility = goVisibility(sym.Name)
		case "typealias.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "typealias.name")
			sym.Label = graph.LabelTypeAlias
		case "var.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "var.name")
			sym.Label = graph.LabelVariable
		case "const.def":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "const.name")
			sym.Label = graph.LabelConst
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Each match (import.decl or import.multi) produces one ImportInfo.
func (g *GoProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices that are outer import captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "import.decl" || cap.NodeType == "import.multi" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	imports := make([]*pipeline.ImportInfo, 0, len(seen))

	for matchIdx := range seen {
		path := strings.Trim(findCaptureTextInMatch(captures, matchIdx, "import.path"), "\"")
		alias := findCaptureTextInMatch(captures, matchIdx, "import.alias")
		isDot := alias == "."

		// Find start line from the outer capture
		var line int
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && (cap.NodeType == "import.decl" || cap.NodeType == "import.multi") {
				line = cap.StartRow
				break
			}
		}

		imports = append(imports, &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			Alias:      alias,
			IsWildcard: isDot,
			Line:       line,
		})
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Each match produces one TypeBindingInfo.
func (g *GoProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.receiver", "type-binding.receiver.value",
			"type-binding.constructor", "type-binding.return", "type-binding.return.method",
			"type-binding.assignment", "type-binding.alias":
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
			case "type-binding.receiver", "type-binding.receiver.value":
				tb.Kind = "receiver"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.receiver.type")
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.var.type")
				tb.StartLine = cap.StartRow
			case "type-binding.return", "type-binding.return.method":
				tb.Kind = "return"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.return.type")
				tb.StartLine = cap.StartRow
			case "type-binding.assignment":
				tb.Kind = "assignment"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.var.type")
				tb.StartLine = cap.StartRow
			case "type-binding.alias":
				tb.Kind = "alias"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.alias.target")
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
func (g *GoProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.free_call", "reference.member_call", "reference.constructor", "reference.field_write":
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
			case "reference.free_call":
				ref.Kind = "free_call"
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.free_call.name")
				ref.StartLine = cap.StartRow
			case "reference.member_call":
				ref.Kind = "member_call"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.member_call.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.member_call.method")
				ref.StartLine = cap.StartRow
			case "reference.constructor":
				ref.Kind = "constructor"
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.constructor.type")
				ref.StartLine = cap.StartRow
			case "reference.field_write":
				ref.Kind = "field_write"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.field_write.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.field_write.name")
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

// goVisibility returns the visibility string for a Go identifier.
func goVisibility(name string) string {
	if GoIsExported(name) {
		return "public"
	}
	return "private"
}

// goFirstLine returns the first line of a text string.
func goFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// goParseFuncOrMethodName extracts the function or method name from a declaration line.
// Handles: "func Name(" or "func (recv) Name("
func goParseFuncOrMethodName(line string) string {
	idx := strings.Index(line, "func ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+5:] // after "func "

	// Check for receiver: "func ("
	if strings.HasPrefix(rest, "(") {
		closeIdx := strings.Index(rest, ") ")
		if closeIdx < 0 {
			return ""
		}
		rest = rest[closeIdx+2:] // after ") "
	}

	// Extract name before ( or space or {
	for i, ch := range rest {
		if ch == '(' || ch == ' ' || ch == '{' {
			if i == 0 {
				return "" // func() or func { — anonymous
			}
			return rest[:i]
		}
	}
	return rest
}

// goParseTypeName extracts the type name from a type declaration line.
// Handles: "type Name struct" or "type Name interface"
func goParseTypeName(line string) string {
	idx := strings.Index(line, "type ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+5:] // after "type "

	for i, ch := range rest {
		if ch == ' ' || ch == '{' || ch == '[' {
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// goExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the function/method/type name.
func goExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := goFirstLine(text)

	switch kind {
	case "function", "method":
		return goParseFuncOrMethodName(firstLine)
	case "class":
		return goParseTypeName(firstLine)
	}
	return ""
}

// ============ Go Language Helper Functions ============

// GoPackageName infers Go package name from file path
func GoPackageName(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(dir)
	// special handling for main package
	if base == "cmd" || base == "main" {
		return "main"
	}
	return base
}

// GoIsExported checks if a Go identifier is exported
func GoIsExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

// GoImportPath infers import path from file path and module path
func GoImportPath(filePath, modulePath string) string {
	dir := filepath.Dir(filePath)
	rel, err := filepath.Rel(filepath.Dir(modulePath), dir)
	if err != nil {
		return dir
	}
	return filepath.ToSlash(filepath.Join(modulePath, rel))
}

// GoResolvePackage resolves Go import path to package directory
func GoResolvePackage(importPath, gopath, gomod string) string {
	// standard library
	if !strings.Contains(importPath, ".") && !strings.HasPrefix(importPath, "github.com/") {
		return filepath.Join(gopath, "src", importPath)
	}
	// third-party library
	if gomod != "" {
		return filepath.Join(gomod, importPath)
	}
	return filepath.Join(gopath, "pkg", "mod", importPath)
}

// GoReceiverType extracts type name from receiver expression
func GoReceiverType(recv string) string {
	recv = strings.TrimSpace(recv)
	// *Type or Type
	recv = strings.TrimPrefix(recv, "*")
	// (r *Type) -> extract Type
	if idx := strings.Index(recv, "*"); idx >= 0 {
		recv = recv[idx+1:]
	}
	if idx := strings.Index(recv, " "); idx >= 0 {
		recv = recv[idx+1:]
	}
	return strings.TrimSpace(recv)
}

// ============ Go ScopeResolver ============

// GoScopeResolver Go scope resolver
type GoScopeResolver struct {
	provider *GoProvider
}

// NewGoScopeResolver creates a Go scope resolver
func NewGoScopeResolver(provider *GoProvider) *GoScopeResolver {
	return &GoScopeResolver{provider: provider}
}

// Language returns language label
func (r *GoScopeResolver) Language() graph.Label {
	return graph.LabelGoFile
}

// LanguageProvider returns language provider
func (r *GoScopeResolver) LanguageProvider() *GoProvider {
	return r.provider
}

// PopulateOwners populates owner relationships (function/method -> file, struct -> file, etc.)
func (r *GoScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	repo := gs.Repo()
	return model.ForEachSymbol(func(key string, entry *pipeline.SymbolEntry) error {
		// Find file node
		fileNodes, err := gs.GetNodesByFile(repo, entry.FilePath)
		if err != nil || len(fileNodes) == 0 {
			return nil // skip, not a fatal error
		}

		var fileNode *graph.Node
		for _, n := range fileNodes {
			if n.Label == graph.LabelFile {
				fileNode = n
				break
			}
		}
		if fileNode == nil {
			return nil
		}

		// If the symbol already has a NodeID, look it up; otherwise create a new node
		var symNode *graph.Node
		if entry.NodeID != "" {
			existing, err := gs.GetNode(entry.NodeID)
			if err == nil && existing != nil {
				symNode = existing
			}
		}
		if symNode == nil {
			symNode = &graph.Node{
				Name:     entry.Name,
				Label:    r.kindToLabel(entry.Kind),
				FilePath: entry.FilePath,
			}
			if err := gs.BufferNode(symNode); err != nil {
				return fmt.Errorf("add symbol node %s: %w", entry.Name, err)
			}
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

// BuildMRO builds method resolution order (Go uses struct embedding, similar to multiple inheritance)
func (r *GoScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	repo := gs.Repo()
	// Go's MRO strategy: depth-first, no diamond problem
	for _, clsNode := range classes {
		if len(clsNode.Props.BaseTypes) == 0 {
			continue
		}

		for _, baseName := range clsNode.Props.BaseTypes {
			baseNodes, err := gs.GetNodesByName(repo, baseName)
			if err != nil || len(baseNodes) == 0 {
				continue
			}

			// EMBRACES edge: subclass -> base class
			for _, baseNode := range baseNodes {
				_ = gs.BufferEdge((&graph.Edge{
					Type:   graph.RelEmbraces,
					Source: clsNode.ID,
					Target: baseNode.ID,
				}).WithProp("confidence", 0.95))
			}
		}
	}
	return nil
}

// ResolveImportTarget resolves import target
func (r *GoScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	// Go's wildcard import: import path -> package directory -> all exported symbols in package
	repo := gs.Repo()
	path := imp.Path

	// standard library prefix
	if !strings.Contains(path, "/") {
		// standard library, cannot resolve in local repository
		return nil, nil
	}

	// find matching directory nodes in repository
	// simplified implementation: find files by import path prefix
	var targets []*graph.Node
	allNodes := gs.GetAllNodes(repo, 10000)
	for _, node := range allNodes {
		if node.Label == graph.LabelFile && strings.Contains(node.FilePath, path) {
			targets = append(targets, node)
		}
	}

	return targets, nil
}

// ============ Boolean Switches ============

// PropagatesReturnTypesAcrossImports Go supports propagating return types across imports
func (r *GoScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }

// FieldFallbackOnMethodLookup Go is statically typed, no method fallback needed
func (r *GoScopeResolver) FieldFallbackOnMethodLookup() bool { return false }

// UnwrapCollectionAccessor Go does not unwrap collection accessors
func (r *GoScopeResolver) UnwrapCollectionAccessor() bool { return false }

// CollapseMemberCallsByCallerTarget Go uses one edge per caller-target
func (r *GoScopeResolver) CollapseMemberCallsByCallerTarget() bool { return true }

// PopulateNamespaceSiblings Go has implicit visibility within same package
func (r *GoScopeResolver) PopulateNamespaceSiblings() bool { return true }

// HoistTypeBindingsToModule Go does not hoist type bindings to module
func (r *GoScopeResolver) HoistTypeBindingsToModule() bool { return false }

// ============ 4 Core Methods ============

// MergeBindings merges local binding set and imported binding set
// Go strategy: local first, import supplements (Go package-level exported symbol merging)
func (r *GoScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	result.IsImported = false

	// local bindings first
	for name, ids := range local.Bindings {
		result.Bindings[name] = ids
	}

	// imported bindings supplement (do not override local)
	for name, ids := range imported.Bindings {
		if _, exists := result.Bindings[name]; !exists {
			result.Bindings[name] = ids
		}
	}

	return result
}

// ArityCompatibility checks arity compatibility between call site and target
// Go is statically typed, strict parameter count check
func (r *GoScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true // no arity info, default compatible
	}
	return caller.Args == targetArity
}

// ImportEdgeReason returns import edge reason description
func (r *GoScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "wildcard-import"
	}
	if len(imp.Symbols) > 0 {
		return "named-import"
	}
	return "package-import"
}

// IsSuperReceiver checks if receiver is a super class receiver
// Go uses interface embedding, check if it's an interface type
func (r *GoScopeResolver) IsSuperReceiver(recv string) bool {
	// In Go, receivers starting with uppercase may be interface methods
	if len(recv) == 0 {
		return false
	}
	return recv[0] >= 'A' && recv[0] <= 'Z'
}

// ============ 4 Function-type Hooks ============

// PopulateRangeBindings for-range variable bindings
// Go specific: variables in for range statements need binding
func (r *GoScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
	// Go's for-range variables are captured as local variables during tree-sitter parsing
	// This is a no-op, Go does not need additional range binding logic
}

// CollectScopeContextPaths collects scope context file paths
// Go has implicit visibility within same package, no additional paths needed
func (r *GoScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	// Go same-package files are handled via PopulateNamespaceSiblings
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

// EmitPostResolutionEdges post-processing edge emission
// Go does not need additional post-processing edges (no template/event system)
func (r *GoScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
	// Go has no post-processing edge requirements
}

// EmitUnresolvedReceiverEdges untyped receiver fallback
// Go is statically typed, no untyped receiver fallback needed
func (r *GoScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	// Go is statically typed, return 0 fallback edges
	return 0
}

// ============ Helper Methods ============

func (r *GoScopeResolver) kindToLabel(kind string) graph.Label {
	switch kind {
	case "function":
		return graph.LabelFunction
	case "method":
		return graph.LabelMethod
	case "struct":
		return graph.LabelStruct
	case "interface":
		return graph.LabelInterface
	case "variable":
		return graph.LabelVariable
	case "constant":
		return graph.LabelConst
	case "field":
		return graph.LabelField
	case "import":
		return graph.LabelImport
	default:
		return graph.LabelUnknown
	}
}
