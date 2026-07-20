package search

import (
	"context"
	"github.com/mengshi02/codetrip/internal/search/semantic"
	"github.com/mengshi02/codetrip/internal/search/symbol"
	"sort"
)

// HybridSearch combines lexical + semantic search + RRF fusion
type HybridSearch struct {
	lexical *symbol.LexicalIndex
	vector  *semantic.VectorSearch
}

// symbolTypePriority returns a priority value for a symbol label.
// Lower values = higher priority. Used to break ties in RRF score ranking.
// Interface > Struct/Class > Function/Method > other types.
func symbolTypePriority(label string) int {
	switch label {
	case "Interface":
		return 1
	case "Struct", "Class":
		return 2
	case "Function", "Method":
		return 3
	case "Trait":
		return 4
	case "Constructor":
		return 5
	default:
		return 10
	}
}

// NewHybridSearch creates a hybrid search engine
func NewHybridSearch(lexical *symbol.LexicalIndex, vector *semantic.VectorSearch) *HybridSearch {
	return &HybridSearch{
		lexical: lexical,
		vector:  vector,
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
	Sources       []string // "lexical" | "semantic"
	NodeID        string
	Name          string
	Label         string
	StartLine     int
	EndLine       int
	LexicalScore  float64
	SemanticScore float64
}

// Search executes hybrid search
// RRF formula: score(doc) = Σ 1/(K + rank_i), K = 60
func (h *HybridSearch) Search(query string, limit int) (*HybridResult, error) {
	const K = 60.0

	// Execute lexical and semantic search in parallel
	type bm25Result struct {
		results []symbol.LexicalResult
		err     error
	}
	bm25Ch := make(chan bm25Result, 1)
	go func() {
		results, err := h.lexical.Search(query, limit)
		bm25Ch <- bm25Result{results, err}
	}()

	// Semantic search (if available)
	var semanticResults []semantic.SemanticResult
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
			item.Sources = append(item.Sources, "lexical")
			item.LexicalScore = r.Score
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:       r.NodeID,
				FilePath:     r.FilePath,
				Name:         r.Name,
				Label:        r.Label,
				Score:        rrfScore,
				Sources:      []string{"lexical"},
				LexicalScore: r.Score,
				StartLine:    r.StartLine,
				EndLine:      r.EndLine,
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

	// Sort by RRF score descending; break ties by symbol type priority
	// (Interface > Struct > Function > other types) for better result relevance
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return symbolTypePriority(items[i].Label) < symbolTypePriority(items[j].Label)
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

// SearchDualModal executes hybrid search using dual-modal vector search + lexical.
// Uses VectorSearch.SearchDualModal for parallel desc/code HNSW search with RRF,
// then fuses with lexical results using another RRF pass.
func (h *HybridSearch) SearchDualModal(ctx context.Context, query string, limit int) (*HybridResult, error) {
	const K = 60.0

	// Execute lexical and dual-modal semantic search in parallel
	type bm25Result struct {
		results []symbol.LexicalResult
		err     error
	}
	bm25Ch := make(chan bm25Result, 1)
	go func() {
		results, err := h.lexical.Search(query, limit)
		bm25Ch <- bm25Result{results, err}
	}()

	// Dual-modal semantic search (if available)
	var semanticResults []semantic.SemanticResult
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
			item.Sources = append(item.Sources, "lexical")
			item.LexicalScore = r.Score
		} else {
			scores[r.NodeID] = &HybridSearchItem{
				NodeID:       r.NodeID,
				FilePath:     r.FilePath,
				Name:         r.Name,
				Label:        r.Label,
				Score:        rrfScore,
				Sources:      []string{"lexical"},
				LexicalScore: r.Score,
				StartLine:    r.StartLine,
				EndLine:      r.EndLine,
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

	// Sort by RRF score descending; break ties by symbol type priority
	items := make([]HybridSearchItem, 0, len(scores))
	for _, item := range scores {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return symbolTypePriority(items[i].Label) < symbolTypePriority(items[j].Label)
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
