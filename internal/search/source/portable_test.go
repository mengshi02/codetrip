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
	results, err := index.Search(context.Background(), "lang:go file:main ParseConfig", ScopeCode, 10, 1)
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
	results, err = reopened.Search(context.Background(), "regex:true Parse(Config|Options)", ScopeCode, 10, 0)
	if err != nil || len(results) != 2 {
		t.Fatalf("regex results=%#v err=%v", results, err)
	}
}

func TestPortableIndexFiltersSourceScopes(t *testing.T) {
	repository := t.TempDir()
	dataDir := t.TempDir()
	files := map[string]string{
		"main.go":        "package sample\nfunc NewRackRepo() {}\n",
		"config.yml":     "constructor: NewRackRepo\n",
		"CMakeLists.txt": "NewRackRepo engineering configuration\n",
		"README.md":      "NewRackRepo documentation\n",
		"notes.log":      "NewRackRepo ignored\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	index := newPortableIndex(dataDir, "snapshot")
	if err := index.Build(repository, "snapshot"); err != nil {
		t.Fatal(err)
	}
	defer index.Close()

	code, err := index.Search(context.Background(), "NewRackRepo", ScopeCode, 10, 0)
	if err != nil || len(code) != 3 {
		t.Fatalf("code results=%#v err=%v", code, err)
	}
	docs, err := index.Search(context.Background(), "NewRackRepo", ScopeDocs, 10, 0)
	if err != nil || len(docs) != 1 || docs[0].FilePath != "README.md" {
		t.Fatalf("docs results=%#v err=%v", docs, err)
	}
	all, err := index.Search(context.Background(), "NewRackRepo", ScopeAll, 10, 0)
	if err != nil || len(all) != 4 {
		t.Fatalf("all results=%#v err=%v", all, err)
	}
}

func TestPortableQueryRejectsInvalidRegularExpression(t *testing.T) {
	if _, err := parsePortableQuery("regex:true ["); err == nil {
		t.Fatal("expected invalid regular expression error")
	}
}
