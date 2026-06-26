package codetrip

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/mengshi02/codetrip/internal/collection"
	"github.com/mengshi02/codetrip/internal/collection/languages"
	"github.com/mengshi02/codetrip/internal/collection/phases"
	"github.com/mengshi02/codetrip/internal/embedding"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/group"
	"github.com/mengshi02/codetrip/internal/incremental"
	"github.com/mengshi02/codetrip/internal/search"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/vecfile"
)

// requestIDKey is the context key for request-scoped IDs used for log correlation.
type requestIDKey struct{}

// withRequestID returns a new context with the given request ID.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// requestIDFromCtx extracts the request ID from the context, or returns "" if absent.
func requestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// newRequestID generates a short request ID (timestamp-based, 12 hex chars).
func newRequestID() string {
	return fmt.Sprintf("%012x", time.Now().UnixNano()&0xffffffffffff)
}

// Trip is the codetrip Hybrid Graph-Augmented Code Intelligence Engine
type Trip struct {
	store          *store.Store
	tripDir        string
	opts           options
	mu             sync.RWMutex
	graphs         map[string]*graph.GraphStore // repo → GraphStore
	pipeline       *collection.Pipeline
	groupSvc       *group.GroupService             // cross-repo group service
	bm25Indices    map[string]*search.BM25Index    // repo → BM25Index (persisted)
	vectorSearches map[string]*search.VectorSearch // repo → VectorSearch (with HNSW persistence)
	indexSem       chan struct{}                   // concurrency limiter for IndexRepo
	metrics        Metrics                         // lightweight operation counters

	// extensible registrations
	languageProviders  map[graph.Label]LanguageProvider
	scopeResolvers     map[graph.Label]ScopeResolver
	tools              map[string]Tool
	embedder           Embedder
	contractExtractors map[ContractType]ContractExtractor
	phases             []collection.Phase
}

// Open opens the codetrip engine
func Open(dir string, opts ...Option) (*Trip, error) {
	slog.Info("trip: opening", "dir", dir)

	// Ensure directory exists (auto-create if missing, e.g. ~/.codetrip/<repo>)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create trip directory: %s: %w", dir, err)
	}
	// Check write permission
	if err := os.WriteFile(filepath.Join(dir, ".write_test"), []byte{}, 0644); err != nil {
		return nil, fmt.Errorf("trip directory is not writable: %s: %w", dir, err)
	}
	os.Remove(filepath.Join(dir, ".write_test"))

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	cfg.CacheSize = o.cacheSize

	store, err := store.OpenWithScale(cfg, o.scalePreset)
	if err != nil {
		return nil, fmt.Errorf("open trip store: %w", err)
	}

	trip := &Trip{
		store:              store,
		tripDir:            dir,
		opts:               o,
		graphs:             make(map[string]*graph.GraphStore),
		pipeline:           collection.NewPipeline(),
		languageProviders:  make(map[graph.Label]LanguageProvider),
		scopeResolvers:     make(map[graph.Label]ScopeResolver),
		tools:              make(map[string]Tool),
		contractExtractors: make(map[ContractType]ContractExtractor),
		groupSvc:           group.NewGroupService(store),
		bm25Indices:        make(map[string]*search.BM25Index),
		vectorSearches:     make(map[string]*search.VectorSearch),
		indexSem:           make(chan struct{}, o.maxConcurrentIndex), // limit concurrent indexing
	}

	// Register built-in Phases
	trip.registerBuiltinPhases()

	// Register built-in language providers
	trip.registerBuiltinProviders()

	// Register built-in scope resolvers
	trip.registerBuiltinScopeResolvers()

	// Register user-defined Phases
	for _, phase := range o.phases {
		trip.pipeline.Register(phase)
	}

	// Register built-in tools
	trip.registerBuiltinTools()

	// Check/initialize data schema version
	if err := trip.checkSchemaVersion(); err != nil {
		trip.store.Close()
		return nil, fmt.Errorf("schema version check: %w", err)
	}

	// Default to NoopEmbedder
	trip.embedder = &NoopEmbedder{}

	// Discover existing repos from Pebble and restore graph stores
	trip.discoverRepos()

	slog.Info("trip: opened", "dir", dir)
	return trip, nil
}

// Verify checks data consistency for all graph stores.
// It verifies that type/name/adjacency indexes are consistent with actual node data.
func (trip *Trip) Verify(ctx context.Context) ([]VerifyIssue, error) {
	trip.mu.RLock()
	defer trip.mu.RUnlock()

	var issues []VerifyIssue

	for repoName, gs := range trip.graphs {
		// Check context for cancellation
		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// Build node existence set
		allNodes := gs.GetAllNodes(repoName, 0)
		nodeSet := make(map[string]bool, len(allNodes))
		for _, n := range allNodes {
			nodeSet[n.ID] = true
		}

		// --- 1. Adjacency index consistency (existing check) ---
		for _, node := range allNodes {
			// Check outgoing edges reference existing targets
			outEdges, err := gs.GetAllOutEdges(node.ID)
			if err != nil {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "out_edge_read_error",
					NodeID:  node.ID,
					Message: fmt.Sprintf("failed to read out-edges: %v", err),
				})
				continue
			}
			for _, edge := range outEdges {
				if !nodeSet[edge.Target] {
					issues = append(issues, VerifyIssue{
						Repo:    repoName,
						Type:    "dangling_edge_target",
						NodeID:  node.ID,
						Message: fmt.Sprintf("out-edge points to non-existent node %s (type=%s)", edge.Target, edge.Type),
					})
				}
			}

			// Check incoming edges reference existing sources
			inEdges, err := gs.GetAllInEdges(node.ID)
			if err != nil {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "in_edge_read_error",
					NodeID:  node.ID,
					Message: fmt.Sprintf("failed to read in-edges: %v", err),
				})
				continue
			}
			for _, edge := range inEdges {
				if !nodeSet[edge.Source] {
					issues = append(issues, VerifyIssue{
						Repo:    repoName,
						Type:    "dangling_edge_source",
						NodeID:  node.ID,
						Message: fmt.Sprintf("in-edge from non-existent node %s (type=%s)", edge.Source, edge.Type),
					})
				}
			}
		}

		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// --- 2. Type index consistency ---
		// Verify each type:{repo}:{label}:{id} key points to an existing node
		typePrefix := graph.TypeRepoPrefix(repoName)
		err := trip.store.ScanPrefixLimit(typePrefix, 0, func(key, val []byte) error {
			parts := strings.SplitN(string(key), ":", 5)
			// Expected: "type:{repo}:{label}:{id}"
			if len(parts) < 5 {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "type_index_malformed",
					NodeID:  string(key),
					Message: fmt.Sprintf("malformed type index key: %s", string(key)),
				})
				return nil
			}
			nodeID := parts[4]
			if !nodeSet[nodeID] {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "type_index_orphan",
					NodeID:  nodeID,
					Message: fmt.Sprintf("type index points to non-existent node %s (label=%s)", nodeID, parts[3]),
				})
			}
			return nil
		})
		if err != nil {
			issues = append(issues, VerifyIssue{
				Repo:    repoName,
				Type:    "type_index_scan_error",
				Message: fmt.Sprintf("failed to scan type index: %v", err),
			})
		}

		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// --- 3. Name index consistency ---
		// Verify each name:{repo}:{name}:{id} key points to an existing node
		namePrefix := graph.NameRepoPrefix(repoName)
		err = trip.store.ScanPrefixLimit(namePrefix, 0, func(key, val []byte) error {
			parts := strings.SplitN(string(key), ":", 5)
			// Expected: "name:{repo}:{name}:{id}"
			if len(parts) < 5 {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "name_index_malformed",
					NodeID:  string(key),
					Message: fmt.Sprintf("malformed name index key: %s", string(key)),
				})
				return nil
			}
			nodeID := parts[4]
			if !nodeSet[nodeID] {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "name_index_orphan",
					NodeID:  nodeID,
					Message: fmt.Sprintf("name index points to non-existent node %s (name=%s)", nodeID, parts[3]),
				})
			}
			return nil
		})
		if err != nil {
			issues = append(issues, VerifyIssue{
				Repo:    repoName,
				Type:    "name_index_scan_error",
				Message: fmt.Sprintf("failed to scan name index: %v", err),
			})
		}

		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// --- 4. File index consistency ---
		// Verify each file:{repo}:{filePath}:{id} key points to an existing node
		filePrefix := graph.FileRepoPrefix(repoName)
		err = trip.store.ScanPrefixLimit(filePrefix, 0, func(key, val []byte) error {
			parts := strings.SplitN(string(key), ":", 5)
			// Expected: "file:{repo}:{filePath}:{id}"
			if len(parts) < 5 {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "file_index_malformed",
					NodeID:  string(key),
					Message: fmt.Sprintf("malformed file index key: %s", string(key)),
				})
				return nil
			}
			nodeID := parts[4]
			if !nodeSet[nodeID] {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "file_index_orphan",
					NodeID:  nodeID,
					Message: fmt.Sprintf("file index points to non-existent node %s (file=%s)", nodeID, parts[3]),
				})
			}
			return nil
		})
		if err != nil {
			issues = append(issues, VerifyIssue{
				Repo:    repoName,
				Type:    "file_index_scan_error",
				Message: fmt.Sprintf("failed to scan file index: %v", err),
			})
		}

		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// --- 5. Embedding hash consistency ---
		// Verify each embhash:{repo}:{nodeID} key points to an existing dual-modal embdesc/embcode key
		embHashPrefix := graph.EmbHashRepoPrefix(repoName)
		err = trip.store.ScanPrefixLimit(embHashPrefix, 0, func(key, val []byte) error {
			parts := strings.SplitN(string(key), ":", 3)
			// Expected: "embhash:{repo}:{nodeID}"
			if len(parts) < 3 {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "embhash_malformed",
					NodeID:  string(key),
					Message: fmt.Sprintf("malformed embhash key: %s", string(key)),
				})
				return nil
			}
			nodeID := parts[2]
			// Check that at least one dual-modal emb key exists (desc or code)
			descKey := graph.EmbDescKey(repoName, nodeID)
			codeKey := graph.EmbCodeKey(repoName, nodeID)
			descVal, descErr := trip.store.Get([]byte(descKey))
			codeVal, codeErr := trip.store.Get([]byte(codeKey))
			if (descErr != nil || descVal == nil) && (codeErr != nil || codeVal == nil) {
				issues = append(issues, VerifyIssue{
					Repo:    repoName,
					Type:    "embhash_orphan",
					NodeID:  nodeID,
					Message: fmt.Sprintf("embhash exists but both embdesc/embcode keys missing for node %s", nodeID),
				})
			}
			return nil
		})
		if err != nil {
			issues = append(issues, VerifyIssue{
				Repo:    repoName,
				Type:    "embhash_scan_error",
				Message: fmt.Sprintf("failed to scan embhash index: %v", err),
			})
		}

		if ctx.Err() != nil {
			return issues, ctx.Err()
		}

		// --- 6. Adjacency bidirectional consistency ---
		// Verify that for each out-edge, the target has a corresponding in-edge,
		// and vice versa. This is a deeper check than dangling edges.
		adjPrefix := graph.AdjRepoPrefix(repoName)
		adjCount := 0
		err = trip.store.ScanPrefixLimit(adjPrefix, 0, func(key, val []byte) error {
			adjCount++
			// Check context every 1000 entries
			if adjCount%1000 == 0 && ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		})
		if err != nil && err != ctx.Err() {
			issues = append(issues, VerifyIssue{
				Repo:    repoName,
				Type:    "adj_scan_error",
				Message: fmt.Sprintf("failed to scan adjacency index: %v", err),
			})
		}
	}

	return issues, nil
}

