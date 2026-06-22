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

// ============ Python LanguageProvider ============

// PythonProvider is the Python language provider
type PythonProvider struct{}

// NewPythonProvider creates a Python language provider
func NewPythonProvider() *PythonProvider {
	return &PythonProvider{}
}

// Language returns language label
func (p *PythonProvider) Language() graph.Label {
	return graph.LabelPythonFile
}

// Captures returns node capture rules
func (p *PythonProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(function_definition name: (identifier) @fn.name) @fn.def
			(class_definition name: (identifier) @class.name) @class.def
			(decorator) @decorator.def
			(assignment left: (identifier) @var.name) @var.def
			(import_statement name: (dotted_name (identifier) @import.module)) @import.def
			(import_from_statement module_name: (dotted_name (identifier) @import.module) name: (import_list (dotted_name (identifier) @import.item))?) @import.from
		`,
		CaptureMap: map[string]graph.Label{
			"fn.def":        graph.LabelFunction,
			"class.def":     graph.LabelClass,
			"decorator.def": graph.LabelDecorator,
			"var.def":       graph.LabelVariable,
			"import.def":    graph.LabelImport,
			"import.from":   graph.LabelImport,
		},
	}
}

// CallExtractConfig returns call extraction configuration
func (p *PythonProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query: `
			(call
				function: (attribute
					object: (identifier) @call.receiver
					attribute: (identifier) @call.method
				)
			) @call.site
			(call
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
func (p *PythonProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query: `
			(class_definition name: (identifier) @class.name superclasses: (argument_list (identifier) @class.base)) @class.def
		`,
		CaptureMap: map[string]graph.Label{
			"class.def": graph.LabelClass,
		},
		NameKey: "class.name",
		BaseKey: "class.base",
	}
}

// FieldExtractConfig returns field extraction configuration
func (p *PythonProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query: `
			(class_definition
				body: (block
					(expression_statement
						(assignment left: (identifier) @field.name)
					)
				)
			) @field.def
		`,
		CaptureMap: map[string]graph.Label{
			"field.def": graph.LabelField,
		},
		NameKey: "field.name",
		TypeKey: "",
	}
}

// ImportResolveConfig returns import resolution configuration
func (p *PythonProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query: `
			(import_statement name: (dotted_name (identifier) @import.path)) @import.decl
			(import_from_statement module_name: (dotted_name (identifier) @import.path) name: (import_list (dotted_name (identifier) @import.item))?) @import.from
		`,
		CaptureMap: map[string]graph.Label{
			"import.decl": graph.LabelImport,
			"import.from": graph.LabelImport,
		},
		PathKey:     "import.path",
		AliasKey:    "import.alias",
		ItemsKey:    "import.item",
		IsDotImport: false,
		IsWildcard:  true,
	}
}

