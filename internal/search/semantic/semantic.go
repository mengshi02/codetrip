package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/coder/hnsw"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/util"
)

// SemanticResult represents a semantic search result
type SemanticResult struct {
	NodeID    string
	Name      string
	Label     string
	FilePath  string
	Distance  float64
	StartLine int
	EndLine   int
}

// VectorSearch is a semantic search based on embedding vectors
type VectorSearch struct {
	embedder       Embedder     // Embedding model
	store          *store.Store // Pebble stores vector data
	graph          *graph.GraphStore
	hnswMu         sync.RWMutex      // HNSW index read-write lock
	dataDir        string            // Base data directory for HNSW persistence
	vecFile        *VectorFileReader // mmap'd quantized vector file (nil if not loaded)
	twoStageSearch bool              // Enable two-stage search (int8 coarse + float32 refine)

	// Dual-modal HNSW indices
	descHnswIdx   *hnsw.Graph[string] // Description modality HNSW index
	codeHnswIdx   *hnsw.Graph[string] // Code modality HNSW index
	dualHnswBuilt bool                // Whether dual-modal HNSW indices are built
}

// NewVectorSearch creates a vector search instance
func NewVectorSearch(embedder Embedder, store *store.Store, graphStore *graph.GraphStore) *VectorSearch {
	return &VectorSearch{
		embedder: embedder,
		store:    store,
		graph:    graphStore,
	}
}

// NewVectorSearchWithDir creates a vector search instance with data directory for HNSW persistence
func NewVectorSearchWithDir(embedder Embedder, store *store.Store, graphStore *graph.GraphStore, dataDir string) *VectorSearch {
	return &VectorSearch{
		embedder: embedder,
		store:    store,
		graph:    graphStore,
		dataDir:  dataDir,
	}
}

// SetTwoStageSearch enables or disables two-stage search (int8 coarse + float32 refine).
func (s *VectorSearch) SetTwoStageSearch(enabled bool) {
	s.twoStageSearch = enabled
}

// SetEmbedder sets the embedding model for the vector search.
// Used when an embedder becomes available after VectorSearch creation (e.g. after EmbedRepo).
func (s *VectorSearch) SetEmbedder(embedder Embedder) {
	s.embedder = embedder
}

// LoadVectorFile loads the quantized vector file for the repo.
// This enables int8 quantized HNSW search with significantly reduced memory usage.
func (s *VectorSearch) LoadVectorFile() error {
	if s.dataDir == "" {
		return nil
	}
	repo := s.graph.Repo()
	path := VectorFilePath(s.dataDir, repo)
	if !VectorFileExists(s.dataDir, repo) {
		return nil // no vector file yet
	}
	vf, err := OpenVectorFile(path)
	if err != nil {
		return fmt.Errorf("open vector file: %w", err)
	}
	s.vecFile = vf
	slog.Info("vector search: loaded quantized vector file",
		"repo", repo,
		"nodes", vf.NodeCount(),
		"chunks", vf.ChunkCount(),
		"dim", vf.Dim(),
	)
	return nil
}

// Close releases resources held by VectorSearch.
func (s *VectorSearch) Close() {
	if s.vecFile != nil {
		s.vecFile.Close()
		s.vecFile = nil
	}
}

// Search performs semantic vector search (delegates to SearchDualModal).
func (s *VectorSearch) Search(ctx context.Context, query string, limit int) ([]SemanticResult, error) {
	return s.SearchDualModal(ctx, query, limit)
}

// SearchDualModal performs dual-modal semantic search with parallel HNSW search
// and Reciprocal Rank Fusion (RRF). It searches both description and code modality
// HNSW indices in parallel, then merges results using RRF.
func (s *VectorSearch) SearchDualModal(ctx context.Context, query string, limit int) ([]SemanticResult, error) {
	if s.embedder == nil || s.embedder.Dimensions() == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 20
	}

	// Convert query to embedding vector
	embeddings, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}
	queryVec := embeddings[0]

	searchStart := time.Now()

	// Check if dual-modal indices are available
	s.hnswMu.RLock()
	useDual := s.dualHnswBuilt && (s.descHnswIdx != nil || s.codeHnswIdx != nil)
	s.hnswMu.RUnlock()

	if !useDual {
		return nil, nil
	}

	results, err := s.searchDualHNSW(queryVec, limit)
	if err != nil {
		return nil, err
	}

	slog.Debug("dual-modal vector search",
		"method", "dual_modal_hnsw",
		"results", len(results),
		"duration_ms", time.Since(searchStart).Milliseconds(),
	)

	return results, nil
}