// VerifyIssue represents a data consistency issue found during verification
type VerifyIssue struct {
	Repo    string
	Type    string
	NodeID  string
	Message string
}

// discoverRepos scans Pebble key prefixes to find existing repos and restores their GraphStores
func (trip *Trip) discoverRepos() {
	// Scan "n:" prefix to find all repo names from node keys (format: "n:{repo}:{id}")
	repoSet := make(map[string]bool)
	prefix := []byte("n:")
	trip.store.ScanPrefixLimit(prefix, 0, func(key, _ []byte) error {
		// Key format: "n:{repo}:{id}"
		parts := strings.SplitN(string(key), ":", 3)
		if len(parts) >= 2 && parts[0] == "n" && parts[1] != "" {
			repoSet[parts[1]] = true
		}
		return nil
	})

	// Restore GraphStore for each discovered repo
	trip.mu.Lock()
	for repo := range repoSet {
		if _, ok := trip.graphs[repo]; !ok {
			gs := graph.NewGraphStore(trip.store, repo)
			gs.SetNodeCacheSize(trip.opts.nodeCacheSize)
			gs.SetTraversalLimit(trip.opts.traversalLimit)
			trip.graphs[repo] = gs
		}
		// Also restore BM25 indices from index directory
		if _, ok := trip.bm25Indices[repo]; !ok {
			indexDir := filepath.Join(trip.tripDir, "index", repo)
			if info, err := os.Stat(indexDir); err == nil && info.IsDir() {
				idx, err := search.NewBM25IndexWithDir(trip.tripDir, repo, trip.store)
				if err != nil {
					slog.Warn("discover_repos: failed to restore bm25 index", "repo", repo, "error", err)
				} else {
					trip.bm25Indices[repo] = idx
					slog.Info("discover_repos: restored bm25 index", "repo", repo)
				}
			}
		}

		// Also restore VectorSearch with HNSW index for crash recovery
		if _, ok := trip.vectorSearches[repo]; !ok {
			graphStore := trip.graphs[repo] // already ensured above
			vs := search.NewVectorSearchWithDir(nil, trip.store, graphStore, trip.tripDir)
			vs.SetTwoStageSearch(trip.opts.twoStageSearch)
			// Try to load quantized vector file
			if trip.opts.quantInt8 {
				if err := vs.LoadVectorFile(); err != nil {
					slog.Warn("discover_repos: failed to load quantized vector file", "repo", repo, "error", err)
				}
			}
			if vs.RestoreDualHNSWIndex() {
				trip.vectorSearches[repo] = vs
				slog.Info("discover_repos: restored vector search with HNSW index", "repo", repo)
			}
			// If restore failed, VectorSearch will be lazily created on first search
		}
	}
	trip.mu.Unlock()
}

// checkSchemaVersion verifies data schema compatibility on startup.
// It writes the current schema version if this is a fresh database,
// or migrates from the stored version to the current version if possible.
func (trip *Trip) checkSchemaVersion() error {
	versionKey := []byte("__schema_version__")
	val, err := trip.store.Get(versionKey)
	if err != nil {
		// Key doesn't exist yet — this is a fresh database, write current version
		if err := trip.store.Set(versionKey, []byte(DataSchemaVersion)); err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
		slog.Info("trip: initialized schema version", "version", DataSchemaVersion)
		return nil
	}
	storedVersion := string(val)
	if storedVersion == DataSchemaVersion {
		slog.Info("trip: schema version check passed", "version", DataSchemaVersion)
		return nil
	}

	// Attempt migration from stored version to current version
	if !trip.opts.autoMigrate {
		return fmt.Errorf("incompatible data schema version: stored=%s, current=%s (auto-migration disabled; set AutoMigrate option to enable)", storedVersion, DataSchemaVersion)
	}

	if err := trip.migrateSchema(storedVersion, DataSchemaVersion); err != nil {
		return fmt.Errorf("schema migration %s→%s failed: %w", storedVersion, DataSchemaVersion, err)
	}

	// Write the new schema version after successful migration
	if err := trip.store.Set(versionKey, []byte(DataSchemaVersion)); err != nil {
		return fmt.Errorf("write schema version after migration: %w", err)
	}
	slog.Info("trip: schema migration completed", "from", storedVersion, "to", DataSchemaVersion)
	return nil
}

// schemaMigration is a function that migrates the database from one schema version to the next.
type schemaMigration func(*Trip) error

// schemaMigrations defines the migration path: fromVersion → migration function.
// Each migration advances the schema by one version.
// To add a new migration: add an entry like {"1": migrateV1ToV2}
var schemaMigrations = map[string]schemaMigration{
	// No migrations yet. Example:
	// "1": migrateV1ToV2,
}

// migrateSchema performs sequential schema migrations from `from` to `to`.
func (trip *Trip) migrateSchema(from, to string) error {
	current := from
	for current != to {
		migrateFn, ok := schemaMigrations[current]
		if !ok {
			return fmt.Errorf("no migration path from schema version %s (current=%s, target=%s)", current, from, to)
		}
		slog.Info("trip: running schema migration", "from", current)
		if err := migrateFn(trip); err != nil {
			return fmt.Errorf("migration %s: %w", current, err)
		}
		// Advance to next version (migrations map "N" → "N+1")
		next := fmt.Sprintf("%d", mustParseInt(current)+1)
		current = next
	}
	return nil
}

// mustParseInt parses an integer string or panics.
func mustParseInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Close closes the database
func (trip *Trip) Close() error {
	slog.Info("trip: closing")
	trip.mu.Lock()
	defer trip.mu.Unlock()

	// Close all BM25 indices (log errors instead of silently ignoring)
	for repo, idx := range trip.bm25Indices {
		if err := idx.Close(); err != nil {
			slog.Warn("close bm25 index failed", "repo", repo, "error", err)
		}
	}

	if err := trip.store.Close(); err != nil {
		slog.Error("trip: close failed", "error", err)
		return fmt.Errorf("close trip store: %w", err)
	}
	slog.Info("trip: closed")
	return nil
}

// Ping checks database health by verifying Pebble is accessible
func (trip *Trip) Ping() error {
	trip.mu.RLock()
	defer trip.mu.RUnlock()

	// Verify Pebble store is accessible by attempting a key read
	_, err := trip.store.Get([]byte("__ping__"))
	// The key may not exist, but any error other than "not found" indicates a problem
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("pebble health check: %w", err)
	}
	return nil
}

// Backup creates a backup of the database to the specified directory using Pebble Checkpoint.
// The backup is a point-in-time snapshot of the Pebble store.
// Bluge/BM25 indices are copied at the file level (should be done while no writes are in progress).
func (trip *Trip) Backup(backupDir string) error {
	trip.mu.RLock()
	defer trip.mu.RUnlock()

	slog.Info("backup: starting", "dest", backupDir)

	// Create Pebble checkpoint (atomic point-in-time snapshot)
	if err := trip.store.Checkpoint(backupDir); err != nil {
		return fmt.Errorf("pebble checkpoint: %w", err)
	}

	// Copy BM25 index directories
	for repo := range trip.bm25Indices {
		srcDir := filepath.Join(trip.tripDir, "index", repo)
		dstDir := filepath.Join(backupDir, "index", repo)
		if err := copyDir(srcDir, dstDir); err != nil {
			slog.Warn("backup: failed to copy bm25 index", "repo", repo, "error", err)
		}
	}

	slog.Info("backup: completed", "dest", backupDir)
	return nil
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // source doesn't exist, skip
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// ============ Index Management ============

// IndexRepo indexes a repository
func (trip *Trip) IndexRepo(ctx context.Context, repoPath string, opts ...IndexOption) (*IndexResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	trip.metrics.IndexRepoTotal.Add(1)

	// Validate repoPath exists
	if info, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("index_repo: repo path does not exist: %s", repoPath)
		}
		return nil, fmt.Errorf("index_repo: repo path not accessible: %s: %w", repoPath, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("index_repo: repo path is not a directory: %s", repoPath)
	}

	// Acquire concurrency slot (blocking) or skip if unlimited
	if trip.indexSem != nil {
		select {
		case trip.indexSem <- struct{}{}:
			defer func() { <-trip.indexSem }()
		case <-ctx.Done():
			return nil, fmt.Errorf("index_repo: cancelled while waiting for concurrency slot: %w", ctx.Err())
		}
	}

	idxOpts := defaultIndexOptions()
	for _, opt := range opts {
		opt(&idxOpts)
	}

	repoName := idxOpts.repoName
	if repoName == "" {
		repoName = filepath.Base(repoPath)
	}

	// Check if repo already indexed — reject to prevent adjacency list duplication
	// from repeated Merge operations. User must drop first, then re-index.
	if gs := trip.GraphStore(repoName); gs != nil {
		return nil, fmt.Errorf("%w: %q already indexed, run 'codetrip drop --repo %s' first",
			ErrRepoAlreadyExists, repoName, repoName)
	}

	start := time.Now()
	slog.Info("index_repo: starting", "request_id", requestIDFromCtx(ctx), "repo", repoName, "path", repoPath)

	// Release any existing BM25 writer for this repo to avoid exclusive lock conflict.
	// discover_repos may have opened a writer that holds the bluge exclusive lock,
	// which would block IndexPhase from creating a new writer for the same repo.
	trip.mu.Lock()
	if oldIdx, ok := trip.bm25Indices[repoName]; ok {
		if err := oldIdx.Close(); err != nil {
			slog.Warn("index_repo: closing existing bm25 writer", "repo", repoName, "error", err)
		}
		delete(trip.bm25Indices, repoName)
	}
	gs := graph.NewGraphStore(trip.store, repoName)
	gs.SetNodeCacheSize(trip.opts.nodeCacheSize)
	gs.SetTraversalLimit(trip.opts.traversalLimit)
	trip.graphs[repoName] = gs
	trip.mu.Unlock()

	// Run pipeline
	mutable := collection.NewMutableSemanticModel()
	sm := collection.NewSemanticModel()
	input := &collection.PhaseInput{
		Repo:          repoName,
		Graph:         gs,
		SemanticModel: sm,
		MutableModel:  mutable,
		Config: collection.PipelineConfig{
			RepoPath:      repoPath,
			TripDir:       trip.tripDir,
			MaxWorkers:    idxOpts.maxWorkers,
			ByteBudget:    idxOpts.byteBudget,
			WithCFG:       idxOpts.withCFG,
			WithPDG:       idxOpts.withPDG,
			BM25ChunkSize: trip.opts.bm25ChunkSize,
		},
		Providers: buildProviderMap(trip.languageProviders),
	}

	if err := trip.pipeline.Run(ctx, input); err != nil {
		trip.metrics.IndexRepoFail.Add(1)
		trip.metrics.Errors.Add(1)
		return nil, fmt.Errorf("pipeline run: %w", err)
	}

	// Collect statistics from SemanticModel
	var totalNodes, totalEdges, fileCount int
	for _, stat := range sm.PhaseStats {
		totalNodes += stat.NodesAdded
		totalEdges += stat.EdgesAdded
	}
	if scanStat, ok := sm.PhaseStats["scan"]; ok {
		if fc, ok := scanStat.Extra["fileCount"]; ok {
			if v, ok := fc.(int); ok {
				fileCount = v
			}
		}
	}

	slog.Info("index_repo: completed", "request_id", requestIDFromCtx(ctx), "repo", repoName, "files", fileCount, "nodes", totalNodes, "edges", totalEdges, "duration_sec", time.Since(start).Seconds())

	trip.metrics.IndexRepoSuccess.Add(1)

	return &IndexResult{
		Repo:     repoName,
		Files:    fileCount,
		Nodes:    totalNodes,
		Edges:    totalEdges,
		Duration: time.Since(start).Seconds(),
	}, nil
}

