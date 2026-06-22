package incremental

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ParseFunc file parse callback function type
// Provided by caller (Trip layer injects Pipeline's Parse logic)
type ParseFunc func(ctx context.Context, filePath string, content []byte, contentHash string) error

// EmbedFunc incremental embedding callback function type
type EmbedFunc func(ctx context.Context, nodeIDs []string) error

// BM25Func incremental BM25 index callback function type
// Called with nodes that need to be added/updated in the BM25 full-text index.
type BM25Func func(ctx context.Context, nodes []*graph.Node) error

// DeleteNodesFunc callback function type for notifying about deleted node IDs.
// Called with node IDs that have been removed from the graph during incremental re-indexing.
// The caller (Trip layer) uses this to clean up BM25 documents, HNSW vectors, and embedding data.
type DeleteNodesFunc func(ctx context.Context, nodeIDs []string) error

// IncrementalIndexer incremental indexer
// Strategy: SHA1 content hash driven, only recompute changed parts
type IncrementalIndexer struct {
	graph       *graph.GraphStore
	parseFn     ParseFunc       // file parse callback
	embedFn     EmbedFunc       // incremental embedding callback
	bm25Fn      BM25Func        // incremental BM25 index callback
	deleteNodes DeleteNodesFunc // callback for deleted node IDs (BM25/HNSW cleanup)
	workers     int             // parallel worker count
	batchSize   int             // batch processing size
}

// NewIncrementalIndexer creates incremental indexer
func NewIncrementalIndexer(gs *graph.GraphStore) *IncrementalIndexer {
	return &IncrementalIndexer{
		graph:     gs,
		workers:   4,
		batchSize: 16,
	}
}

// WithParseFunc sets file parse callback
func (idx *IncrementalIndexer) WithParseFunc(fn ParseFunc) *IncrementalIndexer {
	idx.parseFn = fn
	return idx
}

// WithEmbedFunc sets incremental embedding callback
func (idx *IncrementalIndexer) WithEmbedFunc(fn EmbedFunc) *IncrementalIndexer {
	idx.embedFn = fn
	return idx
}

// WithBM25Func sets incremental BM25 index callback.
// When set, changed nodes are passed to this callback for incremental BM25 index updates
// instead of requiring a full index rebuild.
func (idx *IncrementalIndexer) WithBM25Func(fn BM25Func) *IncrementalIndexer {
	idx.bm25Fn = fn
	return idx
}

// WithDeleteNodesFunc sets callback for deleted node IDs.
// When set, node IDs that are removed during incremental re-indexing are passed to this
// callback so the caller can clean up associated BM25 documents, HNSW vectors, and embeddings.
func (idx *IncrementalIndexer) WithDeleteNodesFunc(fn DeleteNodesFunc) *IncrementalIndexer {
	idx.deleteNodes = fn
	return idx
}

// WithWorkers sets parallel worker count
func (idx *IncrementalIndexer) WithWorkers(n int) *IncrementalIndexer {
	if n > 0 {
		idx.workers = n
	}
	return idx
}

// ChangeType change type
type ChangeType int

const (
	ChangeAdded     ChangeType = iota // added file
	ChangeModified                    // modified file
	ChangeDeleted                     // deleted file
	ChangeUnchanged                   // unchanged
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeDeleted:
		return "deleted"
	case ChangeUnchanged:
		return "unchanged"
	default:
		return "unknown"
	}
}

// FileChange file change
type FileChange struct {
	Path    string
	Type    ChangeType
	OldHash string
	NewHash string
}

// IndexResult index result
type IndexResult struct {
	Added          int
	Modified       int
	Deleted        int
	Unchanged      int
	DeletedNodeIDs []string // node IDs that were removed (for downstream cleanup)
	ChangedNodeIDs []string // node IDs that were added/modified (for embedding/BM25 update)
}

// ScanAndIndex scans directory and executes incremental indexing (full flow)
// 1. Scan file system, compute SHA1 for each file
// 2. Compare with indexed files' contentHash
// 3. Added files → full parse + index
// 4. Modified files → delete old nodes/edges + re-parse + index
// 5. Deleted files → delete associated nodes/edges
// 6. Unchanged files → skip
// 7. Incremental embedding: only re-embed changed nodes
func (idx *IncrementalIndexer) ScanAndIndex(ctx context.Context, repoPath string) (*IndexResult, error) {
	// 1. Scan file system
	currentFiles, err := idx.scanFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("scan files: %w", err)
	}

	return idx.Index(ctx, repoPath, currentFiles)
}

