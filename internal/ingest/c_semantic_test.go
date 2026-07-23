package ingest

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestCVoidParameterMeansZeroArguments(t *testing.T) {
	source := []byte(`void run(void);`)
	lang, err := NewLanguageRegistry().GetLanguage("c")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	var declaration *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil || declaration != nil {
			return
		}
		if node.Kind() == "declaration" {
			declaration = node
			return
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if declaration == nil {
		t.Fatalf("declaration missing: %s", tree.RootNode().ToSexp())
	}
	signature := ExtractMethodSignature(declaration, source)
	if signature.ParameterCount == nil || *signature.ParameterCount != 0 {
		t.Fatalf("void parameter arity = %v, want zero; tree=%s", signature.ParameterCount, tree.RootNode().ToSexp())
	}
}