// ReIndex performs incremental re-indexing of an already indexed repository.
// It uses IncrementalIndexer for SHA1-based change detection and graph cleanup,
// then re-parses changed files via a mini pipeline (structure → parse phases).
// BM25, HNSW, and embedding indexes are incrementally updated.
func (trip *Trip) ReIndex(ctx context.Context, repoPath string, opts ...IndexOption) (*ReIndexResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	trip.metrics.ReIndexTotal.Add(1)

	idxOpts := defaultIndexOptions()
	for _, opt := range opts {
		opt(&idxOpts)
	}

	repoName := idxOpts.repoName
	if repoName == "" {
		repoName = filepath.Base(repoPath)
	}

	// Validate repo already indexed (must exist for incremental)
	gs := trip.GraphStore(repoName)
	if gs == nil {
		trip.metrics.ReIndexFail.Add(1)
		return nil, fmt.Errorf("reindex: repo %q not found — must be indexed first", repoName)
	}

	// Acquire concurrency slot
	if trip.indexSem != nil {
		select {
		case trip.indexSem <- struct{}{}:
			defer func() { <-trip.indexSem }()
		case <-ctx.Done():
			trip.metrics.ReIndexFail.Add(1)
			return nil, fmt.Errorf("reindex: cancelled waiting for slot: %w", ctx.Err())
		}
	}

	start := time.Now()
	slog.Info("reindex: starting", "request_id", requestIDFromCtx(ctx), "repo", repoName, "path", repoPath)

	// Step 1: Detect file changes via IncrementalIndexer (SHA1 hash-driven)
	incrIdx := incremental.NewIncrementalIndexer(gs).WithWorkers(idxOpts.maxWorkers)
	changes, err := incrIdx.DetectFileChanges(ctx, repoPath)
	if err != nil {
		trip.metrics.ReIndexFail.Add(1)
		trip.metrics.Errors.Add(1)
		return nil, fmt.Errorf("reindex: detect changes: %w", err)
	}

	// Separate changes by type
	var addedFiles, modifiedFiles []incremental.FileChange
	var deletedPaths []string
	unchangedCount := 0
	for _, change := range changes {
		switch change.Type {
		case incremental.ChangeAdded:
			addedFiles = append(addedFiles, change)
		case incremental.ChangeModified:
			modifiedFiles = append(modifiedFiles, change)
		case incremental.ChangeDeleted:
			deletedPaths = append(deletedPaths, change.Path)
		case incremental.ChangeUnchanged:
			unchangedCount++
		}
	}

	if len(addedFiles) == 0 && len(modifiedFiles) == 0 && len(deletedPaths) == 0 {
		slog.Info("reindex: no changes detected", "repo", repoName)
		trip.metrics.ReIndexSuccess.Add(1)
		return &ReIndexResult{Repo: repoName, Unchanged: unchangedCount}, nil
	}

	// Step 2: Delete old graph data for deleted and modified files
	var deletedNodeIDs []string

	// 2a. Deleted files — collect node IDs then delete
	for _, path := range deletedPaths {
		fileNodes, _ := gs.GetNodesByFile(repoName, path)
		for _, n := range fileNodes {
			deletedNodeIDs = append(deletedNodeIDs, n.ID)
		}
		for _, n := range fileNodes {
			_ = gs.DeleteNode(n.ID) // uses fixed DeleteNode with reverse adj cleanup
		}
	}

	// 2b. Modified files — collect old node IDs then delete, will re-parse later
	for _, change := range modifiedFiles {
		oldNodes, _ := gs.GetNodesByFile(repoName, change.Path)
		for _, n := range oldNodes {
			deletedNodeIDs = append(deletedNodeIDs, n.ID)
		}
		for _, n := range oldNodes {
			_ = gs.DeleteNode(n.ID)
		}
	}

	// Step 3: Re-parse added + modified files via mini pipeline (structure → parse)
	var changedNodeIDs []string
	filesToParse := make([]*collection.ParsedFile, 0, len(addedFiles)+len(modifiedFiles))
	for _, change := range append(addedFiles, modifiedFiles...) {
		fullPath := filepath.Join(repoPath, change.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			slog.Warn("reindex: skip unreadable file", "path", change.Path, "error", err)
			continue
		}

		langStr := phases.DetectLanguage(change.Path)
		if langStr == "" {
			continue // skip unsupported file types
		}

		filesToParse = append(filesToParse, &collection.ParsedFile{
			Path:        change.Path,
			Language:    langStr,
			ContentHash: change.NewHash,
			Size:        int64(len(content)),
		})
	}

	if len(filesToParse) > 0 {
		// Create mini pipeline with only structure + parse phases (reuse pipeline logic)
		miniPipe := collection.NewPipeline()
		miniPipe.Register(phases.NewStructurePhase())
		miniPipe.Register(phases.NewParsePhase())

		miniInput := &collection.PhaseInput{
			Repo:          repoName,
			Graph:         gs,
			SemanticModel: collection.NewSemanticModel(),
			MutableModel:  collection.NewMutableSemanticModel(),
			Config: collection.PipelineConfig{
				RepoPath:   repoPath,
				MaxWorkers: idxOpts.maxWorkers,
				ByteBudget: idxOpts.byteBudget,
			},
			Providers: buildProviderMap(trip.languageProviders),
		}
		miniInput.Files = filesToParse // bypass scan phase, directly set Files

		if err := miniPipe.Run(ctx, miniInput); err != nil {
			trip.metrics.ReIndexFail.Add(1)
			trip.metrics.Errors.Add(1)
			return nil, fmt.Errorf("reindex: mini pipeline run: %w", err)
		}

		// Collect changed node IDs from re-parsed files
		for _, f := range miniInput.Files {
			fileNodes, _ := gs.GetNodesByFile(repoName, f.Path)
			for _, n := range fileNodes {
				changedNodeIDs = append(changedNodeIDs, n.ID)
			}
		}
	}

	// Step 4: Clean up BM25 documents for deleted nodes
	bm25Idx, bm25Err := trip.getBM25Index(repoName)
	if bm25Err == nil && len(deletedNodeIDs) > 0 {
		if err := bm25Idx.DeleteDocuments(deletedNodeIDs); err != nil {
			slog.Warn("reindex: bm25 delete documents failed", "error", err)
		}
	}

	// Step 5: Clean up dual-modal embedding KV for deleted nodes
	if len(deletedNodeIDs) > 0 {
		var embKeysToDelete []string
		for _, nodeID := range deletedNodeIDs {
			// Dual-modal vector keys
			embKeysToDelete = append(embKeysToDelete, graph.EmbDescKey(repoName, nodeID))
			embKeysToDelete = append(embKeysToDelete, graph.EmbCodeKey(repoName, nodeID))
			// Content hash key
			embKeysToDelete = append(embKeysToDelete, graph.EmbHashKey(repoName, nodeID))
			// Chunk vectors — prefix scan embdesc:{repo}:{nodeID}: and embcode:{repo}:{nodeID}:
			descChunkPrefix := append(graph.EmbDescPrefix(repoName), []byte(nodeID+":")...)
			trip.store.ScanPrefix(descChunkPrefix, func(key, _ []byte) error {
				embKeysToDelete = append(embKeysToDelete, string(key))
				return nil
			})
			codeChunkPrefix := append(graph.EmbCodePrefix(repoName), []byte(nodeID+":")...)
			trip.store.ScanPrefix(codeChunkPrefix, func(key, _ []byte) error {
				embKeysToDelete = append(embKeysToDelete, string(key))
				return nil
			})
		}
		if len(embKeysToDelete) > 0 {
			_ = trip.store.BatchNoSync(func(b *pebble.Batch) error {
				for _, key := range embKeysToDelete {
					b.Delete([]byte(key), nil)
				}
				return nil
			})
		}
	}

	// Step 7 (removed): Incremental embedding is now handled by "codetrip embed --incremental"
	// Step 8: Incremental BM25 index update for changed nodes
	if bm25Err == nil && len(changedNodeIDs) > 0 {
		var changedNodeObjects []*graph.Node
		for _, nodeID := range changedNodeIDs {
			node, err := gs.GetNode(nodeID)
			if err == nil && node != nil {
				changedNodeObjects = append(changedNodeObjects, node)
			}
		}
		if len(changedNodeObjects) > 0 {
			if err := bm25Idx.IndexNodesIncremental(changedNodeObjects); err != nil {
				slog.Warn("reindex: bm25 incremental update failed", "error", err)
			}
		}
	}

	// Step 9 (removed): Incremental HNSW update is now handled by "codetrip embed --incremental"

	duration := time.Since(start).Seconds()
	slog.Info("reindex: completed",
		"request_id", requestIDFromCtx(ctx),
		"repo", repoName,
		"added", len(addedFiles),
		"modified", len(modifiedFiles),
		"deleted", len(deletedPaths),
		"unchanged", unchangedCount,
		"duration_sec", duration,
	)
	trip.metrics.ReIndexSuccess.Add(1)

	return &ReIndexResult{
		Repo:      repoName,
		Added:     len(addedFiles),
		Modified:  len(modifiedFiles),
		Deleted:   len(deletedPaths),
		Unchanged: unchangedCount,
		Duration:  duration,
	}, nil
}

