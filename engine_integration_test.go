package codetrip

import (
	"context"
	"errors"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func TestIndexRepoPersistsValidatedGraphAndExportsCSV(t *testing.T) {
	repository := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "data")
	source := []byte("package fixture\n\nfunc Work() {}\nfunc Run() { Work() }\n")
	if err := os.WriteFile(filepath.Join(repository, "main.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("Work is documented here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "CMakeLists.txt"), []byte("# Work engineering configuration\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	csvDir := filepath.Join(t.TempDir(), "csv")
	result, err := engine.IndexRepo(context.Background(), repository,
		WithRepoName("fixture"), WithCSVExport(csvDir), WithCSVExportStrict(true))
	if err != nil {
		t.Fatal(err)
	}
	if result.Nodes == 0 || result.Edges == 0 || result.CSVPath != csvDir {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(csvDir, "relations.csv")); err != nil {
		t.Fatalf("relationships CSV missing: %v", err)
	}
	graphStore := engine.graphStore("fixture")
	if graphStore == nil {
		t.Fatal("graph store not registered")
	}
	if graphStore.Repo() == "fixture" {
		t.Fatal("active graph was not isolated in a physical snapshot namespace")
	}
	nodes, err := graphStore.GetNodesByName(graphStore.Repo(), "Run")
	if err != nil || len(nodes) != 1 {
		t.Fatalf("Run nodes: %d, error: %v", len(nodes), err)
	}
	edges, err := graphStore.GetOutEdges(nodes[0].ID, "CALLS")
	if err != nil || len(edges) != 1 {
		t.Fatalf("Run CALLS edges: %d, error: %v", len(edges), err)
	}
	traversal, err := engine.Traverse(context.Background(), &TraverseRequest{
		Repo: "fixture", StartNodeID: nodes[0].ID, Direction: TraverseOutgoing,
		MaxDepth: 1, RelationTypes: []string{"CALLS"},
	})
	if err != nil || len(traversal.Nodes) != 1 || traversal.Nodes[0].Name != "Work" {
		t.Fatalf("CALLS traversal=%#v, error=%v", traversal, err)
	}
	if len(traversal.Edges) != 1 || traversal.Edges[0].Type != "CALLS" || traversal.Edges[0].ID == "" {
		t.Fatalf("CALLS traversal edges=%#v", traversal.Edges)
	}
	symbolContext, err := engine.Context(context.Background(), &ContextRequest{
		Repo: "fixture", NodeID: traversal.Nodes[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if symbolContext.Symbol.Name != "Work" || len(symbolContext.Relations) != 1 {
		t.Fatalf("symbol context=%#v", symbolContext)
	}
	if !strings.Contains(symbolContext.Content, "func Work()") {
		manifest, _ := os.ReadFile(filepath.Join(engine.repoDirs["fixture"], "manifest.json"))
		t.Fatalf("symbol context content=%q symbol=%#v manifest=%s", symbolContext.Content, symbolContext.Symbol, manifest)
	}
	if relation := symbolContext.Relations[0]; relation.Direction != "in" ||
		relation.Relation.Type != "CALLS" || relation.Node.Name != "Run" {
		t.Fatalf("unexpected context relation=%#v", relation)
	}
	impact, err := engine.Impact(context.Background(), &ImpactRequest{
		Repo: "fixture", NodeID: traversal.Nodes[0].ID, MaxDepth: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(impact.Impacted) != 1 || impact.Impacted[0].Node.Name != "Run" ||
		impact.Impacted[0].Depth != 1 || impact.Impacted[0].Via.Type != "CALLS" {
		t.Fatalf("impact=%#v", impact)
	}
	rename, err := engine.Rename(context.Background(), &RenameRequest{
		Repo: "fixture", NodeID: traversal.Nodes[0].ID, NewName: "Execute",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rename.Safe || rename.OldName != "Work" || rename.NewName != "Execute" ||
		len(rename.SemanticReferences) != 1 || rename.SemanticReferences[0].Node.Name != "Run" {
		t.Fatalf("rename=%#v", rename)
	}
	var declarations, semanticReferences int
	for _, occurrence := range rename.Occurrences {
		switch occurrence.Kind {
		case "declaration":
			declarations++
		case "semantic-reference":
			semanticReferences++
		}
	}
	if declarations != 1 || semanticReferences != 1 {
		t.Fatalf("rename occurrences=%#v", rename.Occurrences)
	}
	conflictingRename, err := engine.Rename(context.Background(), &RenameRequest{
		Repo: "fixture", NodeID: traversal.Nodes[0].ID, NewName: "Run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if conflictingRename.Safe || len(conflictingRename.Conflicts) != 1 ||
		conflictingRename.Conflicts[0].Severity != "error" {
		t.Fatalf("conflicting rename=%#v", conflictingRename)
	}
	check, err := engine.Check(context.Background(), &CheckRequest{Repo: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if check.Summary.NodesScanned != result.Nodes || check.Summary.EdgesScanned != result.Edges ||
		check.Summary.Errors != 0 || check.Summary.Warnings != 0 || len(check.Findings) != 0 {
		t.Fatalf("check=%#v", check)
	}
	path, err := engine.ShortestPath(context.Background(), &PathRequest{
		Repo: "fixture", SourceNodeID: nodes[0].ID, TargetNodeID: traversal.Nodes[0].ID,
	})
	if err != nil || len(path.Edges) != 1 || path.Edges[0].Type != "CALLS" || path.Edges[0].ID == "" {
		t.Fatalf("shortest path=%#v, error=%v", path, err)
	}
	searchResult, err := engine.Search(context.Background(), &SearchRequest{Repo: "fixture", Query: "Run", Limit: 10})
	if err != nil || len(searchResult.Results) == 0 || searchResult.Results[0].Name != "Run" {
		t.Fatalf("symbol search result=%#v, error=%v", searchResult, err)
	}
	sourceResult, err := engine.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "Work", Limit: 10})
	if err != nil || len(sourceResult.Results) == 0 || sourceResult.Results[0].FilePath != "main.go" {
		t.Fatalf("source search=%#v, error=%v", sourceResult, err)
	}
	for _, match := range sourceResult.Results {
		if match.FilePath == "README.md" {
			t.Fatalf("default code scope returned documentation: %#v", sourceResult)
		}
	}
	docsResult, err := engine.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "Work", Scope: "docs", Limit: 10})
	if err != nil || len(docsResult.Results) != 1 || docsResult.Results[0].FilePath != "README.md" {
		t.Fatalf("docs source search=%#v, error=%v", docsResult, err)
	}
	allResult, err := engine.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "Work", Scope: "all", Limit: 10})
	if err != nil || len(allResult.Results) < 3 {
		t.Fatalf("all source search=%#v, error=%v", allResult, err)
	}
	if _, err := engine.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "Work", Scope: "invalid"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid source scope error=%v, want ErrInvalidRequest", err)
	}
	embedResult, err := engine.EmbedRepo(context.Background(), "fixture", deterministicEmbedder{}, nil)
	if err != nil || embedResult.NodesEmbedded == 0 {
		t.Fatalf("embed result=%#v, error=%v", embedResult, err)
	}
	hybrid, err := engine.HybridSearch(context.Background(), &HybridSearchRequest{Repo: "fixture", Query: "Run", Limit: 10})
	if err != nil || len(hybrid.Results) == 0 {
		t.Fatalf("hybrid search=%#v, error=%v", hybrid, err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repositories, err := reopened.ListRepos()
	if err != nil || len(repositories) != 1 || repositories[0].Name != "fixture" {
		t.Fatalf("reopened repositories: %#v, error: %v", repositories, err)
	}
	if reopened.graphStore("fixture") == nil {
		t.Fatal("reopened graph store missing")
	}
	reopenedSearch, err := reopened.Search(context.Background(), &SearchRequest{Repo: "fixture", Query: "Work", Limit: 10})
	if err != nil || len(reopenedSearch.Results) == 0 {
		t.Fatalf("reopened symbol search=%#v, error=%v", reopenedSearch, err)
	}
	reopenedSource, err := reopened.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "Run", Limit: 10})
	if err != nil || len(reopenedSource.Results) == 0 {
		t.Fatalf("reopened source search=%#v, error=%v", reopenedSource, err)
	}
	if err := reopened.AttachEmbedder("fixture", deterministicEmbedder{}); err != nil {
		t.Fatalf("attach persisted vectors: %v", err)
	}
	reopenedHybrid, err := reopened.HybridSearch(context.Background(), &HybridSearchRequest{Repo: "fixture", Query: "Work", Limit: 10})
	if err != nil || len(reopenedHybrid.Results) == 0 {
		t.Fatalf("reopened hybrid search=%#v, error=%v", reopenedHybrid, err)
	}
	fullDir := filepath.Join(t.TempDir(), "full")
	manifest, err := reopened.ExportFullCSV("fixture", fullDir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.NodeCount != result.Nodes || manifest.EdgeCount != result.Edges {
		t.Fatalf("full export counts=%d/%d, index counts=%d/%d", manifest.NodeCount, manifest.EdgeCount, result.Nodes, result.Edges)
	}
	for _, name := range []string{"nodes.csv", "edges.csv", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(fullDir, name)); err != nil {
			t.Fatalf("full export %s missing: %v", name, err)
		}
	}
}

func TestRepositoriesUseIndependentStoresAndDeleteAtomically(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	firstSource := t.TempDir()
	secondSource := t.TempDir()
	if err := os.WriteFile(filepath.Join(firstSource, "first.go"), []byte("package first\nfunc First() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondSource, "second.go"), []byte("package second\nfunc Second() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	creator, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creator.IndexRepo(context.Background(), firstSource, WithRepoName("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := creator.IndexRepo(context.Background(), secondSource, WithRepoName("second")); err != nil {
		t.Fatal(err)
	}
	firstRoot, secondRoot := creator.repoDirs["first"], creator.repoDirs["second"]
	if firstRoot == secondRoot || firstRoot == "" || secondRoot == "" {
		t.Fatalf("repository roots are not isolated: first=%q second=%q", firstRoot, secondRoot)
	}
	if err := creator.Close(); err != nil {
		t.Fatal(err)
	}

	firstEngine, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer firstEngine.Close()
	secondEngine, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer secondEngine.Close()
	if _, err := firstEngine.Search(context.Background(), &SearchRequest{Repo: "first", Query: "First"}); err != nil {
		t.Fatalf("open first repository: %v", err)
	}
	if _, err := secondEngine.Search(context.Background(), &SearchRequest{Repo: "second", Query: "Second"}); err != nil {
		t.Fatalf("second repository was blocked by first repository lock: %v", err)
	}
	if err := secondEngine.DeleteRepo(context.Background(), "first"); err == nil {
		t.Fatal("deleted a repository locked by another engine")
	}
	if _, err := os.Stat(firstRoot); err != nil {
		t.Fatalf("locked repository was modified: %v", err)
	}
	secondEngine.mu.Lock()
	secondEngine.indexing["second"] = struct{}{}
	secondEngine.mu.Unlock()
	if err := secondEngine.DeleteRepo(context.Background(), "second"); !errors.Is(err, ErrRepoBusy) {
		t.Fatalf("delete busy repository error=%v, want ErrRepoBusy", err)
	}
	secondEngine.mu.Lock()
	delete(secondEngine.indexing, "second")
	secondEngine.mu.Unlock()
	if err := secondEngine.DeleteRepo(context.Background(), "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(secondRoot); !os.IsNotExist(err) {
		t.Fatalf("deleted repository directory remains: %v", err)
	}
	if _, err := os.Stat(firstRoot); err != nil {
		t.Fatalf("unrelated repository was affected: %v", err)
	}
	repositories, err := secondEngine.ListRepos()
	if err != nil || len(repositories) != 1 || repositories[0].Name != "first" {
		t.Fatalf("repositories after delete=%#v err=%v", repositories, err)
	}
	if err := secondEngine.DeleteRepo(context.Background(), "second"); !errors.Is(err, ErrRepoNotFound) {
		t.Fatalf("second delete error=%v, want ErrRepoNotFound", err)
	}
}

type deterministicEmbedder struct{}

func (deterministicEmbedder) Dimensions() int { return 4 }

func (deterministicEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, value := range texts {
		hash := fnv.New32a()
		_, _ = hash.Write([]byte(value))
		n := hash.Sum32()
		result[i] = []float32{float32(n&255) / 255, float32((n>>8)&255) / 255, float32((n>>16)&255) / 255, 1}
	}
	return result, nil
}

func TestRepositoriesWithIdenticalSymbolsRemainIsolated(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	engine, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	for _, repo := range []string{"first", "second"} {
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "main.go"), []byte("package p\nfunc Work() {}\nfunc Run() { Work() }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.IndexRepo(context.Background(), directory, WithRepoName(repo)); err != nil {
			t.Fatal(err)
		}
	}

	first := engine.graphStore("first")
	second := engine.graphStore("second")
	if first == nil || second == nil || first.Repo() == second.Repo() {
		t.Fatalf("snapshot stores are not isolated: first=%v second=%v", first, second)
	}
	for _, graphStore := range []*graph.GraphStore{first, second} {
		nodes, err := graphStore.GetNodesByName(graphStore.Repo(), "Run")
		if err != nil || len(nodes) != 1 {
			t.Fatalf("Run nodes in %s: %d, error=%v", graphStore.Repo(), len(nodes), err)
		}
		edges, err := graphStore.GetOutEdges(nodes[0].ID, "CALLS")
		if err != nil || len(edges) != 1 {
			t.Fatalf("CALLS edges in %s: %d, error=%v", graphStore.Repo(), len(edges), err)
		}
	}
}

func TestReplaceRepositoryPublishesNewSnapshotAndCollectsOldOne(t *testing.T) {
	repository := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(filepath.Join(repository, "main.go"), []byte("package p\nfunc OldName() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.IndexRepo(context.Background(), repository, WithRepoName("fixture")); err != nil {
		t.Fatal(err)
	}
	oldPhysical := engine.graphStore("fixture").Repo()
	if err := os.WriteFile(filepath.Join(repository, "main.go"), []byte("package p\nfunc NewName() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.IndexRepo(context.Background(), repository, WithRepoName("fixture")); !errors.Is(err, ErrRepoAlreadyExists) {
		t.Fatalf("index without replace error=%v", err)
	}
	if _, err := engine.IndexRepo(context.Background(), repository, WithRepoName("fixture"), WithReplaceExisting(true)); err != nil {
		t.Fatal(err)
	}
	newPhysical := engine.graphStore("fixture").Repo()
	if newPhysical == oldPhysical {
		t.Fatal("replacement reused physical snapshot")
	}
	result, err := engine.SearchSource(context.Background(), &SourceSearchRequest{Repo: "fixture", Query: "NewName", Limit: 10})
	if err != nil || len(result.Results) == 0 {
		t.Fatalf("new source snapshot=%#v error=%v", result, err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.graphStore("fixture").Repo() != newPhysical {
		t.Fatal("replacement was not durable")
	}
	oldStore := graph.NewGraphStore(reopened.stores["fixture"], oldPhysical)
	oldNodes, err := oldStore.GetNodesByName(oldPhysical, "OldName")
	if err != nil {
		t.Fatal(err)
	}
	if len(oldNodes) != 0 {
		t.Fatalf("retired graph still has %d nodes", len(oldNodes))
	}
	repoRoot := reopened.repoDirs["fixture"]
	for _, path := range []string{filepath.Join(repoRoot, "index", oldPhysical), filepath.Join(repoRoot, "content", oldPhysical), filepath.Join(repoRoot, "vectors", oldPhysical)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("retired artifact remains: %s", path)
		}
	}
}