// Index executes incremental indexing (known file hashes)
func (idx *IncrementalIndexer) Index(ctx context.Context, repoPath string, currentFiles map[string]string) (*IndexResult, error) {
	result := &IndexResult{}

	// 1. Get indexed files
	indexedFiles, err := idx.getIndexedFiles()
	if err != nil {
		return nil, err
	}

	// 2. Compare changes
	changes := idx.detectChanges(indexedFiles, currentFiles)

	// 3. Process changes
	var (
		changedNodes []string // node IDs involved in changes (for incremental embedding)
		deletedNodes []string // node IDs that were removed (for BM25/HNSW cleanup)
	)

	// Group by change type
	var addedFiles, modifiedFiles []FileChange
	var deletedPaths []string
	for _, change := range changes {
		switch change.Type {
		case ChangeAdded:
			addedFiles = append(addedFiles, change)
		case ChangeModified:
			modifiedFiles = append(modifiedFiles, change)
		case ChangeDeleted:
			deletedPaths = append(deletedPaths, change.Path)
		}
	}

	// 3a. Process deleted files — collect node IDs before deleting
	for _, path := range deletedPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Collect node IDs from this file before deleting them from the graph
		fileNodes, _ := idx.graph.GetNodesByFile(idx.graph.Repo(), path)
		for _, n := range fileNodes {
			deletedNodes = append(deletedNodes, n.ID)
		}
		if err := idx.deleteFileNodes(path); err != nil {
			continue
		}
		result.Deleted++
	}

	// 3b. Process added files in parallel
	if idx.parseFn != nil {
		idx.processFilesParallel(ctx, repoPath, addedFiles, func(ctx context.Context, path string, content []byte, hash string) error {
			return idx.parseFn(ctx, path, content, hash)
		})
	}
	result.Added += len(addedFiles)

	// 3c. Process modified files — collect old node IDs for deletion before re-parse
	for _, change := range modifiedFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// Collect old node IDs — these will be deleted and need BM25/HNSW cleanup
		oldNodes, _ := idx.graph.GetNodesByFile(idx.graph.Repo(), change.Path)
		for _, n := range oldNodes {
			deletedNodes = append(deletedNodes, n.ID)
		}
		if err := idx.deleteFileNodes(change.Path); err != nil {
			continue
		}
		// Re-parse
		if idx.parseFn != nil {
			filePath := filepath.Join(repoPath, change.Path)
			content, err := os.ReadFile(filePath)
			if err == nil {
				if err := idx.parseFn(ctx, change.Path, content, change.NewHash); err == nil {
					// Collect new nodes for embedding + BM25 update
					newNodes, _ := idx.graph.GetNodesByFile(idx.graph.Repo(), change.Path)
					for _, n := range newNodes {
						changedNodes = append(changedNodes, n.ID)
					}
				}
			}
		}
		result.Modified++
	}

	// 3d. Count unchanged files
	for _, change := range changes {
		if change.Type == ChangeUnchanged {
			result.Unchanged++
		}
	}

	// 4. Incremental embedding
	if idx.embedFn != nil && len(changedNodes) > 0 {
		if embedErr := idx.embedFn(ctx, changedNodes); embedErr != nil {
			slog.Warn("incremental: embedding failed", "error", embedErr)
		}
	}

	// 5. Clean up deleted nodes from BM25, HNSW, and embedding indexes
	if idx.deleteNodes != nil && len(deletedNodes) > 0 {
		if deleteErr := idx.deleteNodes(ctx, deletedNodes); deleteErr != nil {
			slog.Warn("incremental: delete nodes callback failed", "error", deleteErr)
		}
	}

	// 6. Incremental BM25 index update for changed (added/modified) nodes
	if idx.bm25Fn != nil && len(changedNodes) > 0 {
		var changedNodeObjects []*graph.Node
		for _, nodeID := range changedNodes {
			node, err := idx.graph.GetNode(nodeID)
			if err == nil && node != nil {
				changedNodeObjects = append(changedNodeObjects, node)
			}
		}
		if len(changedNodeObjects) > 0 {
			if bm25Err := idx.bm25Fn(ctx, changedNodeObjects); bm25Err != nil {
				slog.Warn("incremental: bm25 update failed", "error", bm25Err)
			}
		}
	}

	result.DeletedNodeIDs = deletedNodes
	result.ChangedNodeIDs = changedNodes

	return result, nil
}