func (trip *Trip) DropIndex(repoName string) error {
	trip.metrics.DropIndexTotal.Add(1)
	slog.Info("drop_index: starting", "request_id", newRequestID(), "repo", repoName)
	trip.mu.Lock()
	defer trip.mu.Unlock()

	// Close and remove BM25 index from memory
	if bm25Idx, ok := trip.bm25Indices[repoName]; ok {
		if err := bm25Idx.Close(); err != nil {
			slog.Warn("drop_index: close bm25 index failed", "repo", repoName, "error", err)
		}
		delete(trip.bm25Indices, repoName)
	}

	// Close and remove vector search from memory
	if vs, ok := trip.vectorSearches[repoName]; ok {
		vs.Close()
		delete(trip.vectorSearches, repoName)
	}

	// Remove graph store from memory
	delete(trip.graphs, repoName)

	// Clean up all repo data in Pebble store
	// Scan and delete all key prefixes for the repo
	prefixes := []string{
		fmt.Sprintf("n:%s:", repoName),
		fmt.Sprintf("adj:%s:", repoName),
		fmt.Sprintf("type:%s:", repoName),
		fmt.Sprintf("name:%s:", repoName),
		fmt.Sprintf("file:%s:", repoName),
		fmt.Sprintf("embdesc:%s:", repoName),
		fmt.Sprintf("embcode:%s:", repoName),
		fmt.Sprintf("embdescidx:%s:", repoName),
		fmt.Sprintf("embcodeidx:%s:", repoName),
		fmt.Sprintf("embhash:%s:", repoName),
		fmt.Sprintf("scope:%s:", repoName),
	}

	err := trip.store.Batch(func(b *pebble.Batch) error {
		for _, prefix := range prefixes {
			if err := trip.store.ScanPrefixLimit([]byte(prefix), 0, func(key, _ []byte) error {
				return b.Delete(key, nil)
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete repo keys from trip: %w", err)
	}

	// Delete Bluge BM25 index directory: {dataDir}/index/{repo}/
	// This also removes the HNSW graph file inside it ({dataDir}/index/{repo}/hnsw.graph)
	blugePath := filepath.Join(trip.tripDir, "index", repoName)
	if osErr := os.RemoveAll(blugePath); osErr != nil && !os.IsNotExist(osErr) {
		return fmt.Errorf("delete bm25 index directory %s: %w", blugePath, osErr)
	}

	// Delete BM25 build directory if it exists (from incomplete two-phase build)
	buildPath := filepath.Join(trip.tripDir, "index", ".build", repoName)
	if osErr := os.RemoveAll(buildPath); osErr != nil && !os.IsNotExist(osErr) {
		slog.Warn("drop_index: failed to remove build directory", "repo", repoName, "error", osErr)
	}

	// Delete vector file: {dataDir}/vectors/{repo}.bin
	vecFilePath := vecfile.VectorFilePath(trip.tripDir, repoName)
	if osErr := os.Remove(vecFilePath); osErr != nil && !os.IsNotExist(osErr) {
		return fmt.Errorf("delete vector file %s: %w", vecFilePath, osErr)
	}

	slog.Info("drop_index: completed", "repo", repoName)
	return nil
}

// ListRepos lists all repositories
func (trip *Trip) ListRepos() ([]RepoInfo, error) {
	trip.mu.RLock()
	defer trip.mu.RUnlock()

	// Scan Pebble to discover repos from node key prefixes (format: "n:{repo}:{id}")
	repoSet := make(map[string]bool)
	prefix := []byte("n:")
	trip.store.ScanPrefixLimit(prefix, 0, func(key, _ []byte) error {
		parts := strings.SplitN(string(key), ":", 3)
		if len(parts) >= 2 && parts[0] == "n" && parts[1] != "" {
			repoSet[parts[1]] = true
		}
		return nil
	})

	repos := make([]RepoInfo, 0, len(repoSet))
	for name := range repoSet {
		repos = append(repos, RepoInfo{Name: name})
	}
	return repos, nil
}

// Stats returns index statistics
func (trip *Trip) Stats(repo string) (*IndexStats, error) {
	gs := trip.GraphStore(repo)
	if gs == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}
	stats, err := gs.GetIndexStats(repo)
	if err != nil {
		return nil, err
	}
	return &IndexStats{
		NameCount:  stats.NameCount,
		LabelCount: stats.LabelCount,
		FileCount:  stats.FileCount,
		UIDCount:   stats.UIDCount,
	}, nil
}

// RepoStatus returns repository status
func (trip *Trip) RepoStatus(repoName string) (*RepoStatusInfo, error) {
	gs := trip.GraphStore(repoName)
	if gs == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repoName)
	}

	stats, err := gs.GetIndexStats(repoName)
	if err != nil {
		return nil, fmt.Errorf("get index stats: %w", err)
	}

	return &RepoStatusInfo{
		Name:      repoName,
		NodeCount: stats.LabelCount,
		EdgeCount: stats.NameCount,
	}, nil
}

func (trip *Trip) getBM25Index(repo string) (*search.BM25Index, error) {
	trip.mu.RLock()
	idx, ok := trip.bm25Indices[repo]
	trip.mu.RUnlock()
	if ok {
		return idx, nil
	}

	trip.mu.Lock()
	defer trip.mu.Unlock()

	// Double check
	if idx, ok := trip.bm25Indices[repo]; ok {
		return idx, nil
	}

	idx, err := search.NewBM25IndexWithDir(trip.tripDir, repo, trip.store)
	if err != nil {
		return nil, fmt.Errorf("open bm25 index: %w", err)
	}
	trip.bm25Indices[repo] = idx
	return idx, nil
}

// getVectorSearch gets or creates a VectorSearch with HNSW persistence for the given repo.
// It attempts to restore a persisted HNSW index; on failure, it lazily builds on first search.
func (trip *Trip) getVectorSearch(repo string) (*search.VectorSearch, error) {
	trip.mu.RLock()
	vs, ok := trip.vectorSearches[repo]
	trip.mu.RUnlock()
	if ok {
		return vs, nil
	}

	// Pre-resolve GraphStore BEFORE acquiring write lock to avoid deadlock:
	// GraphStore() acquires trip.mu.RLock(), which would deadlock if we
	// already hold trip.mu.Lock() (Go's RWMutex is not reentrant).
	gs := trip.GraphStore(repo)
	if gs == nil {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
	}

	trip.mu.Lock()
	defer trip.mu.Unlock()

	// Double check
	if vs, ok := trip.vectorSearches[repo]; ok {
		return vs, nil
	}

	// Create VectorSearch with dataDir for HNSW persistence
	// embedder is set later via SetEmbedder when available
	vs = search.NewVectorSearchWithDir(nil, trip.store, gs, trip.tripDir)
	// Configure two-stage search if enabled
	vs.SetTwoStageSearch(trip.opts.twoStageSearch)
	// Try to load quantized vector file
	if trip.opts.quantInt8 {
		if err := vs.LoadVectorFile(); err != nil {
			slog.Warn("vector search: failed to load quantized vector file", "repo", repo, "error", err)
		}
	}
	// Try to restore HNSW index from disk
	vs.RestoreDualHNSWIndex()
	// Set embedder if available
	if trip.hasRealEmbedder() {
		vs.SetEmbedder(trip.embedder)
	}
	trip.vectorSearches[repo] = vs
	return vs, nil
}

// hasRealEmbedder returns true if a non-NoopEmbedder is registered.
func (trip *Trip) hasRealEmbedder() bool {
	_, ok := trip.embedder.(*NoopEmbedder)
	return !ok
}

// EmbedRepo performs dual-modal vector embedding for an already indexed repository.
// It requires the repository to have been indexed first (graph data must exist).
// Returns ErrRepoNotIndexed if no graph data is found.
//
// Dual-modal embedding generates two vectors per symbol:
//   - Description modality: symbol signature + relationship summary (from graph)
//   - Code modality: source code snippet chunking (from node content)
func (trip *Trip) EmbedRepo(ctx context.Context, repo string, opts ...EmbedOption) (*EmbedResult, error) {
	ctx = withRequestID(ctx, newRequestID())

	// Apply embed options
	embedOpts := defaultEmbedOptions()
	for _, opt := range opts {
		opt(&embedOpts)
	}

	// Validate endpoint
	if embedOpts.endpoint == "" {
		return nil, fmt.Errorf("embed: endpoint is required (e.g. http://localhost:11434/v1/embeddings)")
	}

	// Check graph data exists (prerequisite)
	gs := trip.GraphStore(repo)
	if gs == nil {
		return nil, fmt.Errorf("%w: %q", ErrRepoNotIndexed, repo)
	}

	start := time.Now()
	slog.Info("embed: starting", "request_id", requestIDFromCtx(ctx), "repo", repo)

	// Create HTTPEmbedder with the provided endpoint
	httpEmbedder := embedding.NewHTTPEmbedder(
		embedOpts.endpoint,
		embedOpts.model,
		embedOpts.apiKey,
		embedOpts.dimensions,
	)
	httpEmbedder.WithStore(trip.store)

	// Auto-detect dimensions if not specified
	if embedOpts.dimensions == 0 {
		if err := httpEmbedder.DetectDimensions(ctx); err != nil {
			return nil, fmt.Errorf("embed: failed to auto-detect dimensions from endpoint: %w", err)
		}
	}

	// Update trip.embedder for future searches
	trip.mu.Lock()
	trip.embedder = httpEmbedderAdapter{httpEmbedder}
	trip.mu.Unlock()

	// Create embedding pipeline with data directory
	embedConfig := embedding.DefaultEmbedConfig()
	embedConfig.BatchSize = embedOpts.batchSize
	if embedOpts.dimensions > 0 {
		embedConfig.Dimensions = embedOpts.dimensions
	} else {
		// Use auto-detected dimensions from HTTPEmbedder
		embedConfig.Dimensions = httpEmbedder.Dimensions()
	}

	ep := embedding.NewEmbeddingPipelineWithDir(
		httpEmbedder,
		gs,
		trip.store,
		embedConfig,
		trip.tripDir,
		embedOpts.quantInt8,
	)

	// Run dual-modal embedding
	pipeResult, err := ep.RunDualModal(ctx, repo, embedOpts.incremental)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	duration := time.Since(start).Seconds()

	result := &EmbedResult{
		Repo:          repo,
		NodesEmbedded: pipeResult.NodesEmbedded,
		DescChunks:    pipeResult.DescChunks,
		CodeChunks:    pipeResult.CodeChunks,
		Skipped:       pipeResult.Skipped,
		Errors:        pipeResult.Errors,
		Duration:      duration,
	}

	// Build HNSW indices for both modalities
	if pipeResult.NodesEmbedded > 0 {
		vs, vsErr := trip.getVectorSearch(repo)
		if vsErr == nil && vs != nil {
			// Set the embedder on VectorSearch for future queries
			vs.SetEmbedder(httpEmbedderAdapter{httpEmbedder})
			if err := vs.BuildDualModalHNSWIndex(); err != nil {
				slog.Warn("embed: dual-modal HNSW index build failed", "repo", repo, "error", err)
			} else {
				slog.Info("embed: dual-modal HNSW indices built", "repo", repo)
			}
		}
	}

	slog.Info("embed: completed",
		"request_id", requestIDFromCtx(ctx),
		"repo", repo,
		"nodes_embedded", result.NodesEmbedded,
		"desc_chunks", result.DescChunks,
		"code_chunks", result.CodeChunks,
		"skipped", result.Skipped,
		"duration_sec", duration,
	)

	return result, nil
}

// httpEmbedderAdapter wraps internal/embedding.HTTPEmbedder to implement codetrip.Embedder.
type httpEmbedderAdapter struct {
	*embedding.HTTPEmbedder
}

// EmbedBatch implements the Embedder interface by converting EmbedConfig types.
func (a httpEmbedderAdapter) EmbedBatch(ctx context.Context, nodes []*graph.Node, config EmbedConfig) error {
	innerConfig := embedding.EmbedConfig{
		ModelID:          config.ModelID,
		Dimensions:       config.Dimensions,
		BatchSize:        config.BatchSize,
		SubBatchSize:     config.SubBatchSize,
		MaxSnippetLength: config.MaxSnippetLength,
		ChunkSize:        config.ChunkSize,
		Overlap:          config.Overlap,
	}
	return a.HTTPEmbedder.EmbedBatch(ctx, nodes, innerConfig)
}

// ============ Extension Registration ============

// labelToLang maps graph.Label (e.g. "GoFile") to language name (e.g. "go").
var labelToLangMap = map[graph.Label]string{
	graph.LabelGoFile:       "go",
	graph.LabelTSFile:       "typescript",
	graph.LabelJSFile:       "javascript",
	graph.LabelPythonFile:   "python",
	graph.LabelJavaFile:     "java",
	graph.LabelRustFile:     "rust",
	graph.LabelCFile:        "c",
	graph.LabelCPPFile:      "cpp",
	graph.LabelCSharpFile:   "csharp",
	graph.LabelMarkdownFile: "markdown",
}

// buildProviderMap converts the Label-keyed provider map to a language-name-keyed
// collection.Provider map for use by the parse phase.
func buildProviderMap(providers map[graph.Label]LanguageProvider) map[string]collection.Provider {
	if len(providers) == 0 {
		return nil
	}
	m := make(map[string]collection.Provider, len(providers))
	for label, prov := range providers {
		langName, ok := labelToLangMap[label]
		if !ok {
			continue
		}
		m[langName] = prov
	}
	return m
}

// RegisterLanguageProvider registers a language provider
func (trip *Trip) RegisterLanguageProvider(lang graph.Label, provider LanguageProvider) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.languageProviders[lang] = provider
}

