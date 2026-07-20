package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableIndexBuildSearchAndReopen(t *testing.T) {
	repository := t.TempDir()
	dataDir := t.TempDir()
	content := "package sample\n\nfunc ParseConfig() {}\nfunc run() { ParseConfig() }\n"
	if err := os.WriteFile(filepath.Join(repository, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	index := newPortableIndex(dataDir, "snapshot")
	if err := index.Build(repository, "snapshot"); err != nil {
		t.Fatal(err)
	}
	results, err := index.Search(context.Background(), "lang:go file:main ParseConfig", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Line != 3 || results[0].After == "" {
		t.Fatalf("unexpected results: %#v", results)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newPortableIndex(dataDir, "snapshot")
	if err := reopened.Open(); err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	results, err = reopened.Search(context.Background(), "regex:true Parse(Config|Options)", 10, 0)
	if err != nil || len(results) != 2 {
		t.Fatalf("regex results=%#v err=%v", results, err)
	}
}

func TestPortableQueryRejectsInvalidRegularExpression(t *testing.T) {
	if _, err := parsePortableQuery("regex:true ["); err == nil {
		t.Fatal("expected invalid regular expression error")
	}
}
