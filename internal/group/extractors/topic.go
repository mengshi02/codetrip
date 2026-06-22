package extractors

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ExtractTopicContracts extracts message topic contracts
// Finds pub/sub patterns (EMITS_EVENT / BINDS_EVENT_HANDLER calls)
func ExtractTopicContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Traverse all nodes to find EMITS_EVENT out edges (publisher/Provider)
	allNodes := gs.GetAllNodes(gs.Repo(), 0)
	for _, node := range allNodes {
		outEdges, err := gs.GetAllOutEdges(node.ID)
		if err != nil {
			slog.Warn("extract_topic: failed to get out-edges", "node_id", node.ID, "error", err)
			continue
		}
		for _, edge := range outEdges {
			if edge.Type == graph.RelEmitsEvent {
				topicName := edge.GetPropString("topic")
				if topicName == "" {
					topicName = edge.GetPropString("name")
				}
				if topicName == "" {
					topicName = node.Name
				}

				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "topic-provider", node.ID),
					ContractID: fmt.Sprintf("topic:%s:%s", repo, topicName),
					Type:       "topic",
					Role:       "provider",
					Repo:       repo,
					SymbolUID:  node.UID(),
					Confidence: 0.9,
					Meta: map[string]any{
						"topic": topicName,
						"name":  node.Name,
					},
				})
			}
		}
	}

	// Find BINDS_EVENT_HANDLER in edges (subscriber/Consumer)
	for _, node := range allNodes {
		outEdges, err := gs.GetAllOutEdges(node.ID)
		if err != nil {
			slog.Warn("extract_topic: failed to get out-edges", "node_id", node.ID, "error", err)
			continue
		}
		for _, edge := range outEdges {
			if edge.Type == graph.RelBindsEventHandler {
				topicName := edge.GetPropString("topic")
				if topicName == "" {
					topicName = edge.GetPropString("name")
				}
				if topicName == "" {
					topicName = node.Name
				}

				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "topic-consumer", node.ID),
					ContractID: fmt.Sprintf("topic:%s:%s", repo, topicName),
					Type:       "topic",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  node.UID(),
					Confidence: 0.85,
					Meta: map[string]any{
						"topic": topicName,
						"name":  node.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}