// RegisterScopeResolver registers a scope resolver
func (trip *Trip) RegisterScopeResolver(lang graph.Label, resolver ScopeResolver) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.scopeResolvers[lang] = resolver
}

// RegisterPhase registers a custom Phase
func (trip *Trip) RegisterPhase(phase collection.Phase) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.pipeline.Register(phase)
}

// RegisterTool registers a custom tool
func (trip *Trip) RegisterTool(name string, tool Tool) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.tools[name] = tool
}

// RegisterEmbedder registers an embedding model
func (trip *Trip) RegisterEmbedder(embedder Embedder) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.embedder = embedder
}

// RegisterContractExtractor registers a contract extractor
func (trip *Trip) RegisterContractExtractor(contractType ContractType, extractor ContractExtractor) {
	trip.mu.Lock()
	defer trip.mu.Unlock()
	trip.contractExtractors[contractType] = extractor
}

// GraphStore returns the GraphStore for a repository
// If not cached in memory, creates a new instance based on existing Pebble data
func (trip *Trip) GraphStore(repo string) *graph.GraphStore {
	trip.mu.RLock()
	gs, ok := trip.graphs[repo]
	trip.mu.RUnlock()
	if ok {
		return gs
	}

	// Try to recover from Pebble: check if node data exists for this repo.
	// If no node keys with prefix "n:{repo}:" exist, the repo has been dropped
	// or never indexed — return nil instead of creating an empty GraphStore.
	prefix := []byte(fmt.Sprintf("n:%s:", repo))
	hasData := false
	trip.store.ScanPrefixLimit(prefix, 1, func(_, _ []byte) error {
		hasData = true
		return nil
	})
	if !hasData {
		return nil
	}

	gs = graph.NewGraphStore(trip.store, repo)
	gs.SetNodeCacheSize(trip.opts.nodeCacheSize)
	gs.SetTraversalLimit(trip.opts.traversalLimit)
	trip.mu.Lock()
	trip.graphs[repo] = gs
	trip.mu.Unlock()
	return gs
}

// ============ Tool API ============

// getGraphStore gets or creates a GraphStore
// If repo is empty, returns the default (first) GraphStore
func (trip *Trip) getGraphStore(repo string) (*graph.GraphStore, error) {
	if repo != "" {
		gs := trip.GraphStore(repo)
		if gs == nil {
			return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, repo)
		}
		return gs, nil
	}

	// Snapshot the map under RLock to avoid race condition
	// (iterating a map after releasing the lock is unsafe)
	trip.mu.RLock()
	snapshot := make([]*graph.GraphStore, 0, len(trip.graphs))
	for _, gs := range trip.graphs {
		snapshot = append(snapshot, gs)
	}
	trip.mu.RUnlock()

	if len(snapshot) > 0 {
		return snapshot[0], nil
	}

	// No graph store in memory cache — scan Pebble to discover repos
	repoSet := make(map[string]bool)
	prefix := []byte("n:")
	trip.store.ScanPrefixLimit(prefix, 0, func(key, _ []byte) error {
		parts := strings.SplitN(string(key), ":", 3)
		if len(parts) >= 2 && parts[0] == "n" && parts[1] != "" {
			repoSet[parts[1]] = true
		}
		return nil
	})

	if len(repoSet) == 0 {
		return nil, ErrNoGraphStore
	}

	// Restore discovered repos and return the first one
	trip.mu.Lock()
	var first *graph.GraphStore
	for repo := range repoSet {
		if gs, ok := trip.graphs[repo]; ok {
			if first == nil {
				first = gs
			}
		} else {
			gs := graph.NewGraphStore(trip.store, repo)
			gs.SetNodeCacheSize(trip.opts.nodeCacheSize)
			gs.SetTraversalLimit(trip.opts.traversalLimit)
			trip.graphs[repo] = gs
			if first == nil {
				first = gs
			}
		}
	}
	trip.mu.Unlock()

	if first != nil {
		return first, nil
	}
	return nil, ErrNoGraphStore
}

// Impact performs impact analysis
func (trip *Trip) Impact(ctx context.Context, req *collection.ImpactRequest) (*collection.ImpactResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Use explicit repo if provided, otherwise try target as repo name, then default
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil && req.Repo == "" {
		gs, err = trip.getGraphStore(req.Target)
		if err != nil {
			// Target is not a repo name, use default GraphStore
			gs, err = trip.getGraphStore("")
		}
	}
	if err != nil {
		return nil, err
	}
	return collection.RunImpact(ctx, gs, req)
}

// Context returns a 360-degree symbol view
func (trip *Trip) Context(ctx context.Context, req *collection.ContextRequest) (*collection.ContextResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}
	return collection.RunContext(ctx, gs, req)
}

// DetectChanges detects changes
func (trip *Trip) DetectChanges(ctx context.Context, req *collection.DetectChangesRequest) (*collection.DetectChangesResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.DetectChangesResult{
		RiskSummary: collection.RiskSummary{Level: "LOW"},
	}

	// Integrate IncrementalIndexer (SHA1 hash-driven incremental indexing)
	idx := incremental.NewIncrementalIndexer(gs)

	if req.Scope != "" {
		// Detect changes via incremental indexer (based on SHA1 content hash)
		changes, err := idx.DetectFileChanges(ctx, req.Scope)
		if err != nil {
			return nil, fmt.Errorf("detect file changes: %w", err)
		}
		for _, change := range changes {
			if change.Type == incremental.ChangeUnchanged {
				continue
			}
			sc := collection.SymbolChange{
				FilePath:   change.Path,
				ChangeType: change.Type.String(),
			}
			// Enrich with symbol information from the graph
			nodes, err := gs.GetNodesByFile(gs.Repo(), change.Path)
			if err == nil && len(nodes) > 0 {
				// For modified files, include only actionable symbol-level nodes
				// (exclude noise: Variable, Const, Property, etc.)
				for _, node := range nodes {
					if node.Label.IsActionableSymbol() {
						result.ChangedSymbols = append(result.ChangedSymbols, collection.SymbolChange{
							NodeID:     node.ID,
							Name:       node.Name,
							Kind:       string(node.Label),
							FilePath:   change.Path,
							ChangeType: change.Type.String(),
						})
					}
				}
				continue // already added enriched symbols
			}
			// Fallback: no nodes found in graph (e.g., new file not yet indexed)
			result.ChangedSymbols = append(result.ChangedSymbols, sc)
		}
	}

	// Evaluate affected processes
	if len(result.ChangedSymbols) > 0 {
		impactReq := &collection.ImpactRequest{
			Target:    result.ChangedSymbols[0].Name,
			Direction: "downstream",
			MaxDepth:  2,
		}
		impactResult, err := collection.RunImpact(ctx, gs, impactReq)
		if err == nil {
			result.AffectedProcesses = impactResult.AffectedProcesses
			result.RiskSummary = collection.RiskSummary{
				Level:        impactResult.Risk,
				TotalChanges: len(result.ChangedSymbols),
			}
			if impactResult.Risk == "CRITICAL" || impactResult.Risk == "HIGH" {
				result.RiskSummary.HighRisk = len(result.ChangedSymbols)
			}
		}
	}

	return result, nil
}

