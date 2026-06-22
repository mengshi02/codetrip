package phases

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
)

// ============ Provider-Driven Parse Framework ============
//
// This file implements the scope-based pipeline extraction using LanguageProvider.
// It replaces the hardcoded extractXxxNodes functions in parse.go.
//
// Flow per file:
//   1. Parse source with tree-sitter (1 time)
//   2. Execute ScopeQuery → InterpretScope → scope tree
//   3. Execute DeclarationQuery → InterpretDeclaration → symbols
//   4. Execute ImportQuery → InterpretImport → imports
//   5. Execute TypeBindingQuery → InterpretTypeBinding → type bindings
//   6. Execute ReferenceQuery → InterpretReference → classified references

// compiledQueries holds pre-compiled tree-sitter queries for a language.
// Query compilation is expensive; this struct is created once per provider
// and reused across all files of the same language.
type compiledQueries struct {
	scope       *gotreesitter.Query
	declaration *gotreesitter.Query
	importQ     *gotreesitter.Query
	typeBinding *gotreesitter.Query
	reference   *gotreesitter.Query
}

// langProvider extends pipeline.Provider with tree-sitter language access.
// The parse phase needs the *gotreesitter.Language to create parsers.
type langProvider interface {
	pipeline.Provider
	TreeSitterLanguage() *gotreesitter.Language
}

// compileQueries compiles all queries for a given language provider.
// Returns nil for queries that are empty strings (not supported by the language).
func compileQueries(provider langProvider, lang *gotreesitter.Language) (*compiledQueries, error) {
	qs := provider.QuerySet()
	cq := &compiledQueries{}
	var err error

	if qs.Scope != "" {
		if cq.scope, err = gotreesitter.NewQuery(qs.Scope, lang); err != nil {
			return nil, fmt.Errorf("compile scope query: %w", err)
		}
	}
	if qs.Declaration != "" {
		if cq.declaration, err = gotreesitter.NewQuery(qs.Declaration, lang); err != nil {
			return nil, fmt.Errorf("compile declaration query: %w", err)
		}
	}
	if qs.Import != "" {
		if cq.importQ, err = gotreesitter.NewQuery(qs.Import, lang); err != nil {
			return nil, fmt.Errorf("compile import query: %w", err)
		}
	}
	if qs.TypeBinding != "" {
		if cq.typeBinding, err = gotreesitter.NewQuery(qs.TypeBinding, lang); err != nil {
			return nil, fmt.Errorf("compile type-binding query: %w", err)
		}
	}
	if qs.Reference != "" {
		if cq.reference, err = gotreesitter.NewQuery(qs.Reference, lang); err != nil {
			return nil, fmt.Errorf("compile reference query: %w", err)
		}
	}

	return cq, nil
}

// parseWithProvider parses a file using a LanguageProvider.
// This is the unified extraction entry point that replaces parseWithTreeSitter + extractXxxNodes.
func parseWithProvider(f *pipeline.ParsedFile, provider langProvider, cq *compiledQueries) {
	lang := provider.TreeSitterLanguage()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(f.Content)
	if err != nil || tree == nil {
		return
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return
	}

	// Skip extraction for effectively empty files (root has no named children)
	if int(root.NamedChildCount()) == 0 {
		return
	}

	source := f.Content

	// Execute each query and interpret results
	if cq.scope != nil {
		captures := executeAndConvertQuery(cq.scope, root, lang, source)
		f.Scopes = provider.InterpretScope(captures, source, f.Path)
	}

	if cq.declaration != nil {
		captures := executeAndConvertQuery(cq.declaration, root, lang, source)
		f.Symbols = provider.InterpretDeclaration(captures, source, f.Path)
	}

	if cq.importQ != nil {
		captures := executeAndConvertQuery(cq.importQ, root, lang, source)
		f.Imports = provider.InterpretImport(captures, source, f.Path)
	}

	if cq.typeBinding != nil {
		captures := executeAndConvertQuery(cq.typeBinding, root, lang, source)
		f.TypeBindings = provider.InterpretTypeBinding(captures, source, f.Path)
	}

	if cq.reference != nil {
		captures := executeAndConvertQuery(cq.reference, root, lang, source)
		f.References = provider.InterpretReference(captures, source, f.Path)
	}
}

