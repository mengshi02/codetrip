package ingest

import (
	"strings"
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestPythonTypeEnvironmentInfersConstructorsAndInstanceFields(t *testing.T) {
	source := []byte(`
class Repository:
    def save(self):
        return None

class AppService:
    def __init__(self, repository: Repository):
        self.repository = repository

    def run(self):
        self.repository.save()

def main():
    repository = Repository()
    service = AppService(repository)
    service.run()
`)
	lang, err := NewLanguageRegistry().GetLanguage("python")
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

	env := BuildTypeEnv(tree.RootNode(), "python", source)
	var saveCall, runCall *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "call" {
			text := node.Utf8Text(source)
			if strings.HasPrefix(text, "self.repository.save") {
				saveCall = node
			}
			if strings.HasPrefix(text, "service.run") {
				runCall = node
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if saveCall == nil || runCall == nil {
		t.Fatalf("calls not found in %s", tree.RootNode().ToSexp())
	}
	if got := LookupTypeEnv(env, "self.repository", saveCall, source); got != "Repository" {
		t.Fatalf("self.repository type = %q, want Repository; tree=%s env=%v", got, tree.RootNode().ToSexp(), env)
	}
	if got := LookupTypeEnv(env, "service", runCall, source); got != "AppService" {
		t.Fatalf("service type = %q, want AppService; env=%v", got, env)
	}
	name := saveCall.ChildByFieldName("function").ChildByFieldName("attribute")
	if got := ExtractReceiverName(name, source); got != "self.repository" {
		t.Fatalf("save receiver = %q, want self.repository", got)
	}
}

func TestRustTypeEnvironmentInfersStructValuesAndFields(t *testing.T) {
	source := []byte(`
struct Repository;
struct Service { repository: Repository }
impl Service { fn run(&self) { self.repository.save(); } }
fn main() { let service = Service { repository: Repository {} }; service.run(); }
`)
	lang, err := NewLanguageRegistry().GetLanguage("rust")
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
	env := BuildTypeEnv(tree.RootNode(), "rust", source)
	var saveCall, runCall *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "call_expression" {
			text := node.Utf8Text(source)
			if strings.HasPrefix(text, "self.repository.save") {
				saveCall = node
			}
			if strings.HasPrefix(text, "service.run") {
				runCall = node
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if saveCall == nil || runCall == nil {
		t.Fatalf("calls not found: %s", tree.RootNode().ToSexp())
	}
	if got := LookupTypeEnv(env, "self.repository", saveCall, source); got != "Repository" {
		t.Fatalf("self.repository type = %q, want Repository; tree=%s env=%v", got, tree.RootNode().ToSexp(), env)
	}
	saveName := saveCall.ChildByFieldName("function").ChildByFieldName("field")
	if got := ExtractReceiverName(saveName, source); got != "self.repository" {
		t.Fatalf("save receiver = %q, want self.repository", got)
	}
	if got := LookupTypeEnv(env, "service", runCall, source); got != "Service" {
		t.Fatalf("service type = %q, want Service; tree=%s env=%v", got, tree.RootNode().ToSexp(), env)
	}
}

func TestPythonNamedImportsPreserveBindings(t *testing.T) {
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g,
		[]FileInput{{Path: "main.py", Content: "from api import Query, Path as RoutePath\n"}},
		st, NewLanguageRegistry(), nil,
	)
	want := map[string]string{"Query": "Query", "RoutePath": "Path"}
	for _, imported := range extracted.Imports {
		if exported, ok := want[imported.NamedBinding]; ok && exported == imported.ExportedName {
			delete(want, imported.NamedBinding)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing Python named import bindings: %v; extracted=%#v", want, extracted.Imports)
	}
}
