package ingest

import "testing"

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
