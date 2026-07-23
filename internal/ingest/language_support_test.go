package ingest

import (
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestDeclaredLanguagesHaveImplementedParsingQueries(t *testing.T) {
	registry := NewLanguageRegistry()
	for extension, language := range SupportedLanguages {
		parser := ParserID(extension)
		if !registry.HasParser(parser) || LanguageQueries(language) == "" {
			t.Errorf("language %q parser %q is declared supported without a binding and parsing queries", language, parser)
		}
	}
}

func TestRubyIsNotDeclaredSupported(t *testing.T) {
	registry := NewLanguageRegistry()
	if registry.HasParser("ruby") || LanguageID(".rb") != "" {
		t.Fatal("Ruby must remain disabled until its ingest queries and validation corpus are implemented")
	}
}

func TestTSXUsesTypeScriptLanguageAndTSXParser(t *testing.T) {
	if language := LanguageID(".tsx"); language != "typescript" {
		t.Fatalf("TSX language=%q, want typescript", language)
	}
	if parser := ParserID(".tsx"); parser != "tsx" {
		t.Fatalf("TSX parser=%q, want tsx", parser)
	}
	knowledgeGraph := graph.NewKnowledgeGraph()
	ProcessParsing(knowledgeGraph, []FileInput{{
		Path: "component.tsx", Content: "export function View() { return <main>Hello</main> }",
	}}, NewSymbolTable(), NewLanguageRegistry(), nil)
	found := false
	knowledgeGraph.ForEachNode(func(node *graph.GraphNode) {
		if node.Properties.Name == "View" {
			found = true
			if node.Properties.Language != "typescript" {
				t.Errorf("TSX graph language=%q, want typescript", node.Properties.Language)
			}
		}
	})
	if !found {
		t.Fatal("TSX parser did not extract View")
	}
}
