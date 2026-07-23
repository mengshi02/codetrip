package ingest

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestPHPConstructorPromotionAndAssignmentTypes(t *testing.T) {
	source := []byte(`<?php
class Repository { public function save(): void {} }
class Service {
    public function __construct(private Repository $repository) {}
    public function run(): void { $this->repository->save(); }
}
function main(): void {
    $service = new Service(new Repository());
    $service->run();
}
`)
	lang, err := NewLanguageRegistry().GetLanguage("php")
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
	env := BuildTypeEnv(tree.RootNode(), "php", source)
	var saveCall, runCall, saveName, runName *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "member_call_expression" {
			text := node.Utf8Text(source)
			if strings.Contains(text, "repository->save") {
				saveCall = node
				saveName = node.ChildByFieldName("name")
			}
			if strings.Contains(text, "service->run") {
				runCall = node
				runName = node.ChildByFieldName("name")
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if saveCall == nil || runCall == nil {
		t.Fatalf("calls missing: %s", tree.RootNode().ToSexp())
	}
	saveReceiver := ExtractReceiverName(saveName, source)
	if saveReceiver != "$this->repository" {
		t.Fatalf("save receiver = %q; tree=%s", saveReceiver, tree.RootNode().ToSexp())
	}
	if got := LookupTypeEnv(env, saveReceiver, saveCall, source); got != "Repository" {
		t.Fatalf("save receiver type = %q, want Repository; env=%v tree=%s", got, env, tree.RootNode().ToSexp())
	}
	runReceiver := ExtractReceiverName(runName, source)
	if got := LookupTypeEnv(env, runReceiver, runCall, source); got != "Service" {
		t.Fatalf("run receiver type = %q, want Service; receiver=%q env=%v tree=%s", got, runReceiver, env, tree.RootNode().ToSexp())
	}
}
