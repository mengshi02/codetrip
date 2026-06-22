package extractors

import (
	"context"
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ExtractHTTPContracts extracts HTTP contracts
// Extracts HTTP contracts from Route nodes (provider: HANDLES_ROUTE, consumer: FETCHES)
func ExtractHTTPContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Find Route nodes
	routeNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelRoute))
	if err != nil {
		return nil, fmt.Errorf("get route nodes: %w", err)
	}

	for _, node := range routeNodes {
		path := node.GetPropString("path")
		method := node.GetPropString("method")

		// Provider: HANDLES_ROUTE in edge
		inEdges, _ := gs.GetAllInEdges(node.ID)
		for _, edge := range inEdges {
			if edge.Type == graph.RelHandlesRoute {
				src, e := gs.GetNode(edge.Source)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "http-provider", node.ID),
					ContractID: fmt.Sprintf("http:%s:%s:%s", repo, method, path),
					Type:       "http",
					Role:       "provider",
					Repo:       repo,
					SymbolUID:  src.UID(),
					Confidence: 1.0,
					Meta: map[string]any{
						"path":   path,
						"method": method,
						"name":   node.Name,
					},
				})
			}
		}

		// Consumer: FETCHES out edge
		outEdges, _ := gs.GetAllOutEdges(node.ID)
		for _, edge := range outEdges {
			if edge.Type == graph.RelFetches {
				tgt, e := gs.GetNode(edge.Target)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "http-consumer", tgt.ID),
					ContractID: fmt.Sprintf("http:%s:%s:%s", repo, method, path),
					Type:       "http",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  tgt.UID(),
					Confidence: 0.9,
					Meta: map[string]any{
						"path":   path,
						"method": method,
						"name":   tgt.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}