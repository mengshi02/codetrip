package extractors

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ExtractIncludeContracts extracts C/C++ #include directives
// Finds #include preprocessor directives
func ExtractIncludeContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Find Import nodes for C/C++ includes
	importNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelImport))
	if err != nil {
		return contracts, nil
	}

	for _, node := range importNodes {
		// Check if it's an include in C/C++ files
		filePath := node.FilePath
		if !strings.HasSuffix(filePath, ".h") &&
			!strings.HasSuffix(filePath, ".hpp") &&
			!strings.HasSuffix(filePath, ".c") &&
			!strings.HasSuffix(filePath, ".cpp") &&
			!strings.HasSuffix(filePath, ".cc") {
			continue
		}

		includePath := node.GetPropString("include")
		if includePath == "" {
			includePath = node.Name
		}

		// Provider: the included header file
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "include-provider", node.ID),
			ContractID: fmt.Sprintf("include:%s:%s", repo, includePath),
			Type:       "include",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.9,
			Meta: map[string]any{
				"include": includePath,
				"name":    node.Name,
			},
		})

		// Consumer: source files that include this header
		inEdges, _ := gs.GetAllInEdges(node.ID)
		for _, edge := range inEdges {
			if edge.Type == graph.RelImports || edge.Type == graph.RelContains {
				src, e := gs.GetNode(edge.Source)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "include-consumer", src.ID),
					ContractID: fmt.Sprintf("include:%s:%s", repo, includePath),
					Type:       "include",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  src.UID(),
					Confidence: 0.85,
					Meta: map[string]any{
						"include": includePath,
						"name":    src.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}