// searchDualHNSW performs parallel search on both desc and code HNSW indices
// and merges results using Reciprocal Rank Fusion (RRF).
func (s *VectorSearch) searchDualHNSW(queryVec []float32, limit int) ([]SemanticResult, error) {
	const K = 60.0 // RRF constant

	type searchResult struct {
		results []hnsw.Node[string]
		err     error
	}

	// Over-fetch for better RRF fusion quality
	searchLimit := limit * 2

	// Parallel search on both modality indices
	descCh := make(chan searchResult, 1)
	codeCh := make(chan searchResult, 1)

	go func() {
		s.hnswMu.RLock()
		defer s.hnswMu.RUnlock()
		if s.descHnswIdx == nil {
			descCh <- searchResult{nil, nil}
			return
		}
		res, err := s.descHnswIdx.Search(queryVec, searchLimit)
		descCh <- searchResult{res, err}
	}()

	go func() {
		s.hnswMu.RLock()
		defer s.hnswMu.RUnlock()
		if s.codeHnswIdx == nil {
			codeCh <- searchResult{nil, nil}
			return
		}
		res, err := s.codeHnswIdx.Search(queryVec, searchLimit)
		codeCh <- searchResult{res, err}
	}()

	descRes := <-descCh
	codeRes := <-codeCh

	if descRes.err != nil && codeRes.err != nil {
		return nil, fmt.Errorf("both modality searches failed: desc=%v, code=%v", descRes.err, codeRes.err)
	}

	// RRF fusion
	type rrfEntry struct {
		nodeID    string
		rrfScore  float64
		descScore float64
		codeScore float64
		descRank  int
		codeRank  int
	}

	scores := make(map[string]*rrfEntry)

	// Process description modality results
	for rank, r := range descRes.results {
		rrfScore := 1.0 / (K + float64(rank+1))
		cosScore := cosineSimilarity(queryVec, r.Value)
		if entry, ok := scores[r.Key]; ok {
			entry.rrfScore += rrfScore
			entry.descScore = cosScore
			entry.descRank = rank + 1
		} else {
			scores[r.Key] = &rrfEntry{
				nodeID:    r.Key,
				rrfScore:  rrfScore,
				descScore: cosScore,
				descRank:  rank + 1,
			}
		}
	}

	// Process code modality results
	for rank, r := range codeRes.results {
		rrfScore := 1.0 / (K + float64(rank+1))
		cosScore := cosineSimilarity(queryVec, r.Value)
		if entry, ok := scores[r.Key]; ok {
			entry.rrfScore += rrfScore
			entry.codeScore = cosScore
			entry.codeRank = rank + 1
		} else {
			scores[r.Key] = &rrfEntry{
				nodeID:    r.Key,
				rrfScore:  rrfScore,
				codeScore: cosScore,
				codeRank:  rank + 1,
			}
		}
	}

	// Sort by RRF score descending
	entries := make([]*rrfEntry, 0, len(scores))
	for _, entry := range scores {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].rrfScore > entries[j].rrfScore
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	// Enrich with graph node metadata
	semanticResults := make([]SemanticResult, 0, len(entries))
	for _, entry := range entries {
		// Use the best available similarity score
		bestScore := entry.descScore
		if entry.codeScore > bestScore {
			bestScore = entry.codeScore
		}

		node, err := s.graph.GetNode(entry.nodeID)
		if err != nil {
			semanticResults = append(semanticResults, SemanticResult{
				NodeID:   entry.nodeID,
				Distance: bestScore,
			})
			continue
		}

		semanticResults = append(semanticResults, SemanticResult{
			NodeID:    entry.nodeID,
			Name:      node.Name,
			Label:     string(node.Label),
			FilePath:  node.FilePath,
			Distance:  bestScore,
			StartLine: node.GetPropInt("startLine"),
			EndLine:   node.GetPropInt("endLine"),
		})
	}

	return semanticResults, nil
}

