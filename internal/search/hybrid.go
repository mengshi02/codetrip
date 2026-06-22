package search

import (
	"context"
	"sort"
)

// HybridSearch combines BM25 + semantic search + RRF fusion
type HybridSearch struct {
	bm25   *BM25Index
	vector *VectorSearch
}

// NewHybridSearch creates a hybrid search engine
func NewHybridSearch(bm25 *BM25Index, vector *VectorSearch) *HybridSearch {
	return &HybridSearch{
		bm25:   bm25,
		vector: vector,
	}
}

// HybridResult represents hybrid search results
type HybridResult struct {
	Results []HybridSearchItem
}

// HybridSearchItem represents a hybrid search result item
type HybridSearchItem struct {
	FilePath      string
	Score         float64 // RRF score
	Rank          int
	Sources       []string // "bm25" | "semantic"
	NodeID        string
	Name          string
	Label         string
	StartLine     int
	EndLine       int
	BM25Score     float64
	SemanticScore float64
}

// Search executes hybrid search
// RRF formula: score(doc) = Σ 1/(K + rank_i), K = 60
func (h *HybridSearch) Search(query string, limit int) (*HybridResult, error) {
	const K = 60.0

	// Execute BM25 and semantic search in parallel
	type bm25Result struct {
		results []BM25Result
		err     error
	}
	bm25Ch := make(chan bm25Result, 1)
	go func() {
		results, err := h.bm25.Search(query, limit)
		bm25Ch <- bm25Result{results, err}
	}()

	// Semantic search (if available)
	var semanticResults []SemanticResult
	if h.vector != nil {
		ctx := context.Background()
		results, err := h.vector.Search(ctx, query, limit)
		if err == nil {
			semanticResults = results
		}
	}

	bm25Res := <-bm25Ch
	if bm25Res.err != nil {
		return nil, bm25Res.err
	}

	// RRF fusion
	scores := make(map[string]*HybridSearchItem)

	for rank, r := range bm25Res.results {
		rrfScore := 1.0 / (K + float64(rank+1))
		if item, ok := scores[r.NodeID]; ok {
			item.Score += rrfScore
			item.Sources = append(item.Sources, "bm25")
			item.BM25Score = r.Score
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:    r.NodeID,
				FilePath:  r.FilePath,
				Name:      r.Name,
				Label:     r.Label,
				Score:     rrfScore,
				Sources:   []string{"bm25"},
				BM25Score: r.Score,
				StartLine: r.StartLine,
				EndLine:   r.EndLine,
			}
		}
	}

	for rank, r := range semanticResults {
		rrfScore := 1.0 / (K + float64(rank+1))
		if item, ok := scores[r.NodeID]; ok {
			item.Score += rrfScore
			item.Sources = append(item.Sources, "semantic")
			item.SemanticScore = r.Distance
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:        r.NodeID,
				Name:          r.Name,
				Label:         r.Label,
				FilePath:      r.FilePath,
				Score:         rrfScore,
				Sources:       []string{"semantic"},
				SemanticScore: r.Distance,
				StartLine:     r.StartLine,
				EndLine:       r.EndLine,
			}
		}
	}

	// Sort
	items := make([]HybridSearchItem, 0, len(scores))
	for _, item := range scores {
		items = append(items, *item)
	}

	// Sort by RRF score descending (sort.Slice O(n log n), replacing O(n²) bubble sort)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	// Set rank
	for i := range items {
		items[i].Rank = i + 1
	}

	return &HybridResult{Results: items}, nil
}

// SearchDualModal executes hybrid search using dual-modal vector search + BM25.
// Uses VectorSearch.SearchDualModal for parallel desc/code HNSW search with RRF,
// then fuses with BM25 results using another RRF pass.
func (h *HybridSearch) SearchDualModal(ctx context.Context, query string, limit int) (*HybridResult, error) {
	const K = 60.0

	// Execute BM25 and dual-modal semantic search in parallel
	type bm25Result struct {
		results []BM25Result
		err     error
	}
	bm25Ch := make(chan bm25Result, 1)
	go func() {
		results, err := h.bm25.Search(query, limit)
		bm25Ch <- bm25Result{results, err}
	}()

	// Dual-modal semantic search (if available)
	var semanticResults []SemanticResult
	if h.vector != nil {
		results, err := h.vector.SearchDualModal(ctx, query, limit)
		if err == nil {
			semanticResults = results
		}
	}

	bm25Res := <-bm25Ch
	if bm25Res.err != nil {
		return nil, bm25Res.err
	}

	// RRF fusion (same logic as Search, but with dual-modal semantic results)
	scores := make(map[string]*HybridSearchItem)

	for rank, r := range bm25Res.results {
		rrfScore := 1.0 / (K + float64(rank+1))
		if item, ok := scores[r.NodeID]; ok {
			item.Score += rrfScore
			item.Sources = append(item.Sources, "bm25")
			item.BM25Score = r.Score
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:    r.NodeID,
				FilePath:  r.FilePath,
				Name:      r.Name,
				Label:     r.Label,
				Score:     rrfScore,
				Sources:   []string{"bm25"},
				BM25Score: r.Score,
				StartLine: r.StartLine,
				EndLine:   r.EndLine,
			}
		}
	}

	for rank, r := range semanticResults {
		rrfScore := 1.0 / (K + float64(rank+1))
		if item, ok := scores[r.NodeID]; ok {
			item.Score += rrfScore
			item.Sources = append(item.Sources, "semantic")
			item.SemanticScore = r.Distance
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:        r.NodeID,
				Name:          r.Name,
				Label:         r.Label,
				FilePath:      r.FilePath,
				Score:         rrfScore,
				Sources:       []string{"semantic"},
				SemanticScore: r.Distance,
				StartLine:     r.StartLine,
				EndLine:       r.EndLine,
			}
		}
	}

	// Sort by RRF score descending
	items := make([]HybridSearchItem, 0, len(scores))
	for _, item := range scores {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	// Set rank
	for i := range items {
		items[i].Rank = i + 1
	}

	return &HybridResult{Results: items}, nil
}