// executeAndConvertQuery runs a tree-sitter query against a node and converts
// the results to pipeline.LangCapture format for provider Interpret methods.
// Captures are grouped by match: all captures belonging to the same match
// share the same MatchIndex.
func executeAndConvertQuery(q *gotreesitter.Query, root *gotreesitter.Node, lang *gotreesitter.Language, source []byte) []pipeline.LangCapture {
	matches := q.ExecuteNode(root, lang, source)
	if len(matches) == 0 {
		return nil
	}

	// Pre-allocate with exact total
	totalCaptures := 0
	for _, m := range matches {
		totalCaptures += len(m.Captures)
	}
	captures := make([]pipeline.LangCapture, 0, totalCaptures)
	for matchIdx, match := range matches {
		for _, qc := range match.Captures {
			cap := queryCaptureToLangCapture(qc, source, lang)
			cap.MatchIndex = matchIdx
			captures = append(captures, cap)
		}
	}
	return captures
}

// queryCaptureToLangCapture converts a gotreesitter.QueryCapture to pipeline.LangCapture.
// NodeType is set to the capture name (e.g., "scope.function") because Interpret methods
// use NodeType to distinguish capture semantics, not the raw AST node type.
func queryCaptureToLangCapture(qc gotreesitter.QueryCapture, source []byte, lang *gotreesitter.Language) pipeline.LangCapture {
	node := qc.Node
	cap := pipeline.LangCapture{
		Name:     qc.Name,
		Text:     qc.Text(source),
		NodeType: qc.Name,
	}
	if node != nil {
		sp := node.StartPoint()
		ep := node.EndPoint()
		cap.StartRow = int(sp.Row) + 1 // 1-based
		cap.StartCol = int(sp.Column)
		cap.EndRow = int(ep.Row) + 1
		cap.EndCol = int(ep.Column)
	}
	return cap
}

// createSymbolNode creates a graph Node from a SymbolInfo.
// Centralizes the node creation logic that was previously inline in parse.go.
func createSymbolNode(repo string, sym *pipeline.SymbolInfo) *graph.Node {
	n := graph.NewNode(repo, sym.Label, sym.Name).
		WithFile(sym.FilePath).
		WithProp("startLine", sym.StartLine).
		WithProp("endLine", sym.EndLine)

	// Write structured fields as node properties
	if sym.QualifiedName != "" {
		n = n.WithProp("qualifiedName", sym.QualifiedName)
	}
	if sym.Visibility != "" {
		n = n.WithProp("visibility", sym.Visibility)
	}
	if sym.IsStatic {
		n = n.WithProp("isStatic", true)
	}
	if sym.IsAbstract {
		n = n.WithProp("isAbstract", true)
	}
	if sym.IsFinal {
		n = n.WithProp("isFinal", true)
	}
	if sym.IsVirtual {
		n = n.WithProp("isVirtual", true)
	}
	if sym.IsOverride {
		n = n.WithProp("isOverride", true)
	}
	if sym.IsAsync {
		n = n.WithProp("isAsync", true)
	}
	if sym.ReturnType != "" {
		n = n.WithProp("returnType", sym.ReturnType)
	}
	if len(sym.Annotations) > 0 {
		n = n.WithProp("annotations", sym.Annotations)
	}
	if sym.ScopeID != "" {
		n = n.WithProp("scopeID", sym.ScopeID)
	}
	if len(sym.Parameters) > 0 {
		n = n.WithProp("paramCount", len(sym.Parameters))
	}

	// Preserve backward compatibility: also write Props map
	if sym.Props != nil {
		for k, v := range sym.Props {
			n = n.WithProp(k, v)
		}
	}

	return n
}