package codetrip

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
	eDir           string
	opts           options
	mu             sync.RWMutex
	stores         map[string]*store.Store
	repoDirs       map[string]string
	graphs         map[string]*graph.GraphStore
	lexical        map[string]*symbol.LexicalIndex
	vectors        map[string]*semantic.VectorSearch
	sources        map[string]*source.Index
	indexing       map[string]struct{}
	repoOps        map[string]*sync.RWMutex
	retiredLexical map[string][]*symbol.LexicalIndex
	retiredVectors map[string][]*semantic.VectorSearch
	retiredSources map[string][]*source.Index
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

type repositoryManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	SourcePath    string `json:"sourcePath,omitempty"`
}

const repositorySchemaVersion = 1

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

// Open opens or creates a Codetrip data directory. Repository databases are
// isolated under repos/<stable-id>/graph/db and opened independently.
func Open(dir string, opts ...Option) (*Engine, error) {
	if dir == "" {
		return nil, fmt.Errorf("e directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create engine directory: %w", err)
	}

	configuration := defaultOptions()
	for _, option := range opts {
		option(&configuration)
	}
	for _, directory := range []string{filepath.Join(dir, "repos"), filepath.Join(dir, "trash")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create engine directory: %w", err)
		}
	}

	e := &Engine{
		eDir:           dir,
		opts:           configuration,
		stores:         make(map[string]*store.Store),
		repoDirs:       make(map[string]string),
		graphs:         make(map[string]*graph.GraphStore),
		lexical:        make(map[string]*symbol.LexicalIndex),
		vectors:        make(map[string]*semantic.VectorSearch),
		sources:        make(map[string]*source.Index),
		indexing:       make(map[string]struct{}),
		repoOps:        make(map[string]*sync.RWMutex),
		retiredLexical: make(map[string][]*symbol.LexicalIndex),
		retiredVectors: make(map[string][]*semantic.VectorSearch),
		retiredSources: make(map[string][]*source.Index),
	}
	if configuration.maxConcurrentIndex > 0 {
		e.indexSem = make(chan struct{}, configuration.maxConcurrentIndex)
	}
	if err := e.cleanupTrash(); err != nil {
		return nil, err
	}
	if err := e.discoverRepos(); err != nil {
		_ = e.Close()
		return nil, err
	}
	return e, nil
}

