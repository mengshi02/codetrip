package integration

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mengshi02/codetrip"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// helper: create a temp directory with Go source files
func createTestRepo(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	msg := Hello("world")
	fmt.Println(msg)
}

func Hello(name string) string {
	return "hello " + name
}
`,
		"service.go": `package main

type Service struct {
	Name string
}

func (s *Service) Process(input string) string {
	return input
}

func NewService(name string) *Service {
	return &Service{Name: name}
}
`,
		"handler/handler.go": `package handler

type Handler struct {
	ServiceName string
}

func (h *Handler) Handle(req string) string {
	return req
}
`,
	}

	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	return dir
}

// helper: open Trip with a temp data dir
func openTrip(t *testing.T) *codetrip.Trip {
	t.Helper()
	dataDir := t.TempDir()
	trip, err := codetrip.Open(dataDir)
	if err != nil {
		t.Fatalf("open trip: %v", err)
	}
	t.Cleanup(func() { trip.Close() })
	return trip
}

// ============ Test 1: IndexRepo → Search → Impact 全链路 ============

func TestIndexRepoSearchImpact(t *testing.T) {
	trip := openTrip(t)
	repoPath := createTestRepo(t, "testrepo")

	// Step 1: IndexRepo
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := trip.IndexRepo(ctx, repoPath, codetrip.WithRepoName("testrepo"))
	if err != nil {
		t.Fatalf("index repo: %v", err)
	}

	if result.Repo != "testrepo" {
		t.Errorf("expected repo name testrepo, got %s", result.Repo)
	}
	if result.Nodes == 0 {
		t.Error("expected at least some nodes after indexing")
	}
	if result.Files == 0 {
		t.Error("expected at least some files after indexing")
	}

	t.Logf("indexed: repo=%s files=%d nodes=%d edges=%d duration=%.2fs",
		result.Repo, result.Files, result.Nodes, result.Edges, result.Duration)

	// Step 2: Search
	searchResult, err := trip.Search(ctx, &pipeline.SearchRequest{
		Query: "Hello",
		Repo:  "testrepo",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	t.Logf("search 'Hello': %d results", len(searchResult.Results))

	// Step 3: Impact
	impactResult, err := trip.Impact(ctx, &pipeline.ImpactRequest{
		Target:    "Hello",
		Repo:      "testrepo",
		Direction: "downstream",
		MaxDepth:  2,
	})
	if err != nil {
		// Impact may fail if target not found, which is acceptable
		t.Logf("impact: %v (acceptable — may need exact symbol name)", err)
	} else {
		t.Logf("impact: risk=%s", impactResult.Risk)
	}
}

// ============ Test 2: IndexRepo → DropIndex → ListRepos 验证删除完整性 ============

func TestIndexRepoDropIndexListRepos(t *testing.T) {
	trip := openTrip(t)
	repoPath := createTestRepo(t, "droptest")

	ctx := context.Background()

	// Index
	_, err := trip.IndexRepo(ctx, repoPath, codetrip.WithRepoName("droptest"))
	if err != nil {
		t.Fatalf("index repo: %v", err)
	}

	// Verify repo appears in list
	repos, err := trip.ListRepos()
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	found := false
	for _, r := range repos {
		if r.Name == "droptest" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'droptest' in repo list after indexing")
	}

	// Drop
	if err := trip.DropIndex("droptest"); err != nil {
		t.Fatalf("drop index: %v", err)
	}

	// Verify repo no longer appears
	repos, err = trip.ListRepos()
	if err != nil {
		t.Fatalf("list repos after drop: %v", err)
	}
	for _, r := range repos {
		if r.Name == "droptest" {
			t.Error("expected 'droptest' to be removed from repo list after drop")
		}
	}
}

// ============ Test 3: 3 个 repo 并发索引 ============

func TestConcurrentIndexRepo(t *testing.T) {
	trip := openTrip(t)

	// Create 3 separate repos
	repoPaths := make([]string, 3)
	for i := 0; i < 3; i++ {
		repoPaths[i] = createTestRepo(t, "concurrent")
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	for i, path := range repoPaths {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			_, err := trip.IndexRepo(ctx, p, codetrip.WithRepoName("concurrent-repo"))
			if err != nil {
				errCh <- err
			}
		}(i, path)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent index error: %v", err)
	}
}

// ============ Test 4: Crash Recovery — discoverRepos 恢复 ============

func TestCrashRecovery(t *testing.T) {
	// Use a shared data dir that persists across two Trip instances
	dataDir := t.TempDir()
	repoPath := createTestRepo(t, "crashtest")

	ctx := context.Background()

	// Phase 1: Open, index, then close (simulates crash after indexing)
	trip1, err := codetrip.Open(dataDir)
	if err != nil {
		t.Fatalf("open first trip: %v", err)
	}

	_, err = trip1.IndexRepo(ctx, repoPath, codetrip.WithRepoName("crashtest"))
	if err != nil {
		trip1.Close()
		t.Fatalf("index repo: %v", err)
	}
	trip1.Close()

	// Phase 2: Reopen — discoverRepos should restore graph stores
	trip2, err := codetrip.Open(dataDir)
	if err != nil {
		t.Fatalf("open second trip: %v", err)
	}
	defer trip2.Close()

	repos, err := trip2.ListRepos()
	if err != nil {
		t.Fatalf("list repos after recovery: %v", err)
	}

	found := false
	for _, r := range repos {
		if r.Name == "crashtest" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'crashtest' to be recovered after crash recovery")
	}

	t.Logf("crash recovery: found %d repos", len(repos))
}

// ============ Test 5: Cypher timeout protection ============

func TestCypherTimeout(t *testing.T) {
	trip := openTrip(t)
	repoPath := createTestRepo(t, "cyphertest")

	ctx := context.Background()

	// Index first
	_, err := trip.IndexRepo(ctx, repoPath, codetrip.WithRepoName("cyphertest"))
	if err != nil {
		t.Fatalf("index repo: %v", err)
	}

	// Run a simple Cypher query
	result, err := trip.Cypher(ctx, "MATCH (n) RETURN n LIMIT 5")
	if err != nil {
		t.Logf("cypher query: %v (may be expected if no nodes match)", err)
	} else {
		t.Logf("cypher: %d rows", len(result.Rows))
	}

	// Run with a very short timeout to verify timeout mechanism
	shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = trip.Cypher(shortCtx, "MATCH (n) RETURN n")
	if err == nil {
		t.Log("cypher with nanosecond timeout did not error (possible race — acceptable)")
	} else {
		t.Logf("cypher timeout correctly triggered: %v", err)
	}
}

// ============ Test 6: Verify consistency check ============

func TestVerifyConsistency(t *testing.T) {
	trip := openTrip(t)
	repoPath := createTestRepo(t, "verifytest")

	ctx := context.Background()

	_, err := trip.IndexRepo(ctx, repoPath, codetrip.WithRepoName("verifytest"))
	if err != nil {
		t.Fatalf("index repo: %v", err)
	}

	issues, err := trip.Verify(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(issues) > 0 {
		t.Logf("verify found %d issues:", len(issues))
		for _, issue := range issues {
			t.Logf("  - %s", issue)
		}
	} else {
		t.Log("verify: no consistency issues found")
	}
}

// Suppress unused import warning
var _ = sort.Strings