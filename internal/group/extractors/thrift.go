package extractors

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ExtractThriftContracts extracts Thrift contracts
func ExtractThriftContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Find nodes with .thrift files
	interfaceNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelInterface))
	if err != nil {
		return contracts, nil
	}

	for _, node := range interfaceNodes {
		if !strings.Contains(node.FilePath, ".thrift") {
			continue
		}

		serviceName := node.Name

		// Provider
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "thrift-provider", node.ID),
			ContractID: fmt.Sprintf("thrift:%s:%s", repo, serviceName),
			Type:       "thrift",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.9,
			Meta: map[string]any{
				"service": serviceName,
				"name":    node.Name,
			},
		})

		// Consumer: find callers
		inEdges, _ := gs.GetAllInEdges(node.ID)
		for _, edge := range inEdges {
			if edge.Type == graph.RelCalls {
				src, e := gs.GetNode(edge.Source)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "thrift-consumer", src.ID),
					ContractID: fmt.Sprintf("thrift:%s:%s", repo, serviceName),
					Type:       "thrift",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  src.UID(),
					Confidence: 0.8,
					Meta: map[string]any{
						"service": serviceName,
						"name":    src.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}