// Interpret interprets capture results
func (p *PythonProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
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
		case "decorator.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelDecorator,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "var.def":
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name: cap.Name, Label: graph.LabelVariable,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		case "import.def", "import.decl":
			imp := ImportInfo{
				Path: cap.Name, FilePath: captures.Filepath, StartLine: cap.StartRow,
			}
			result.Imports = append(result.Imports, imp)
		case "import.from":
			imp := ImportInfo{
				Path: cap.Name, FilePath: captures.Filepath, StartLine: cap.StartRow,
			}
			for _, child := range cap.Children {
				if child.NodeType == "import.item" {
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
				FilePath:   captures.Filepath, StartLine: cap.StartRow, EndLine: cap.EndRow,
			})
		case "field.def":
			name := ""
			for _, child := range cap.Children {
				if child.NodeType == "field.name" {
					name = child.Text
				}
			}
			result.Fields = append(result.Fields, FieldInfo{
				Name:     name,
				FilePath: captures.Filepath, StartLine: cap.StartRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (Python uses namespace)
func (p *PythonProvider) ImportSemantics() ImportSemantics {
	return ImportSemanticsNamespace
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the Python tree-sitter language.
func (p *PythonProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.PythonLanguage()
}

// QuerySet returns all S-expression queries for Python extraction.
// It combines the existing Captures, CallExtract, ClassExtract, FieldExtract,
// and ImportResolve queries into a single LangQuerySet.
func (p *PythonProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(module) @scope.module
			(class_definition) @scope.class
			(function_definition) @scope.function
			(lambda) @scope.function
		`,
		Declaration: `
			(class_definition name: (identifier) @declaration.name) @declaration.class
			(function_definition name: (identifier) @declaration.name) @declaration.function
			(decorated_definition (decorator (identifier) @_decorator) (function_definition name: (identifier) @declaration.name)) @declaration.property
			(assignment left: (identifier) @declaration.name) @declaration.variable
			(for_statement left: (identifier) @declaration.name) @declaration.variable
		`,
		Import: `
			(import_statement) @import.statement
			(import_from_statement) @import.statement
		`,
		TypeBinding: `
			(typed_parameter (identifier) @type-binding.name type: (type) @type-binding.type) @type-binding.parameter
			(typed_default_parameter name: (identifier) @type-binding.name type: (type) @type-binding.type) @type-binding.parameter
			(function_definition name: (identifier) @type-binding.name return_type: (type) @type-binding.type) @type-binding.return
			(assignment left: (identifier) @type-binding.name right: (call function: (identifier) @type-binding.type)) @type-binding.constructor
		`,
		Reference: `
			(call function: (identifier) @reference.name) @reference.call.free
			(call function: (attribute object: (_) @reference.receiver attribute: (identifier) @reference.name)) @reference.call.member
			(assignment left: (attribute object: (_) @reference.receiver attribute: (identifier) @reference.name)) @reference.write.member
		`,
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Each match produces one scope node. After all scopes are created,
// ParentID is computed by nesting (child scope's line range is within
// parent scope's line range).
// Scope queries do not include name sub-captures; names are extracted
// from the captured node's source text instead.
func (p *PythonProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
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
				name = pythonModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.class":
				kind = "class"
				name = pythonExtractScopeName(cap.Text, "class")
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.function":
				kind = "function"
				name = pythonExtractScopeName(cap.Text, "function")
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
// Each match produces one symbol. Sub-captures (declaration.name) are
// looked up within the same match to avoid cross-match contamination.
func (p *PythonProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
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
			case "declaration.class", "declaration.function", "declaration.variable", "declaration.property":
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
		case "declaration.function":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelFunction
		case "declaration.class":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelClass
		case "declaration.property":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelProperty
		case "declaration.variable":
			sym.Name = findCaptureTextInMatch(captures, matchIdx, "declaration.name")
			sym.Label = graph.LabelVariable
		default:
			continue
		}

		symbols = append(symbols, sym)
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Each match (import.statement) produces one ImportInfo.
// Since the query uses single-anchor captures (no sub-captures), the path
// and symbols are parsed from the captured node's text.
func (p *PythonProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
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
		imp := &pipeline.ImportInfo{
			SourceFile: filePath,
		}

		// Find the outer capture text and start line
		for _, cap := range captures {
			if cap.MatchIndex == matchIdx && cap.NodeType == "import.statement" {
				imp.Line = cap.StartRow
				path, symbols := pyParseImportText(cap.Text)
				imp.Path = path
				imp.Symbols = symbols
				break
			}
		}

		imports = append(imports, imp)
	}
	return imports
}

// pyParseImportText parses a Python import statement text into path and symbols.
// Handles:
//   - "import os" → path="os", symbols=[]
//   - "import os, sys" → path="os", symbols=["os","sys"]
//   - "from typing import List" → path="typing", symbols=["List"]
//   - "from typing import List, Dict" → path="typing", symbols=["List","Dict"]
func pyParseImportText(text string) (path string, symbols []string) {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "from ") {
		// "from <module> import <item1>, <item2>"
		rest := text[5:] // after "from "
		importIdx := strings.Index(rest, " import ")
		if importIdx < 0 {
			// malformed, treat entire rest as path
			return strings.TrimSpace(rest), nil
		}
		path = strings.TrimSpace(rest[:importIdx])
		itemsStr := strings.TrimSpace(rest[importIdx+8:]) // after " import "
		for _, item := range strings.Split(itemsStr, ",") {
			item = strings.TrimSpace(item)
			// Handle "import List as L" -> take "List"
			if asIdx := strings.Index(item, " as "); asIdx >= 0 {
				item = strings.TrimSpace(item[:asIdx])
			}
			if item != "" {
				symbols = append(symbols, item)
			}
		}
		return path, symbols
	}

	if strings.HasPrefix(text, "import ") {
		// "import os" or "import os, sys"
		rest := text[7:] // after "import "
		parts := strings.Split(rest, ",")
		for i, part := range parts {
			part = strings.TrimSpace(part)
			// Handle "import os as operating_system" -> take "os"
			if asIdx := strings.Index(part, " as "); asIdx >= 0 {
				part = strings.TrimSpace(part[:asIdx])
			}
			if part != "" {
				if i == 0 {
					path = part
				}
				symbols = append(symbols, part)
			}
		}
		return path, symbols
	}

	return text, nil
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Each match produces one TypeBindingInfo. Python type bindings come from:
//   - typed_parameter / typed_default_parameter (parameter annotations)
//   - function_definition return type annotations
//   - assignment with constructor call (inferred variable types)
func (p *PythonProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices for type-binding outer captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "type-binding.parameter", "type-binding.return", "type-binding.constructor":
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
			case "type-binding.return":
				tb.Kind = "return"
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

		// Fill BoundNode name for parameter and constructor bindings
		tb.BoundNode = findCaptureTextInMatch(captures, matchIdx, "type-binding.name")

		bindings = append(bindings, tb)
	}
	return bindings
}

// InterpretReference extracts classified references from reference query captures.
// Each match produces one ReferenceInfo. Python references come from:
//   - call.free: bare function call like foo()
//   - call.member: method call like obj.method()
//   - write.member: attribute write like obj.attr = value
func (p *PythonProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices for reference outer captures
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "reference.call.free", "reference.call.member", "reference.write.member":
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
			case "reference.write.member":
				ref.Kind = "write.member"
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

// ============ Python Scope Name Extraction Helpers ============

// pythonFirstLine returns the first line of a text string.
func pythonFirstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// pythonParseFuncName extracts the function name from a declaration line.
// Handles: "def foo(" or "async def foo("
func pythonParseFuncName(line string) string {
	idx := strings.Index(line, "def ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+4:] // after "def "
	// skip whitespace
	rest = strings.TrimSpace(rest)
	for i, ch := range rest {
		if ch == '(' || ch == ':' || ch == ' ' {
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// pythonParseClassName extracts the class name from a declaration line.
// Handles: "class Foo(" or "class Foo:" or "class Foo(Base):"
func pythonParseClassName(line string) string {
	idx := strings.Index(line, "class ")
	if idx < 0 {
		return ""
	}
	rest := line[idx+6:] // after "class "
	rest = strings.TrimSpace(rest)
	for i, ch := range rest {
		if ch == '(' || ch == ':' || ch == ' ' {
			if i == 0 {
				return ""
			}
			return rest[:i]
		}
	}
	return rest
}

// pythonExtractScopeName extracts the declaration name from captured node text.
// Parses the first line of the text to find the function/class name.
func pythonExtractScopeName(text string, kind string) string {
	if text == "" {
		return ""
	}
	firstLine := pythonFirstLine(text)
	switch kind {
	case "function":
		return pythonParseFuncName(firstLine)
	case "class":
		return pythonParseClassName(firstLine)
	}
	return ""
}

// pythonModuleName infers a Python module name from a file path.
// For "pkg/module.py" returns "module"; for "__init__.py" returns the parent directory.
func pythonModuleName(filePath string) string {
	base := filepath.Base(filePath)
	if base == "__init__.py" {
		return filepath.Base(filepath.Dir(filePath))
	}
	if strings.HasSuffix(base, ".py") {
		return base[:len(base)-3]
	}
	return base
}

// ============ Python ScopeResolver ============

// PythonScopeResolver is the Python scope resolver
type PythonScopeResolver struct {
	provider *PythonProvider
}

// NewPythonScopeResolver creates a Python scope resolver
func NewPythonScopeResolver(provider *PythonProvider) *PythonScopeResolver {
	return &PythonScopeResolver{provider: provider}
}

// Language returns language label
func (r *PythonScopeResolver) Language() graph.Label {
	return graph.LabelPythonFile
}

// LanguageProvider returns language provider
func (r *PythonScopeResolver) LanguageProvider() *PythonProvider {
	return r.provider
}

// PopulateOwners populates owner relationships (function/method -> file, class -> file, etc.)
// Python symbols are attributed to file nodes via module semantics
func (r *PythonScopeResolver) PopulateOwners(gs *graph.GraphStore, model *ScopeModel) error {
	if model == nil {
		return nil
	}
	repo := gs.Repo()
	// Iterate over the mutable model's symbol table entries
	return model.ForEachSymbol(func(_ string, entry *pipeline.SymbolEntry) error {
		// Find file node
		fileNodes, err := gs.GetNodesByFile(repo, entry.FilePath)
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
// Python uses C3 linearization algorithm (multiple inheritance)
func (r *PythonScopeResolver) BuildMRO(gs *graph.GraphStore, classes []*graph.Node) error {
	// Python's MRO strategy: C3 linearization, depth-first search
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
// Python module import: import path -> module file -> symbols in module
func (r *PythonScopeResolver) ResolveImportTarget(gs *graph.GraphStore, imp *ImportRef) ([]*graph.Node, error) {
	path := imp.Path
	// Standard library prefixes
	stdlibPrefixes := []string{"os", "sys", "json", "collections", "itertools", "functools", "typing", "abc", "io", "pathlib", "re", "math", "datetime", "logging", "unittest", "http", "urllib", "socket", "threading", "subprocess", "hashlib", "copy", "pprint", "enum", "dataclasses", "argparse", "configparser", "csv", "xml", "html", "email", "ssl", "asyncio", "concurrent"}
	for _, prefix := range stdlibPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			// Standard library, cannot resolve in local repository
			return nil, nil
		}
	}

	// Find matching file nodes in repository
	var targets []*graph.Node
	allNodes := gs.GetAllNodes("", 10000)
	for _, node := range allNodes {
		if node.Label == graph.LabelFile {
			// Python module path to file path: a.b.c -> a/b/c.py
			modulePath := strings.ReplaceAll(path, ".", "/")
			if strings.Contains(node.FilePath, modulePath) {
				targets = append(targets, node)
			}
		}
	}

	return targets, nil
}

// ============ Boolean Switches ============

// PropagatesReturnTypesAcrossImports Python type annotations do not propagate across imports
func (r *PythonScopeResolver) PropagatesReturnTypesAcrossImports() bool { return false }

// FieldFallbackOnMethodLookup Python dynamic language, fallback to method when field lookup fails
func (r *PythonScopeResolver) FieldFallbackOnMethodLookup() bool { return true }

// UnwrapCollectionAccessor Python unwraps collection accessors (list/dict destructuring)
func (r *PythonScopeResolver) UnwrapCollectionAccessor() bool { return true }

// CollapseMemberCallsByCallerTarget Python does not collapse call edges (duck typing needs each edge)
func (r *PythonScopeResolver) CollapseMemberCallsByCallerTarget() bool { return false }

// PopulateNamespaceSiblings Python has no implicit cross-file visibility
func (r *PythonScopeResolver) PopulateNamespaceSiblings() bool { return false }

// HoistTypeBindingsToModule Python hoists type bindings to module
func (r *PythonScopeResolver) HoistTypeBindingsToModule() bool { return true }

// ============ Core Methods ============

// MergeBindings merges local binding set and imported binding set
// Python strategy: namespace priority (from X import Y: Y overrides local, import X: X supplements)
func (r *PythonScopeResolver) MergeBindings(local, imported *BindingSet) *BindingSet {
	result := NewBindingSet()
	result.FilePath = local.FilePath
	result.IsImported = false

	// Import bindings first (namespace import priority)
	for name, ids := range imported.Bindings {
		result.Bindings[name] = ids
	}

	// Local bindings supplement (do not override imports)
	for name, ids := range local.Bindings {
		if _, exists := result.Bindings[name]; !exists {
			result.Bindings[name] = ids
		}
	}

	return result
}

// ArityCompatibility checks arity compatibility between call site and target
// Python relaxed check: supports *args/**kwargs, parameter count need not match exactly
func (r *PythonScopeResolver) ArityCompatibility(caller *CallSiteRef, target *graph.Node) bool {
	targetArity := target.Props.Arity
	if targetArity == 0 {
		return true // No arity info, default compatible
	}
	// Python relaxed check: supports variadic args, difference within 2 is compatible
	return caller.Args >= targetArity-1 && caller.Args <= targetArity+2
}

// ImportEdgeReason returns import edge reason description
func (r *PythonScopeResolver) ImportEdgeReason(imp *ImportRef) string {
	if imp.IsWildcard {
		return "wildcard-import"
	}
	if len(imp.Symbols) > 0 {
		return "from-import"
	}
	return "namespace-import"
}

// IsSuperReceiver checks if receiver is super class receiver
// Python super() calls on inheritance chain
func (r *PythonScopeResolver) IsSuperReceiver(recv string) bool {
	return recv == "super"
}

// ============ 4 Function-type Hooks ============

// PopulateRangeBindings for-in / list comprehension variable bindings
// Python comprehension variables are captured as local variables during tree-sitter parsing
func (r *PythonScopeResolver) PopulateRangeBindings(files FileSet, indexes *IndexSet, ctx *RangeBindContext) {
	// Python for-loop variables are captured as local variables during tree-sitter parsing
	// This is a no-op, Python does not need additional range binding logic
}

// CollectScopeContextPaths collects scope context file paths
// Python module scope, current file path
func (r *PythonScopeResolver) CollectScopeContextPaths(opts *ScopeContextOptions) map[string]struct{} {
	result := make(map[string]struct{})
	if opts != nil {
		result[opts.FilePath] = struct{}{}
	}
	return result
}

// EmitPostResolutionEdges post-processing edge emission
// Python does not need additional post-processing edges
func (r *PythonScopeResolver) EmitPostResolutionEdges(graphStore *graph.GraphStore, files FileSet, lookup GraphNodeLookup, indexes *IndexSet, ctx *EmitContext) {
	// Python has no post-processing edge requirements
}

// EmitUnresolvedReceiverEdges untyped receiver fallback
// Python dynamic language needs fallback mechanism, return 0 as placeholder
func (r *PythonScopeResolver) EmitUnresolvedReceiverEdges(graphStore *graph.GraphStore, scopes *ScopeMapType, files FileSet, lookup GraphNodeLookup, handledSites map[string]struct{}, model *ScopeModel) int {
	// Python dynamic language needs fallback mechanism, return 0 as placeholder
	return 0
}