// BuildSemanticIndex builds separate HNSW indices for description and code modalities.
// It reads vectors stored with embdesc:/embcode: key prefixes and creates two independent
// HNSW graphs, enabling dual-modal parallel search with RRF fusion.
// When a quantized vector file is available, it uses int8 quantized vectors to reduce memory.
func (s *VectorSearch) BuildSemanticIndex() error {
	if s.embedder == nil || s.embedder.Dimensions() == 0 {
		return nil
	}

	buildStart := time.Now()
	repo := s.graph.Repo()

	// Build description and code modality HNSW indices in parallel
	type modalityBuildResult struct {
		idx   *hnsw.Graph[string]
		count int
		err   error
	}
	descCh := make(chan modalityBuildResult, 1)
	codeCh := make(chan modalityBuildResult, 1)

	go func() {
		idx, count, err := s.buildModalityHNSW(repo, "desc")
		descCh <- modalityBuildResult{idx, count, err}
	}()
	go func() {
		idx, count, err := s.buildModalityHNSW(repo, "code")
		codeCh <- modalityBuildResult{idx, count, err}
	}()

	descRes := <-descCh
	codeRes := <-codeCh

	if descRes.err != nil {
		return fmt.Errorf("build desc HNSW: %w", descRes.err)
	}
	if codeRes.err != nil {
		return fmt.Errorf("build code HNSW: %w", codeRes.err)
	}

	descIdx := descRes.idx
	descCount := descRes.count
	codeIdx := codeRes.idx
	codeCount := codeRes.count

	s.hnswMu.Lock()
	s.descHnswIdx = descIdx
	s.codeHnswIdx = codeIdx
	s.dualHnswBuilt = true
	s.hnswMu.Unlock()

	slog.Info("dual-modal HNSW indices built",
		"repo", repo,
		"desc_nodes", descCount,
		"code_nodes", codeCount,
		"duration_ms", time.Since(buildStart).Milliseconds(),
	)

	// Persist dual-modal indices to disk
	if s.dataDir != "" {
		if descIdx != nil {
			s.persistSemanticIndex(descIdx, repo, "desc")
		}
		if codeIdx != nil {
			s.persistSemanticIndex(codeIdx, repo, "code")
		}
	}

	return nil
}

// buildModalityHNSW builds a single-modality HNSW index.
// modality is "desc" or "code", determining the key prefix and index key.
func (s *VectorSearch) buildModalityHNSW(repo, modality string) (*hnsw.Graph[string], int, error) {
	var idxKey string
	var keyFn func(string, string) string
	switch modality {
	case "desc":
		idxKey = graph.EmbDescIdxKey(repo)
		keyFn = graph.EmbDescKey
	case "code":
		idxKey = graph.EmbCodeIdxKey(repo)
		keyFn = graph.EmbCodeKey
	default:
		return nil, 0, fmt.Errorf("unknown modality: %s", modality)
	}

	idxData, err := s.store.Get([]byte(idxKey))
	if err != nil {
		return nil, 0, nil // No vector data for this modality, skip
	}

	nodeIDs, decErr := util.DecodeStringList(idxData)
	if decErr != nil {
		return nil, 0, fmt.Errorf("decode %s embedding index: %w", modality, decErr)
	}

	if len(nodeIDs) == 0 {
		return nil, 0, nil
	}

	idx := hnsw.NewGraph[string]()
	idx.M = 16
	idx.Ml = 0.25
	idx.EfSearch = 20

	// Check if quantized vector file is available for int8 mode
	useQuant := s.vecFile != nil && s.vecFile.NodeCount() > 0

	if useQuant {
		idx.QuantType = hnsw.QuantInt8
		idx.QuantParams = hnsw.QuantParams{
			Scale:  s.vecFile.Scale(),
			Offset: s.vecFile.Offset(),
		}
		idx.Distance = hnsw.CosineDistance
		slog.Info("building HNSW with int8 quantization",
			"repo", repo, "modality", modality, "nodes", len(nodeIDs), "dim", s.vecFile.Dim())
	} else {
		idx.Distance = hnsw.CosineDistance
	}

	for _, nodeID := range nodeIDs {
		vecKey := keyFn(repo, nodeID)
		vecData, err := s.store.Get([]byte(vecKey))
		if err != nil {
			continue
		}

		vec := util.DecodeFloat32Vec(vecData)

		if err := idx.Add(hnsw.MakeNode(nodeID, vec)); err != nil {
			slog.Warn("hnsw add node failed", "modality", modality, "nodeID", nodeID, "error", err)
			continue
		}
	}

	return idx, len(nodeIDs), nil
}

// semanticIndexPath returns the file path for a dual-modal persisted HNSW index.
// modality is "desc" or "code".
func (s *VectorSearch) semanticIndexPath(repo, modality string) string {
	return filepath.Join(s.dataDir, "vectors", repo, fmt.Sprintf("%s.graph", modality))
}