func (e *Engine) discoverRepos() error {
	entries, err := os.ReadDir(filepath.Join(e.eDir, "repos"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(e.eDir, "repos", entry.Name())
		manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
		if err != nil {
			return fmt.Errorf("read repository manifest %s: %w", root, err)
		}
		var manifest repositoryManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil || manifest.SchemaVersion != repositorySchemaVersion || manifest.Name == "" {
			return fmt.Errorf("repository manifest %s is invalid or incompatible", filepath.Join(root, "manifest.json"))
		}
		if entry.Name() != repositoryID(manifest.Name) {
			return fmt.Errorf("repository manifest %s does not match its storage directory", filepath.Join(root, "manifest.json"))
		}
		e.repoDirs[manifest.Name] = root
	}
	return nil
}

func (e *Engine) ensureRepositoryOpen(repo string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stores[repo] != nil && e.graphs[repo] != nil {
		return nil
	}
	root := e.repoDirs[repo]
	if root == "" {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	db, err := e.openRepositoryStore(root)
	if err != nil {
		return fmt.Errorf("open repository %q: %w", repo, err)
	}
	var physical string
	if err := db.ScanPrefix([]byte("snapshot:active:"), func(_, value []byte) error {
		physical = string(value)
		return nil
	}); err != nil || physical == "" {
		_ = db.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: %s has no active snapshot", ErrRepoNotIndexed, repo)
	}
	if err := e.cleanupRetiredSnapshots(db, root); err != nil {
		_ = db.Close()
		return err
	}
	e.stores[repo] = db
	e.graphs[repo] = e.newGraphStore(db, physical)
	return nil
}

func (e *Engine) openRepositoryStore(root string) (*store.Store, error) {
	configuration := store.DefaultConfig(filepath.Join(root, "graph", "db"))
	configuration.CacheSize = e.opts.cacheSize
	return store.OpenWithScale(configuration, e.opts.scalePreset)
}

func (e *Engine) newGraphStore(db *store.Store, repo string) *graph.GraphStore {
	result := graph.NewGraphStore(db, repo)
	result.SetNodeCacheSize(e.opts.nodeCacheSize)
	result.SetTraversalLimit(e.opts.traversalLimit)
	return result
}

// Close flushes graph buffers and closes Pebble.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
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
	for _, indexes := range e.retiredLexical {
		for _, index := range indexes {
			_ = index.Close()
		}
	}
	for _, vectors := range e.retiredVectors {
		for _, vector := range vectors {
			vector.Close()
		}
	}
	for _, sources := range e.retiredSources {
		for _, source := range sources {
			source.Close()
		}
	}
	for repo, db := range e.stores {
		if err := e.cleanupRetiredSnapshots(db, e.repoDirs[repo]); err != nil {
			return err
		}
		if err := db.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Ping verifies that the underlying Pebble store is readable.
func (e *Engine) Ping() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for repo, db := range e.stores {
		if db.Metrics() == nil {
			return fmt.Errorf("repository %q metrics unavailable", repo)
		}
	}
	return nil
}

func repositoryID(name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	return hex.EncodeToString(sum[:16])
}

func (e *Engine) repositoryRoot(name string) string {
	return filepath.Join(e.eDir, "repos", repositoryID(name))
}

func writeRepositoryManifest(root string, manifest repositoryManifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	temporary := filepath.Join(root, "manifest.json.tmp")
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(root, "manifest.json"))
}

func (e *Engine) cleanupTrash() error {
	entries, err := os.ReadDir(filepath.Join(e.eDir, "trash"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(e.eDir, "trash", entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) repositoryOperationLock(repo string) *sync.RWMutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	lock := e.repoOps[repo]
	if lock == nil {
		lock = &sync.RWMutex{}
		e.repoOps[repo] = lock
	}
	return lock
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
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, ErrInvalidRequest
	}
	operation := e.repositoryOperationLock(repo)
	operation.RLock()
	defer operation.RUnlock()

	e.mu.Lock()
	_, exists := e.repoDirs[repo]
	_, busy := e.indexing[repo]
	db := e.stores[repo]
	repoRoot := e.repoDirs[repo]
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
	if exists {
		if err := e.ensureRepositoryOpen(repo); err != nil {
			return nil, err
		}
		e.mu.RLock()
		db, repoRoot = e.stores[repo], e.repoDirs[repo]
		e.mu.RUnlock()
	}

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
	newRepository := db == nil
	if newRepository {
		repoRoot = e.repositoryRoot(repo)
		if err := os.RemoveAll(repoRoot); err != nil {
			return nil, fmt.Errorf("prepare repository storage: %w", err)
		}
		if err := os.MkdirAll(repoRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create repository storage: %w", err)
		}
		db, err = e.openRepositoryStore(repoRoot)
		if err != nil {
			_ = os.RemoveAll(repoRoot)
			return nil, fmt.Errorf("open repository store: %w", err)
		}
		defer func() {
			if newRepository {
				_ = db.Close()
				_ = os.RemoveAll(repoRoot)
			}
		}()
	}

	physicalRepo := fmt.Sprintf("snapshot@%x", time.Now().UnixNano())
	graphStore := e.newGraphStore(db, physicalRepo)
	if err := graphStore.ImportKnowledgeGraph(pipelineResult.Graph); err != nil {
		_ = e.deleteGraphNamespace(db, physicalRepo)
		e.metrics.IndexRepoFail.Add(1)
		e.metrics.Errors.Add(1)
		return nil, fmt.Errorf("persist graph: %w", err)
	}
	if err := graphStore.Flush(); err != nil {
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("flush graph: %w", err)
	}
	// Use the same physical snapshot name for graph and text indexes. The
	// logical active pointer therefore publishes both as one version.
	index, err := symbol.NewLexicalIndexWithDir(repoRoot, physicalRepo, db)
	if err != nil {
		_ = e.deleteSearchNamespace(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
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
		_ = e.deleteSearchNamespace(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("collect symbols: %w", err)
	}
	if err := index.BatchIndexChunked(symbols, 10000); err != nil {
		index.Close()
		_ = e.deleteSearchNamespace(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("index symbols: %w", err)
	}
	if err := index.FinalizeBuild(); err != nil {
		index.Close()
		_ = e.deleteSearchNamespace(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("publish symbol index: %w", err)
	}
	contentIndex := source.New(repoRoot, physicalRepo)
	if err := contentIndex.Build(repoPath, physicalRepo); err != nil {
		index.Close()
		_ = e.deleteSnapshotArtifacts(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("build source index: %w", err)
	}
	e.mu.RLock()
	oldGraph, oldLexical, oldVector, oldSource := e.graphs[repo], e.lexical[repo], e.vectors[repo], e.sources[repo]
	e.mu.RUnlock()
	oldPhysical := ""
	if oldGraph != nil {
		oldPhysical = oldGraph.Repo()
	}
	if err := db.Batch(func(batch *pebble.Batch) error {
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
		_ = e.deleteSnapshotArtifacts(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("publish snapshot: %w", err)
	}
	if err := writeRepositoryManifest(repoRoot, repositoryManifest{SchemaVersion: repositorySchemaVersion, Name: repo, SourcePath: repoPath}); err != nil {
		index.Close()
		contentIndex.Close()
		_ = db.Batch(func(batch *pebble.Batch) error {
			if oldPhysical == "" {
				if err := batch.Delete([]byte("snapshot:active:"+repo), nil); err != nil {
					return err
				}
			} else {
				if err := batch.Set([]byte("snapshot:active:"+repo), []byte(oldPhysical), nil); err != nil {
					return err
				}
				if err := batch.Delete([]byte("snapshot:retired:"+oldPhysical), nil); err != nil {
					return err
				}
			}
			return nil
		})
		_ = e.deleteSnapshotArtifacts(repoRoot, physicalRepo)
		_ = e.deleteGraphNamespace(db, physicalRepo)
		return nil, fmt.Errorf("publish repository manifest: %w", err)
	}
	e.mu.Lock()
	e.stores[repo] = db
	e.repoDirs[repo] = repoRoot
	e.graphs[repo] = graphStore
	e.lexical[repo] = index
	e.sources[repo] = contentIndex
	delete(e.vectors, repo)
	if oldLexical != nil {
		e.retiredLexical[repo] = append(e.retiredLexical[repo], oldLexical)
	}
	if oldVector != nil {
		e.retiredVectors[repo] = append(e.retiredVectors[repo], oldVector)
	}
	if oldSource != nil {
		e.retiredSources[repo] = append(e.retiredSources[repo], oldSource)
	}
	e.mu.Unlock()
	newRepository = false

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
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
	e.mu.RLock()
	index := e.lexical[request.Repo]
	e.mu.RUnlock()
	if index == nil {
		graphStore := e.graphStore(request.Repo)
		if graphStore == nil {
			return nil, fmt.Errorf("repository %q is not indexed", request.Repo)
		}
		var err error
		e.mu.RLock()
		db, repoRoot := e.stores[request.Repo], e.repoDirs[request.Repo]
		e.mu.RUnlock()
		index, err = symbol.NewLexicalIndexWithDir(repoRoot, graphStore.Repo(), db)
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

func (e *Engine) deleteSearchNamespace(repoRoot, repo string) error {
	var firstErr error
	for _, path := range []string{
		filepath.Join(repoRoot, "index", repo),
		filepath.Join(repoRoot, "index", ".build", repo),
	} {
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Engine) deleteSnapshotArtifacts(repoRoot, repo string) error {
	firstErr := e.deleteSearchNamespace(repoRoot, repo)
	for _, path := range []string{
		filepath.Join(repoRoot, "content", repo),
		filepath.Join(repoRoot, "content", repo+".build"),
		filepath.Join(repoRoot, "vectors", repo),
		filepath.Join(repoRoot, "vectors", repo+".bin"),
	} {
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Engine) deleteGraphNamespace(db *store.Store, repo string) error {
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
	return db.Batch(func(batch *pebble.Batch) error {
		for _, prefix := range prefixes {
			if err := db.ScanPrefix(prefix, func(key, _ []byte) error {
				copied := append([]byte(nil), key...)
				return batch.Delete(copied, nil)
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Engine) cleanupRetiredSnapshots(db *store.Store, repoRoot string) error {
	var snapshots []string
	if err := db.ScanPrefix([]byte("snapshot:retired:"), func(key, _ []byte) error {
		snapshots = append(snapshots, strings.TrimPrefix(string(key), "snapshot:retired:"))
		return nil
	}); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if snapshot == "" {
			continue
		}
		if err := e.deleteGraphNamespace(db, snapshot); err != nil {
			return err
		}
		if err := e.deleteSnapshotArtifacts(repoRoot, snapshot); err != nil {
			return err
		}
		if err := db.Delete([]byte("snapshot:retired:" + snapshot)); err != nil {
			return err
		}
	}
	return nil
}

// graphStore returns the active internal graph store for a repository.
func (e *Engine) graphStore(repo string) *graph.GraphStore {
	if err := e.ensureRepositoryOpen(repo); err != nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.graphs[repo]
}

func (e *Engine) ListRepos() ([]RepoInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]RepoInfo, 0, len(e.repoDirs))
	for repo := range e.repoDirs {
		result = append(result, RepoInfo{Name: repo})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// DeleteRepo removes one logical repository and all of its graph, lexical,
// content, and vector snapshots. The repository directory is first atomically
// moved out of the active namespace so partial filesystem cleanup cannot leave
// a queryable half-deleted repository.
func (e *Engine) DeleteRepo(ctx context.Context, repo string) error {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ErrInvalidRequest
	}
	e.mu.RLock()
	root := e.repoDirs[repo]
	_, busy := e.indexing[repo]
	e.mu.RUnlock()
	if root == "" {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	if busy {
		return fmt.Errorf("%w: %s", ErrRepoBusy, repo)
	}
	operation := e.repositoryOperationLock(repo)
	locked := make(chan struct{})
	go func() {
		operation.Lock()
		close(locked)
	}()
	select {
	case <-ctx.Done():
		go func() {
			<-locked
			operation.Unlock()
		}()
		return ctx.Err()
	case <-locked:
	}
	defer operation.Unlock()
	// Opening the repository first acquires its database lock. This prevents a
	// Unix rename/remove from deleting files still used by another process.
	if err := e.ensureRepositoryOpen(repo); err != nil {
		return err
	}

	e.mu.RLock()
	root = e.repoDirs[repo]
	_, busy = e.indexing[repo]
	if root == "" {
		e.mu.RUnlock()
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	if busy {
		e.mu.RUnlock()
		return fmt.Errorf("%w: %s", ErrRepoBusy, repo)
	}
	graphStore, index, vector, content, db := e.graphs[repo], e.lexical[repo], e.vectors[repo], e.sources[repo], e.stores[repo]
	retiredIndexes := e.retiredLexical[repo]
	retiredVectors := e.retiredVectors[repo]
	retiredSources := e.retiredSources[repo]
	e.mu.RUnlock()

	if graphStore != nil {
		if err := graphStore.Flush(); err != nil {
			return err
		}
	}
	if index != nil {
		_ = index.Close()
	}
	if vector != nil {
		vector.Close()
	}
	if content != nil {
		content.Close()
	}
	for _, retired := range retiredIndexes {
		_ = retired.Close()
	}
	for _, retired := range retiredVectors {
		retired.Close()
	}
	for _, retired := range retiredSources {
		retired.Close()
	}
	if db != nil {
		if err := db.Close(); err != nil {
			return err
		}
	}

	trashPath := filepath.Join(e.eDir, "trash", fmt.Sprintf("%s-%x", repositoryID(repo), time.Now().UnixNano()))
	if err := os.Rename(root, trashPath); err != nil {
		e.mu.Lock()
		delete(e.graphs, repo)
		delete(e.lexical, repo)
		delete(e.vectors, repo)
		delete(e.sources, repo)
		delete(e.stores, repo)
		delete(e.retiredLexical, repo)
		delete(e.retiredVectors, repo)
		delete(e.retiredSources, repo)
		e.mu.Unlock()
		return fmt.Errorf("move repository to trash: %w", err)
	}
	e.mu.Lock()
	delete(e.graphs, repo)
	delete(e.lexical, repo)
	delete(e.vectors, repo)
	delete(e.sources, repo)
	delete(e.stores, repo)
	delete(e.repoDirs, repo)
	delete(e.retiredLexical, repo)
	delete(e.retiredVectors, repo)
	delete(e.retiredSources, repo)
	e.mu.Unlock()
	if err := os.RemoveAll(trashPath); err != nil {
		return fmt.Errorf("repository deleted but trash cleanup failed: %w", err)
	}
	return nil
}

// ExportFullCSV exports the active persisted snapshot for storage inspection.
func (e *Engine) ExportFullCSV(repo, directory string) (*CSVManifest, error) {
	operation := e.repositoryOperationLock(repo)
	operation.RLock()
	defer operation.RUnlock()
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
	Direction     TraverseDirection `json:"direction" jsonschema:"edge direction: out, in, or both; common aliases such as forward, downstream, call, backward, upstream, any, and all are accepted"`
	MaxDepth      int               `json:"maxDepth"`
	RelationTypes []string          `json:"relationTypes,omitempty" jsonschema:"optional relationship filter, for example CALLS, IMPORTS, EXTENDS, or IMPLEMENTS"`
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
	Edges []GraphEdge `json:"edges"`
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

// ContextRequest describes a direct semantic-neighborhood query for one symbol.
type ContextRequest struct {
	Repo          string   `json:"repo"`
	NodeID        string   `json:"nodeId"`
	RelationTypes []string `json:"relationTypes,omitempty" jsonschema:"optional relationship filter; defaults to semantic and ownership relationships"`
	Limit         int      `json:"limit,omitempty"`
}

// ContextRelation explains how a neighboring node relates to the requested symbol.
type ContextRelation struct {
	Direction string    `json:"direction"`
	Relation  GraphEdge `json:"relation"`
	Node      GraphNode `json:"node"`
}

// ContextResult is the agent-oriented context for one persisted symbol.
type ContextResult struct {
	Symbol    GraphNode         `json:"symbol"`
	Content   string            `json:"content,omitempty"`
	Relations []ContextRelation `json:"relations"`
	Truncated bool              `json:"truncated,omitempty"`
}

// ImpactRequest describes a reverse semantic-dependency analysis.
type ImpactRequest struct {
	Repo          string   `json:"repo"`
	NodeID        string   `json:"nodeId"`
	MaxDepth      int      `json:"maxDepth,omitempty"`
	RelationTypes []string `json:"relationTypes,omitempty" jsonschema:"optional relationship filter; defaults to incoming calls, imports, inheritance, implementations, overrides, dispatch, routes, tools, and event bindings"`
	Limit         int      `json:"limit,omitempty"`
}

// ImpactNode records one affected symbol and the edge that first reached it.
type ImpactNode struct {
	Node       GraphNode `json:"node"`
	Depth      int       `json:"depth"`
	Via        GraphEdge `json:"via"`
	Confidence float64   `json:"confidence"`
}

// ImpactResult contains the bounded reverse dependency tree for a symbol.
type ImpactResult struct {
	Origin    GraphNode    `json:"origin"`
	Impacted  []ImpactNode `json:"impacted"`
	Edges     []GraphEdge  `json:"edges"`
	Truncated bool         `json:"truncated,omitempty"`
}

// CheckRequest selects repository-wide structural checks.
type CheckRequest struct {
	Repo          string   `json:"repo"`
	Checks        []string `json:"checks,omitempty" jsonschema:"checks to run: integrity, cycles, confidence; defaults to integrity and cycles"`
	MinConfidence float64  `json:"minConfidence,omitempty" jsonschema:"minimum semantic edge confidence when the confidence check is enabled; defaults to 0.7"`
	Limit         int      `json:"limit,omitempty"`
}

// CheckFinding is one actionable structural problem.
type CheckFinding struct {
	Code     string      `json:"code"`
	Severity string      `json:"severity"`
	Message  string      `json:"message"`
	Nodes    []GraphNode `json:"nodes,omitempty"`
	Edges    []GraphEdge `json:"edges,omitempty"`
}

// CheckSummary summarizes the repository scan.
type CheckSummary struct {
	NodesScanned int `json:"nodesScanned"`
	EdgesScanned int `json:"edgesScanned"`
	Errors       int `json:"errors"`
	Warnings     int `json:"warnings"`
	Infos        int `json:"infos"`
}

// CheckResult contains bounded findings from repository-wide structural checks.
type CheckResult struct {
	Repo      string         `json:"repo"`
	Summary   CheckSummary   `json:"summary"`
	Findings  []CheckFinding `json:"findings"`
	Truncated bool           `json:"truncated,omitempty"`
}

// DiffRequest selects a Git comparison and optional impact expansion.
type DiffRequest struct {
	Repo       string `json:"repo"`
	BaseRef    string `json:"baseRef,omitempty" jsonschema:"base Git ref; defaults to HEAD"`
	TargetRef  string `json:"targetRef,omitempty" jsonschema:"optional target Git ref; when omitted compares the base with the working tree"`
	MaxDepth   int    `json:"maxDepth,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	SkipImpact bool   `json:"skipImpact,omitempty"`
}

// DiffRange describes one zero-context Git hunk using one-based line numbers.
type DiffRange struct {
	OldStart int `json:"oldStart"`
	OldLines int `json:"oldLines"`
	NewStart int `json:"newStart"`
	NewLines int `json:"newLines"`
}

// DiffFile describes one file changed by Git.
type DiffFile struct {
	Path      string      `json:"path"`
	OldPath   string      `json:"oldPath,omitempty"`
	Status    string      `json:"status"`
	Additions int         `json:"additions"`
	Deletions int         `json:"deletions"`
	Ranges    []DiffRange `json:"ranges,omitempty"`
}

// DiffSymbol maps changed lines to one persisted actionable symbol.
type DiffSymbol struct {
	Node   GraphNode   `json:"node"`
	Status string      `json:"status"`
	Ranges []DiffRange `json:"ranges,omitempty"`
}

// DiffImpact aggregates one affected symbol across all changed causes.
type DiffImpact struct {
	Node       GraphNode `json:"node"`
	Depth      int       `json:"depth"`
	Confidence float64   `json:"confidence"`
	Via        GraphEdge `json:"via"`
	Causes     []string  `json:"causes"`
}

// DiffResult maps a Git diff to graph symbols and their reverse dependencies.
type DiffResult struct {
	Repo      string       `json:"repo"`
	BaseRef   string       `json:"baseRef"`
	TargetRef string       `json:"targetRef,omitempty"`
	Files     []DiffFile   `json:"files"`
	Symbols   []DiffSymbol `json:"symbols"`
	Impacted  []DiffImpact `json:"impacted,omitempty"`
	Truncated bool         `json:"truncated,omitempty"`
}

// RenameRequest asks for a non-mutating symbol rename analysis.
type RenameRequest struct {
	Repo    string `json:"repo"`
	NodeID  string `json:"nodeId"`
	NewName string `json:"newName"`
	Limit   int    `json:"limit,omitempty"`
}

// RenameConflict describes an existing symbol that may collide with the new name.
type RenameConflict struct {
	Severity string    `json:"severity"`
	Scope    string    `json:"scope"`
	Message  string    `json:"message"`
	Existing GraphNode `json:"existing"`
}

// RenameOccurrence is one exact textual occurrence requiring a source edit or review.
type RenameOccurrence struct {
	FilePath       string  `json:"filePath"`
	Line           int     `json:"line"`
	Content        string  `json:"content"`
	Kind           string  `json:"kind"`
	Confidence     float64 `json:"confidence"`
	RequiresReview bool    `json:"requiresReview"`
}

// RenameResult is a safe analysis plan; it never modifies repository files.
type RenameResult struct {
	Symbol             GraphNode          `json:"symbol"`
	OldName            string             `json:"oldName"`
	NewName            string             `json:"newName"`
	Safe               bool               `json:"safe"`
	RequiresReview     bool               `json:"requiresReview"`
	Conflicts          []RenameConflict   `json:"conflicts"`
	SemanticReferences []ContextRelation  `json:"semanticReferences"`
	Occurrences        []RenameOccurrence `json:"occurrences"`
	Truncated          bool               `json:"truncated,omitempty"`
}

var defaultContextRelations = []string{
	"CALLS", "IMPORTS", "EXTENDS", "IMPLEMENTS", "INHERITS", "OVERRIDES",
	"METHOD_OVERRIDES", "METHOD_IMPLEMENTS", "DISPATCHES_TO", "HAS_METHOD",
	"HAS_PROPERTY", "HANDLES_ROUTE", "HANDLES_TOOL", "BINDS_EVENT_HANDLER",
	"EMITS_EVENT", "WRAPS", "DECORATES",
}

var defaultImpactRelations = []string{
	"CALLS", "IMPORTS", "EXTENDS", "IMPLEMENTS", "INHERITS", "OVERRIDES",
	"METHOD_OVERRIDES", "METHOD_IMPLEMENTS", "DISPATCHES_TO", "HANDLES_ROUTE",
	"HANDLES_TOOL", "BINDS_EVENT_HANDLER", "EMITS_EVENT", "WRAPS",
}

// Traverse performs repository-scoped BFS. Results exclude the start node.
func (e *Engine) Traverse(ctx context.Context, request *TraverseRequest) (*TraverseResult, error) {
	if request == nil || request.Repo == "" || request.StartNodeID == "" || request.MaxDepth < 1 {
		return nil, ErrInvalidRequest
	}
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
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

	nodes, edges, err := graphStore.BFSWithEdges(ctx, request.StartNodeID, direction, request.MaxDepth, filter)
	if err != nil {
		return nil, err
	}
	result := &TraverseResult{
		Nodes: make([]GraphNode, 0, len(nodes)),
		Edges: make([]GraphEdge, 0, len(edges)),
	}
	for _, node := range nodes {
		result.Nodes = append(result.Nodes, publicGraphNode(node))
	}
	for _, edge := range edges {
		result.Edges = append(result.Edges, publicGraphEdge(edge))
	}
	return result, nil
}

// Context returns the requested symbol and its direct semantic neighborhood.
// Structural graph expansion is excluded by default so agents receive useful
// symbol relationships rather than File, Folder, and Community noise.
func (e *Engine) Context(ctx context.Context, request *ContextRequest) (*ContextResult, error) {
	if request == nil || strings.TrimSpace(request.Repo) == "" || strings.TrimSpace(request.NodeID) == "" {
		return nil, ErrInvalidRequest
	}
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	symbol, err := graphStore.GetNode(request.NodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, request.NodeID)
	}
	allowed := normalizedRelationSet(request.RelationTypes, defaultContextRelations)
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}

	type directedEdge struct {
		direction string
		edge      *graph.Edge
		nodeID    string
	}
	var candidates []directedEdge
	outgoing, err := graphStore.GetAllOutEdges(request.NodeID)
	if err != nil {
		return nil, err
	}
	for _, edge := range outgoing {
		if _, ok := allowed[strings.ToUpper(string(edge.Type))]; ok {
			candidates = append(candidates, directedEdge{direction: "out", edge: edge, nodeID: edge.Target})
		}
	}
	incoming, err := graphStore.GetAllInEdges(request.NodeID)
	if err != nil {
		return nil, err
	}
	for _, edge := range incoming {
		if _, ok := allowed[strings.ToUpper(string(edge.Type))]; ok {
			candidates = append(candidates, directedEdge{direction: "in", edge: edge, nodeID: edge.Source})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].edge.Type != candidates[j].edge.Type {
			return candidates[i].edge.Type < candidates[j].edge.Type
		}
		if candidates[i].direction != candidates[j].direction {
			return candidates[i].direction < candidates[j].direction
		}
		return candidates[i].nodeID < candidates[j].nodeID
	})

	result := &ContextResult{
		Symbol:    publicGraphNode(symbol),
		Content:   symbol.GetPropString("content"),
		Relations: make([]ContextRelation, 0, min(limit, len(candidates))),
		Truncated: len(candidates) > limit,
	}
	if result.Content == "" {
		result.Content = e.readSymbolContent(request.Repo, symbol)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := candidate.direction + "\x00" + candidate.edge.ID
		if _, ok := seen[key]; ok {
			continue
		}
		neighbor, err := graphStore.GetNode(candidate.nodeID)
		if err != nil {
			continue
		}
		seen[key] = struct{}{}
		result.Relations = append(result.Relations, ContextRelation{
			Direction: candidate.direction,
			Relation:  publicGraphEdge(candidate.edge),
			Node:      publicGraphNode(neighbor),
		})
		if len(result.Relations) >= limit {
			break
		}
	}
	return result, nil
}

func (e *Engine) readSymbolContent(repo string, symbol *graph.Node) string {
	if symbol == nil || symbol.FilePath == "" {
		return ""
	}
	sourceRoot, err := e.repositorySourcePath(repo)
	if err != nil {
		return ""
	}
	sourceRoot, err = filepath.Abs(sourceRoot)
	if err != nil {
		return ""
	}
	sourcePath, err := filepath.Abs(filepath.Join(sourceRoot, filepath.FromSlash(symbol.FilePath)))
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(sourceRoot, sourcePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	if _, ok := symbol.Props.GetProp("startLine"); !ok {
		return ""
	}
	// Parser positions are stored as zero-based rows while source excerpts are
	// read from the one-based line stream.
	startLine := symbol.GetPropInt("startLine") + 1
	endLine := symbol.GetPropInt("endLine") + 1
	if startLine < 1 {
		return ""
	}
	if endLine < startLine {
		endLine = startLine
	}
	if endLine-startLine >= 200 {
		endLine = startLine + 199
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make([]string, 0, endLine-startLine+1)
	for lineNumber := 1; scanner.Scan() && lineNumber <= endLine; lineNumber++ {
		if lineNumber >= startLine {
			lines = append(lines, scanner.Text())
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Engine) repositorySourcePath(repo string) (string, error) {
	e.mu.RLock()
	repositoryRoot := e.repoDirs[repo]
	e.mu.RUnlock()
	if repositoryRoot == "" {
		return "", fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	encoded, err := os.ReadFile(filepath.Join(repositoryRoot, "manifest.json"))
	if err != nil {
		return "", err
	}
	var manifest repositoryManifest
	if json.Unmarshal(encoded, &manifest) != nil || manifest.SourcePath == "" {
		return "", fmt.Errorf("repository %q has no source path", repo)
	}
	return manifest.SourcePath, nil
}

// Impact walks incoming semantic dependencies to identify callers,
// importers, implementations, derived types, overrides, and bound entry
// points that may be affected by changing a symbol.
func (e *Engine) Impact(ctx context.Context, request *ImpactRequest) (*ImpactResult, error) {
	if request == nil || strings.TrimSpace(request.Repo) == "" || strings.TrimSpace(request.NodeID) == "" {
		return nil, ErrInvalidRequest
	}
	maxDepth := request.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		return nil, fmt.Errorf("%w: maxDepth must not exceed 10", ErrInvalidRequest)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return nil, fmt.Errorf("%w: limit must not exceed 1000", ErrInvalidRequest)
	}

	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	origin, err := graphStore.GetNode(request.NodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, request.NodeID)
	}
	allowed := normalizedRelationSet(request.RelationTypes, defaultImpactRelations)
	result := &ImpactResult{
		Origin:   publicGraphNode(origin),
		Impacted: make([]ImpactNode, 0, limit),
		Edges:    make([]GraphEdge, 0, limit),
	}
	type impactQueueEntry struct {
		nodeID string
		depth  int
	}
	queue := []impactQueueEntry{{nodeID: request.NodeID, depth: 0}}
	visited := map[string]struct{}{request.NodeID: {}}

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}
		edges, err := graphStore.GetAllInEdges(current.nodeID)
		if err != nil {
			return nil, err
		}
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].Confidence() != edges[j].Confidence() {
				return edges[i].Confidence() > edges[j].Confidence()
			}
			if edges[i].Type != edges[j].Type {
				return edges[i].Type < edges[j].Type
			}
			return edges[i].Source < edges[j].Source
		})
		for _, edge := range edges {
			if _, ok := allowed[strings.ToUpper(string(edge.Type))]; !ok {
				continue
			}
			if _, ok := visited[edge.Source]; ok {
				continue
			}
			node, err := graphStore.GetNode(edge.Source)
			if err != nil || !isImpactNode(origin, node) {
				continue
			}
			visited[edge.Source] = struct{}{}
			depth := current.depth + 1
			publicEdge := publicGraphEdge(edge)
			result.Impacted = append(result.Impacted, ImpactNode{
				Node:       publicGraphNode(node),
				Depth:      depth,
				Via:        publicEdge,
				Confidence: edge.Confidence(),
			})
			result.Edges = append(result.Edges, publicEdge)
			queue = append(queue, impactQueueEntry{nodeID: edge.Source, depth: depth})
			if len(result.Impacted) >= limit {
				result.Truncated = len(queue) > 0 || len(edges) > 0
				return result, nil
			}
		}
	}
	return result, nil
}

// Check scans the persisted graph for integrity violations, inheritance and
// import cycles, and optionally low-confidence semantic relationships.
func (e *Engine) Check(ctx context.Context, request *CheckRequest) (*CheckResult, error) {
	if request == nil || strings.TrimSpace(request.Repo) == "" {
		return nil, ErrInvalidRequest
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return nil, fmt.Errorf("%w: limit must not exceed 1000", ErrInvalidRequest)
	}
	checks, err := normalizedCheckSet(request.Checks)
	if err != nil {
		return nil, err
	}
	minConfidence := request.MinConfidence
	if minConfidence == 0 {
		minConfidence = 0.7
	}
	if minConfidence < 0 || minConfidence > 1 {
		return nil, fmt.Errorf("%w: minConfidence must be between 0 and 1", ErrInvalidRequest)
	}

	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}

	nodes := make(map[string]*graph.Node)
	if err := graphStore.ForEachNode(func(node *graph.Node) error {
		if len(nodes)%1000 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		nodes[node.ID] = node
		return nil
	}); err != nil {
		return nil, err
	}

	result := &CheckResult{
		Repo:     request.Repo,
		Findings: make([]CheckFinding, 0, min(limit, 32)),
		Summary:  CheckSummary{NodesScanned: len(nodes)},
	}
	var inheritanceEdges, importEdges []*graph.Edge
	var confidenceFindings []CheckFinding
	confidenceFindingCount := 0
	invalidSelfTypes := map[graph.RelType]struct{}{
		graph.RelImports: {}, graph.RelExtends: {}, graph.RelImplements: {},
		graph.RelInherits: {}, graph.RelMethodOverrides: {}, graph.RelMethodImplements: {},
	}
	semanticTypes := normalizedRelationSet(nil, defaultImpactRelations)

	if err := graphStore.ForEachEdge(func(edge *graph.Edge) error {
		result.Summary.EdgesScanned++
		if result.Summary.EdgesScanned%1000 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if _, ok := checks["integrity"]; ok {
			source, sourceOK := nodes[edge.Source]
			target, targetOK := nodes[edge.Target]
			if !sourceOK || !targetOK {
				finding := CheckFinding{
					Code: "DANGLING_EDGE", Severity: "error",
					Message: fmt.Sprintf("%s edge references a missing endpoint", edge.Type),
					Edges:   []GraphEdge{publicGraphEdge(edge)},
				}
				if sourceOK {
					finding.Nodes = append(finding.Nodes, publicGraphNode(source))
				}
				if targetOK {
					finding.Nodes = append(finding.Nodes, publicGraphNode(target))
				}
				addCheckFinding(result, finding, limit)
			}
			if edge.Source == edge.Target {
				if _, invalid := invalidSelfTypes[edge.Type]; invalid {
					finding := CheckFinding{
						Code: "INVALID_SELF_DEPENDENCY", Severity: "error",
						Message: fmt.Sprintf("%s relationship points back to the same node", edge.Type),
						Edges:   []GraphEdge{publicGraphEdge(edge)},
					}
					if sourceOK {
						finding.Nodes = []GraphNode{publicGraphNode(source)}
					}
					addCheckFinding(result, finding, limit)
				}
			}
		}
		if _, ok := checks["cycles"]; ok {
			switch edge.Type {
			case graph.RelExtends, graph.RelInherits:
				inheritanceEdges = append(inheritanceEdges, edge)
			case graph.RelImports:
				importEdges = append(importEdges, edge)
			}
		}
		if _, ok := checks["confidence"]; ok {
			if _, semantic := semanticTypes[strings.ToUpper(string(edge.Type))]; semantic &&
				edge.Confidence() < minConfidence {
				confidenceFindingCount++
				if len(confidenceFindings) >= limit {
					result.Truncated = true
					return nil
				}
				finding := CheckFinding{
					Code: "LOW_CONFIDENCE_RELATION", Severity: "info",
					Message: fmt.Sprintf("%s relationship confidence %.2f is below %.2f",
						edge.Type, edge.Confidence(), minConfidence),
					Edges: []GraphEdge{publicGraphEdge(edge)},
				}
				if source := nodes[edge.Source]; source != nil {
					finding.Nodes = append(finding.Nodes, publicGraphNode(source))
				}
				if target := nodes[edge.Target]; target != nil {
					finding.Nodes = append(finding.Nodes, publicGraphNode(target))
				}
				confidenceFindings = append(confidenceFindings, finding)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if _, ok := checks["cycles"]; ok {
		for _, component := range directedCycleComponents(inheritanceEdges) {
			addCheckFinding(result, cycleFinding(
				"INHERITANCE_CYCLE", "error", "Inheritance cycle detected",
				component, inheritanceEdges, nodes,
			), limit)
		}
		for _, component := range directedCycleComponents(importEdges) {
			addCheckFinding(result, cycleFinding(
				"IMPORT_CYCLE", "warning", "Import cycle detected",
				component, importEdges, nodes,
			), limit)
		}
	}
	for _, finding := range confidenceFindings {
		addCheckFinding(result, finding, limit)
	}
	result.Summary.Infos += confidenceFindingCount - len(confidenceFindings)
	sort.SliceStable(result.Findings, func(i, j int) bool {
		left, right := checkSeverityRank(result.Findings[i].Severity), checkSeverityRank(result.Findings[j].Severity)
		if left != right {
			return left < right
		}
		if result.Findings[i].Code != result.Findings[j].Code {
			return result.Findings[i].Code < result.Findings[j].Code
		}
		return result.Findings[i].Message < result.Findings[j].Message
	})
	return result, nil
}

// Diff maps a Git comparison to persisted symbols and optionally expands
// reverse semantic impact from every changed symbol.
func (e *Engine) Diff(ctx context.Context, request *DiffRequest) (*DiffResult, error) {
	if request == nil || strings.TrimSpace(request.Repo) == "" {
		return nil, ErrInvalidRequest
	}
	baseRef := strings.TrimSpace(request.BaseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	targetRef := strings.TrimSpace(request.TargetRef)
	maxDepth := request.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		return nil, fmt.Errorf("%w: maxDepth must not exceed 10", ErrInvalidRequest)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return nil, fmt.Errorf("%w: limit must not exceed 1000", ErrInvalidRequest)
	}
	sourceRoot, err := e.repositorySourcePath(request.Repo)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(sourceRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("repository source path %q is unavailable", sourceRoot)
	}

	args := []string{"-c", "core.quotepath=false", "diff", "--relative", "--no-ext-diff", "--no-color", "--find-renames", "--unified=0", baseRef}
	if targetRef != "" {
		args = append(args, targetRef)
	}
	args = append(args, "--", ".")
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = sourceRoot
	output, err := command.Output()
	if err != nil {
		if executionError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(executionError.Stderr)))
		}
		return nil, fmt.Errorf("run git diff: %w", err)
	}
	files, err := parseGitPatch(string(output))
	if err != nil {
		return nil, err
	}
	result := &DiffResult{
		Repo: request.Repo, BaseRef: baseRef, TargetRef: targetRef,
		Files: files, Symbols: make([]DiffSymbol, 0),
	}
	if len(files) == 0 {
		return result, nil
	}

	changedIDs := make(map[string]struct{})
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		operation.RUnlock()
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			operation.RUnlock()
			return nil, err
		}
		lookupPath := file.Path
		nodes, err := graphStore.GetNodesByFile(graphStore.Repo(), lookupPath)
		if (err != nil || len(nodes) == 0) && file.OldPath != "" {
			lookupPath = file.OldPath
			nodes, err = graphStore.GetNodesByFile(graphStore.Repo(), lookupPath)
		}
		if err != nil {
			operation.RUnlock()
			return nil, err
		}
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].GetPropInt("startLine") != nodes[j].GetPropInt("startLine") {
				return nodes[i].GetPropInt("startLine") < nodes[j].GetPropInt("startLine")
			}
			return nodes[i].ID < nodes[j].ID
		})
		for _, node := range nodes {
			if !node.Label.IsActionableSymbol() || !nodeIntersectsDiff(node, file) {
				continue
			}
			if _, exists := changedIDs[node.ID]; exists {
				continue
			}
			changedIDs[node.ID] = struct{}{}
			result.Symbols = append(result.Symbols, DiffSymbol{
				Node: publicGraphNode(node), Status: file.Status, Ranges: file.Ranges,
			})
			if len(result.Symbols) >= limit {
				result.Truncated = true
				break
			}
		}
		if result.Truncated {
			break
		}
	}
	operation.RUnlock()

	if request.SkipImpact || len(result.Symbols) == 0 {
		return result, nil
	}
	impacts := make(map[string]*DiffImpact)
	for _, changed := range result.Symbols {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		impact, err := e.Impact(ctx, &ImpactRequest{
			Repo: request.Repo, NodeID: changed.Node.ID,
			MaxDepth: maxDepth, Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		if impact.Truncated {
			result.Truncated = true
		}
		for _, affected := range impact.Impacted {
			if _, changedDirectly := changedIDs[affected.Node.ID]; changedDirectly {
				continue
			}
			existing := impacts[affected.Node.ID]
			if existing == nil {
				impacts[affected.Node.ID] = &DiffImpact{
					Node: affected.Node, Depth: affected.Depth, Confidence: affected.Confidence,
					Via: affected.Via, Causes: []string{changed.Node.ID},
				}
				continue
			}
			if affected.Depth < existing.Depth ||
				(affected.Depth == existing.Depth && affected.Confidence > existing.Confidence) {
				existing.Depth = affected.Depth
				existing.Confidence = affected.Confidence
				existing.Via = affected.Via
			}
			if !containsString(existing.Causes, changed.Node.ID) {
				existing.Causes = append(existing.Causes, changed.Node.ID)
			}
		}
	}
	result.Impacted = make([]DiffImpact, 0, min(limit, len(impacts)))
	for _, impact := range impacts {
		sort.Strings(impact.Causes)
		result.Impacted = append(result.Impacted, *impact)
	}
	sort.Slice(result.Impacted, func(i, j int) bool {
		if result.Impacted[i].Depth != result.Impacted[j].Depth {
			return result.Impacted[i].Depth < result.Impacted[j].Depth
		}
		if result.Impacted[i].Confidence != result.Impacted[j].Confidence {
			return result.Impacted[i].Confidence > result.Impacted[j].Confidence
		}
		return result.Impacted[i].Node.ID < result.Impacted[j].Node.ID
	})
	if len(result.Impacted) > limit {
		result.Impacted = result.Impacted[:limit]
		result.Truncated = true
	}
	return result, nil
}

// Rename produces a conflict and reference plan without modifying source code.
func (e *Engine) Rename(ctx context.Context, request *RenameRequest) (*RenameResult, error) {
	if request == nil || strings.TrimSpace(request.Repo) == "" ||
		strings.TrimSpace(request.NodeID) == "" || strings.TrimSpace(request.NewName) == "" {
		return nil, ErrInvalidRequest
	}
	newName := strings.TrimSpace(request.NewName)
	if !validSymbolName(newName) {
		return nil, fmt.Errorf("%w: %q is not a supported identifier", ErrInvalidRequest, newName)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		return nil, fmt.Errorf("%w: limit must not exceed 1000", ErrInvalidRequest)
	}

	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		operation.RUnlock()
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	symbol, err := graphStore.GetNode(request.NodeID)
	if err != nil {
		operation.RUnlock()
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, request.NodeID)
	}
	if !symbol.Label.IsActionableSymbol() {
		operation.RUnlock()
		return nil, fmt.Errorf("%w: %s is not a renameable symbol", ErrInvalidRequest, request.NodeID)
	}
	if symbol.Name == newName {
		operation.RUnlock()
		return nil, fmt.Errorf("%w: new name is unchanged", ErrInvalidRequest)
	}
	result := &RenameResult{
		Symbol: publicGraphNode(symbol), OldName: symbol.Name, NewName: newName, Safe: true,
		Conflicts: make([]RenameConflict, 0), SemanticReferences: make([]ContextRelation, 0),
		Occurrences: make([]RenameOccurrence, 0),
	}

	conflicts, err := graphStore.GetNodesByName(graphStore.Repo(), newName)
	if err != nil {
		operation.RUnlock()
		return nil, err
	}
	for _, existing := range conflicts {
		if existing.ID == symbol.ID || !existing.Label.IsActionableSymbol() {
			continue
		}
		conflict := RenameConflict{
			Severity: "warning", Scope: "repository",
			Message:  "symbol with the requested name already exists in the repository",
			Existing: publicGraphNode(existing),
		}
		if existing.FilePath == symbol.FilePath {
			conflict.Severity = "error"
			conflict.Scope = "file"
			conflict.Message = "symbol with the requested name already exists in the same file"
			result.Safe = false
		}
		result.Conflicts = append(result.Conflicts, conflict)
	}
	sort.Slice(result.Conflicts, func(i, j int) bool {
		if result.Conflicts[i].Severity != result.Conflicts[j].Severity {
			return result.Conflicts[i].Severity < result.Conflicts[j].Severity
		}
		return result.Conflicts[i].Existing.ID < result.Conflicts[j].Existing.ID
	})

	allowed := normalizedRelationSet(nil, defaultContextRelations)
	type semanticCandidate struct {
		direction string
		edge      *graph.Edge
		nodeID    string
	}
	var semanticCandidates []semanticCandidate
	incoming, err := graphStore.GetAllInEdges(symbol.ID)
	if err != nil {
		operation.RUnlock()
		return nil, err
	}
	for _, edge := range incoming {
		if _, ok := allowed[strings.ToUpper(string(edge.Type))]; ok {
			semanticCandidates = append(semanticCandidates, semanticCandidate{direction: "in", edge: edge, nodeID: edge.Source})
		}
	}
	semanticNodes := make(map[string]*graph.Node)
	seenEdges := make(map[string]struct{})
	for _, candidate := range semanticCandidates {
		if _, seen := seenEdges[candidate.direction+"\x00"+candidate.edge.ID]; seen {
			continue
		}
		neighbor, err := graphStore.GetNode(candidate.nodeID)
		if err != nil || !neighbor.Label.IsActionableSymbol() {
			continue
		}
		seenEdges[candidate.direction+"\x00"+candidate.edge.ID] = struct{}{}
		semanticNodes[neighbor.ID] = neighbor
		result.SemanticReferences = append(result.SemanticReferences, ContextRelation{
			Direction: candidate.direction,
			Relation:  publicGraphEdge(candidate.edge),
			Node:      publicGraphNode(neighbor),
		})
	}
	sort.Slice(result.SemanticReferences, func(i, j int) bool {
		if result.SemanticReferences[i].Relation.Type != result.SemanticReferences[j].Relation.Type {
			return result.SemanticReferences[i].Relation.Type < result.SemanticReferences[j].Relation.Type
		}
		return result.SemanticReferences[i].Node.ID < result.SemanticReferences[j].Node.ID
	})
	operation.RUnlock()

	exactIdentifier := regexp.MustCompile(identifierSearchPattern(symbol.Name))
	searchLimit := min(1000, limit*5)
	sourceResult, err := e.SearchSource(ctx, &SourceSearchRequest{
		Repo: request.Repo, Query: symbol.Name, Scope: SourceScopeCode,
		Limit: searchLimit, ContextLines: 0,
	})
	if err != nil {
		return nil, err
	}
	for _, match := range sourceResult.Results {
		if !exactIdentifier.MatchString(match.Content) {
			continue
		}
		occurrence := RenameOccurrence{
			FilePath: match.FilePath, Line: match.Line, Content: match.Content,
			Kind: "text-candidate", Confidence: 0.5, RequiresReview: true,
		}
		if match.FilePath == symbol.FilePath &&
			lineWithinNode(match.Line, symbol) {
			occurrence.Kind = "declaration"
			occurrence.Confidence = 1
			occurrence.RequiresReview = false
		} else {
			for _, neighbor := range semanticNodes {
				if match.FilePath == neighbor.FilePath && lineWithinNode(match.Line, neighbor) {
					occurrence.Kind = "semantic-reference"
					occurrence.Confidence = 0.9
					occurrence.RequiresReview = false
					break
				}
			}
		}
		if occurrence.RequiresReview {
			result.RequiresReview = true
		}
		result.Occurrences = append(result.Occurrences, occurrence)
		if len(result.Occurrences) >= limit {
			result.Truncated = true
			break
		}
	}
	if len(sourceResult.Results) >= searchLimit {
		result.Truncated = true
	}
	if len(result.Occurrences) == 0 {
		result.RequiresReview = true
		result.Safe = false
	}
	return result, nil
}

func validSymbolName(name string) bool {
	for index, character := range []rune(name) {
		if index == 0 {
			if character != '_' && character != '$' && !unicode.IsLetter(character) {
				return false
			}
			continue
		}
		if character != '_' && character != '$' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return name != ""
}

func identifierSearchPattern(name string) string {
	quoted := regexp.QuoteMeta(name)
	return `(^|[^[:alnum:]_$])` + quoted + `([^[:alnum:]_$]|$)`
}

func lineWithinNode(line int, node *graph.Node) bool {
	if node == nil {
		return false
	}
	if _, ok := node.Props.GetProp("startLine"); !ok {
		return false
	}
	start := node.GetPropInt("startLine") + 1
	end := node.GetPropInt("endLine") + 1
	if end < start {
		end = start
	}
	return line >= start && line <= end
}

var gitHunkPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func parseGitPatch(patch string) ([]DiffFile, error) {
	var files []DiffFile
	var current *DiffFile
	flush := func() {
		if current == nil {
			return
		}
		if current.Path == "" {
			current.Path = current.OldPath
		}
		if current.Status == "" {
			current.Status = "modified"
		}
		files = append(files, *current)
		current = nil
	}
	scanner := bufio.NewScanner(strings.NewReader(patch))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			current = &DiffFile{Status: "modified"}
		case current == nil:
			continue
		case strings.HasPrefix(line, "new file mode "):
			current.Status = "added"
		case strings.HasPrefix(line, "deleted file mode "):
			current.Status = "deleted"
		case strings.HasPrefix(line, "rename from "):
			current.Status = "renamed"
			current.OldPath = strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			current.Status = "renamed"
			current.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "--- "):
			path := strings.TrimPrefix(line, "--- ")
			if path != "/dev/null" && current.OldPath == "" {
				current.OldPath = trimGitPath(path)
			}
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimPrefix(line, "+++ ")
			if path != "/dev/null" {
				current.Path = trimGitPath(path)
			}
		default:
			matches := gitHunkPattern.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			oldStart, _ := strconv.Atoi(matches[1])
			oldLines := parseGitRangeCount(matches[2])
			newStart, _ := strconv.Atoi(matches[3])
			newLines := parseGitRangeCount(matches[4])
			current.Ranges = append(current.Ranges, DiffRange{
				OldStart: oldStart, OldLines: oldLines,
				NewStart: newStart, NewLines: newLines,
			})
			current.Additions += newLines
			current.Deletions += oldLines
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	flush()
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func trimGitPath(path string) string {
	path = strings.TrimSuffix(path, "\t")
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

func parseGitRangeCount(value string) int {
	if value == "" {
		return 1
	}
	count, _ := strconv.Atoi(value)
	return count
}

func nodeIntersectsDiff(node *graph.Node, file DiffFile) bool {
	if len(file.Ranges) == 0 {
		return true
	}
	if _, ok := node.Props.GetProp("startLine"); !ok {
		return false
	}
	nodeStart := node.GetPropInt("startLine") + 1
	nodeEnd := node.GetPropInt("endLine") + 1
	if nodeEnd < nodeStart {
		nodeEnd = nodeStart
	}
	for _, changed := range file.Ranges {
		start, lines := changed.NewStart, changed.NewLines
		if file.Status == "deleted" || lines == 0 {
			start, lines = changed.OldStart, changed.OldLines
		}
		if lines == 0 {
			lines = 1
		}
		end := start + lines - 1
		if nodeStart <= end && nodeEnd >= start {
			return true
		}
	}
	return false
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func normalizedCheckSet(requested []string) (map[string]struct{}, error) {
	if len(requested) == 0 {
		requested = []string{"integrity", "cycles"}
	}
	result := make(map[string]struct{}, len(requested))
	for _, check := range requested {
		check = strings.ToLower(strings.TrimSpace(check))
		switch check {
		case "integrity", "cycles", "confidence":
			result[check] = struct{}{}
		case "":
			continue
		default:
			return nil, fmt.Errorf("%w: unsupported check %q; use integrity, cycles, or confidence", ErrInvalidRequest, check)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: at least one check is required", ErrInvalidRequest)
	}
	return result, nil
}

func addCheckFinding(result *CheckResult, finding CheckFinding, limit int) {
	switch finding.Severity {
	case "error":
		result.Summary.Errors++
	case "warning":
		result.Summary.Warnings++
	default:
		result.Summary.Infos++
	}
	if len(result.Findings) < limit {
		result.Findings = append(result.Findings, finding)
	} else {
		result.Truncated = true
	}
}

func checkSeverityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func cycleFinding(code, severity, message string, component []string, edges []*graph.Edge, nodes map[string]*graph.Node) CheckFinding {
	finding := CheckFinding{Code: code, Severity: severity, Message: message}
	member := make(map[string]struct{}, len(component))
	for _, nodeID := range component {
		member[nodeID] = struct{}{}
		if node := nodes[nodeID]; node != nil && len(finding.Nodes) < 20 {
			finding.Nodes = append(finding.Nodes, publicGraphNode(node))
		}
	}
	sort.Slice(finding.Nodes, func(i, j int) bool { return finding.Nodes[i].ID < finding.Nodes[j].ID })
	for _, edge := range edges {
		_, sourceOK := member[edge.Source]
		_, targetOK := member[edge.Target]
		if sourceOK && targetOK && len(finding.Edges) < 20 {
			finding.Edges = append(finding.Edges, publicGraphEdge(edge))
		}
	}
	sort.Slice(finding.Edges, func(i, j int) bool { return finding.Edges[i].ID < finding.Edges[j].ID })
	return finding
}

// directedCycleComponents returns strongly connected components containing
// more than one node. Self-dependencies are reported by the integrity check.
func directedCycleComponents(edges []*graph.Edge) [][]string {
	adjacency := make(map[string][]string)
	reverse := make(map[string][]string)
	nodeSet := make(map[string]struct{})
	for _, edge := range edges {
		if edge.Source == edge.Target {
			continue
		}
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		reverse[edge.Target] = append(reverse[edge.Target], edge.Source)
		nodeSet[edge.Source] = struct{}{}
		nodeSet[edge.Target] = struct{}{}
	}
	nodes := make([]string, 0, len(nodeSet))
	for nodeID := range nodeSet {
		nodes = append(nodes, nodeID)
		sort.Strings(adjacency[nodeID])
		sort.Strings(reverse[nodeID])
	}
	sort.Strings(nodes)

	type frame struct {
		node  string
		index int
	}
	visited := make(map[string]bool, len(nodes))
	order := make([]string, 0, len(nodes))
	for _, start := range nodes {
		if visited[start] {
			continue
		}
		visited[start] = true
		stack := []frame{{node: start}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			neighbors := adjacency[top.node]
			if top.index < len(neighbors) {
				next := neighbors[top.index]
				top.index++
				if !visited[next] {
					visited[next] = true
					stack = append(stack, frame{node: next})
				}
				continue
			}
			order = append(order, top.node)
			stack = stack[:len(stack)-1]
		}
	}

	assigned := make(map[string]bool, len(nodes))
	var components [][]string
	for index := len(order) - 1; index >= 0; index-- {
		start := order[index]
		if assigned[start] {
			continue
		}
		assigned[start] = true
		component := []string{}
		stack := []string{start}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, current)
			for _, next := range reverse[current] {
				if !assigned[next] {
					assigned[next] = true
					stack = append(stack, next)
				}
			}
		}
		if len(component) > 1 {
			sort.Strings(component)
			components = append(components, component)
		}
	}
	sort.Slice(components, func(i, j int) bool { return components[i][0] < components[j][0] })
	return components
}

func isImpactNode(origin, candidate *graph.Node) bool {
	if candidate == nil {
		return false
	}
	if candidate.Label.IsActionableSymbol() {
		return true
	}
	// File-to-file IMPORTS impact is useful when a file is the requested
	// origin, but file nodes remain excluded from symbol-level analysis.
	return origin != nil && origin.Label == graph.LabelFile && candidate.Label == graph.LabelFile
}

func normalizedRelationSet(requested, defaults []string) map[string]struct{} {
	if len(requested) == 0 {
		requested = defaults
	}
	result := make(map[string]struct{}, len(requested))
	for _, relation := range requested {
		relation = strings.ToUpper(strings.TrimSpace(relation))
		if relation != "" {
			result[relation] = struct{}{}
		}
	}
	return result
}

// ShortestPath returns the shortest directed path between two persisted nodes.
func (e *Engine) ShortestPath(ctx context.Context, request *PathRequest) (*PathResult, error) {
	if request == nil || request.Repo == "" || request.SourceNodeID == "" || request.TargetNodeID == "" {
		return nil, ErrInvalidRequest
	}
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
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
		result.Edges = append(result.Edges, publicGraphEdge(edge))
	}
	return result, nil
}

func publicGraphEdge(edge *graph.Edge) GraphEdge {
	return GraphEdge{
		ID: edge.ID, Source: edge.Source, Target: edge.Target, Type: string(edge.Type),
		Confidence: edge.Confidence(), Reason: edge.GetPropString("reason"),
	}
}

func internalDirection(direction TraverseDirection) (graph.TraverseDir, error) {
	normalized := strings.ToLower(strings.TrimSpace(string(direction)))
	switch normalized {
	case "", "out", "outgoing", "forward", "down", "downstream", "call", "calls", "callee", "callees":
		return graph.TraverseOut, nil
	case "in", "incoming", "reverse", "backward", "back", "up", "upstream", "caller", "callers":
		return graph.TraverseIn, nil
	case "both", "any", "all", "bidirectional", "two-way", "twoway":
		return graph.TraverseBoth, nil
	default:
		return graph.TraverseOut, fmt.Errorf("%w: unsupported traversal direction %q; use out, in, or both", ErrInvalidRequest, direction)
	}
}

func publicGraphNode(node *graph.Node) GraphNode {
	return GraphNode{
		ID: node.ID, Label: string(node.Label), Name: node.Name, FilePath: node.FilePath,
		Language: node.GetPropString("language"), StartLine: node.GetPropInt("startLine"),
		EndLine: node.GetPropInt("endLine"), Description: node.GetPropString("description"),
	}
}

type SourceScope string

const (
	SourceScopeCode SourceScope = "code"
	SourceScopeDocs SourceScope = "docs"
	SourceScopeAll  SourceScope = "all"
)

type SourceSearchRequest struct {
	Repo         string      `json:"repo"`
	Query        string      `json:"query"`
	Scope        SourceScope `json:"scope,omitempty" jsonschema:"search scope: code (default, including engineering configuration), docs, or all"`
	Limit        int         `json:"limit"`
	ContextLines int         `json:"contextLines"`
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
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
	scope := source.Scope(strings.ToLower(strings.TrimSpace(string(request.Scope))))
	if scope == "" {
		scope = source.ScopeCode
	}
	if scope != source.ScopeCode && scope != source.ScopeDocs && scope != source.ScopeAll {
		return nil, fmt.Errorf("%w: source scope must be code, docs, or all", ErrInvalidRequest)
	}
	graphStore := e.graphStore(request.Repo)
	if graphStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, request.Repo)
	}
	e.mu.RLock()
	index := e.sources[request.Repo]
	e.mu.RUnlock()
	if index == nil {
		e.mu.RLock()
		repoRoot := e.repoDirs[request.Repo]
		e.mu.RUnlock()
		index = source.New(repoRoot, graphStore.Repo())
		if err := index.Open(); err != nil {
			return nil, err
		}
		e.mu.Lock()
		e.sources[request.Repo] = index
		e.mu.Unlock()
	}
	matches, err := index.Search(ctx, request.Query, scope, request.Limit, request.ContextLines)
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
	operation := e.repositoryOperationLock(repo)
	operation.RLock()
	defer operation.RUnlock()
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
	e.mu.RLock()
	db, repoRoot := e.stores[repo], e.repoDirs[repo]
	e.mu.RUnlock()
	pipeline := semantic.NewEmbeddingPipelineWithDir(embedder, graphStore, db, config, repoRoot, quantized)
	pipelineResult, err := pipeline.RunDualModal(ctx, physicalRepo)
	if err != nil {
		return nil, fmt.Errorf("embed repository: %w", err)
	}
	vector := semantic.NewVectorSearchWithDir(embedder, db, graphStore, repoRoot)
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
	operation := e.repositoryOperationLock(repo)
	operation.RLock()
	defer operation.RUnlock()
	graphStore := e.graphStore(repo)
	if graphStore == nil {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	e.mu.RLock()
	db, repoRoot := e.stores[repo], e.repoDirs[repo]
	e.mu.RUnlock()
	vector := semantic.NewVectorSearchWithDir(embedder, db, graphStore, repoRoot)
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
	operation := e.repositoryOperationLock(request.Repo)
	operation.RLock()
	defer operation.RUnlock()
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
		e.mu.RLock()
		db, repoRoot := e.stores[request.Repo], e.repoDirs[request.Repo]
		e.mu.RUnlock()
		lexical, err = symbol.NewLexicalIndexWithDir(repoRoot, graphStore.Repo(), db)
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
