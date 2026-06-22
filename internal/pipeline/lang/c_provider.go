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

// ============ C LanguageProvider ============

// CProvider C language provider
type CProvider struct{}

// NewCProvider creates a C language provider
func NewCProvider() *CProvider {
	return &CProvider{}
}

// Language returns the language label
func (p *CProvider) Language() graph.Label {
	return graph.LabelCFile
}

// Captures returns node capture rules
func (p *CProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_definition declarator: (function_declarator declarator: (identifier) @fn.name)) @fn.def
			(struct_specifier name: (type_identifier) @struct.name) @struct.def
			(enum_specifier name: (type_identifier) @enum.name) @enum.def
			(type_definition declarator: (type_identifier) @typedef.name) @typedef.def
			(preproc_include path: (string_literal) @include.path) @include.def
			(preproc_define name: (identifier) @macro.name) @macro.def
			(declaration declarator: (identifier) @var.name) @var.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":      graph.LabelFunction,
			"struct.def":  graph.LabelStruct,
			"enum.def":    graph.LabelEnum,
			"typedef.def": graph.LabelTypedef,
			"include.def": graph.LabelImport,
			"macro.def":   graph.LabelMacro,
			"var.def":     graph.LabelVariable,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *CProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call_expression function: (identifier) @call.fn) @call.fn.site
		`,
		CaptureMap: map[string]graph.Label{
			"call.fn.site": graph.LabelCallSite,
		},
		ReceiverKey: "",
		MethodKey:   "",
		ArgsKey:     "call.args",
	}
}

// ClassExtractConfig returns class extraction config (C has no classes, uses struct instead)
func (p *CProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(struct_specifier name: (type_identifier) @class.name) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelStruct,
		},
		NameKey: "class.name",
		BaseKey: "",
	}
}

// FieldExtractConfig returns field extraction config
func (p *CProvider) FieldExtractConfig() *FieldExtractConfig {
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
func (p *CProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(preproc_include path: (string_literal) @import.path) @import.decl
			(preproc_include path: (system_lib_string) @import.path) @import.sys
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl": graph.LabelImport,
			"import.sys":  graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "",
		ItemsKey:    "",
		IsDotImport: false,
		IsWildcard:  true,
	}
}

// Interpret interprets capture results
func (p *CProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
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
		case "typedef.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTypedef,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "macro.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelMacro,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "include.def", "import.decl", "import.sys":
			path := strings.Trim(cap.Text, "\"<>")
			result.Imports = append(result.Imports, ImportInfo{
				Path: path, FilePath: captures.Filepath, StartLine: cap.StartRow,
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

// ImportSemantics returns import semantics (C uses wildcard-transitive)
func (p *CProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsWildcardTransitive
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the C tree-sitter language.
func (p *CProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.CLanguage()
}

// QuerySet returns all S-expression queries for C extraction.
// Uses capture namespace:
//   - Scope: no name sub-captures (avoids double-matching); names extracted from cap.Text
//   - Declaration: @declaration.name sub-capture + @declaration.<kind> outer
//   - Import: @import.statement outer capture
//   - TypeBinding: @type-binding.name / @type-binding.type sub-captures
//   - Reference: @reference.name / @reference.receiver sub-captures
func (p *CProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(translation_unit) @scope.module
			(struct_specifier) @scope.class
			(union_specifier) @scope.class
			(function_definition) @scope.function
			(compound_statement) @scope.block
		`,
		Declaration: `
			(struct_specifier
			  name: (type_identifier) @declaration.name
			  body: (field_declaration_list)) @declaration.struct

			(type_definition
			  type: (struct_specifier
			    body: (field_declaration_list))
			  declarator: (type_identifier) @declaration.name) @declaration.struct

			(union_specifier
			  name: (type_identifier) @declaration.name
			  body: (field_declaration_list)) @declaration.union

			(type_definition
			  type: (union_specifier
			    body: (field_declaration_list))
			  declarator: (type_identifier) @declaration.name) @declaration.union

			(enum_specifier
			  name: (type_identifier) @declaration.name) @declaration.enum

			(type_definition
			  type: (enum_specifier
			    body: (enumerator_list))
			  declarator: (type_identifier) @declaration.name) @declaration.enum

			(function_definition
			  declarator: (function_declarator
			    declarator: (identifier) @declaration.name)) @declaration.function

			(function_definition
			  declarator: (pointer_declarator
			    declarator: (function_declarator
			      declarator: (identifier) @declaration.name))) @declaration.function

			(declaration
			  declarator: (function_declarator
			    declarator: (identifier) @declaration.name)) @declaration.function

			(type_definition
			  declarator: (type_identifier) @declaration.name) @declaration.typedef

			(field_declaration
			  declarator: (field_identifier) @declaration.name) @declaration.field

			(declaration
			  declarator: (init_declarator
			    declarator: (identifier) @declaration.name)) @declaration.variable

			(declaration
			  declarator: (identifier) @declaration.name) @declaration.variable

			(preproc_def
			  name: (identifier) @declaration.name) @declaration.macro

			(preproc_function_def
			  name: (identifier) @declaration.name) @declaration.macro

			(enumerator
			  name: (identifier) @declaration.name) @declaration.const
		`,
		Import: `
			(preproc_include) @import.statement
		`,
		TypeBinding: `
			(parameter_declaration
			  type: (_) @type-binding.type
			  declarator: (identifier) @type-binding.name) @type-binding.parameter

			(declaration
			  type: (_) @type-binding.type
			  declarator: (init_declarator
			    declarator: (identifier) @type-binding.name)) @type-binding.assignment

			(declaration
			  type: (_) @type-binding.type
			  declarator: (identifier) @type-binding.name) @type-binding.assignment
		`,
		Reference: `
			(call_expression
			  function: (identifier) @reference.name) @reference.call.free

			(call_expression
			  function: (field_expression
			    argument: (_) @reference.receiver
			    field: (field_identifier) @reference.name)) @reference.call.member

			(field_expression
			  argument: (_) @reference.receiver
			  field: (field_identifier) @reference.name) @reference.read

			(assignment_expression
			  left: (field_expression
			    argument: (_) @reference.receiver
			    field: (field_identifier) @reference.name)) @reference.write
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. Scope queries have no name sub-captures
// to avoid tree-sitter double-matching; names are extracted from cap.Text
// using helper functions (cExtractScopeName).
func (p *CProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = cExtractModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = cExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = cExtractScopeName(cap.Text, "function")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.block":
				kind = "block"
				name = ""
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
// Uses @declaration.<kind> outer capture and @declaration.name sub-capture.
func (p *CProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "declaration.struct", "declaration.union", "declaration.enum",
				"declaration.function", "declaration.typedef", "declaration.field",
				"declaration.variable", "declaration.macro", "declaration.const":
				outerNodeType = cap.NodeType
				startLine = cap.StartRow
				endLine = cap.EndRow
			}
		}
		if outerNodeType == "" {
			continue
		}

		// Extract name from the @declaration.name sub-capture within this match
		symName := findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		sym := &pipeline.SymbolInfo{
			Name:      symName,
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
		}

		switch outerNodeType {
		case "declaration.function":
			sym.Label = graph.LabelFunction
		case "declaration.struct":
			sym.Label = graph.LabelStruct
		case "declaration.union":
			sym.Label = graph.LabelStruct
		case "declaration.enum":
			sym.Label = graph.LabelEnum
		case "declaration.typedef":
			sym.Label = graph.LabelTypedef
		case "declaration.field":
			sym.Label = graph.LabelField
		case "declaration.variable":
			sym.Label = graph.LabelVariable
		case "declaration.macro":
			sym.Label = graph.LabelMacro
		case "declaration.const":
			sym.Label = graph.LabelConst
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Uses @import.statement outer capture; path is parsed from cap.Text.
func (p *CProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
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
		// Extract import path from the outer capture's text
		var path string
		var line int
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && cap.NodeType == "import.statement" {
				path = cExtractImportPath(cap.Text)
				line = cap.StartRow
				break
			}
		}

		imports = append(imports, &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			IsWildcard: true,
			Line:       line,
		})
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Uses @type-binding.name / @type-binding.type sub-captures.
func (p *CProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.assignment":
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
				tb.StartLine = cap.StartRow
			case "type-binding.assignment":
				tb.Kind = "assignment"
				tb.StartLine = cap.StartRow
			}
		}
		if tb.Kind == "" {
			continue
		}

		// Extract name and type from sub-captures
		tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
		_ = findCaptureTextInMatch(captures, matchIdx, "type-binding.name") // name available for future use

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// Uses @reference.name / @reference.receiver sub-captures with kinds:
// call.free, call.member, read, write.
func (p *CProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.read", "reference.write":
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
				ref.Kind = "call.free"
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.call.member":
				ref.Kind = "call.member"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.read":
				ref.Kind = "read"
				ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")
				ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
				ref.StartLine = cap.StartRow
			case "reference.write":
				ref.Kind = "write"
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

// ============ C Language Helper Functions ============

// cFirstLine returns the first line of a text string.
func cFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// cParseFuncName extracts the function name from a C function definition line.
// Handles: "type name(" or "type *name("
func cParseFuncName(line string) string {
	// Find the opening parenthesis of the parameter list
	parenIdx := strings.Index(line, "(")
	if parenIdx < 0 {
		return ""
	}
	// Walk backwards from '(' to find the identifier name
	before := strings.TrimSpace(line[:parenIdx])
	// Handle pointer return types: "void *func" -> "func"
	for i := len(before) - 1; i >= 0; i-- {
		ch := before[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			nameEnd := i + 1
			nameStart := i
			for nameStart > 0 {
				ch2 := before[nameStart-1]
				if (ch2 >= 'a' && ch2 <= 'z') || (ch2 >= 'A' && ch2 <= 'Z') || (ch2 >= '0' && ch2 <= '9') || ch2 == '_' {
					nameStart--
				} else {
					break
				}
			}
			return before[nameStart:nameEnd]
		}
	}
	return ""
}

// cParseTypeName extracts the type name from a C struct/union/enum specifier line.
// Handles: "struct Name {" or "union Name {" or "enum Name {"
func cParseTypeName(line string) string {
	for _, prefix := range []string{"struct ", "union ", "enum "} {
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(prefix):]
		// Extract identifier before '{', ' ', or end
		for i, ch := range rest {
			if ch == '{' || ch == ' ' || ch == '\t' || ch == ';' || ch == '\n' {
				if i == 0 {
					return "" // anonymous: "struct {"
				}
				return rest[:i]
			}
		}
		if len(rest) > 0 {
			return rest
		}
	}
	return ""
}

// cExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the function/type name.
func cExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := cFirstLine(text)
	switch kind {
	case "function":
		return cParseFuncName(firstLine)
	case "class":
		return cParseTypeName(firstLine)
	}
	return ""
}

// cExtractModuleName derives the C module name from the file path.
// Uses the base filename without extension (e.g., "main.c" -> "main").
func cExtractModuleName(filePath string) string {
	base := filepath.Base(filePath)
	if idx := strings.LastIndex(base, "."); idx > 0 {
		return base[:idx]
	}
	return base
}

// cExtractImportPath extracts the include path from a preproc_include text.
// Handles: #include "path.h" and #include <path.h>
func cExtractImportPath(text string) string {
	// Find quoted path: #include "path"
	if idx := strings.Index(text, "\""); idx >= 0 {
		rest := text[idx+1:]
		if end := strings.Index(rest, "\""); end >= 0 {
			return rest[:end]
		}
	}
	// Find angle-bracket path: #include <path>
	if idx := strings.Index(text, "<"); idx >= 0 {
		rest := text[idx+1:]
		if end := strings.Index(rest, ">"); end >= 0 {
			return rest[:end]
		}
	}
	return strings.TrimSpace(text)
}

// ============ C ScopeResolver ============

// CScopeResolver C scope resolver
type CScopeResolver struct {
	provider *CProvider
}

// NewCScopeResolver creates a C scope resolver
func NewCScopeResolver(provider *CProvider) *CScopeResolver {
	return &CScopeResolver{provider: provider}
}

func (r *CScopeResolver) Language() graph.Label {
	return graph.LabelCFile
}

func (r *CScopeResolver) LanguageProvider() *CProvider {
	return r.provider
}

func (r *CScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	return nil
}

func (r *CScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	// C has no classes/inheritance, MRO is a no-op
	return nil
}

func (r *CScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	// C #include is transitive expansion, handled in cross_file phase
	return nil, nil
}

// ============ Boolean switches ============

func (r *CScopeResolver) PropagatesReturnTypesAcrossImports() bool { return false }
func (r *CScopeResolver) FieldFallbackOnMethodLookup() bool        { return true }
func (r *CScopeResolver) UnwrapCollectionAccessor() bool           { return false }
func (r *CScopeResolver) CollapseMemberCallsByCallerTarget() bool  { return false }
func (r *CScopeResolver) PopulateNamespaceSiblings() bool          { return true } // C header files implicitly visible
func (r *CScopeResolver) HoistTypeBindingsToModule() bool          { return false }

// ============ 4 core methods ============

// MergeBindings merges binding sets (C #include transitive, import overrides local)
func (r *CScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	// #include transitive expansion, imported definitions override local
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

func (r *CScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	// C function signatures are strict, but no overloading
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true
	}
	return caller.Args == targetArity
}

func (r *CScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "include-all"
	}
	return "include"
}

func (r *CScopeResolver) IsSuperReceiver(recv string) bool {
	// C has no inheritance/super mechanism
	return false
}

// ============ 4 functional hooks ============

func (r *CScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
}

func (r *CScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

func (r *CScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
}

func (r *CScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	return 0
}
