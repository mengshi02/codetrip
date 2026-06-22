package search

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blugelabs/bluge"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

const defaultBM25ChunkSize = 10000 // Default chunk size for BatchIndexChunked (10K nodes per batch)

// BM25Index is a Bluge-based BM25 full-text index
// High-performance design:
//   - Bluge built-in BM25 scoring
//   - Batch indexing (BatchIndex)
//   - Chunked batch indexing for large repos (BatchIndexChunked)
//   - Incremental index updates (IndexNodesIncremental)
//   - Multi-field weighted search (name boost=3.0, filePath boost=1.5)
type BM25Index struct {
	store       *store.Store // Retained for compatibility, no longer used for FTS
	repo        string
	dataDir     string
	blugeWriter *bluge.Writer
	mu          sync.RWMutex
}

// NewBM25IndexWithDir creates a persistent BM25 index using two-phase build.
// Phase 1: Build index into a temporary directory ({dataDir}/index/.build/{repo}/).
// Phase 2: On successful build, atomic rename to the final directory ({dataDir}/index/{repo}/).
// This prevents corrupted indexes from a crash during build — if .build/ directory exists
// on startup, it indicates an incomplete build and is cleaned up automatically.
func NewBM25IndexWithDir(dataDir string, repo string, store *store.Store) (*BM25Index, error) {
	idx := &BM25Index{
		repo:    repo,
		dataDir: dataDir,
		store:   store,
	}

	// Clean up any stale build directory from a previous incomplete build
	buildDir := filepath.Join(dataDir, "index", ".build", repo)
	if _, err := os.Stat(buildDir); err == nil {
		slog.Warn("bm25: cleaning up stale build directory from incomplete previous build", "repo", repo, "path", buildDir)
		if err := os.RemoveAll(buildDir); err != nil {
			return nil, fmt.Errorf("remove stale build directory %s: %w", buildDir, err)
		}
	}

	blugePath := filepath.Join(dataDir, "index", repo)

	// If no existing index, build into temporary directory first (two-phase build)
	if _, err := os.Stat(blugePath); os.IsNotExist(err) {
		return idx.buildTwoPhase(dataDir, repo, buildDir, blugePath)
	}

	// Existing index found — open it directly
	config := bluge.DefaultConfig(blugePath)
	writer, err := bluge.OpenWriter(config)
	if err != nil {
		return nil, fmt.Errorf("open bluge writer at %s: %w", blugePath, err)
	}

	idx.blugeWriter = writer
	return idx, nil
}

// buildTwoPhase builds a BM25 index in a temporary directory and atomically renames
// it to the final location on success. This prevents corrupted indexes from crashes.
func (idx *BM25Index) buildTwoPhase(dataDir, repo, buildDir, finalDir string) (*BM25Index, error) {
	// Phase 1: Build into temporary directory
	config := bluge.DefaultConfig(buildDir)
	writer, err := bluge.OpenWriter(config)
	if err != nil {
		return nil, fmt.Errorf("create bluge writer at build dir %s: %w", buildDir, err)
	}

	idx.blugeWriter = writer
	slog.Info("bm25: two-phase build started", "repo", repo, "build_dir", buildDir)
	return idx, nil
}