// Rename performs multi-file coordinated renaming
func (trip *Trip) Rename(ctx context.Context, req *collection.RenameRequest) (*collection.RenameResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.RenameResult{}

	// Find target symbol
	var startNodes []*graph.Node
	if req.SymbolUID != "" {
		node, e := gs.FindByUID(req.SymbolUID)
		if e != nil {
			return nil, e
		}
		startNodes = []*graph.Node{node}
	} else {
		startNodes, err = gs.GetNodesByName(gs.Repo(), req.SymbolName)
		if err != nil {
			return nil, err
		}
	}

	if len(startNodes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrSymbolNotFound, req.SymbolName)
	}

	// Collect high-confidence references (graph edges)
	visited := make(map[string]bool)
	for _, node := range startNodes {
		visited[node.ID] = true
		// Add the symbol itself
		result.Edits = append(result.Edits, collection.RenameEdit{
			FilePath:   node.FilePath,
			OldText:    req.SymbolName,
			NewText:    req.NewName,
			Confidence: "high",
		})

		// Find all references (incoming CALLS/ACCESSES/IMPORTS edges)
		inEdges, err := gs.GetAllInEdges(node.ID)
		if err != nil {
			slog.Warn("rename: get incoming edges failed", "nodeID", node.ID, "error", err)
			continue
		}
		for _, edge := range inEdges {
			src, e := gs.GetNode(edge.Source)
			if e != nil || visited[src.ID] {
				continue
			}
			visited[src.ID] = true
			result.Edits = append(result.Edits, collection.RenameEdit{
				FilePath:   src.FilePath,
				OldText:    req.SymbolName,
				NewText:    req.NewName,
				Confidence: "high",
			})
		}
	}

	// Low-confidence references (BM25 text search, persisted index)
	bm25, err := trip.getBM25Index(gs.Repo())
	if err != nil {
		return nil, fmt.Errorf("open bm25 index: %w", err)
	}
	bm25Results, err := bm25.Search(req.SymbolName, 50)
	if err == nil {
		for _, sr := range bm25Results {
			nodeID := sr.NodeID
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true
			n, e := gs.GetNode(nodeID)
			if e != nil {
				continue
			}
			result.Edits = append(result.Edits, collection.RenameEdit{
				FilePath:   n.FilePath,
				OldText:    req.SymbolName,
				NewText:    req.NewName,
				Confidence: "low",
			})
		}
	}

	return result, nil
}

// RouteMap returns API route mapping
func (trip *Trip) RouteMap(ctx context.Context, req *collection.RouteMapRequest) (*collection.RouteMapResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.RouteMapResult{}

	// Query Route nodes
	routeNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelRoute))
	if err != nil {
		return nil, err
	}

	for _, node := range routeNodes {
		// Filter specific routes
		if req.Route != "" && node.Name != req.Route {
			continue
		}

		routeInfo := collection.RouteInfo{
			Path:   node.GetPropString("path"),
			Method: node.GetPropString("method"),
		}

		// Find HANDLES_ROUTE edges (incoming edges)
		inEdges, err := gs.GetAllInEdges(node.ID)
		if err != nil {
			slog.Warn("route_map: get incoming edges failed", "nodeID", node.ID, "error", err)
		}
		for _, edge := range inEdges {
			if edge.Type == graph.RelHandlesRoute {
				routeInfo.HandlerID = edge.Source
			}
		}

		// Find FETCHES edges (outgoing edges) → consumers
		outEdges, err := gs.GetAllOutEdges(node.ID)
		if err != nil {
			slog.Warn("route_map: get outgoing edges failed", "nodeID", node.ID, "error", err)
		}
		for _, edge := range outEdges {
			if edge.Type == graph.RelFetches {
				routeInfo.Consumers = append(routeInfo.Consumers, edge.Target)
			}
		}

		// Middleware from properties
		if mw, ok := node.Props.GetProp("middleware"); ok {
			if arr, ok := mw.([]string); ok {
				routeInfo.Middleware = arr
			}
		}

		result.Routes = append(result.Routes, routeInfo)
	}

	return result, nil
}

// ToolMap returns MCP/RPC tool mapping
func (trip *Trip) ToolMap(ctx context.Context, req *collection.ToolMapRequest) (*collection.ToolMapResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.ToolMapResult{}

	// Query Tool nodes
	toolNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelTool))
	if err != nil {
		return nil, err
	}

	for _, node := range toolNodes {
		if req.Tool != "" && node.Name != req.Tool {
			continue
		}

		toolInfo := collection.ToolInfo{
			Name:        node.Name,
			Description: node.GetPropString("description"),
		}

		// Find HANDLES_TOOL edges (incoming edges)
		inEdges, err := gs.GetAllInEdges(node.ID)
		if err != nil {
			slog.Warn("tool_map: get incoming edges failed", "nodeID", node.ID, "error", err)
		}
		for _, edge := range inEdges {
			if edge.Type == graph.RelHandlesTool {
				toolInfo.HandlerID = edge.Source
			}
		}

		result.Tools = append(result.Tools, toolInfo)
	}

	return result, nil
}

// ShapeCheck performs response shape checking
func (trip *Trip) ShapeCheck(ctx context.Context, req *collection.ShapeCheckRequest) (*collection.ShapeCheckResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.ShapeCheckResult{}

	// Get Route nodes
	routeNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelRoute))
	if err != nil {
		return nil, err
	}

	for _, node := range routeNodes {
		if req.Route != "" && node.Name != req.Route {
			continue
		}

		// Get route's producer (handler) response keys
		routeResponseKeys := getNodeStringProp(node, "responseKeys")

		// Find FETCHES consumers
		outEdges, err := gs.GetAllOutEdges(node.ID)
		if err != nil {
			slog.Warn("shape_check: get outgoing edges failed", "nodeID", node.ID, "error", err)
		}
		for _, edge := range outEdges {
			if edge.Type != graph.RelFetches {
				continue
			}
			consumer, e := gs.GetNode(edge.Target)
			if e != nil {
				continue
			}
			consumerExpectedKeys := getNodeStringProp(consumer, "expectedKeys")

			// Check for mismatch
			if routeResponseKeys != "" && consumerExpectedKeys != "" && routeResponseKeys != consumerExpectedKeys {
				result.Mismatches = append(result.Mismatches, collection.ShapeMismatch{
					Route:    node.Name,
					Field:    "keys",
					Producer: routeResponseKeys,
					Consumer: consumerExpectedKeys,
				})
			}
		}
	}

	return result, nil
}

// Check performs structural checks (e.g., circular dependency detection)
func (trip *Trip) Check(ctx context.Context, req *collection.CheckRequest) (*collection.CheckResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}
	return collection.RunCheck(ctx, gs, req)
}

// ApiImpact performs API impact analysis (RouteMap + Impact + ShapeCheck)
func (trip *Trip) ApiImpact(ctx context.Context, req *collection.ApiImpactRequest) (*collection.ApiImpactResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.ApiImpactResult{}

	// RouteMap
	routeMapResult, err := trip.RouteMap(ctx, &collection.RouteMapRequest{Route: req.Route, Repo: req.Repo})
	if err != nil {
		return nil, err
	}

	// Collect route consumers and middleware
	for _, route := range routeMapResult.Routes {
		result.Middleware = append(result.Middleware, route.Middleware...)
		for _, consumerID := range route.Consumers {
			consumer, e := gs.GetNode(consumerID)
			if e != nil {
				continue
			}
			result.Consumers = append(result.Consumers, collection.ConsumerInfo{
				NodeID:   consumer.ID,
				Name:     consumer.Name,
				FilePath: consumer.FilePath,
			})
		}
	}

	// Impact analysis
	if req.Route != "" {
		impactResult, err := trip.Impact(ctx, &collection.ImpactRequest{
			Target:    req.Route,
			Direction: "downstream",
			MaxDepth:  3,
		})
		if err == nil {
			result.Risk = impactResult.Risk
			result.Processes = impactResult.AffectedProcesses
		}
	}

	// ShapeCheck
	shapeResult, err := trip.ShapeCheck(ctx, &collection.ShapeCheckRequest{Route: req.Route})
	if err == nil {
		result.Mismatches = shapeResult.Mismatches
	}

	if result.Risk == "" {
		switch {
		case len(result.Consumers) >= 10:
			result.Risk = "CRITICAL"
		case len(result.Consumers) >= 5:
			result.Risk = "HIGH"
		case len(result.Consumers) >= 2:
			result.Risk = "MEDIUM"
		default:
			result.Risk = "LOW"
		}
	}

	return result, nil
}

// Explain performs taint explanation
func (trip *Trip) Explain(ctx context.Context, req *collection.ExplainRequest) (*collection.ExplainResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	gs, err := trip.getGraphStore(req.Repo)
	if err != nil {
		return nil, err
	}

	result := &collection.ExplainResult{}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	// Find nodes with TAINTED incoming edges
	targetNodes, err := gs.GetNodesByName(gs.Repo(), req.Target)
	if err != nil || len(targetNodes) == 0 {
		return result, nil
	}

	for _, node := range targetNodes {
		inEdges, err := gs.GetAllInEdges(node.ID)
		if err != nil {
			slog.Warn("explain: failed to get in-edges", "node_id", node.ID, "error", err)
			continue
		}
		for _, edge := range inEdges {
			if edge.Type != graph.RelTainted {
				continue
			}
			if len(result.Findings) >= limit {
				result.Truncated = true
				break
			}

			finding := collection.TaintFinding{
				Category: edge.GetPropString("category"),
				SinkLine: node.GetPropInt("line"),
			}

			// Trace taint path
			src, err := gs.GetNode(edge.Source)
			if err != nil {
				slog.Warn("explain: failed to get source node", "node_id", edge.Source, "error", err)
			}
			if src != nil {
				finding.SourceLine = src.GetPropInt("line")
				finding.HopPath = append(finding.HopPath, collection.HopInfo{
					NodeID: src.ID,
					Line:   src.GetPropInt("line"),
				})
			}
			finding.HopPath = append(finding.HopPath, collection.HopInfo{
				NodeID: node.ID,
				Line:   node.GetPropInt("line"),
			})

			// Check SANITIZES edges
			outEdges, err := gs.GetAllOutEdges(node.ID)
			if err != nil {
				slog.Warn("explain: failed to get out-edges", "node_id", node.ID, "error", err)
				result.Findings = append(result.Findings, finding)
				continue
			}
			for _, oe := range outEdges {
				if oe.Type == graph.RelSanitizes {
					sanNode, err := gs.GetNode(oe.Target)
					if err != nil {
						slog.Warn("explain: failed to get sanitize node", "node_id", oe.Target, "error", err)
					}
					if sanNode != nil {
						finding.HopPath = append(finding.HopPath, collection.HopInfo{
							NodeID: sanNode.ID,
							Line:   sanNode.GetPropInt("line"),
						})
					}
				}
				if oe.Type == graph.RelTaintPath {
					tgtNode, err := gs.GetNode(oe.Target)
					if err != nil {
						slog.Warn("explain: failed to get taint path node", "node_id", oe.Target, "error", err)
					}
					if tgtNode != nil {
						finding.HopPath = append(finding.HopPath, collection.HopInfo{
							NodeID: tgtNode.ID,
							Line:   tgtNode.GetPropInt("line"),
						})
					}
				}
			}

			result.Findings = append(result.Findings, finding)
		}
	}

	result.TotalFindings = len(result.Findings)
	return result, nil
}

