package ingest

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func parseJavaDeclaration(t *testing.T, source string) (*sitter.Tree, *sitter.Node) {
	t.Helper()
	registry := NewLanguageRegistry()
	lang, err := registry.GetLanguage("java")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree := parser.Parse([]byte(source), nil)
	if tree == nil {
		t.Fatal("java parser returned nil tree")
	}
	root := tree.RootNode()
	if root.NamedChildCount() == 0 {
		t.Fatal("java source has no declaration")
	}
	return tree, root.NamedChild(0)
}

func findJavaKind(node *sitter.Node, kind string) *sitter.Node {
	if node.Kind() == kind {
		return node
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if found := findJavaKind(node.NamedChild(i), kind); found != nil {
			return found
		}
	}
	return nil
}

func TestJavaExportCheckerDeclarationModifiers(t *testing.T) {
	tests := []struct {
		name, source, symbol string
		exported             bool
	}{
		{"public class", "public class PublicType {}", "PublicType", true},
		{"package private class", "class InternalType {}", "InternalType", false},
		{"public method", "class Holder { public void run() {} }", "run", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, declaration := parseJavaDeclaration(t, tt.source)
			defer tree.Close()
			if tt.name == "public method" {
				declaration = findJavaKind(declaration, "method_declaration")
				if declaration == nil {
					t.Fatal("method declaration not found")
				}
			}
			if got := javaExportChecker(declaration, tt.symbol, []byte(tt.source)); got != tt.exported {
				t.Fatalf("javaExportChecker(%q) = %v, want %v", tt.symbol, got, tt.exported)
			}
		})
	}
}
