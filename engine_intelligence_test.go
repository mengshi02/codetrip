package codetrip

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func TestIntelligenceRequestsValidateInput(t *testing.T) {
	engine, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if _, err := engine.Context(context.Background(), nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Context error=%v, want ErrInvalidRequest", err)
	}
	if _, err := engine.Impact(context.Background(), &ImpactRequest{Repo: "repo", NodeID: "node", MaxDepth: 11}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Impact maxDepth error=%v, want ErrInvalidRequest", err)
	}
	if _, err := engine.Impact(context.Background(), &ImpactRequest{Repo: "repo", NodeID: "node", Limit: 1001}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Impact limit error=%v, want ErrInvalidRequest", err)
	}
	if _, err := engine.Check(context.Background(), &CheckRequest{Repo: "repo", Checks: []string{"unknown"}}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Check error=%v, want ErrInvalidRequest", err)
	}
}

func TestNormalizedRelationSet(t *testing.T) {
	relations := normalizedRelationSet([]string{" calls ", "implements", ""}, nil)
	for _, relation := range []string{"CALLS", "IMPLEMENTS"} {
		if _, ok := relations[relation]; !ok {
			t.Errorf("normalized relations missing %s: %#v", relation, relations)
		}
	}
}

func TestValidSymbolName(t *testing.T) {
	for _, name := range []string{"Execute", "_value2", "$handler", "创建"} {
		if !validSymbolName(name) {
			t.Errorf("validSymbolName(%q)=false", name)
		}
	}
	for _, name := range []string{"", "2value", "has space", "a-b"} {
		if validSymbolName(name) {
			t.Errorf("validSymbolName(%q)=true", name)
		}
	}
}

func TestImpactNodeFilteringKeepsFilesOnlyForFileAnalysis(t *testing.T) {
	file := graph.NewNode("repo", graph.LabelFile, "main.go")
	function := graph.NewNode("repo", graph.LabelFunction, "Run")
	folder := graph.NewNode("repo", graph.LabelFolder, "pkg")

	if !isImpactNode(file, file) {
		t.Fatal("file impact should retain importing files")
	}
	if isImpactNode(function, file) {
		t.Fatal("symbol impact should exclude file noise")
	}
	if !isImpactNode(function, function) {
		t.Fatal("symbol impact should retain actionable symbols")
	}
	if isImpactNode(file, folder) {
		t.Fatal("impact should exclude folder noise")
	}
}

func TestDirectedCycleComponents(t *testing.T) {
	edges := []*graph.Edge{
		graph.NewEdge(graph.RelImports, "a", "b"),
		graph.NewEdge(graph.RelImports, "b", "a"),
		graph.NewEdge(graph.RelImports, "b", "c"),
		graph.NewEdge(graph.RelImports, "d", "e"),
		graph.NewEdge(graph.RelImports, "e", "f"),
		graph.NewEdge(graph.RelImports, "f", "d"),
		graph.NewEdge(graph.RelImports, "self", "self"),
	}
	components := directedCycleComponents(edges)
	if len(components) != 2 {
		t.Fatalf("components=%#v, want two cycles", components)
	}
	if strings.Join(components[0], ",") != "a,b" || strings.Join(components[1], ",") != "d,e,f" {
		t.Fatalf("unexpected components=%#v", components)
	}
}

func TestAddCheckFindingCountsBeyondOutputLimit(t *testing.T) {
	result := &CheckResult{}
	addCheckFinding(result, CheckFinding{Severity: "error"}, 1)
	addCheckFinding(result, CheckFinding{Severity: "warning"}, 1)
	if len(result.Findings) != 1 || !result.Truncated ||
		result.Summary.Errors != 1 || result.Summary.Warnings != 1 {
		t.Fatalf("result=%#v", result)
	}
}

func TestParseGitPatch(t *testing.T) {
	patch := `diff --git a/main.go b/main.go
index 1111111..2222222 100644
--- a/main.go
+++ b/main.go
@@ -3,2 +3,3 @@
-old
+new
+line
diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`
	files, err := parseGitPatch(patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%#v", files)
	}
	if files[0].Path != "main.go" || files[0].Status != "modified" ||
		files[0].Additions != 3 || files[0].Deletions != 2 ||
		len(files[0].Ranges) != 1 {
		t.Fatalf("modified file=%#v", files[0])
	}
	if files[1].Path != "new.go" || files[1].OldPath != "old.go" || files[1].Status != "renamed" {
		t.Fatalf("renamed file=%#v", files[1])
	}
}

func TestDiffMapsChangedSymbolAndImpact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	gitRoot := t.TempDir()
	repository := filepath.Join(gitRoot, "project")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(repository, "main.go")
	outsidePath := filepath.Join(gitRoot, "outside.go")
	original := "package fixture\n\nfunc Work() int { return 1 }\nfunc Run() int { return Work() }\n"
	if err := os.WriteFile(sourcePath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, gitRoot, "init", "-q")
	runGitTestCommand(t, gitRoot, "config", "user.email", "codetrip@example.invalid")
	runGitTestCommand(t, gitRoot, "config", "user.name", "Codetrip Test")
	runGitTestCommand(t, gitRoot, "add", ".")
	runGitTestCommand(t, gitRoot, "commit", "-q", "-m", "fixture")

	engine, err := Open(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if _, err := engine.IndexRepo(context.Background(), repository, WithRepoName("fixture")); err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(original, "return 1", "return 2", 1)
	if err := os.WriteFile(sourcePath, []byte(modified), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsidePath, []byte("package outside\n// unrelated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := engine.Diff(context.Background(), &DiffRequest{Repo: "fixture", MaxDepth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "main.go" {
		t.Fatalf("files=%#v", result.Files)
	}
	if len(result.Symbols) != 1 || result.Symbols[0].Node.Name != "Work" {
		t.Fatalf("symbols=%#v", result.Symbols)
	}
	if len(result.Impacted) != 1 || result.Impacted[0].Node.Name != "Run" ||
		len(result.Impacted[0].Causes) != 1 || result.Impacted[0].Causes[0] != result.Symbols[0].Node.ID {
		t.Fatalf("impacted=%#v", result.Impacted)
	}
}

func runGitTestCommand(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
