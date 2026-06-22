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

// ============ Java LanguageProvider ============

// JavaProvider Java language provider
type JavaProvider struct{}

// NewJavaProvider creates a Java language provider
func NewJavaProvider() *JavaProvider {
	return &JavaProvider{}
}

// Language returns the language label
func (p *JavaProvider) Language() graph.Label {
	return graph.LabelJavaFile
}

// Captures returns node capture rules
func (p *JavaProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(method_declaration name: (identifier) @method.name) @method.def
			(class_declaration name: (identifier) @class.name) @class.def
			(interface_declaration name: (identifier) @iface.name) @iface.def
			(enum_declaration name: (identifier) @enum.name) @enum.def
			(constructor_declaration name: (identifier) @ctor.name) @ctor.def
			(field_declaration declarator: (variable_declarator name: (identifier) @field.name)) @field.def
			(local_variable_declaration declarator: (variable_declarator name: (identifier) @var.name)) @var.def
			(import_declaration (scoped_identifier) @import.path) @import.def
		`,
		CaptureMap: map[string]graph.Label{
			"method.def": graph.LabelMethod,
			"class.def":  graph.LabelClass,
			"iface.def":  graph.LabelInterface,
			"enum.def":   graph.LabelEnum,
			"ctor.def":   graph.LabelConstructor,
			"field.def":  graph.LabelField,
			"var.def":    graph.LabelVariable,
			"import.def": graph.LabelImport,
		},
	}
}

// CallExtractConfig returns call extraction config
func (p *JavaProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(method_invocation
				object: (identifier) @call.receiver
				name: (identifier) @call.method
			) @call.site
			(method_invocation
				name: (identifier) @call.fn
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
func (p *JavaProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_declaration name: (identifier) @class.name (superclass (type_identifier) @class.base)?) @class.def
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
func (p *JavaProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(field_declaration declarator: (variable_declarator name: (identifier) @field.name) type: (_) @field.type) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "field.type",
	}
}

// ImportResolveConfig returns import resolution config
func (p *JavaProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(import_declaration (scoped_identifier) @import.path) @import.decl
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "",
		ItemsKey:    "",
		IsDotImport: false,
		IsWildcard:  true,
	}
}

// Interpret interprets capture results
func (p *JavaProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
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
		case "import.def", "import.decl":
			path := cap.Text
			// Remove trailing .* wildcard
			if strings.HasSuffix(path, ".*") {
				path = strings.TrimSuffix(path, ".*")
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
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (Java uses named)
func (p *JavaProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamed
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the Java tree-sitter language.
func (p *JavaProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.JavaLanguage()
}

// QuerySet returns all S-expression queries for Java extraction.
// Follows the capture naming conventions:
//   - Scope: @scope.module, @scope.class, @scope.function (no name sub-captures)
//   - Declaration: @declaration.* with @declaration.name sub-capture
//   - Import: @import.statement (single capture per import)
//   - TypeBinding: @type-binding.* with @type-binding.name and @type-binding.type
//   - Reference: @reference.call.* with unified @reference.name and @reference.receiver
func (p *JavaProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(program) @scope.module
			(class_declaration) @scope.class
			(interface_declaration) @scope.class
			(enum_declaration) @scope.class
			(record_declaration) @scope.class
			(annotation_type_declaration) @scope.class
			(method_declaration) @scope.function
			(constructor_declaration) @scope.function
		`,
		Declaration: `
			(class_declaration name: (identifier) @declaration.name) @declaration.class
			(interface_declaration name: (identifier) @declaration.name) @declaration.interface
			(enum_declaration name: (identifier) @declaration.name) @declaration.enum
			(record_declaration name: (identifier) @declaration.name) @declaration.record
			(method_declaration name: (identifier) @declaration.name) @declaration.method
			(constructor_declaration name: (identifier) @declaration.name) @declaration.constructor
			(field_declaration declarator: (variable_declarator name: (identifier) @declaration.name)) @declaration.property
			(local_variable_declaration declarator: (variable_declarator name: (identifier) @declaration.name)) @declaration.variable
		`,
		Import: `
			(import_declaration) @import.statement
		`,
		TypeBinding: `
			(formal_parameter type: (type_identifier) @type-binding.type name: (identifier) @type-binding.name) @type-binding.parameter
			(formal_parameter type: (generic_type) @type-binding.type name: (identifier) @type-binding.name) @type-binding.parameter
			(local_variable_declaration type: (type_identifier) @type-binding.type declarator: (variable_declarator name: (identifier) @type-binding.name)) @type-binding.annotation
			(method_declaration type: (type_identifier) @type-binding.type name: (identifier) @type-binding.name) @type-binding.return
			(object_creation_expression type: (type_identifier) @type-binding.type) @type-binding.constructor
		`,
		Reference: `
			(method_invocation object: (_) @reference.receiver name: (identifier) @reference.name) @reference.call.member
			(method_invocation name: (identifier) @reference.name) @reference.call.free
			(object_creation_expression type: (type_identifier) @reference.name) @reference.call.constructor
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. Names are extracted from the
// capture node's text (no name sub-captures in scope queries).
// After all scopes are created, ParentID is computed by nesting.
func (p *JavaProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = javaModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = javaExtractScopeName(cap, source)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = javaExtractScopeName(cap, source)
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
// Each match produces one symbol. Uses unified @declaration.name sub-capture
// for all declaration types.
func (p *JavaProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "declaration.class", "declaration.interface", "declaration.enum",
				"declaration.record", "declaration.method", "declaration.constructor",
				"declaration.variable", "declaration.property":
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

		// All declaration types use the unified @declaration.name sub-capture
		name := findCaptureTextInMatch(captures, matchIdx, "declaration.name")

		switch outerNodeType {
		case "declaration.class":
			sym.Name = name
			sym.Label = graph.LabelClass
		case "declaration.interface":
			sym.Name = name
			sym.Label = graph.LabelInterface
		case "declaration.enum":
			sym.Name = name
			sym.Label = graph.LabelEnum
		case "declaration.record":
			sym.Name = name
			sym.Label = graph.LabelClass
		case "declaration.method":
			sym.Name = name
			sym.Label = graph.LabelMethod
		case "declaration.constructor":
			sym.Name = name
			sym.Label = graph.LabelConstructor
		case "declaration.property":
			sym.Name = name
			sym.Label = graph.LabelProperty
		case "declaration.variable":
			sym.Name = name
			sym.Label = graph.LabelVariable
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Uses @import.statement single capture. Path extracted from node text.
func (p *JavaProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
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
		// Find the outer import statement capture
		var line int
		var text string
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && cap.NodeType == "import.statement" {
				line = cap.StartRow
				text = cap.Text
				break
			}
		}

		// Extract path from node text: "import com.example.Foo;" -> "com.example.Foo"
		path := javaExtractImportPath(text)

		isWildcard := strings.HasSuffix(path, ".*")
		if isWildcard {
			path = strings.TrimSuffix(path, ".*")
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
// Uses unified @type-binding.name and @type-binding.type sub-captures.
func (p *JavaProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.annotation",
			"type-binding.return", "type-binding.constructor":
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
				tb.BoundNode = findCaptureTextInMatch(captures, matchIdx, "type-binding.name")
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.annotation":
				tb.Kind = "annotation"
				tb.BoundNode = findCaptureTextInMatch(captures, matchIdx, "type-binding.name")
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.return":
				tb.Kind = "return"
				tb.BoundNode = findCaptureTextInMatch(captures, matchIdx, "type-binding.name")
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
				tb.StartLine = cap.StartRow
			case "type-binding.constructor":
				tb.Kind = "constructor"
				tb.TypeName = findCaptureTextInMatch(captures, matchIdx, "type-binding.type")
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
// Uses unified @reference.name and @reference.receiver sub-captures.
func (p *JavaProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.call.constructor":
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
			}
		}
		if ref.Kind == "" {
			continue
		}

		refs = append(refs, ref)
	}
	return refs
}

// ============ Java Language Helper Functions ============

// javaExtractScopeName extracts the name from a scope capture node's text.
// Scope queries have no name sub-captures; names are parsed from the node text.
func javaExtractScopeName(cap pipeline.LangCapture, source []byte) string {
	text := firstLineOfCapture(cap, source)
	switch {
	case strings.HasPrefix(text, "class "):
		return javaParseTypeName(text, "class ")
	case strings.HasPrefix(text, "interface "):
		return javaParseTypeName(text, "interface ")
	case strings.HasPrefix(text, "enum "):
		return javaParseTypeName(text, "enum ")
	case strings.HasPrefix(text, "record "):
		return javaParseTypeName(text, "record ")
	case strings.HasPrefix(text, "@interface "):
		return javaParseTypeName(text, "@interface ")
	case strings.Contains(text, " void "):
		return javaParseFuncName(text)
	case strings.Contains(text, " static "):
		return javaParseFuncName(text)
	default:
		// constructor or method: extract name before (
		return javaParseFuncName(text)
	}
}

// javaParseTypeName extracts the type name from a declaration line.
// Handles: "class Foo", "interface Foo", "enum Foo", "record Foo", etc.
func javaParseTypeName(text, prefix string) string {
	rest := strings.TrimPrefix(text, prefix)
	rest = strings.TrimSpace(rest)
	// Type name may be followed by <, {, space, extends, implements
	for i, ch := range rest {
		if ch == '<' || ch == '{' || ch == ' ' || ch == '\t' {
			return rest[:i]
		}
	}
	return rest
}

// javaParseFuncName extracts the function or constructor name from a declaration line.
// Handles: "public void foo(", "String bar(", "MyClass("
func javaParseFuncName(text string) string {
	// Find the last identifier before (
	if idx := strings.Index(text, "("); idx > 0 {
		before := text[:idx]
		// Get last word
		parts := strings.Fields(before)
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			// Remove generic type params: <T>
			if gi := strings.Index(name, "<"); gi > 0 {
				name = name[:gi]
			}
			return name
		}
	}
	return ""
}

// javaModuleName infers the module name from the file path.
// Java uses the file name (without .java extension) as the module-level scope name.
func javaModuleName(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// javaExtractImportPath extracts the import path from an import declaration node text.
// Handles: "import com.example.Foo;" -> "com.example.Foo"
// Handles: "import static com.example.Bar.method;" -> "com.example.Bar.method"
// Handles: "import com.example.*;" -> "com.example.*"
func javaExtractImportPath(text string) string {
	// Remove "import " prefix
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "import ")
	text = strings.TrimPrefix(text, "static ")
	text = strings.TrimSpace(text)
	// Remove trailing semicolon
	text = strings.TrimSuffix(text, ";")
	return strings.TrimSpace(text)
}

// ============ Java ScopeResolver ============

// JavaScopeResolver Java scope resolver
type JavaScopeResolver struct {
	provider *JavaProvider
}

// NewJavaScopeResolver creates a Java scope resolver
func NewJavaScopeResolver(provider *JavaProvider) *JavaScopeResolver {
	return &JavaScopeResolver{provider: provider}
}

func (r *JavaScopeResolver) Language() graph.Label {
	return graph.LabelJavaFile
}

func (r *JavaScopeResolver) LanguageProvider() *JavaProvider {
	return r.provider
}

// PopulateOwners populates owner relationships
func (r *JavaScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	return nil
}

// BuildMRO builds MRO (Java single inheritance + interface implementation, depth-first)
func (r *JavaScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	return nil
}

// ResolveImportTarget resolves import target (Java fully qualified name -> class path)
func (r *JavaScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	return nil, nil
}

// ============ Boolean switches ============

func (r *JavaScopeResolver) PropagatesReturnTypesAcrossImports() bool { return true }
func (r *JavaScopeResolver) FieldFallbackOnMethodLookup() bool        { return false }
func (r *JavaScopeResolver) UnwrapCollectionAccessor() bool           { return false }
func (r *JavaScopeResolver) CollapseMemberCallsByCallerTarget() bool  { return true }
func (r *JavaScopeResolver) PopulateNamespaceSiblings() bool          { return false }
func (r *JavaScopeResolver) HoistTypeBindingsToModule() bool          { return true }

// ============ 4 core methods ============

// MergeBindings merges binding sets (Java named import takes precedence)
func (r *JavaScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	// Java: import explicitly specifies names, import takes precedence
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

// ArityCompatibility Java static typing, strict checking (supports overloading)
func (r *JavaScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true
	}
	return caller.Args == targetArity
}

// ImportEdgeReason returns import edge reason
func (r *JavaScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "wildcard-import"
	}
	if len(imp.Symbols) > 0 {
		return "named-import"
	}
	return "fully-qualified-import"
}

// IsSuperReceiver Java super keyword
func (r *JavaScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super"
}

// ============ 4 functional hooks ============

func (r *JavaScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
}

func (r *JavaScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

func (r *JavaScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
}

func (r *JavaScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	return 0
}