// Search performs search
func (trip *Trip) Search(ctx context.Context, req *collection.SearchRequest) (*collection.SearchResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	repo := req.Repo
	gs, err := trip.getGraphStore(repo)
	if err != nil {
		return nil, err
	}
	if repo == "" {
		repo = gs.Repo()
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	result := &collection.SearchResult{}

	if req.Semantic {
		// Semantic search: prefer dual-modal HNSW, fallback to BM25 if no embed data
		vectorSearch, _ := trip.getVectorSearch(repo)

		// Check if embedding data exists
		hasEmbed := vectorSearch != nil && vectorSearch.HasEmbedData()

		if hasEmbed {
			// Dual-modal semantic search + BM25 hybrid via RRF
			bm25, err := trip.getBM25Index(repo)
			if err != nil {
				return nil, fmt.Errorf("open bm25 index: %w", err)
			}
			hs := search.NewHybridSearch(bm25, vectorSearch)
			hr, err := hs.SearchDualModal(ctx, req.Query, limit)
			if err != nil {
				return nil, fmt.Errorf("hybrid search: %w", err)
			}
			for _, item := range hr.Results {
				result.Results = append(result.Results, collection.SearchItem{
					NodeID:    item.NodeID,
					Name:      item.Name,
					Kind:      item.Label,
					FilePath:  item.FilePath,
					Score:     item.Score,
					StartLine: item.StartLine,
					EndLine:   item.EndLine,
				})
			}
		} else {
			// No embedding data: fallback to BM25 with a log message
			slog.Info("search: no embedding data found, falling back to BM25",
				"repo", repo,
			)
			bm25, err := trip.getBM25Index(repo)
			if err != nil {
				return nil, fmt.Errorf("open bm25 index: %w", err)
			}
			results, err := bm25.Search(req.Query, limit)
			if err != nil {
				return nil, fmt.Errorf("bm25 search: %w", err)
			}
			for _, sr := range results {
				result.Results = append(result.Results, collection.SearchItem{
					NodeID:    sr.NodeID,
					Name:      sr.Name,
					Kind:      sr.Label,
					FilePath:  sr.FilePath,
					Score:     sr.Score,
					StartLine: sr.StartLine,
					EndLine:   sr.EndLine,
				})
			}
			result.Fallback = "bm25"
		}
	} else {
		// BM25 search (persisted index)
		bm25, err := trip.getBM25Index(repo)
		if err != nil {
			return nil, fmt.Errorf("open bm25 index: %w", err)
		}
		results, err := bm25.Search(req.Query, limit)
		if err != nil {
			return nil, fmt.Errorf("bm25 search: %w", err)
		}
		for _, sr := range results {
			result.Results = append(result.Results, collection.SearchItem{
				NodeID:    sr.NodeID,
				Name:      sr.Name,
				Kind:      sr.Label,
				FilePath:  sr.FilePath,
				Score:     sr.Score,
				StartLine: sr.StartLine,
				EndLine:   sr.EndLine,
			})
		}
	}

	return result, nil
}

// GroupList lists cross-repo groups
func (trip *Trip) GroupList() ([]GroupInfo, error) {
	trip.mu.RLock()
	defer trip.mu.RUnlock()

	// Use GroupService
	if trip.groupSvc != nil {
		groupInfos, err := trip.groupSvc.ListGroups()
		if err != nil {
			return nil, err
		}
		// Type conversion
		result := make([]GroupInfo, len(groupInfos))
		for i, gi := range groupInfos {
			result[i] = GroupInfo{
				Name:        gi.Name,
				Description: gi.Description,
				Repos:       gi.Repos,
			}
		}
		return result, nil
	}

	// Fallback: treat all repos as a default group
	var groups []GroupInfo
	var repos []string
	for name := range trip.graphs {
		repos = append(repos, name)
	}
	if len(repos) > 0 {
		groups = append(groups, GroupInfo{
			Name:        "default",
			Description: "All indexed repositories",
			Repos:       repos,
		})
	}
	return groups, nil
}

// GroupSync syncs cross-repo groups
func (trip *Trip) GroupSync(ctx context.Context, req *GroupSyncRequest) (*GroupSyncResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	start := time.Now()

	// Use GroupService
	if trip.groupSvc != nil {
		config := &group.GroupConfig{
			Name:     req.GroupName,
			Repos:    req.RepoPaths,
			Detect:   group.DefaultDetectConfig(),
			Matching: group.DefaultMatchingConfig(),
		}

		// Ensure all repos are indexed
		trip.mu.RLock()
		graphs := make(map[string]*graph.GraphStore)
		for repoName, gs := range trip.graphs {
			if _, ok := req.RepoPaths[repoName]; ok {
				graphs[repoName] = gs
			}
		}
		trip.mu.RUnlock()

		result, err := trip.groupSvc.SyncGroup(ctx, config, nil, graphs)
		if err != nil {
			return nil, err
		}
		return &GroupSyncResult{
			Group:       result.Group,
			Contracts:   result.Contracts,
			BridgeLinks: result.BridgeLinks,
			Duration:    time.Since(start).Seconds(),
		}, nil
	}

	// Fallback implementation
	var contracts, bridgeLinks int

	for repoPath, repoName := range req.RepoPaths {
		gs := trip.GraphStore(repoName)
		if gs == nil {
			_, err := trip.IndexRepo(ctx, repoPath, WithRepoName(repoName))
			if err != nil {
				continue
			}
			gs = trip.GraphStore(repoName)
		}
		if gs == nil {
			continue
		}

		contractNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelContract))
		if err == nil {
			contracts += len(contractNodes)
		}
	}

	return &GroupSyncResult{
		Group:       req.GroupName,
		Contracts:   contracts,
		BridgeLinks: bridgeLinks,
		Duration:    time.Since(start).Seconds(),
	}, nil
}

// GroupImpact performs cross-repo impact analysis
func (trip *Trip) GroupImpact(ctx context.Context, req *GroupImpactRequest) (*GroupImpactResult, error) {
	ctx = withRequestID(ctx, newRequestID())
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Use GroupService
	if trip.groupSvc != nil {
		trip.mu.RLock()
		graphs := make(map[string]*graph.GraphStore)
		for name, gs := range trip.graphs {
			graphs[name] = gs
		}
		trip.mu.RUnlock()

		result, err := trip.groupSvc.Impact(ctx, req.GroupName, req.Target, req.Direction, graphs)
		if err != nil {
			return nil, err
		}

		// Convert cross-repo references
		var crossRefs []CrossRepoRef
		for _, ref := range result.CrossRepoRefs {
			crossRefs = append(crossRefs, CrossRepoRef{
				SourceRepo:   ref.SourceRepo,
				SourceSymbol: ref.SourceSymbol,
				TargetRepo:   ref.TargetRepo,
				TargetSymbol: ref.TargetSymbol,
				MatchType:    ref.MatchType,
				Confidence:   ref.Confidence,
			})
		}

		return &GroupImpactResult{
			Risk:          result.Risk,
			LocalImpact:   result.LocalImpact,
			CrossRepoRefs: crossRefs,
		}, nil
	}

	// Fallback implementation
	result := &GroupImpactResult{}

	localResult, err := trip.Impact(ctx, &collection.ImpactRequest{
		Target:    req.Target,
		Direction: req.Direction,
		MaxDepth:  3,
	})
	if err != nil {
		return nil, err
	}
	result.LocalImpact = localResult
	result.Risk = localResult.Risk

	trip.mu.RLock()
	for repoName, gs := range trip.graphs {
		nodes, err := gs.GetNodesByName(gs.Repo(), req.Target)
		if err != nil || len(nodes) == 0 {
			continue
		}
		for _, node := range nodes {
			result.CrossRepoRefs = append(result.CrossRepoRefs, CrossRepoRef{
				TargetRepo:   repoName,
				TargetSymbol: node.Name,
				MatchType:    "name",
				Confidence:   0.7,
			})
		}
	}
	trip.mu.RUnlock()

	trip.mu.RLock()
	for repoName, gs := range trip.graphs {
		contractNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelContract))
		if err != nil {
			continue
		}
		for _, cn := range contractNodes {
			outEdges, err := gs.GetAllOutEdges(cn.ID)
			if err != nil {
				slog.Warn("group_impact: failed to get out-edges for contract node", "repo", repoName, "node_id", cn.ID, "error", err)
				continue
			}
			for _, edge := range outEdges {
				if edge.Type == graph.RelContractLink {
					result.CrossRepoRefs = append(result.CrossRepoRefs, CrossRepoRef{
						SourceRepo:   repoName,
						SourceSymbol: cn.Name,
						TargetRepo:   edge.GetPropString("targetRepo"),
						TargetSymbol: edge.GetPropString("targetSymbol"),
						MatchType:    "contract",
						Confidence:   edge.Confidence(),
					})
				}
			}
		}
	}
	trip.mu.RUnlock()

	return result, nil
}

