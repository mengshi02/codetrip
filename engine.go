package codetrip

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/mengshi02/codetrip/internal/export/csv"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingest"
	"github.com/mengshi02/codetrip/internal/search"
	"github.com/mengshi02/codetrip/internal/search/semantic"
	"github.com/mengshi02/codetrip/internal/search/source"
	"github.com/mengshi02/codetrip/internal/search/symbol"
	"github.com/mengshi02/codetrip/internal/store"
)

// Engine owns the durable code graph and its repository-scoped graph stores.
// Parsing is delegated exclusively to internal/ingest.
type Engine struct {
	store          *store.Store
	eDir        string
	opts           options
	mu             sync.RWMutex
	graphs         map[string]*graph.GraphStore
	lexical        map[string]*symbol.LexicalIndex
	vectors        map[string]*semantic.VectorSearch
	sources        map[string]*source.Index
	indexing       map[string]struct{}
	retiredLexical []*symbol.LexicalIndex
	retiredVectors []*semantic.VectorSearch
	retiredSources []*source.Index
	indexSem       chan struct{}
	metrics        Metrics
}

// IndexResult summarizes one completed repository snapshot.
type IndexResult struct {
	Repo     string  `json:"repo"`
	Files    int     `json:"files"`
	Nodes    int     `json:"nodes"`
	Edges    int     `json:"edges"`
	Duration float64 `json:"duration"`
	CSVPath  string  `json:"csvPath,omitempty"`
}

// RepoInfo describes a repository available in the opened Engine directory.
type RepoInfo struct {
	Name string `json:"name"`
}

// CSVManifest describes a complete persisted graph export.
type CSVManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Repository    string                 `json:"repository"`
	NodeCount     int                    `json:"nodeCount"`
	EdgeCount     int                    `json:"edgeCount"`
	Files         map[string]CSVFileInfo `json:"files"`
}

// CSVFileInfo contains integrity metadata for one exported CSV file.
type CSVFileInfo struct {
	SHA256 string `json:"sha256"`
	Rows   int    `json:"rows"`
}

// Open opens or creates a codee data directory.
func Open(dir string, opts ...Option) (*Engine, error) {
	if dir == "" {
		return nil, fmt.Errorf("e directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create e directory: %w", err)
	}

	configuration := defaultOptions()
	for _, option := range opts {
		option(&configuration)
	}
	storeConfig := store.DefaultConfig(filepath.Join(dir, "db"))
	storeConfig.CacheSize = configuration.cacheSize
	db, err := store.OpenWithScale(storeConfig, configuration.scalePreset)
	if err != nil {
		return nil, fmt.Errorf("open e store: %w", err)
	}

	e := &Engine{
		store:    db,
		eDir:  dir,
		opts:     configuration,
		graphs:   make(map[string]*graph.GraphStore),
		lexical:  make(map[string]*symbol.LexicalIndex),
		vectors:  make(map[string]*semantic.VectorSearch),
		sources:  make(map[string]*source.Index),
		indexing: make(map[string]struct{}),
	}
	if configuration.maxConcurrentIndex > 0 {
		e.indexSem = make(chan struct{}, configuration.maxConcurrentIndex)
	}
	_ = e.cleanupRetiredSnapshots()
	e.discoverRepos()
	return e, nil
}

func (e *Engine) discoverRepos() {
	activeFound := false
	_ = e.store.ScanPrefix([]byte("snapshot:active:"), func(key, value []byte) error {
		logical := strings.TrimPrefix(string(key), "snapshot:active:")
		physical := string(value)
		if logical != "" && physical != "" {
			e.graphs[logical] = e.newGraphStore(physical)
			activeFound = true
		}
		return nil
	})
	if activeFound {
		return
	}

	// Backward-compatible discovery for stores written before snapshot metadata.
	repositories := make(map[string]struct{})
	_ = e.store.ScanPrefix([]byte("n:"), func(key, _ []byte) error {
		parts := strings.SplitN(string(key), ":", 3)
		if len(parts) == 3 && parts[1] != "" {
			repositories[parts[1]] = struct{}{}
		}
		return nil
	})
	for repo := range repositories {
		e.graphs[repo] = e.newGraphStore(repo)
	}
}

