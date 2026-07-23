package ingest

import (
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestResolveGoPackageExcludesTestFiles(t *testing.T) {
	cfg := &GoModuleConfig{ModulePath: "example.com/project"}
	normalized := []string{
		"pkg/api/api.go",
		"pkg/api/api_test.go",
		"pkg/api/internal.go",
	}
	actual := resolveGoPackage("example.com/project/pkg/api", cfg, normalized, normalized)
	if len(actual) != 2 {
		t.Fatalf("resolveGoPackage returned %d files, want 2: %v", len(actual), actual)
	}
	for _, path := range actual {
		if path == "pkg/api/api_test.go" {
			t.Fatalf("resolveGoPackage included test file: %v", actual)
		}
	}
}

func TestResolveJvmSimpleNameFallbackUsesLastCandidate(t *testing.T) {
	files := []string{"tests/MyGame/Example/Monster.java", "tests/MyGame/Example2/Monster.java"}
	got := resolveJvmSimpleNameFallback("MyGame.Sample.Monster", files)
	if got != files[1] {
		t.Fatalf("fallback = %q, want %q", got, files[1])
	}
}

func TestResolveJavaScriptRelativeImportDoesNotSuffixFallback(t *testing.T) {
	paths := []string{"dist-module/wrappers/jquery.node-module-wrapper.js", "src/jquery.js"}
	ctx := BuildImportResolutionContext(paths, paths)
	if got := resolveImportPath(paths[0], "../../dist/jquery.js", "javascript", nil, ctx); got != nil {
		t.Fatalf("missing relative target resolved unexpectedly: %#v", got)
	}
}

func TestResolveRustImportUsesWorkspaceCrateRoot(t *testing.T) {
	files := map[string]bool{
		"crates/cli/src/decompress.rs": true,
		"crates/cli/src/process.rs":    true,
		"src/process.rs":               true,
	}
	got := resolveRustImport(
		"crates/cli/src/decompress.rs",
		"crate::process::{CommandError, CommandReader, CommandReaderBuilder}",
		files,
	)
	if got != "crates/cli/src/process.rs" {
		t.Fatalf("workspace import = %q, want crates/cli/src/process.rs", got)
	}
}

func TestResolveRustImportUsesRootCrateAndIgnoresExternalGroup(t *testing.T) {
	files := map[string]bool{"src/process.rs": true}
	if got := resolveRustImport("src/decompress.rs", "crate::process::CommandReaderBuilder", files); got != "src/process.rs" {
		t.Fatalf("root crate import = %q, want src/process.rs", got)
	}
	if got := resolveRustImport("src/decompress.rs", "std::{io, path}", files); got != "" {
		t.Fatalf("standard library group resolved unexpectedly to %q", got)
	}
}

func TestResolvePhpImportDoesNotBindExternalComposerClassByFilename(t *testing.T) {
	files := []string{"src/Plugin.php", "src/Plugins/UI/Master.php"}
	ctx := BuildImportResolutionContext(files, files)
	cfg := &ComposerConfig{PSR4: map[string]string{"LaravelLang\\Lang\\": "src/"}}
	if got := resolvePhpImport(
		"LaravelLang\\Publisher\\Plugins\\Plugin",
		cfg,
		ctx.AllFilePaths,
		ctx.NormalizedFileList,
		ctx.AllFileList,
		ctx.Index,
	); got != "" {
		t.Fatalf("external Composer class resolved to unrelated local file %q", got)
	}
	if got := resolvePhpImport(
		"LaravelLang\\Lang\\Plugin",
		cfg,
		ctx.AllFilePaths,
		ctx.NormalizedFileList,
		ctx.AllFileList,
		ctx.Index,
	); got != "src/Plugin.php" {
		t.Fatalf("local Composer class = %q, want src/Plugin.php", got)
	}
}

func TestImplicitPackageVisibilityForJavaAndCSharp(t *testing.T) {
	imports := NewImportMap()
	PopulateImplicitPackageVisibility([]FileInput{
		{Path: "java/A.java", Content: "package sample; class A {}"},
		{Path: "java/B.java", Content: "package sample; class B {}"},
		{Path: "cs/A.cs", Content: "namespace Sample; class A {}"},
		{Path: "cs/B.cs", Content: "namespace Sample { class B {} }"},
		{Path: "cs/Other.cs", Content: "namespace Other; class Other {}"},
	}, imports)
	if !imports["java/A.java"]["java/B.java"] || !imports["cs/A.cs"]["cs/B.cs"] {
		t.Fatalf("same-package visibility missing: %#v", imports)
	}
	if imports["java/A.java"]["cs/A.cs"] || imports["cs/A.cs"]["cs/Other.cs"] {
		t.Fatalf("visibility crossed language or namespace: %#v", imports)
	}
}

func TestImplicitPackageVisibilityForSwiftPMTarget(t *testing.T) {
	imports := NewImportMap()
	PopulateImplicitPackageVisibility([]FileInput{
		{Path: "Sources/Core/A.swift"},
		{Path: "Sources/Core/Nested/B.swift"},
		{Path: "Sources/Other/C.swift"},
		{Path: "Tests/CoreTests/A.swift"},
		{Path: "Tests/CoreTests/B.swift"},
	}, imports)
	if !imports["Sources/Core/A.swift"]["Sources/Core/Nested/B.swift"] {
		t.Fatalf("same SwiftPM source target visibility missing: %#v", imports)
	}
	if !imports["Tests/CoreTests/A.swift"]["Tests/CoreTests/B.swift"] {
		t.Fatalf("same SwiftPM test target visibility missing: %#v", imports)
	}
	if imports["Sources/Core/A.swift"]["Sources/Other/C.swift"] || imports["Sources/Core/A.swift"]["Tests/CoreTests/A.swift"] {
		t.Fatalf("Swift visibility crossed target boundary: %#v", imports)
	}
}

func TestApplyImportResultSkipsSelfImport(t *testing.T) {
	knowledgeGraph := graph.NewKnowledgeGraph()
	knowledgeGraph.AddNode(&graph.GraphNode{
		ID: "File:main.php", Label: graph.LabelFile,
		Properties: graph.NodeProperties{Name: "main.php", FilePath: "main.php"},
	})
	applyImportResult(
		knowledgeGraph,
		"main.php",
		&ImportResult{Kind: ImportResultFiles, Files: []string{"main.php"}},
		NewImportMap(),
		NewPackageMap(),
		NewNamedImportMap(),
	)
	if knowledgeGraph.RelationshipCount() != 0 {
		t.Fatalf("self import produced %d relationships", knowledgeGraph.RelationshipCount())
	}
}