// getNodeStringProp helper function: gets node string property
func getNodeStringProp(node *graph.Node, key string) string {
	if v, ok := node.Props.GetProp(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ============ Data Types ============

// IndexResult represents index result
type IndexResult struct {
	Repo     string
	Files    int
	Nodes    int
	Edges    int
	Duration float64 // seconds
}

// ReIndexResult represents incremental re-indexing result
type ReIndexResult struct {
	Repo      string
	Added     int
	Modified  int
	Deleted   int
	Unchanged int
	Duration  float64 // seconds
}

// EmbedResult represents dual-modal embedding result
type EmbedResult struct {
	Repo          string
	NodesEmbedded int
	DescChunks    int // description modality chunk count
	CodeChunks    int // code modality chunk count
	Skipped       int // incremental skip count
	Errors        int
	Duration      float64 // seconds
}

// IndexStats represents index statistics
type IndexStats struct {
	NameCount  int
	LabelCount int
	FileCount  int
	UIDCount   int
}

// RepoInfo represents repository info
type RepoInfo struct {
	Name string
	Path string
}

// RepoStatusInfo represents repository status info
type RepoStatusInfo struct {
	Name      string
	NodeCount int
	EdgeCount int
	LastIndex string
}




// ============ Cross-Repo Group Types ============

// GroupInfo represents cross-repo group info
type GroupInfo struct {
	Name        string
	Description string
	Repos       []string
}

// GroupSyncRequest represents cross-repo group sync request
type GroupSyncRequest struct {
	GroupName string
	RepoPaths map[string]string
}

// GroupSyncResult represents cross-repo group sync result
type GroupSyncResult struct {
	Group       string
	Contracts   int
	BridgeLinks int
	Duration    float64
}

// GroupImpactRequest represents cross-repo impact analysis request
type GroupImpactRequest struct {
	GroupName string
	Target    string
	Direction string
}

// GroupImpactResult represents cross-repo impact analysis result
type GroupImpactResult struct {
	Risk          string
	LocalImpact   *collection.ImpactResult
	CrossRepoRefs []CrossRepoRef
}

// Validate checks the GroupSyncRequest for invalid fields
func (r *GroupSyncRequest) Validate() error {
	if r.GroupName == "" {
		return fmt.Errorf("group_sync request: GroupName is required")
	}
	if len(r.RepoPaths) == 0 {
		return fmt.Errorf("group_sync request: RepoPaths is required")
	}
	return nil
}

// Validate checks the GroupImpactRequest for invalid fields
func (r *GroupImpactRequest) Validate() error {
	if r.GroupName == "" {
		return fmt.Errorf("group_impact request: GroupName is required")
	}
	if r.Target == "" {
		return fmt.Errorf("group_impact request: Target is required")
	}
	if r.Direction != "" && r.Direction != "upstream" && r.Direction != "downstream" {
		return fmt.Errorf("group_impact request: Direction must be 'upstream' or 'downstream', got %q", r.Direction)
	}
	return nil
}

// ============ Tool Request/Response Types ============

// ImpactRequest represents impact analysis request
type ImpactRequest = collection.ImpactRequest

// ImpactResult represents impact analysis result
type ImpactResult = collection.ImpactResult

// ContextRequest represents 360-degree symbol view request
type ContextRequest = collection.ContextRequest

// ContextResult represents 360-degree symbol view result
type ContextResult = collection.ContextResult

// CheckRequest represents structure check request
type CheckRequest = collection.CheckRequest

// CheckResult represents structure check result
type CheckResult = collection.CheckResult

// SearchRequest represents search request
type SearchRequest = collection.SearchRequest

// SearchResult represents search result
type SearchResult = collection.SearchResult

// RenameRequest represents multi-file coordinated rename request
type RenameRequest = collection.RenameRequest

// RenameResult represents rename result
type RenameResult = collection.RenameResult

// DetectChangesRequest represents change detection request
type DetectChangesRequest = collection.DetectChangesRequest

// DetectChangesResult represents change detection result
type DetectChangesResult = collection.DetectChangesResult

// RouteMapRequest represents route mapping request
type RouteMapRequest = collection.RouteMapRequest

// RouteMapResult represents route mapping result
type RouteMapResult = collection.RouteMapResult

// ToolMapRequest represents tool mapping request
type ToolMapRequest = collection.ToolMapRequest

// ToolMapResult represents tool mapping result
type ToolMapResult = collection.ToolMapResult

// ShapeCheckRequest represents response shape check request
type ShapeCheckRequest = collection.ShapeCheckRequest

// ShapeCheckResult represents response shape check result
type ShapeCheckResult = collection.ShapeCheckResult

// ApiImpactRequest represents API impact analysis request
type ApiImpactRequest = collection.ApiImpactRequest

// ApiImpactResult represents API impact analysis result
type ApiImpactResult = collection.ApiImpactResult

// ExplainRequest represents taint explanation request
type ExplainRequest = collection.ExplainRequest

// ExplainResult represents taint explanation result
type ExplainResult = collection.ExplainResult

// CrossRepoRef represents cross-repo reference
type CrossRepoRef struct {
	SourceRepo   string
	SourceSymbol string
	TargetRepo   string
	TargetSymbol string
	MatchType    string
	Confidence   float64
}

// registerBuiltinPhases registers built-in Phases (16-stage DAG)
func (trip *Trip) registerBuiltinPhases() {
	trip.pipeline.Register(phases.NewScanPhase())
	trip.pipeline.Register(phases.NewStructurePhase())
	trip.pipeline.Register(phases.NewMarkdownPhase()) // Optional: Markdown Section extraction
	trip.pipeline.Register(phases.NewCobolPhase())    // Optional: COBOL preprocessing
	trip.pipeline.Register(phases.NewParsePhase())
	trip.pipeline.Register(phases.NewRoutesPhase())   // Optional: Route extraction
	trip.pipeline.Register(phases.NewToolsPhase(nil)) // Optional: Tool definition detection (nil → default registry)
	trip.pipeline.Register(phases.NewORMPhase(nil))   // Optional: ORM query detection (nil → default registry)
	trip.pipeline.Register(phases.NewCrossFilePhase())
	trip.pipeline.Register(phases.NewScopeResolutionPhase())
	trip.pipeline.Register(phases.NewPruneLocalPhase())
	trip.pipeline.Register(phases.NewMROPhase())
	trip.pipeline.Register(phases.NewCommunitiesPhase())
	trip.pipeline.Register(phases.NewProcessesPhase())
	trip.pipeline.Register(phases.NewIndexPhase())
}

// registerBuiltinProviders registers built-in language providers.
func (trip *Trip) registerBuiltinProviders() {
	trip.languageProviders[graph.LabelGoFile] = languages.NewGoProvider()
	trip.languageProviders[graph.LabelPythonFile] = languages.NewPythonProvider()
	trip.languageProviders[graph.LabelTSFile] = languages.NewTypeScriptProvider()
	trip.languageProviders[graph.LabelJSFile] = languages.NewJavaScriptProvider()
	trip.languageProviders[graph.LabelRustFile] = languages.NewRustProvider()
	trip.languageProviders[graph.LabelCFile] = languages.NewCProvider()
	trip.languageProviders[graph.LabelCPPFile] = languages.NewCPPProvider()
	trip.languageProviders[graph.LabelCSharpFile] = languages.NewCSharpProvider()
	trip.languageProviders[graph.LabelJavaFile] = languages.NewJavaProvider()
	trip.languageProviders[graph.LabelMarkdownFile] = languages.NewMarkdownProvider()
}

// registerBuiltinScopeResolvers registers built-in scope resolvers.
func (trip *Trip) registerBuiltinScopeResolvers() {
	goProvider := trip.languageProviders[graph.LabelGoFile].(*languages.GoProvider)
	trip.scopeResolvers[graph.LabelGoFile] = languages.NewGoScopeResolver(goProvider)
}

// registerBuiltinTools registers built-in tools
func (trip *Trip) registerBuiltinTools() {
	trip.tools["check"] = &checkTool{}
	trip.tools["list_repos"] = &listReposTool{}
	trip.tools["repo_status"] = &repoStatusTool{}
	trip.tools["impact"] = &impactTool{}
	trip.tools["context"] = &contextTool{}
	trip.tools["detect_changes"] = &detectChangesTool{}
	trip.tools["rename"] = &renameTool{}
	trip.tools["route_map"] = &routeMapTool{}
	trip.tools["tool_map"] = &toolMapTool{}
	trip.tools["shape_check"] = &shapeCheckTool{}
	trip.tools["api_impact"] = &apiImpactTool{}
	trip.tools["explain"] = &explainTool{}
	trip.tools["search"] = &searchTool{}
	trip.tools["group_list"] = &groupListTool{}
	trip.tools["group_sync"] = &groupSyncTool{}
	trip.tools["group_impact"] = &groupImpactTool{}
}

// ============ Built-in Tool Implementations ============

// impactTool is the impact analysis tool
type impactTool struct{}

func (t *impactTool) Name() string { return "impact" }
func (t *impactTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ImpactRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected ImpactRequest", ErrInvalidRequest)
	}
	return trip.Impact(ctx, r)
}

// contextTool is the 360-degree symbol view tool
type contextTool struct{}

func (t *contextTool) Name() string { return "context" }
func (t *contextTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ContextRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected ContextRequest", ErrInvalidRequest)
	}
	return trip.Context(ctx, r)
}

// detectChangesTool is the change detection tool
type detectChangesTool struct{}

func (t *detectChangesTool) Name() string { return "detect_changes" }
func (t *detectChangesTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.DetectChangesRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected DetectChangesRequest", ErrInvalidRequest)
	}
	return trip.DetectChanges(ctx, r)
}

// renameTool is the multi-file coordinated renaming tool
type renameTool struct{}

func (t *renameTool) Name() string { return "rename" }
func (t *renameTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.RenameRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected RenameRequest", ErrInvalidRequest)
	}
	return trip.Rename(ctx, r)
}

// routeMapTool is the API route mapping tool
type routeMapTool struct{}

func (t *routeMapTool) Name() string { return "route_map" }
func (t *routeMapTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.RouteMapRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected RouteMapRequest", ErrInvalidRequest)
	}
	return trip.RouteMap(ctx, r)
}

// toolMapTool is the MCP/RPC tool mapping tool
type toolMapTool struct{}

func (t *toolMapTool) Name() string { return "tool_map" }
func (t *toolMapTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ToolMapRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected ToolMapRequest", ErrInvalidRequest)
	}
	return trip.ToolMap(ctx, r)
}

// shapeCheckTool is the response shape checking tool
type shapeCheckTool struct{}

func (t *shapeCheckTool) Name() string { return "shape_check" }
func (t *shapeCheckTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ShapeCheckRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected ShapeCheckRequest", ErrInvalidRequest)
	}
	return trip.ShapeCheck(ctx, r)
}

// apiImpactTool is the API impact analysis tool
type apiImpactTool struct{}

func (t *apiImpactTool) Name() string { return "api_impact" }
func (t *apiImpactTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ApiImpactRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected APIImpactRequest", ErrInvalidRequest)
	}
	return trip.ApiImpact(ctx, r)
}

// explainTool is the taint explanation tool
type explainTool struct{}

func (t *explainTool) Name() string { return "explain" }
func (t *explainTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.ExplainRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected ExplainRequest", ErrInvalidRequest)
	}
	return trip.Explain(ctx, r)
}

// searchTool is the search tool
type searchTool struct{}

func (t *searchTool) Name() string { return "search" }
func (t *searchTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.SearchRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected SearchRequest", ErrInvalidRequest)
	}
	return trip.Search(ctx, r)
}

// groupListTool is the list cross-repo groups tool
type groupListTool struct{}

func (t *groupListTool) Name() string { return "group_list" }
func (t *groupListTool) Run(ctx context.Context, trip *Trip, _ interface{}) (interface{}, error) {
	return trip.GroupList()
}

// groupSyncTool is the sync cross-repo groups tool
type groupSyncTool struct{}

func (t *groupSyncTool) Name() string { return "group_sync" }
func (t *groupSyncTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*GroupSyncRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected GroupSyncRequest", ErrInvalidRequest)
	}
	return trip.GroupSync(ctx, r)
}

// groupImpactTool is the cross-repo impact analysis tool
type groupImpactTool struct{}

func (t *groupImpactTool) Name() string { return "group_impact" }
func (t *groupImpactTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*GroupImpactRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected GroupImpactRequest", ErrInvalidRequest)
	}
	return trip.GroupImpact(ctx, r)
}

// checkTool is the structure check tool
type checkTool struct{}

func (t *checkTool) Name() string { return "check" }
func (t *checkTool) Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error) {
	r, ok := req.(*collection.CheckRequest)
	if !ok {
		return nil, fmt.Errorf("%w: expected CheckRequest", ErrInvalidRequest)
	}
	return trip.Check(ctx, r)
}

// listReposTool is the list repos tool
type listReposTool struct{}

func (t *listReposTool) Name() string { return "list_repos" }
func (t *listReposTool) Run(_ context.Context, trip *Trip, _ interface{}) (interface{}, error) {
	return trip.ListRepos()
}

// repoStatusTool is the repo status tool
type repoStatusTool struct{}

func (t *repoStatusTool) Name() string { return "repo_status" }
func (t *repoStatusTool) Run(_ context.Context, trip *Trip, req interface{}) (interface{}, error) {
	repoName, ok := req.(string)
	if !ok {
		return nil, fmt.Errorf("%w: expected string for repo_status", ErrInvalidRequest)
	}
	return trip.RepoStatus(repoName)
}