// scanFiles scans directory, returns filePath → SHA1 mapping
func (idx *IncrementalIndexer) scanFiles(repoPath string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() {
			// Skip hidden directories and common exclude directories
			name := info.Name()
			if len(name) > 0 && name[0] == '.' {
				return filepath.SkipDir
			}
			switch name {
			case "vendor", "node_modules", "__pycache__", ".git", "dist", "build", "target", "bin":
				return filepath.SkipDir
			}
			return nil
		}

		// Only index recognizable source code files
		ext := filepath.Ext(path)
		if !isSourceFile(ext) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			relPath = path
		}

		files[relPath] = util.ContentHash(data)
		return nil
	})

	return files, err
}

// isSourceFile checks if extension is a source code file
func isSourceFile(ext string) bool {
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".cs",
		".c", ".h", ".cpp", ".hpp", ".cc", ".cxx", ".rs", ".rb",
		".swift", ".kt", ".scala", ".md", ".proto", ".thrift",
		".toml", ".yaml", ".yml", ".json", ".xml", ".gradle",
		".mod", ".sum", ".lock":
		return true
	default:
		return false
	}
}

// processFilesParallel processes files in parallel
func (idx *IncrementalIndexer) processFilesParallel(ctx context.Context, repoPath string, files []FileChange, processFunc func(ctx context.Context, path string, content []byte, hash string) error) {
	if len(files) == 0 {
		return
	}

	sem := make(chan struct{}, idx.workers)
	var wg sync.WaitGroup

	for _, change := range files {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // acquire worker slot
		go func(ch FileChange) {
			defer wg.Done()
			defer func() { <-sem }() // release worker slot

			filePath := filepath.Join(repoPath, ch.Path)
			content, err := os.ReadFile(filePath)
			if err != nil {
				return
			}
			if procErr := processFunc(ctx, ch.Path, content, ch.NewHash); procErr != nil {
				slog.Warn("incremental: process file failed", "path", ch.Path, "error", procErr)
			}
		}(change)
	}

	wg.Wait()
}

// getIndexedFiles gets indexed files and their content hashes
func (idx *IncrementalIndexer) getIndexedFiles() (map[string]string, error) {
	files := make(map[string]string)

	// Get all File nodes from graph store
	nodes, err := idx.graph.GetNodesByLabel(idx.graph.Repo(), string(graph.LabelFile))
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		hash := node.GetPropString("contentHash")
		if hash != "" {
			files[node.FilePath] = hash
		}
	}

	return files, nil
}

// DetectFileChanges detects file changes (public interface, for Trip layer to call)
func (idx *IncrementalIndexer) DetectFileChanges(ctx context.Context, repoPath string) ([]FileChange, error) {
	currentFiles, err := idx.scanFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("scan files: %w", err)
	}
	indexedFiles, err := idx.getIndexedFiles()
	if err != nil {
		return nil, err
	}
	return idx.detectChanges(indexedFiles, currentFiles), nil
}

// detectChanges detects file changes
func (idx *IncrementalIndexer) detectChanges(indexed map[string]string, current map[string]string) []FileChange {
	var changes []FileChange

	// Check for added and modified
	for path, hash := range current {
		oldHash, exists := indexed[path]
		if !exists {
			changes = append(changes, FileChange{
				Path:    path,
				Type:    ChangeAdded,
				NewHash: hash,
			})
		} else if oldHash != hash {
			changes = append(changes, FileChange{
				Path:    path,
				Type:    ChangeModified,
				OldHash: oldHash,
				NewHash: hash,
			})
		} else {
			changes = append(changes, FileChange{
				Path: path,
				Type: ChangeUnchanged,
			})
		}
	}

	// Check for deleted
	for path, hash := range indexed {
		if _, exists := current[path]; !exists {
			changes = append(changes, FileChange{
				Path:    path,
				Type:    ChangeDeleted,
				OldHash: hash,
			})
		}
	}

	return changes
}

// deleteFileNodes deletes file-associated nodes and edges
func (idx *IncrementalIndexer) deleteFileNodes(filePath string) error {
	nodes, err := idx.graph.GetNodesByFile(idx.graph.Repo(), filePath)
	if err != nil {
		return fmt.Errorf("get file nodes: %w", err)
	}

	// Batch delete nodes (including associated edges)
	for _, node := range nodes {
		if err := idx.graph.DeleteNode(node.ID); err != nil {
			continue
		}
	}

	return nil
}