// persistSemanticIndex saves a dual-modal HNSW index to disk.
func (s *VectorSearch) persistSemanticIndex(idx *hnsw.Graph[string], repo, modality string) {
	path := s.semanticIndexPath(repo, modality)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("dual HNSW persist: create directory failed", "path", dir, "error", err)
		return
	}

	sg := &hnsw.SavedGraph[string]{Graph: idx, Path: path}
	if err := sg.Save(); err != nil {
		slog.Warn("dual HNSW persist: save failed", "repo", repo, "modality", modality, "path", path, "error", err)
		return
	}
	slog.Info("dual HNSW index persisted", "repo", repo, "modality", modality, "path", path)
}

// RestoreSemanticIndex attempts to restore dual-modal HNSW indices from disk.
// Returns true if both desc and code indices were successfully restored.
func (s *VectorSearch) RestoreSemanticIndex() bool {
	if s.dataDir == "" {
		return false
	}

	repo := s.graph.Repo()
	descRestored := s.restoreSemanticModality(repo, "desc")
	codeRestored := s.restoreSemanticModality(repo, "code")

	if descRestored && codeRestored {
		s.hnswMu.Lock()
		s.dualHnswBuilt = true
		s.hnswMu.Unlock()
		return true
	}

	// Partial restore: if only one modality was restored, still mark as usable
	// but SearchDualModal will handle missing indices gracefully
	if descRestored || codeRestored {
		s.hnswMu.Lock()
		s.dualHnswBuilt = true
		s.hnswMu.Unlock()
		slog.Info("dual HNSW: partial restore",
			"repo", repo,
			"desc", descRestored,
			"code", codeRestored,
		)
	}

	return descRestored && codeRestored
}

// restoreSemanticModality restores a single-modality HNSW index from disk.
func (s *VectorSearch) restoreSemanticModality(repo, modality string) bool {
	path := s.semanticIndexPath(repo, modality)

	sg, err := hnsw.LoadSavedGraph[string](path)
	if err != nil {
		slog.Info("dual HNSW restore: no persisted index",
			"repo", repo, "modality", modality, "error", err)
		return false
	}

	// Verify against index key count
	var idxKey string
	switch modality {
	case "desc":
		idxKey = graph.EmbDescIdxKey(repo)
	case "code":
		idxKey = graph.EmbCodeIdxKey(repo)
	}
	idxData, err := s.store.Get([]byte(idxKey))
	if err != nil {
		return false
	}

	var nodeIDs []string
	if ids, decErr := util.DecodeStringList(idxData); decErr != nil {
		slog.Warn("dual HNSW restore: decode index failed",
			"repo", repo, "modality", modality, "error", decErr)
		return false
	} else {
		nodeIDs = ids
	}

	if sg.Graph.Len() != len(nodeIDs) {
		slog.Info("dual HNSW restore: stale index, will rebuild",
			"repo", repo, "modality", modality,
			"persisted_nodes", sg.Graph.Len(),
			"expected_nodes", len(nodeIDs),
		)
		return false
	}

	s.hnswMu.Lock()
	switch modality {
	case "desc":
		s.descHnswIdx = sg.Graph
	case "code":
		s.codeHnswIdx = sg.Graph
	}
	s.hnswMu.Unlock()

	slog.Info("dual HNSW index restored from disk",
		"repo", repo, "modality", modality, "nodes", sg.Graph.Len(),
	)
	return true
}

// HasEmbedData checks if any dual-modal embedding data exists for the repo.
func (s *VectorSearch) HasEmbedData() bool {
	repo := s.graph.Repo()

	descIdxKey := graph.EmbDescIdxKey(repo)
	if _, err := s.store.Get([]byte(descIdxKey)); err == nil {
		return true
	}
	codeIdxKey := graph.EmbCodeIdxKey(repo)
	if _, err := s.store.Get([]byte(codeIdxKey)); err == nil {
		return true
	}

	return false
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	dim := len(a)

	// Batch calculation to reduce loop overhead
	i := 0
	for i+3 < dim {
		dotProduct += float64(a[i])*float64(b[i]) + float64(a[i+1])*float64(b[i+1]) + float64(a[i+2])*float64(b[i+2]) + float64(a[i+3])*float64(b[i+3])
		normA += float64(a[i])*float64(a[i]) + float64(a[i+1])*float64(a[i+1]) + float64(a[i+2])*float64(a[i+2]) + float64(a[i+3])*float64(a[i+3])
		normB += float64(b[i])*float64(b[i]) + float64(b[i+1])*float64(b[i+1]) + float64(b[i+2])*float64(b[i+2]) + float64(b[i+3])*float64(b[i+3])
		i += 4
	}
	for ; i < dim; i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
