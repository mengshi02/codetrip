package lang

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ============ Markdown LanguageProvider ============

// MarkdownProvider Markdown language provider
// Markdown is a special Provider: no imports, no calls, no classes, only Section extraction (headings -> Section nodes)
type MarkdownProvider struct{}

// NewMarkdownProvider creates a Markdown language provider
func NewMarkdownProvider() *MarkdownProvider {
	return &MarkdownProvider{}
}

// Language returns the language label
func (p *MarkdownProvider) Language() graph.Label {
	return graph.LabelMarkdownFile
}

// Captures returns node capture rules
// Markdown only captures headings as Section nodes
func (p *MarkdownProvider) Captures() *CapturesConfig {
	return &CapturesConfig{
		Query: `
			(atx_heading (atx_h1_marker) @h1.marker) @h1.def
			(atx_heading (atx_h2_marker) @h2.marker) @h2.def
			(atx_heading (atx_h3_marker) @h3.marker) @h3.def
			(atx_heading (atx_h4_marker) @h4.marker) @h4.def
			(atx_heading (atx_h5_marker) @h5.marker) @h5.def
			(atx_heading (atx_h6_marker) @h6.marker) @h6.def
			(setext_heading) @setext.def
		`,
		CaptureMap: map[string]graph.Label{
			"h1.def":      graph.LabelSection,
			"h2.def":      graph.LabelSection,
			"h3.def":      graph.LabelSection,
			"h4.def":      graph.LabelSection,
			"h5.def":      graph.LabelSection,
			"h6.def":      graph.LabelSection,
			"setext.def":  graph.LabelSection,
		},
	}
}

// CallExtractConfig returns call extraction config (Markdown has no calls)
func (p *MarkdownProvider) CallExtractConfig() *CallExtractConfig {
	return &CallExtractConfig{
		Query:      "",
		CaptureMap: map[string]graph.Label{},
	}
}

// ClassExtractConfig returns class extraction config (Markdown has no classes)
func (p *MarkdownProvider) ClassExtractConfig() *ClassExtractConfig {
	return &ClassExtractConfig{
		Query:      "",
		CaptureMap: map[string]graph.Label{},
		NameKey:    "",
		BaseKey:    "",
	}
}

// FieldExtractConfig returns field extraction config (Markdown has no fields)
func (p *MarkdownProvider) FieldExtractConfig() *FieldExtractConfig {
	return &FieldExtractConfig{
		Query:      "",
		CaptureMap: map[string]graph.Label{},
		NameKey:    "",
		TypeKey:    "",
	}
}

// ImportResolveConfig returns import resolution config (Markdown has no imports)
func (p *MarkdownProvider) ImportResolveConfig() *ImportResolveConfig {
	return &ImportResolveConfig{
		Query:      "",
		CaptureMap: map[string]graph.Label{},
	}
}