func (e *Engine) newGraphStore(repo string) *graph.GraphStore {
	result := graph.NewGraphStore(e.store, repo)
	result.SetNodeCacheSize(e.opts.nodeCacheSize)
	result.SetTraversalLimit(e.opts.traversalLimit)
	return result
}

// Close flushes graph buffers and closes Pebble.
func (e *Engine) Close() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, graphStore := range e.graphs {
		if err := graphStore.Flush(); err != nil {
			return err
		}
	}
	for _, index := range e.lexical {
		if err := index.Close(); err != nil {
			return err
		}
	}
	for _, vector := range e.vectors {
		vector.Close()
	}
	for _, source := range e.sources {
		source.Close()
	}
	for _, index := range e.retiredLexical {
		_ = index.Close()
	}
	for _, vector := range e.retiredVectors {
		vector.Close()
	}
	for _, source := range e.retiredSources {
		source.Close()
	}
	if err := e.cleanupRetiredSnapshots(); err != nil {
		return err
	}
	return e.store.Close()
}

// Ping verifies that the underlying Pebble store is readable.
func (e *Engine) Ping() error {
	if e.store.Metrics() == nil {
		return fmt.Errorf("pebble metrics unavailable")
	}
	return nil
}

// IndexRepo parses a repository with the validated ingestion engine and
// persists the resulting graph in Pebble. CSV output is an optional diagnostic
// side effect and is never used as the storage transport.
func (e *Engine) IndexRepo(ctx context.Context, repoPath string, opts ...IndexOption) (*IndexResult, error) {
	e.metrics.IndexRepoTotal.Add(1)
	started := time.Now()

	info, err := os.Stat(repoPath)
	if err != nil || !info.IsDir() {
		e.metrics.IndexRepoFail.Add(1)
		return nil, fmt.Errorf("index repository %q: invalid directory", repoPath)
	}
	configuration := defaultIndexOptions()
	for _, option := range opts {
		option(&configuration)
	}
	repo := configuration.repoName
	if repo == "" {
		repo = filepath.Base(filepath.Clean(repoPath))
	}

	e.mu.Lock()
	_, exists := e.graphs[repo]
	_, busy := e.indexing[repo]
	if busy || (exists && !configuration.replace) {
		e.mu.Unlock()
		e.metrics.IndexRepoFail.Add(1)
		if busy {
			return nil, fmt.Errorf("repository %q is already being indexed", repo)
		}
		return nil, fmt.Errorf("%w: %s", ErrRepoAlreadyExists, repo)
	}
	e.indexing[repo] = struct{}{}
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.indexing, repo); e.mu.Unlock() }()

	if e.indexSem != nil {
		select {
		case e.indexSem <- struct{}{}:
			defer func() { <-e.indexSem }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	pipelineResult, err := ingest.NewPipeline(repoPath, "", false).Run()
	if err != nil {
		e.metrics.IndexRepoFail.Add(1)
		e.metrics.Errors.Add(1)
		return nil, fmt.Errorf("ingest repository: %w", err)
	}

	csvPath := ""
	if configuration.exportCSVPath != "" {
		if _, exportErr := csv.StreamAllCSVsToDisk(pipelineResult.Graph, repoPath, configuration.exportCSVPath); exportErr != nil {
			if configuration.exportStrict {
				e.metrics.IndexRepoFail.Add(1)
				return nil, fmt.Errorf("export validation CSV: %w", exportErr)
			}
			slog.Warn("CSV export failed", "repo", repo, "error", exportErr)
		} else {
			csvPath = configuration.exportCSVPath
		}
	}

	physicalRepo := fmt.Sprintf("%s@%x", repo, time.Now().UnixNano())
	graphStore := e.newGraphStore(physicalRepo)
	if err := graphStore.ImportKnowledgeGraph(pipelineResult.Graph); err != nil {
		_ = e.deleteGraphNamespace(physicalRepo)
		e.metrics.IndexRepoFail.Add(1)
		e.metrics.Errors.Add(1)
		return nil, fmt.Errorf("persist graph: %w", err)
	}
	if err := graphStore.Flush(); err != nil {
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("flush graph: %w", err)
	}
	// Use the same physical snapshot name for graph and text indexes. The
	// logical active pointer therefore publishes both as one version.
	index, err := symbol.NewLexicalIndexWithDir(e.eDir, physicalRepo, e.store)
	if err != nil {
		_ = e.deleteSearchNamespace(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("create symbol index: %w", err)
	}
	var symbols []*graph.Node
	if err := graphStore.ForEachNode(func(node *graph.Node) error {
		if node.Label.IsSymbol() {
			symbols = append(symbols, node)
		}
		return nil
	}); err != nil {
		index.Close()
		_ = e.deleteSearchNamespace(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("collect symbols: %w", err)
	}
	if err := index.BatchIndexChunked(symbols, 10000); err != nil {
		index.Close()
		_ = e.deleteSearchNamespace(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("index symbols: %w", err)
	}
	if err := index.FinalizeBuild(); err != nil {
		index.Close()
		_ = e.deleteSearchNamespace(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("publish symbol index: %w", err)
	}
	contentIndex := source.New(e.eDir, physicalRepo)
	if err := contentIndex.Build(repoPath, physicalRepo); err != nil {
		index.Close()
		_ = e.deleteSnapshotArtifacts(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("build source index: %w", err)
	}
	e.mu.RLock()
	oldGraph, oldLexical, oldVector, oldSource := e.graphs[repo], e.lexical[repo], e.vectors[repo], e.sources[repo]
	e.mu.RUnlock()
	oldPhysical := ""
	if oldGraph != nil {
		oldPhysical = oldGraph.Repo()
	}
	if err := e.store.Batch(func(batch *pebble.Batch) error {
		if err := batch.Set([]byte("snapshot:active:"+repo), []byte(physicalRepo), nil); err != nil {
			return err
		}
		if oldPhysical != "" && oldPhysical != physicalRepo {
			return batch.Set([]byte("snapshot:retired:"+oldPhysical), []byte(repo), nil)
		}
		return nil
	}); err != nil {
		index.Close()
		contentIndex.Close()
		_ = e.deleteSnapshotArtifacts(physicalRepo)
		_ = e.deleteGraphNamespace(physicalRepo)
		return nil, fmt.Errorf("publish snapshot: %w", err)
	}
	e.mu.Lock()
	e.graphs[repo] = graphStore
	e.lexical[repo] = index
	e.sources[repo] = contentIndex
	delete(e.vectors, repo)
	if oldLexical != nil {
		e.retiredLexical = append(e.retiredLexical, oldLexical)
	}
	if oldVector != nil {
		e.retiredVectors = append(e.retiredVectors, oldVector)
	}
	if oldSource != nil {
		e.retiredSources = append(e.retiredSources, oldSource)
	}
	e.mu.Unlock()

	e.metrics.IndexRepoSuccess.Add(1)
	return &IndexResult{
		Repo: repo, Files: pipelineResult.WalkResult.TotalFiles,
		Nodes: pipelineResult.Graph.NodeCount(), Edges: pipelineResult.Graph.RelationshipCount(),
		Duration: time.Since(started).Seconds(), CSVPath: csvPath,
	}, nil
}

type SearchRequest struct {
	Repo  string `json:"repo"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type SearchItem struct {
	NodeID    string  `json:"nodeId"`
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	FilePath  string  `json:"filePath"`
	StartLine int     `json:"startLine"`
	EndLine   int     `json:"endLine"`
	Score     float64 `json:"score"`
}

type SearchResult struct {
	Results []SearchItem `json:"results"`
}

// Search performs symbol-level lexical retrieval. Source substring/regex search
// is intentionally reserved for the Zoekt backend.
func (e *Engine) Search(_ context.Context, request *SearchRequest) (*SearchResult, error) {
	if request == nil || request.Repo == "" || request.Query == "" {
		return nil, ErrInvalidRequest
	}
	e.mu.RLock()
	index := e.lexical[request.Repo]
	e.mu.RUnlock()
	if index == nil {
		graphStore := e.graphStore(request.Repo)
		if graphStore == nil {
			return nil, fmt.Errorf("repository %q is not indexed", request.Repo)
		}
		var err error
		index, err = symbol.NewLexicalIndexWithDir(e.eDir, graphStore.Repo(), e.store)
		if err != nil {
			return nil, err
		}
		e.mu.Lock()
		e.lexical[request.Repo] = index
		e.mu.Unlock()
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	matches, err := index.Search(request.Query, limit)
	if err != nil {
		return nil, err
	}
	result := &SearchResult{Results: make([]SearchItem, 0, len(matches))}
	for _, match := range matches {
		result.Results = append(result.Results, SearchItem{
			NodeID: match.NodeID, Name: match.Name, Label: match.Label, FilePath: match.FilePath,
			StartLine: match.StartLine, EndLine: match.EndLine, Score: match.Score,
		})
	}
	return result, nil
}

func (e *Engine) deleteSearchNamespace(repo string) error {
	var firstErr error
	for _, path := range []string{
		filepath.Join(e.eDir, "index", repo),
		filepath.Join(e.eDir, "index", ".build", repo),
	} {
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Engine) deleteSnapshotArtifacts(repo string) error {
	firstErr := e.deleteSearchNamespace(repo)
	for _, path := range []string{
		filepath.Join(e.eDir, "content", repo),
		filepath.Join(e.eDir, "content", repo+".build"),
		filepath.Join(e.eDir, "vectors", repo),
		filepath.Join(e.eDir, "vectors", repo+".bin"),
	} {
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Engine) deleteGraphNamespace(repo string) error {
	prefixes := [][]byte{
		[]byte("n:" + repo + ":"),
		[]byte("e:" + repo + ":"),
		[]byte("type:" + repo + ":"),
		[]byte("name:" + repo + ":"),
		[]byte("file:" + repo + ":"),
		[]byte("adj:" + repo + ":"),
		[]byte("embdesc:" + repo + ":"),
		[]byte("embcode:" + repo + ":"),
		[]byte("embdescidx:" + repo),
		[]byte("embcodeidx:" + repo),
	}
	return e.store.Batch(func(batch *pebble.Batch) error {
		for _, prefix := range prefixes {
			if err := e.store.ScanPrefix(prefix, func(key, _ []byte) error {
				copied := append([]byte(nil), key...)
				return batch.Delete(copied, nil)
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Engine) cleanupRetiredSnapshots() error {
	var snapshots []string
	if err := e.store.ScanPrefix([]byte("snapshot:retired:"), func(key, _ []byte) error {
		snapshots = append(snapshots, strings.TrimPrefix(string(key), "snapshot:retired:"))
		return nil
	}); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if snapshot == "" {
			continue
		}
		if err := e.deleteGraphNamespace(snapshot); err != nil {
			return err
		}
		if err := e.deleteSnapshotArtifacts(snapshot); err != nil {
			return err
		}
		if err := e.store.Delete([]byte("snapshot:retired:" + snapshot)); err != nil {
			return err
		}
	}
	return nil
}

// graphStore returns the active internal graph store for a repository.
func (e *Engine) graphStore(repo string) *graph.GraphStore {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.graphs[repo]
}

func (e *Engine) ListRepos() ([]RepoInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]RepoInfo, 0, len(e.graphs))
	for repo := range e.graphs {
		result = append(result, RepoInfo{Name: repo})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// ExportFullCSV exports the active persisted snapshot for storage inspection.
func (e *Engine) ExportFullCSV(repo, directory string) (*CSVManifest, error) {
	graphStore := e.graphStore(repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	manifest, err := csv.ExportFull(graphStore, directory)
	if err != nil {
		return nil, err
	}
	result := &CSVManifest{
		SchemaVersion: manifest.SchemaVersion,
		Repository:    manifest.Repository,
		NodeCount:     manifest.NodeCount,
		EdgeCount:     manifest.EdgeCount,
		Files:         make(map[string]CSVFileInfo, len(manifest.Files)),
	}
	for name, file := range manifest.Files {
		result.Files[name] = CSVFileInfo{SHA256: file.SHA256, Rows: file.Rows}
	}
	return result, nil
}

// TraverseDirection controls which side of an edge a traversal follows.
type TraverseDirection string

const (
	TraverseOutgoing TraverseDirection = "out"
	TraverseIncoming TraverseDirection = "in"
	TraverseAny      TraverseDirection = "both"
)

// TraverseRequest describes a bounded breadth-first graph traversal.
type TraverseRequest struct {
	Repo          string            `json:"repo"`
	StartNodeID   string            `json:"startNodeId"`
	Direction     TraverseDirection `json:"direction"`
	MaxDepth      int               `json:"maxDepth"`
	RelationTypes []string          `json:"relationTypes,omitempty"`
}

// GraphNode is the stable public representation of a persisted graph node.
type GraphNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Name        string `json:"name"`
	FilePath    string `json:"filePath,omitempty"`
	Language    string `json:"language,omitempty"`
	StartLine   int    `json:"startLine,omitempty"`
	EndLine     int    `json:"endLine,omitempty"`
	Description string `json:"description,omitempty"`
}

type TraverseResult struct {
	Nodes []GraphNode `json:"nodes"`
}

type PathRequest struct {
	Repo         string `json:"repo"`
	SourceNodeID string `json:"sourceNodeId"`
	TargetNodeID string `json:"targetNodeId"`
}

type GraphEdge struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

type PathResult struct {
	Edges []GraphEdge `json:"edges"`
}

// Traverse performs repository-scoped BFS. Results exclude the start node.
func (e *Engine) Traverse(ctx context.Context, request *TraverseRequest) (*TraverseResult, error) {
	if request == nil || request.Repo == "" || request.StartNodeID == "" || request.MaxDepth < 1 {
		return nil, ErrInvalidRequest
	}
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	direction, err := internalDirection(request.Direction)
	if err != nil {
		return nil, err
	}

	allowed := make(map[string]struct{}, len(request.RelationTypes))
	for _, relation := range request.RelationTypes {
		relation = strings.TrimSpace(relation)
		if relation != "" {
			allowed[relation] = struct{}{}
		}
	}
	var filter graph.EdgeFilter
	if len(allowed) > 0 {
		filter = func(edge *graph.Edge) bool {
			_, ok := allowed[string(edge.Type)]
			return ok
		}
	}

	nodes, err := graphStore.BFS(ctx, request.StartNodeID, direction, request.MaxDepth, filter)
	if err != nil {
		return nil, err
	}
	result := &TraverseResult{Nodes: make([]GraphNode, 0, len(nodes))}
	for _, node := range nodes {
		result.Nodes = append(result.Nodes, publicGraphNode(node))
	}
	return result, nil
}

// ShortestPath returns the shortest directed path between two persisted nodes.
func (e *Engine) ShortestPath(ctx context.Context, request *PathRequest) (*PathResult, error) {
	if request == nil || request.Repo == "" || request.SourceNodeID == "" || request.TargetNodeID == "" {
		return nil, ErrInvalidRequest
	}
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	edges, err := graphStore.ShortestPath(ctx, request.SourceNodeID, request.TargetNodeID)
	if err != nil {
		return nil, err
	}
	result := &PathResult{Edges: make([]GraphEdge, 0, len(edges))}
	for _, edge := range edges {
		result.Edges = append(result.Edges, GraphEdge{
			ID: edge.ID, Source: edge.Source, Target: edge.Target, Type: string(edge.Type),
			Confidence: edge.Confidence(), Reason: edge.GetPropString("reason"),
		})
	}
	return result, nil
}

func internalDirection(direction TraverseDirection) (graph.TraverseDir, error) {
	switch direction {
	case "", TraverseOutgoing:
		return graph.TraverseOut, nil
	case TraverseIncoming:
		return graph.TraverseIn, nil
	case TraverseAny:
		return graph.TraverseBoth, nil
	default:
		return graph.TraverseOut, fmt.Errorf("%w: unsupported traversal direction %q", ErrInvalidRequest, direction)
	}
}

func publicGraphNode(node *graph.Node) GraphNode {
	return GraphNode{
		ID: node.ID, Label: string(node.Label), Name: node.Name, FilePath: node.FilePath,
		Language: node.GetPropString("language"), StartLine: node.GetPropInt("startLine"),
		EndLine: node.GetPropInt("endLine"), Description: node.GetPropString("description"),
	}
}

type SourceSearchRequest struct {
	Repo         string `json:"repo"`
	Query        string `json:"query"`
	Limit        int    `json:"limit"`
	ContextLines int    `json:"contextLines"`
}

type SourceMatch struct {
	FilePath string  `json:"filePath"`
	Language string  `json:"language,omitempty"`
	Line     int     `json:"line"`
	Content  string  `json:"content"`
	Before   string  `json:"before,omitempty"`
	After    string  `json:"after,omitempty"`
	Score    float64 `json:"score"`
}

type SourceSearchResult struct {
	Results []SourceMatch `json:"results"`
}

// SearchSource searches file names and source contents in the active snapshot.
// Query supports literal, regular-expression, file, and language filters.
func (e *Engine) SearchSource(ctx context.Context, request *SourceSearchRequest) (*SourceSearchResult, error) {
	if request == nil || request.Repo == "" || request.Query == "" {
		return nil, ErrInvalidRequest
	}
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	e.mu.RLock()
	index := e.sources[request.Repo]
	e.mu.RUnlock()
	if index == nil {
		index = source.New(e.eDir, graphStore.Repo())
		if err := index.Open(); err != nil {
			return nil, err
		}
		e.mu.Lock()
		e.sources[request.Repo] = index
		e.mu.Unlock()
	}
	matches, err := index.Search(ctx, request.Query, request.Limit, request.ContextLines)
	if err != nil {
		return nil, err
	}
	result := &SourceSearchResult{Results: make([]SourceMatch, 0, len(matches))}
	for _, match := range matches {
		result.Results = append(result.Results, SourceMatch{FilePath: match.FilePath, Language: match.Language, Line: match.Line, Content: match.Content, Before: match.Before, After: match.After, Score: match.Score})
	}
	return result, nil
}

// Embedder converts text batches into vectors. Implementations may call a
// local model, a remote service, or an in-process model.
type Embedder interface {
	Dimensions() int
	Embed(context.Context, []string) ([][]float32, error)
}

// HTTPEmbedder calls an OpenAI-compatible embeddings endpoint while keeping
// the internal embedding pipeline out of the public API.
type HTTPEmbedder struct {
	client *semantic.HTTPEmbedder
}

func NewHTTPEmbedder(endpoint, model, apiKey string, dimensions int) *HTTPEmbedder {
	return &HTTPEmbedder{client: semantic.NewHTTPEmbedder(endpoint, model, apiKey, dimensions)}
}

func (embedder *HTTPEmbedder) Dimensions() int {
	return embedder.client.Dimensions()
}

func (embedder *HTTPEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return embedder.client.Embed(ctx, texts)
}

type EmbedOptions struct {
	BatchSize        int
	SubBatchSize     int
	MaxSnippetLength int
	ChunkSize        int
	Overlap          int
	QuantizeInt8     bool
}

type EmbedResult struct {
	Repo          string `json:"repo"`
	NodesEmbedded int    `json:"nodesEmbedded"`
	ChunksCreated int    `json:"chunksCreated"`
	DescChunks    int    `json:"descChunks"`
	CodeChunks    int    `json:"codeChunks"`
	Errors        int    `json:"errors"`
}

// EmbedRepo builds dual-modal description/code vectors and HNSW indexes for
// the active physical snapshot of a logical repository.
func (e *Engine) EmbedRepo(ctx context.Context, repo string, embedder Embedder, opts *EmbedOptions) (*EmbedResult, error) {
	if repo == "" || embedder == nil || embedder.Dimensions() <= 0 {
		return nil, ErrInvalidRequest
	}
	graphStore := e.graphStore(repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	config := semantic.DefaultEmbedConfig()
	config.Dimensions = embedder.Dimensions()
	quantized := false
	if opts != nil {
		if opts.BatchSize > 0 {
			config.BatchSize = opts.BatchSize
		}
		if opts.SubBatchSize > 0 {
			config.SubBatchSize = opts.SubBatchSize
		}
		if opts.MaxSnippetLength > 0 {
			config.MaxSnippetLength = opts.MaxSnippetLength
		}
		if opts.ChunkSize > 0 {
			config.ChunkSize = opts.ChunkSize
		}
		if opts.Overlap >= 0 && opts.ChunkSize > 0 {
			config.Overlap = opts.Overlap
		}
		quantized = opts.QuantizeInt8
	}
	physicalRepo := graphStore.Repo()
	pipeline := semantic.NewEmbeddingPipelineWithDir(embedder, graphStore, e.store, config, e.eDir, quantized)
	pipelineResult, err := pipeline.RunDualModal(ctx, physicalRepo)
	if err != nil {
		return nil, fmt.Errorf("embed repository: %w", err)
	}
	vector := semantic.NewVectorSearchWithDir(embedder, e.store, graphStore, e.eDir)
	if quantized {
		if err := vector.LoadVectorFile(); err != nil {
			vector.Close()
			return nil, err
		}
		vector.SetTwoStageSearch(true)
	}
	if err := vector.BuildSemanticIndex(); err != nil {
		vector.Close()
		return nil, fmt.Errorf("build vector index: %w", err)
	}
	e.mu.Lock()
	if previous := e.vectors[repo]; previous != nil {
		previous.Close()
	}
	e.vectors[repo] = vector
	e.mu.Unlock()
	return &EmbedResult{
		Repo: repo, NodesEmbedded: pipelineResult.NodesEmbedded, ChunksCreated: pipelineResult.ChunksCreated,
		DescChunks: pipelineResult.DescChunks, CodeChunks: pipelineResult.CodeChunks,
		Errors: pipelineResult.Errors,
	}, nil
}

// AttachEmbedder restores a persisted vector index after Open and associates
// it with the query embedder used by HybridSearch.
func (e *Engine) AttachEmbedder(repo string, embedder Embedder) error {
	if repo == "" || embedder == nil || embedder.Dimensions() <= 0 {
		return ErrInvalidRequest
	}
	graphStore := e.graphStore(repo)
	if graphStore == nil {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	vector := semantic.NewVectorSearchWithDir(embedder, e.store, graphStore, e.eDir)
	_ = vector.LoadVectorFile()
	if !vector.RestoreSemanticIndex() {
		if err := vector.BuildSemanticIndex(); err != nil {
			vector.Close()
			return err
		}
	}
	e.mu.Lock()
	if previous := e.vectors[repo]; previous != nil {
		previous.Close()
	}
	e.vectors[repo] = vector
	e.mu.Unlock()
	return nil
}

type HybridSearchRequest struct {
	Repo  string `json:"repo"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type HybridSearchItem struct {
	SearchItem
	Rank          int      `json:"rank"`
	Sources       []string `json:"sources"`
	LexicalScore  float64  `json:"lexicalScore,omitempty"`
	SemanticScore float64  `json:"semanticScore,omitempty"`
}

type HybridSearchResult struct {
	Results []HybridSearchItem `json:"results"`
}

// HybridSearch fuses lexical and dual-modal HNSW rankings with RRF. EmbedRepo or
// AttachEmbedder must be called first for the repository.
func (e *Engine) HybridSearch(ctx context.Context, request *HybridSearchRequest) (*HybridSearchResult, error) {
	if request == nil || request.Repo == "" || request.Query == "" {
		return nil, ErrInvalidRequest
	}
	e.mu.RLock()
	lexical, vector := e.lexical[request.Repo], e.vectors[request.Repo]
	e.mu.RUnlock()
	if vector == nil {
		return nil, fmt.Errorf("vector index for repository %q is not attached", request.Repo)
	}
	if lexical == nil {
		graphStore := e.graphStore(request.Repo)
		if graphStore == nil {
			return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
		}
		var err error
		lexical, err = symbol.NewLexicalIndexWithDir(e.eDir, graphStore.Repo(), e.store)
		if err != nil {
			return nil, err
		}
		e.mu.Lock()
		e.lexical[request.Repo] = lexical
		e.mu.Unlock()
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	result, err := search.NewHybridSearch(lexical, vector).SearchDualModal(ctx, request.Query, limit)
	if err != nil {
		return nil, err
	}
	output := &HybridSearchResult{Results: make([]HybridSearchItem, 0, len(result.Results))}
	for _, item := range result.Results {
		output.Results = append(output.Results, HybridSearchItem{
			SearchItem: SearchItem{NodeID: item.NodeID, Name: item.Name, Label: item.Label, FilePath: item.FilePath,
				StartLine: item.StartLine, EndLine: item.EndLine, Score: item.Score},
			Rank: item.Rank, Sources: item.Sources, LexicalScore: item.LexicalScore, SemanticScore: item.SemanticScore,
		})
	}
	return output, nil
}
