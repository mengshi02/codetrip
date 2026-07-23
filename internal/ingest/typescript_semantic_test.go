package ingest

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestTypeScriptClassFieldCarriesReceiverType(t *testing.T) {
	source := []byte(`
interface Repository { save(): void }
class Service {
  private repository: Repository;
  run(): void { this.repository.save(); }
}
`)
	lang, err := NewLanguageRegistry().GetLanguage("typescript")
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
	env := BuildTypeEnv(tree.RootNode(), "typescript", source)
	var call, name *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "call_expression" && strings.Contains(node.Utf8Text(source), "repository.save") {
			call = node
			function := node.ChildByFieldName("function")
			if function != nil {
				name = function.ChildByFieldName("property")
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if call == nil || name == nil {
		t.Fatalf("call missing: %s", tree.RootNode().ToSexp())
	}
	receiver := ExtractReceiverName(name, source)
	if receiver != "this.repository" {
		t.Fatalf("receiver = %q, want this.repository; tree=%s", receiver, tree.RootNode().ToSexp())
	}
	if got := LookupTypeEnv(env, receiver, call, source); got != "Repository" {
		t.Fatalf("receiver type = %q, want Repository; tree=%s env=%v", got, tree.RootNode().ToSexp(), env)
	}
}