// Interpret interprets capture results
// Markdown only processes heading Section nodes
func (p *MarkdownProvider) Interpret(captures *CaptureResult) (*InterpretResult, error) {
	result := &InterpretResult{
		Symbols:   make([]pipeline.SymbolInfo, 0),
		Imports:   make([]ImportInfo, 0),
		CallSites: make([]CallSite, 0),
		Classes:   make([]ClassInfo, 0),
		Fields:    make([]FieldInfo, 0),
	}

	for _, cap := range captures.Captures {
		switch cap.NodeType {
		case "h1.def", "h2.def", "h3.def", "h4.def", "h5.def", "h6.def", "setext.def":
			// Extract heading text as Section name
			name := cap.Text
			result.Symbols = append(result.Symbols, pipeline.SymbolInfo{
				Name:      name,
				Label:     graph.LabelSection,
				FilePath:  captures.Filepath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
		}
	}

	return result, nil
}

// ImportSemantics returns import semantics (Markdown has no import semantics)
func (p *MarkdownProvider) ImportSemantics() ImportSemantics {
	return ""
}

// ============ Scope-Based Pipeline Interface Methods ============

// TreeSitterLanguage returns the Markdown tree-sitter language.
func (p *MarkdownProvider) TreeSitterLanguage() *gotreesitter.Language {
	return grammars.MarkdownLanguage()
}

// QuerySet returns all S-expression queries for Markdown extraction.
// Markdown only captures headings as Section nodes; no imports, calls, classes, or fields.
func (p *MarkdownProvider) QuerySet() *pipeline.LangQuerySet {
	return &pipeline.LangQuerySet{
		Scope: `
			(document) @scope.module
			(atx_heading) @scope.section
			(setext_heading) @scope.section
		`,
		Declaration: `
			(atx_heading) @declaration.section
			(setext_heading) @declaration.section
		`,
		Import:      "",
		TypeBinding: "",
		Reference:   "",
	}
}

// InterpretScope builds a scope tree from scope query captures.
// Markdown scopes are heading sections (h1-h6).
func (p *MarkdownProvider) InterpretScope(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ScopeInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices to identify distinct scope nodes
	seen := make(map[int]struct{})
	for _, cap := range captures {
		switch cap.NodeType {
		case "scope.module", "scope.section":
			seen[cap.MatchIndex] = struct{}{}
		}
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
				name = mdModuleName(filePath)
				startLine = cap.StartRow
				endLine = cap.EndRow
			case "scope.section":
				kind = "section"
				name = mdExtractHeadingText(cap.Text)
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
// Markdown only has Section declarations (headings).
func (p *MarkdownProvider) InterpretDeclaration(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.SymbolInfo {
	if len(captures) == 0 {
		return nil
	}

	// Collect unique match indices
	seen := make(map[int]struct{})
	for _, cap := range captures {
		if cap.NodeType == "declaration.section" {
			seen[cap.MatchIndex] = struct{}{}
		}
	}

	symbols := make([]*pipeline.SymbolInfo, 0, len(seen))
	for matchIdx := range seen {
		matchCaps := capturesInMatch(captures, matchIdx)
		for _, cap := range matchCaps {
			if cap.NodeType != "declaration.section" {
				continue
			}
			name := mdExtractHeadingText(cap.Text)
			symbols = append(symbols, &pipeline.SymbolInfo{
				Name:      name,
				Label:     graph.LabelSection,
				FilePath:  filePath,
				StartLine: cap.StartRow,
				EndLine:   cap.EndRow,
			})
			break
		}
	}
	return symbols
}

// InterpretImport extracts import information from import query captures.
// Markdown has no imports — always returns nil.
func (p *MarkdownProvider) InterpretImport(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ImportInfo {
	return nil
}

// InterpretTypeBinding extracts type binding information from type-binding query captures.
// Markdown has no type bindings — always returns nil.
func (p *MarkdownProvider) InterpretTypeBinding(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.TypeBindingInfo {
	return nil
}

// InterpretReference extracts classified references from reference query captures.
// Markdown has no references — always returns nil.
func (p *MarkdownProvider) InterpretReference(captures []pipeline.LangCapture, source []byte, filePath string) []*pipeline.ReferenceInfo {
	return nil
}

// ============ Markdown Helper Functions ============

// mdModuleName derives a module name from the file path.
func mdModuleName(filePath string) string {
	if filePath == "" {
		return "markdown"
	}
	base := filePath
	if idx := lastIndexOf(base, '/'); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := lastIndexOf(base, '\\'); idx >= 0 {
		base = base[idx+1:]
	}
	// Strip extension
	if dot := lastIndexOf(base, '.'); dot > 0 {
		base = base[:dot]
	}
	return base
}

// mdExtractHeadingText extracts heading text, stripping the # markers.
func mdExtractHeadingText(text string) string {
	// Strip leading # markers and spaces
	i := 0
	for i < len(text) && (text[i] == '#' || text[i] == ' ' || text[i] == '\t') {
		i++
	}
	result := text[i:]
	// Trim trailing newlines and whitespace
	for len(result) > 0 && (result[len(result)-1] == '\n' || result[len(result)-1] == '\r' || result[len(result)-1] == ' ') {
		result = result[:len(result)-1]
	}
	return result
}

// lastIndexOf returns the last index of ch in s, or -1 if not found.
func lastIndexOf(s string, ch byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ch {
			return i
		}
	}
	return -1
}