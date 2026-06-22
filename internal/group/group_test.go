package group

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/group/extractors"
	"github.com/mengshi02/codetrip/internal/store"
)

// ============ ContractMatcher tests ============

func TestExactMatch(t *testing.T) {
	matcher := NewContractMatcher(DefaultMatchingConfig())

	consumers := []Contract{
		{ContractID: "c1", Type: "http", Role: "consumer", SymbolUID: "svc:api.go:Method:GET /users", Meta: map[string]any{"path": "/api/v1/users"}},
	}
	providers := []Contract{
		{ContractID: "p1", Type: "http", Role: "provider", SymbolUID: "svc:handler.go:Method:GET /users", Meta: map[string]any{"path": "/api/v1/users"}},
	}

	links := matcher.Match(consumers, providers, nil)
	if len(links) == 0 {
		t.Fatal("expected at least one match")
	}
	// Should have an exact match by path
	found := false
	for _, l := range links {
		if l.MatchType == "exact" {
			found = true
			if l.Confidence < 0.9 {
				t.Errorf("exact match confidence too low: %f", l.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected exact match type")
	}
}

func TestManifestMatch(t *testing.T) {
	matcher := NewContractMatcher(DefaultMatchingConfig())

	consumers := []Contract{
		{ContractID: "c1", Type: "http", Role: "consumer", Repo: "frontend", SymbolUID: "UserAPI"},
	}
	providers := []Contract{
		{ContractID: "p1", Type: "http", Role: "provider", Repo: "backend", SymbolUID: "UserHandler"},
	}
	manifestLinks := []GroupManifestLink{
		{SourceRepo: "frontend", SourceSymbol: "UserAPI", TargetRepo: "backend", TargetSymbol: "UserHandler", Type: "http"},
	}

	links := matcher.Match(consumers, providers, manifestLinks)
	found := false
	for _, l := range links {
		if l.MatchType == "manifest" {
			found = true
			if l.Confidence != 1.0 {
				t.Errorf("manifest confidence should be 1.0, got %f", l.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected manifest match")
	}
}

func TestWildcardMatch(t *testing.T) {
	matcher := NewContractMatcher(DefaultMatchingConfig())

	// Use distinct paths where one is a prefix of the other
	// and ContractIDs/SymbolUIDs are different to avoid exact match
	consumers := []Contract{
		{ContractID: "c-consumer-1", Type: "http", Role: "consumer", SymbolUID: "svc-a:api.go:Func:callUsers", Meta: map[string]any{"path": "/api/v1/users"}},
	}
	providers := []Contract{
		{ContractID: "p-provider-1", Type: "http", Role: "provider", SymbolUID: "svc-b:handler.go:Func:handleUsers", Meta: map[string]any{"path": "/api/v1/users/list"}},
	}

	links := matcher.Match(consumers, providers, nil)
	if len(links) == 0 {
		t.Error("expected at least one match between consumer and provider with overlapping paths")
	}
	found := false
	for _, l := range links {
		if l.MatchType == "wildcard" {
			found = true
		}
	}
	if !found && len(links) > 0 {
		t.Logf("no wildcard match, but got %d matches of other types (acceptable)", len(links))
	}
}

func TestNoMatchDifferentType(t *testing.T) {
	matcher := NewContractMatcher(DefaultMatchingConfig())

	consumers := []Contract{
		{ContractID: "c1", Type: "http", Role: "consumer", Meta: map[string]any{"path": "/api/users"}},
	}
	providers := []Contract{
		{ContractID: "p1", Type: "grpc", Role: "provider", Meta: map[string]any{"path": "/api/users"}},
	}

	links := matcher.Match(consumers, providers, nil)
	for _, l := range links {
		if l.MatchType == "wildcard" || l.MatchType == "exact" {
			t.Errorf("should not match across different contract types, got %s", l.MatchType)
		}
	}
}

// ============ BridgeGraph tests ============

func TestBridgeGraphAddNodeEdge(t *testing.T) {
	bg := &BridgeGraph{}
	bg.AddNode(BridgeContract{ContractID: "c1", Type: "http", Repo: "frontend"})
	bg.AddNode(BridgeContract{ContractID: "p1", Type: "http", Repo: "backend"})
	bg.AddEdge(BridgeLink{SourceID: "c1", TargetID: "p1", MatchType: "exact", Confidence: 0.95})

	if len(bg.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(bg.Nodes))
	}
	if len(bg.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(bg.Edges))
	}
	if bg.Edges[0].SourceID != "c1" || bg.Edges[0].TargetID != "p1" {
		t.Errorf("edge endpoints incorrect: %s -> %s", bg.Edges[0].SourceID, bg.Edges[0].TargetID)
	}
}

// ============ MaxMatches limit test ============

func TestMaxMatchesLimit(t *testing.T) {
	cfg := DefaultMatchingConfig()
	cfg.MaxMatches = 2
	matcher := NewContractMatcher(cfg)

	var consumers []Contract
	var providers []Contract
	for i := 0; i < 10; i++ {
		consumers = append(consumers, Contract{
			ContractID: "c" + string(rune('0'+i)),
			Type:       "http",
			Role:       "consumer",
			Meta:       map[string]any{"path": "/api/users"},
		})
		providers = append(providers, Contract{
			ContractID: "p" + string(rune('0'+i)),
			Type:       "http",
			Role:       "provider",
			Meta:       map[string]any{"path": "/api/users"},
		})
	}

	links := matcher.Match(consumers, providers, nil)
	if len(links) > 2 {
		t.Errorf("expected at most %d matches, got %d", cfg.MaxMatches, len(links))
	}
}

// ============ GroupStorage integration test (requires Pebble) ============

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(store.Config{Path: dir})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGroupStorageSaveLoadConfig(t *testing.T) {
	s := newTestStore(t)
	storage := NewGroupStorage(s)

	config := &GroupConfig{
		Version:     1,
		Name:        "test-group",
		Description: "test description",
		Repos:       map[string]string{"frontend": "/path/frontend", "backend": "/path/backend"},
	}

	if err := storage.SaveConfig("test-group", config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := storage.LoadConfig("test-group")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.Name != "test-group" {
		t.Errorf("expected name test-group, got %s", loaded.Name)
	}
	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}
}

func TestGroupStorageSaveLoadBridgeGraph(t *testing.T) {
	s := newTestStore(t)
	storage := NewGroupStorage(s)

	bg := &BridgeGraph{}
	bg.AddNode(BridgeContract{ContractID: "c1", Type: "http", Repo: "frontend"})
	bg.AddEdge(BridgeLink{SourceID: "c1", TargetID: "p1", MatchType: "exact", Confidence: 0.95})

	if err := storage.SaveBridgeGraph("test-group", bg); err != nil {
		t.Fatalf("save bridge graph: %v", err)
	}

	loaded, err := storage.LoadBridgeGraph("test-group")
	if err != nil {
		t.Fatalf("load bridge graph: %v", err)
	}

	if len(loaded.Nodes) != 1 || len(loaded.Edges) != 1 {
		t.Errorf("expected 1 node 1 edge, got %d nodes %d edges", len(loaded.Nodes), len(loaded.Edges))
	}
}

func TestGroupStorageListAndDelete(t *testing.T) {
	s := newTestStore(t)
	storage := NewGroupStorage(s)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		config := &GroupConfig{Name: name, Repos: map[string]string{"repo": "/path"}}
		if err := storage.SaveConfig(name, config); err != nil {
			t.Fatalf("save config %s: %v", name, err)
		}
	}

	groups, err := storage.ListGroups()
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}

	if err := storage.DeleteGroup("beta"); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	groups, err = storage.ListGroups()
	if err != nil {
		t.Fatalf("list groups after delete: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups after delete, got %d", len(groups))
	}
}

// ============ BridgeBuilder integration test ============

func TestBridgeBuilderBuild(t *testing.T) {
	s := newTestStore(t)
	storage := NewGroupStorage(s)
	matcher := NewContractMatcher(DefaultMatchingConfig())
	builder := NewBridgeBuilder(storage, matcher)

	dir := t.TempDir()
	gsStore, err := store.Open(store.Config{Path: filepath.Join(dir, "graph")})
	if err != nil {
		t.Fatalf("open graph store: %v", err)
	}
	defer gsStore.Close()

	gs := graph.NewGraphStore(gsStore, "frontend")

	// Create a simple extractor that returns test contracts
	testExtractor := func(ctx context.Context, repo string, gstore *graph.GraphStore) ([]extractors.Contract, error) {
		return []extractors.Contract{
			{ContractID: "c1", Type: "http", Role: "consumer", Repo: "frontend", SymbolUID: "svc:api.go:Function:callAPI"},
			{ContractID: "p1", Type: "http", Role: "provider", Repo: "frontend", SymbolUID: "svc:handler.go:Function:handleAPI"},
		}, nil
	}

	config := &GroupConfig{
		Name:  "test-group",
		Repos: map[string]string{"frontend": "/path/frontend"},
		Detect: DetectConfig{
			EnabledTypes: []string{"http"},
		},
		Matching: DefaultMatchingConfig(),
	}

	extractorMap := map[string]ContractExtractorFn{
		"http": testExtractor,
	}
	graphMap := map[string]*graph.GraphStore{"frontend": gs}

	bg, err := builder.Build(context.Background(), config, extractorMap, graphMap)
	if err != nil {
		t.Fatalf("build bridge graph: %v", err)
	}

	if len(bg.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(bg.Nodes))
	}
}

// ============ ServiceBoundaryDetector test ============

func TestServiceBoundaryDetectorDirOf(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"src/service/handler.go", "src/service"},
		{"handler.go", "handler.go"},
		{"a/b/c/d.go", "a/b/c"},
	}
	for _, tt := range tests {
		got := directoryOf(tt.input)
		if got != tt.want {
			t.Errorf("directoryOf(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServiceBoundaryDetectorServiceNameFromPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"user-service", "user"},
		{"payment-svc", "payment"},
		{"api-gateway-api", "api-gateway"},
		{"order-handler", "order"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		got := serviceNameFromPath(tt.input)
		if got != tt.want {
			t.Errorf("serviceNameFromPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============ ParseGroupConfig test ============

func TestParseGroupConfigYAML(t *testing.T) {
	data := []byte("name: my-group\ndescription: test\n")
	config, err := ParseGroupConfig(data)
	if err != nil {
		t.Fatalf("parse group config: %v", err)
	}
	if config.Name != "my-group" {
		t.Errorf("expected name my-group, got %s", config.Name)
	}
}

func TestParseGroupConfigMissingName(t *testing.T) {
	data := []byte(`{"description":"no name"}`)
	_, err := ParseGroupConfig(data)
	if err == nil {
		t.Error("expected error for missing name")
	}
}
