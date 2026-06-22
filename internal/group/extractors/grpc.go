package extractors

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ExtractGRPCContracts extracts gRPC contracts
// Finds proto service definitions and method nodes
func ExtractGRPCContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Find Interface nodes (may represent gRPC service definitions)
	interfaceNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelInterface))
	if err != nil {
		return nil, fmt.Errorf("get interface nodes: %w", err)
	}

	for _, node := range interfaceNodes {
		serviceName := node.GetPropString("service")
		if serviceName == "" {
			serviceName = node.Name
		}

		// Check if it's a gRPC service (via properties or naming conventions)
		isGRPC := node.GetPropBool("grpc") ||
			strings.HasSuffix(serviceName, "Service") ||
			strings.HasSuffix(serviceName, "Server") ||
			strings.Contains(node.FilePath, ".proto")

		if !isGRPC {
			continue
		}

		// Provider: interface that is IMPLEMENTED
		inEdges, _ := gs.GetAllInEdges(node.ID)
		hasImplementor := false
		for _, edge := range inEdges {
			if edge.Type == graph.RelImplements {
				hasImplementor = true
				break
			}
		}
		if hasImplementor {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "grpc-provider", node.ID),
				ContractID: fmt.Sprintf("grpc:%s:%s", repo, serviceName),
				Type:       "grpc",
				Role:       "provider",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.9,
				Meta: map[string]any{
					"service": serviceName,
					"name":    node.Name,
				},
			})
		}

		// Consumer: implementation class with IMPLEMENTS out edge
		outEdges, _ := gs.GetAllOutEdges(node.ID)
		for _, edge := range outEdges {
			if edge.Type == graph.RelImplements || edge.Type == graph.RelCalls {
				tgt, e := gs.GetNode(edge.Target)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "grpc-consumer", tgt.ID),
					ContractID: fmt.Sprintf("grpc:%s:%s", repo, serviceName),
					Type:       "grpc",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  tgt.UID(),
					Confidence: 0.8,
					Meta: map[string]any{
						"service": serviceName,
						"name":    tgt.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}