// FinalizeBuild atomically moves the index from the build directory to the final directory.
// This must be called after all indexing operations are complete (e.g., after BatchIndexChunked).
// If the build directory doesn't exist (index was opened directly), this is a no-op.
func (idx *BM25Index) FinalizeBuild() error {
	if idx.dataDir == "" {
		return nil // no dataDir configured, nothing to finalize
	}

	buildDir := filepath.Join(idx.dataDir, "index", ".build", idx.repo)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return nil // no build directory — index was opened directly
	}

	finalDir := filepath.Join(idx.dataDir, "index", idx.repo)

	// Close writer before rename (Bluge needs to flush all data)
	if idx.blugeWriter != nil {
		if err := idx.blugeWriter.Close(); err != nil {
			return fmt.Errorf("close bluge writer before rename: %w", err)
		}
		idx.blugeWriter = nil
	}

	// Remove final directory if it exists (e.g., from a previous build)
	if _, err := os.Stat(finalDir); err == nil {
		if err := os.RemoveAll(finalDir); err != nil {
			return fmt.Errorf("remove existing final directory %s: %w", finalDir, err)
		}
	}

	// Atomic rename (same filesystem ensures atomicity)
	if err := os.Rename(buildDir, finalDir); err != nil {
		return fmt.Errorf("rename build dir to final dir: %w", err)
	}

	slog.Info("bm25: two-phase build finalized", "repo", idx.repo, "final_dir", finalDir)

	// Re-open the writer at the final location
	config := bluge.DefaultConfig(finalDir)
	writer, err := bluge.OpenWriter(config)
	if err != nil {
		return fmt.Errorf("reopen bluge writer at %s: %w", finalDir, err)
	}
	idx.blugeWriter = writer
	return nil
}

// BM25Result represents a BM25 search result
type BM25Result struct {
	NodeID    string
	FilePath  string
	Name      string
	Label     string
	Score     float64
	StartLine int
	EndLine   int
}

// IndexNode indexes a single graph node
func (idx *BM25Index) IndexNode(node *graph.Node) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}

	doc := NewSearchDocument(node)
	blugeDoc := idx.documentToBluge(doc)
	return idx.blugeWriter.Update(blugeDoc.ID(), blugeDoc)
}

// DeleteNode deletes a node from the index
func (idx *BM25Index) DeleteNode(nodeID string) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}

	return idx.blugeWriter.Delete(bluge.Identifier(nodeID))
}

