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

// ============ C++ LanguageProvider ============

// CPPProvider C++ language provider
type CPPProvider struct{}

// NewCPPProvider creates C++ language provider
func NewCPPProvider() *CPPProvider {
	return &CPPProvider{}
}

// Language returns language label
func (p *CPPProvider) Language() graph.Label {
	return graph.LabelCPPFile
}

// Captures returns node capture rules
func (p *CPPProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_definition declarator: (function_declarator declarator: (identifier) @fn.name)) @fn.def
			(function_definition declarator: (function_declarator declarator: (qualified_identifier name: (identifier) @method.name))) @method.def
			(class_specifier name: (type_identifier) @class.name) @class.def
			(struct_specifier name: (type_identifier) @struct.name) @struct.def
			(enum_specifier name: (type_identifier) @enum.name) @enum.def
			(namespace_definition name: (identifier) @ns.name) @ns.def
			(template_declaration) @template.def
			(type_definition declarator: (type_identifier) @typedef.name) @typedef.def
			(preproc_include path: (string_literal) @include.path) @include.def
			(preproc_define name: (identifier) @macro.name) @macro.def
			(alias_declaration name: (type_identifier) @alias.name) @alias.def
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":       graph.LabelFunction,
			"method.def":   graph.LabelMethod,
			"class.def":    graph.LabelClass,
			"struct.def":   graph.LabelStruct,
			"enum.def":     graph.LabelEnum,
			"ns.def":       graph.LabelNamespace,
			"template.def": graph.LabelTemplate,
			"typedef.def":  graph.LabelTypedef,
			"include.def":  graph.LabelImport,
			"macro.def":    graph.LabelMacro,
			"alias.def":    graph.LabelTypeAlias,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *CPPProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call_expression function: (identifier) @call.fn) @call.fn.site
			(call_expression function: (qualified_identifier name: (identifier) @call.method)) @call.site
			(call_expression function: (field_identifier) @call.method) @call.member.site
		`,
		CaptureMap: map[string]graph.Label{
			"call.fn.site":     graph.LabelCallSite,
			"call.site":        graph.LabelCallSite,
			"call.member.site": graph.LabelCallSite,
		},
		ReceiverKey: "",
		MethodKey:   "call.method",
		ArgsKey:     "call.args",
	}
}

// ClassExtractConfig returns class extraction config
func (p *CPPProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_specifier name: (type_identifier) @class.name (base_class_clause (type_identifier) @class.base)?) @class.def
			(struct_specifier name: (type_identifier) @class.name) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelClass,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction config
func (p *CPPProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(field_declaration declarator: (field_identifier) @field.name type: (_) @field.type) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "field.type",
	}
}

// ImportResolveConfig returns import resolution config
func (p *CPPProvider) ImportResolveConfig() *ImportResolveConfig {
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
func (p *CPPProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
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
		case "ns.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelNamespace,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "template.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTemplate,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "typedef.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTypedef,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "alias.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelTypeAlias,
				FilePath: captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "macro.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelMacro,
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
		case "call.site", "call.member.site":
			method := ""
			for _, child := range cap.Children {
				if child.NodeType == "call.method" {
					method = child.Text
				}
			}
			result.CallSites = append(result.CallSites, CallSite{
				MethodName: method,
				FilePath:   captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
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

// ImportSemantics returns import semantics (C++ uses wildcard-transitive)
func (p *CPPProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsWildcardTransitive
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the C++ tree-sitter language.
func (p *CPPProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.CppLanguage()
}

// QuerySet returns all S-expression queries for C++ extraction.
// Follows capture namespace conventions:
//   - Scope: @scope.<kind> (NO name sub-captures to avoid match doubling)
//   - Declaration: @declaration.<kind> + @declaration.name
//   - Import: @import.statement (single anchor)
//   - TypeBinding: @type-binding.<kind> + @type-binding.name + @type-binding.type
//   - Reference: @reference.call.<kind> + @reference.name + @reference.receiver
func (p *CPPProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(translation_unit) @scope.module
			(namespace_definition) @scope.namespace
			(class_specifier) @scope.class
			(struct_specifier) @scope.class
			(function_definition) @scope.function
			(lambda_expression) @scope.function
			(compound_statement) @scope.block
		`,
		Declaration: `
			(namespace_definition name: (namespace_identifier) @declaration.name) @declaration.namespace
			(class_specifier name: (type_identifier) @declaration.name body: (field_declaration_list)) @declaration.class
			(struct_specifier name: (type_identifier) @declaration.name body: (field_declaration_list)) @declaration.struct
			(union_specifier name: (type_identifier) @declaration.name) @declaration.union
			(enum_specifier name: (type_identifier) @declaration.name) @declaration.enum
			(function_definition declarator: (function_declarator declarator: (identifier) @declaration.name)) @declaration.function
			(function_definition declarator: (function_declarator declarator: (qualified_identifier name: (identifier) @declaration.name))) @declaration.method
			(function_definition declarator: (function_declarator declarator: (field_identifier) @declaration.name)) @declaration.method
			(field_declaration declarator: (field_identifier) @declaration.name) @declaration.property
			(declaration declarator: (init_declarator declarator: (identifier) @declaration.name)) @declaration.variable
			(declaration declarator: (identifier) @declaration.name) @declaration.variable
			(preproc_def name: (identifier) @declaration.name) @declaration.macro
			(preproc_function_def name: (identifier) @declaration.name) @declaration.macro
			(enumerator name: (identifier) @declaration.name) @declaration.const
			(type_definition declarator: (type_identifier) @declaration.name) @declaration.typedef
			(alias_declaration name: (type_identifier) @declaration.name) @declaration.typedef
		`,
		Import: `
			(preproc_include) @import.statement
			(using_declaration) @import.using-decl
		`,
		TypeBinding: `
			(parameter_declaration type: (_) @type-binding.type declarator: (identifier) @type-binding.name) @type-binding.parameter
			(declaration type: (_) @type-binding.type declarator: (init_declarator declarator: (identifier) @type-binding.name)) @type-binding.assignment
			(declaration type: (type_identifier) @type-binding.type declarator: (identifier) @type-binding.name) @type-binding.annotation
			(declaration type: (placeholder_type_specifier) declarator: (init_declarator declarator: (identifier) @type-binding.name value: (call_expression function: (identifier) @type-binding.type))) @type-binding.constructor
			(declaration type: (placeholder_type_specifier) declarator: (init_declarator declarator: (identifier) @type-binding.name value: (identifier) @type-binding.type)) @type-binding.alias
			(function_definition type: (type_identifier) @type-binding.type declarator: (function_declarator declarator: (identifier) @type-binding.name)) @type-binding.return
			(field_declaration type: (type_identifier) @type-binding.type declarator: (field_identifier) @type-binding.name) @type-binding.field
		`,
		Reference: `
			(call_expression function: (identifier) @reference.name) @reference.call.free
			(call_expression function: (field_expression argument: (_) @reference.receiver field: (field_identifier) @reference.name)) @reference.call.member
			(call_expression function: (qualified_identifier scope: (_) @reference.receiver name: (identifier) @reference.name)) @reference.call.qualified
			(new_expression type: (type_identifier) @reference.name) @reference.call.constructor
			(field_expression argument: (_) @reference.receiver field: (field_identifier) @reference.name) @reference.read
			(assignment_expression left: (field_expression argument: (_) @reference.receiver field: (field_identifier) @reference.name)) @reference.write
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. Names are extracted from cap.Text
// using helper functions (NOT from name sub-captures, which cause match
// count doubling with tree-sitter's outer/inner capture semantics).
// After all scopes are created, ParentID is computed by nesting.
func (p *CPPProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = cppModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.namespace":
				kind = "namespace"
				name = cppExtractScopeName(cap.Text, "namespace")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = cppExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = cppExtractScopeName(cap.Text, "function")
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
// Uses conventions: @declaration.<kind> as outer capture + @declaration.name
// as uniform name sub-capture. Each match produces one symbol.
func (p *CPPProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "declaration.namespace", "declaration.class", "declaration.struct",
				"declaration.union", "declaration.enum", "declaration.function", "declaration.method",
				"declaration.property", "declaration.variable", "declaration.macro",
				"declaration.const", "declaration.typedef":
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

		// Extract name from @declaration.name sub-capture (unified across all patterns)
		sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		switch outerNodeType {
		case "declaration.namespace":
			sym.Label = graph.LabelNamespace
		case "declaration.class":
			sym.Label = graph.LabelClass
		case "declaration.struct":
			sym.Label = graph.LabelStruct
		case "declaration.union":
			sym.Label = graph.LabelStruct
		case "declaration.enum":
			sym.Label = graph.LabelEnum
		case "declaration.function":
			sym.Label = graph.LabelFunction
		case "declaration.method":
			sym.Label = graph.LabelMethod
		case "declaration.property":
			sym.Label = graph.LabelProperty
		case "declaration.variable":
			sym.Label = graph.LabelVariable
		case "declaration.macro":
			sym.Label = graph.LabelMacro
		case "declaration.const":
			sym.Label = graph.LabelConst
		case "declaration.typedef":
			sym.Label = graph.LabelTypedef
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Uses convention: @import.statement as single anchor capture.
// Path is extracted from cap.Text using cppExtractIncludePath.
func (p *CPPProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices that are import statement captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "import.statement" || cap.NodeType == "import.using-decl" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	imports := make([]*pipeline.ImportInfo, 0, len(seen))

	for matchIdx := range seen {
		// Find the outer capture to get text and line
		var line int
		var text string
		var nodeType string
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && (cap.NodeType == "import.statement" || cap.NodeType == "import.using-decl") {
				line = cap.StartRow
				text = cap.Text
				nodeType = cap.NodeType
				break
			}
		}

		path := cppExtractIncludePath(text)

		imp := &pipeline.ImportInfo{
			Path:       path,
			SourceFile: filePath,
			IsWildcard: true,
			Line:       line,
		}

		// using declarations are not wildcard
		if nodeType == "import.using-decl" {
			imp.IsWildcard = false
		}

		imports = append(imports, imp)
	}
	return imports
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Uses conventions: @type-binding.<kind> as outer capture +
// uniform @type-binding.name and @type-binding.type sub-captures.
func (p *CPPProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.assignment", "type-binding.annotation",
			"type-binding.constructor", "type-binding.alias", "type-binding.return", "type-binding.field":
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
			case "type-binding.annotation":
				tb.Kind = "annotation"
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.StartLine = cap.StartRow
			case "type-binding.alias":
				tb.Kind = "alias"
				tb.StartLine = cap.StartRow
			case "type-binding.return":
				tb.Kind = "return"
				tb.StartLine = cap.StartRow
			case "type-binding.field":
				tb.Kind = "field"
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
// For @reference.read, filter out matchIdxs that already have @reference.call.member
// to avoid duplicate references (field_expression is shared between read and member call).
func (p *CPPProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.call.qualified",
			"reference.call.constructor", "reference.read", "reference.write":
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	// Build a set of match indices that have @reference.call.member,
	// so we can filter @reference.read duplicates
	memberCallMatchIdxs := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "reference.call.member" {
			memberCallMatchIdxs[cap.MatchIndex] = struct{}{}
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
			case "reference.call.qualified":
				ref.Kind = "qualified_call"
				ref.StartLine = cap.StartRow
			case "reference.call.constructor":
				ref.Kind = "constructor"
				ref.StartLine = cap.StartRow
			case "reference.read":
				ref.Kind = "field_read"
				ref.StartLine = cap.StartRow
			case "reference.write":
				ref.Kind = "field_write"
				ref.StartLine = cap.StartRow
			}
		}
		if ref.Kind == "" {
			continue
		}

		// Filter out @reference.read that duplicates @reference.call.member
		if ref.Kind == "field_read" {
			if _, isDuplicate := memberCallMatchIdxs[matchIdx]; isDuplicate {
				continue
			}
		}

		// Use uniform @reference.name and @reference.receiver
		ref.Name = findCaptureTextInMatch(captures, matchIdx, "reference.name")
		ref.Receiver = findCaptureTextInMatch(captures, matchIdx, "reference.receiver")

		refs = append(refs, ref)
	}
	return refs
}

// ============ C++ ScopeResolver ============

// CPPScopeResolver C++ scope resolver
type CPPScopeResolver struct {
	provider *CPPProvider
}

// NewCPPScopeResolver creates C++ scope resolver
func NewCPPScopeResolver(provider *CPPProvider) *CPPScopeResolver {
	return &CPPScopeResolver{provider: provider}
}

func (r *CPPScopeResolver) Language() graph.Label {
	return graph.LabelCPPFile
}

func (r *CPPScopeResolver) LanguageProvider() *CPPProvider {
	return r.provider
}

func (r *CPPScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	return nil
}

// BuildMRO C++ multiple inheritance, depth-first (first-wins)
func (r *CPPScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	return nil
}

func (r *CPPScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	return nil, nil
}

// ============ Boolean switches ============

func (r *CPPScopeResolver) PropagatesReturnTypesAcrossImports() bool { return false }
func (r *CPPScopeResolver) FieldFallbackOnMethodLookup() bool        { return false }
func (r *CPPScopeResolver) UnwrapCollectionAccessor() bool           { return false }
func (r *CPPScopeResolver) CollapseMemberCallsByCallerTarget() bool  { return true }
func (r *CPPScopeResolver) PopulateNamespaceSiblings() bool          { return true } // C++ header files are implicitly visible
func (r *CPPScopeResolver) HoistTypeBindingsToModule() bool          { return false }

// ============ 4 core methods ============

// MergeBindings merges binding sets (C++ #include transitivity + using declarations)
func (r *CPPScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
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

// ArityCompatibility C++ supports overloading, loose checking
func (r *CPPScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true
	}
	return caller.Args == targetArity
}

func (r *CPPScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "include-all"
	}
	return "include"
}

func (r *CPPScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super" || recv == "Base"
}

// ============ 4 functional hooks ============

func (r *CPPScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
}

func (r *CPPScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

func (r *CPPScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
}

func (r *CPPScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	return 0
}

// ============ C++ Helper Functions ============

// cppFirstLine returns the first line of a text string.
func cppFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// cppParseFuncName extracts the function name from a C++ declaration line.
// Handles: "void foo(", "int MyClass::bar(", "auto lambda = [", etc.
func cppParseFuncName(line string) string {
	line = strings.TrimSpace(line)

	// Find the opening parenthesis of the parameter list
	parenIdx := strings.LastIndex(line, "(")
	if parenIdx < 0 {
		return ""
	}
	beforeParen := line[:parenIdx]

	// The function name is the last identifier before the parenthesis
	// Walk backwards to find the identifier end, then walk backwards to find the start
	end := len(beforeParen) - 1
	for end >= 0 && (beforeParen[end] == ' ' || beforeParen[end] == '\t') {
		end--
	}
	if end < 0 {
		return ""
	}

	// Walk backwards to find the identifier start
	start := end
	for start >= 0 && (isCppIdentChar(beforeParen[start])) {
		start--
	}
	name := beforeParen[start+1 : end+1]

	// If there's a :: before the name, the actual method name is after ::
	if colonIdx := strings.LastIndex(name, "::"); colonIdx >= 0 {
		name = name[colonIdx+2:]
	}

	return name
}

// cppParseTypeName extracts the class/struct/enum/namespace name from a declaration line.
// Handles: "class Foo {", "struct Bar : public Base", "enum class Color",
// "namespace MyNS {", "namespace MyNS::Nested {"
func cppParseTypeName(line string) string {
	line = strings.TrimSpace(line)

	// Find the keyword and skip it
	keywords := []string{"class ", "struct ", "enum ", "namespace "}
	for _, kw := range keywords {
		idx := strings.Index(line, kw)
		if idx >= 0 {
			rest := line[idx+len(kw):]

			// Skip "class " after "enum " (for enum class)
			if kw == "enum " && strings.HasPrefix(rest, "class ") {
				rest = rest[6:]
			}

			return cppExtractIdentifier(rest)
		}
	}
	return ""
}

// cppExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the namespace/class/function name.
func cppExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := cppFirstLine(text)

	switch kind {
	case "function":
		return cppParseFuncName(firstLine)
	case "class", "namespace":
		return cppParseTypeName(firstLine)
	}
	return ""
}

// cppModuleName derives module name from file path.
// For C++, the module name is the top-level directory or the file stem.
func cppModuleName(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(dir)
	if base == "." || base == "" {
		// Use file stem as fallback
		stem := filepath.Base(filePath)
		if dotIdx := strings.LastIndex(stem, "."); dotIdx > 0 {
			return stem[:dotIdx]
		}
		return stem
	}
	return base
}

// cppExtractIncludePath extracts the path from a #include directive text.
// Handles: #include <stdio.h> → stdio.h, #include "foo.h" → foo.h
func cppExtractIncludePath(text string) string {
	// Try angle-bracket include: #include <path>
	if ltIdx := strings.Index(text, "<"); ltIdx >= 0 {
		gtIdx := strings.Index(text[ltIdx:], ">")
		if gtIdx >= 0 {
			return text[ltIdx+1 : ltIdx+gtIdx]
		}
	}
	// Try quoted include: #include "path"
	if qIdx := strings.Index(text, "\""); qIdx >= 0 {
		rest := text[qIdx+1:]
		if q2Idx := strings.Index(rest, "\""); q2Idx >= 0 {
			return rest[:q2Idx]
		}
	}
	return strings.TrimSpace(text)
}

// cppExtractIdentifier extracts a C++ identifier from the start of a string.
// Stops at non-identifier characters (space, {, :, <, ;, etc.)
func cppExtractIdentifier(s string) string {
	s = strings.TrimSpace(s)
	var end int
	for end = 0; end < len(s); end++ {
		if !isCppIdentChar(s[end]) {
			break
		}
	}
	if end == 0 {
		return ""
	}
	return s[:end]
}

// isCppIdentChar returns true if the byte can be part of a C++ identifier.
func isCppIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_' || ch == ':'
}
