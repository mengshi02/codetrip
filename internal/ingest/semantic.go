package ingest

import (
	"fmt"
	"sort"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
)

// SemanticStats is the language-neutral result of a compiler or type-system
// refinement pass. Facts use stable relation or metric names so the pipeline
// does not need to know which language produced them.
type SemanticStats struct {
	Refiner  string
	Language string
	Units    int
	Facts    map[string]int
}

func (stats SemanticStats) String() string {
	keys := make([]string, 0, len(stats.Facts))
	for key := range stats.Facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, stats.Facts[key]))
	}
	return fmt.Sprintf("%s language=%s units=%d %s", stats.Refiner, stats.Language, stats.Units, strings.Join(parts, " "))
}

// SemanticRefiner adds facts that require a language compiler or type system
// and therefore cannot be recovered reliably by the shared syntax pipeline.
type SemanticRefiner interface {
	Name() string
	Supports(*graph.KnowledgeGraph) bool
	Refine(repoPath string, knowledgeGraph *graph.KnowledgeGraph) (SemanticStats, error)
}

var semanticRefiners = []SemanticRefiner{
	goSemanticRefiner{},
}

// ProcessSemanticRefinements runs every applicable language refinement pass.
// A failed refiner does not prevent other languages from being processed.
func ProcessSemanticRefinements(repoPath string, knowledgeGraph *graph.KnowledgeGraph) ([]SemanticStats, []error) {
	stats := make([]SemanticStats, 0, len(semanticRefiners))
	errors := make([]error, 0)
	for _, refiner := range semanticRefiners {
		if !refiner.Supports(knowledgeGraph) {
			continue
		}
		result, err := refiner.Refine(repoPath, knowledgeGraph)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", refiner.Name(), err))
			continue
		}
		stats = append(stats, result)
	}
	return stats, errors
}