// BatchIndex performs batch indexing of nodes (high-performance write)
func (idx *BM25Index) BatchIndex(nodes []*graph.Node) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}
	if len(nodes) == 0 {
		return nil
	}

	start := time.Now()
	batch := bluge.NewBatch()
	for _, node := range nodes {
		doc := NewSearchDocument(node)
		blugeDoc := idx.documentToBluge(doc)
		batch.Update(blugeDoc.ID(), blugeDoc)
	}
	if err := idx.blugeWriter.Batch(batch); err != nil {
		return err
	}

	slog.Debug("bm25 batch index",
		"repo", idx.repo,
		"nodes", len(nodes),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// IndexNodesIncremental performs incremental index update for changed nodes.
// Only indexes new or updated nodes — does not touch unchanged documents.
// This is much faster than full BatchIndex for incremental re-indexing scenarios
// (e.g., when only a few files changed in a repo).
func (idx *BM25Index) IndexNodesIncremental(nodes []*graph.Node) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}
	if len(nodes) == 0 {
		return nil
	}

	start := time.Now()
	batch := bluge.NewBatch()
	for _, node := range nodes {
		doc := NewSearchDocument(node)
		blugeDoc := idx.documentToBluge(doc)
		batch.Update(blugeDoc.ID(), blugeDoc)
	}
	if err := idx.blugeWriter.Batch(batch); err != nil {
		return err
	}

	slog.Debug("bm25 incremental index",
		"repo", idx.repo,
		"nodes", len(nodes),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// DeleteDocuments removes documents from the BM25 index by their node IDs.
// Used during incremental re-indexing to remove stale documents for deleted/modified nodes.
// Each node ID is the Bluge document ID (set during indexing as doc.NodeID).
func (idx *BM25Index) DeleteDocuments(nodeIDs []string) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}
	if len(nodeIDs) == 0 {
		return nil
	}

	start := time.Now()
	batch := bluge.NewBatch()
	for _, nodeID := range nodeIDs {
		batch.Delete(bluge.NewDocument(nodeID).ID())
	}
	if err := idx.blugeWriter.Batch(batch); err != nil {
		return err
	}

	slog.Debug("bm25 delete documents",
		"repo", idx.repo,
		"documents", len(nodeIDs),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// BatchIndexChunked performs batch indexing in chunks to limit memory usage.
// For 1M+ node repos, indexing all nodes in a single batch can consume excessive memory.
// Chunked indexing breaks the batch into smaller segments (default: 10000 nodes per chunk),
// each committed independently so memory is released between chunks.
func (idx *BM25Index) BatchIndexChunked(nodes []*graph.Node, chunkSize int) error {
	if idx.blugeWriter == nil {
		return fmt.Errorf("bluge writer not initialized")
	}
	if len(nodes) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = defaultBM25ChunkSize
	}

	start := time.Now()
	totalIndexed := 0

	for i := 0; i < len(nodes); i += chunkSize {
		end := i + chunkSize
		if end > len(nodes) {
			end = len(nodes)
		}

		chunk := nodes[i:end]
		batch := bluge.NewBatch()
		for _, node := range chunk {
			doc := NewSearchDocument(node)
			blugeDoc := idx.documentToBluge(doc)
			batch.Update(blugeDoc.ID(), blugeDoc)
		}
		if err := idx.blugeWriter.Batch(batch); err != nil {
			return fmt.Errorf("bm25 chunked index chunk %d-%d: %w", i, end, err)
		}
		totalIndexed += len(chunk)
	}

	slog.Debug("bm25 chunked index",
		"repo", idx.repo,
		"total_nodes", totalIndexed,
		"chunk_size", chunkSize,
		"chunks", (len(nodes)+chunkSize-1)/chunkSize,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// Search executes a BM25 search
// Uses Bluge multi-field weighted query:
//   - FieldName boost=3.0 (symbol name has highest weight)
//   - FieldFilePath boost=1.5 (file path has secondary weight)
//   - FieldContent boost=1.0 (composite text has default weight)
func (idx *BM25Index) Search(queryStr string, limit int) ([]BM25Result, error) {
	if idx.blugeWriter == nil {
		return nil, nil
	}

	start := time.Now()

	// Pre-tokenize query text
	tokens := tokenize(queryStr)
	if len(tokens) == 0 {
		return nil, nil
	}
	preparedQuery := prepareSearchText(queryStr)

	// Build multi-field weighted query
	// Use BooleanQuery's AddShould to implement OR query
	booleanQuery := bluge.NewBooleanQuery()

	// Name field query, boost=3.0
	nameQuery := bluge.NewMatchQuery(preparedQuery).SetField(FieldName)
	nameQuery.SetBoost(3.0)
	booleanQuery.AddShould(nameQuery)

	// FilePath field query, boost=1.5
	filePathQuery := bluge.NewMatchQuery(preparedQuery).SetField(FieldFilePath)
	filePathQuery.SetBoost(1.5)
	booleanQuery.AddShould(filePathQuery)

	// Content field query, default boost
	contentQuery := bluge.NewMatchQuery(preparedQuery).SetField(FieldContent)
	booleanQuery.AddShould(contentQuery)

	// Execute search
	searchRequest := bluge.NewTopNSearch(limit, booleanQuery).WithStandardAggregations()

	reader, err := idx.blugeWriter.Reader()
	if err != nil {
		return nil, fmt.Errorf("create bluge reader: %w", err)
	}
	defer reader.Close()

	documentMatchIterator, err := reader.Search(context.Background(), searchRequest)
	if err != nil {
		return nil, fmt.Errorf("bluge search: %w", err)
	}

	// Parse search results
	var results []BM25Result
	match, err := documentMatchIterator.Next()
	for err == nil && match != nil {
		var r BM25Result
		r.Score = match.Score

		// Load stored fields
		err = match.VisitStoredFields(func(field string, value []byte) bool {
			switch field {
			case "_id":
				// Document ID, also store a copy in NodeID
				r.NodeID = string(value)
			case FieldNodeID:
				r.NodeID = string(value)
			case "_originalName":
				r.Name = string(value)
			case FieldName:
				// Fallback: if _originalName not stored, use the tokenized name
				if r.Name == "" {
					r.Name = string(value)
				}
			case FieldLabel:
				r.Label = string(value)
			case "_originalFilePath":
				r.FilePath = string(value)
			case FieldFilePath:
				// Fallback: if _originalFilePath not stored, use the tokenized path
				if r.FilePath == "" {
					r.FilePath = string(value)
				}
			case FieldStartLine:
				// Numeric field needs decoding
				if num, decodeErr := bluge.DecodeNumericFloat64(value); decodeErr == nil {
					r.StartLine = int(num)
				}
			case FieldEndLine:
				if num, decodeErr := bluge.DecodeNumericFloat64(value); decodeErr == nil {
					r.EndLine = int(num)
				}
			}
			return true
		})
		if err != nil {
			break
		}

		results = append(results, r)
		match, err = documentMatchIterator.Next()
	}

	if err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	slog.Debug("bm25 search",
		"repo", idx.repo,
		"query", queryStr,
		"results", len(results),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return results, nil
}

// Close closes the Bluge index
func (idx *BM25Index) Close() error {
	if idx.blugeWriter != nil {
		return idx.blugeWriter.Close()
	}
	return nil
}

// Repo returns the repository name for the index
func (idx *BM25Index) Repo() string {
	return idx.repo
}

// DocumentCount returns the number of documents in the index
func (idx *BM25Index) DocumentCount() (uint64, error) {
	if idx.blugeWriter == nil {
		return 0, nil
	}

	reader, err := idx.blugeWriter.Reader()
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	// Use Reader.Count() to directly get document count
	return reader.Count()
}

// documentToBluge converts SearchDocument to bluge.Document
func (idx *BM25Index) documentToBluge(doc *SearchDocument) *bluge.Document {
	blugeDoc := bluge.NewDocument(doc.NodeID)

	// NodeID field: stored
	blugeDoc.AddField(bluge.NewKeywordField(FieldNodeID, doc.NodeID).StoreValue())

	// Name field: pre-tokenized text for search matching (camelCase/snake_case split),
	// plus original name stored as keyword for display in search results.
	// This dual approach ensures identifiers like "UserRepository" are split into
	// ["user", "repository"] tokens during indexing, so that search queries using
	// prepareSearchText can match them via Bluge's token matching.
	preparedName := prepareSearchText(doc.Name)
	blugeDoc.AddField(bluge.NewTextField(FieldName, preparedName))
	blugeDoc.AddField(bluge.NewKeywordField("_originalName", doc.Name).StoreValue())

	// Label field: keyword stored
	blugeDoc.AddField(bluge.NewKeywordField(FieldLabel, doc.Label).StoreValue())

	// FilePath field: pre-tokenized text for search, original stored for display
	if doc.FilePath != "" {
		preparedFilePath := prepareSearchText(doc.FilePath)
		blugeDoc.AddField(bluge.NewTextField(FieldFilePath, preparedFilePath))
		blugeDoc.AddField(bluge.NewKeywordField("_originalFilePath", doc.FilePath).StoreValue())
	}

	// Content field: already pre-tokenized via prepareSearchText in NewSearchDocument
	if doc.Content != "" {
		blugeDoc.AddField(bluge.NewTextField(FieldContent, doc.Content))
	}

	// Numeric fields: stored
	if doc.StartLine > 0 {
		blugeDoc.AddField(bluge.NewNumericField(FieldStartLine, float64(doc.StartLine)).StoreValue())
	}
	if doc.EndLine > 0 {
		blugeDoc.AddField(bluge.NewNumericField(FieldEndLine, float64(doc.EndLine)).StoreValue())
	}

	return blugeDoc
}
