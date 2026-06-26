package group

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/collection"
)

// CrossRepoImpactResult is the cross-repo impact analysis result
type CrossRepoImpactResult struct {
	Risk          string                    `json:"risk"`
	LocalImpact   *collection.ImpactResult    `json:"localImpact"`
	CrossRepoRefs []CrossRepoRefInternal    `json:"crossRepoRefs"`
}

// CrossRepoRefInternal is the internal cross-repo reference type
type CrossRepoRefInternal struct {
	SourceRepo   string  `json:"sourceRepo"`
	SourceSymbol string  `json:"sourceSymbol"`
	TargetRepo   string  `json:"targetRepo"`
	TargetSymbol string  `json:"targetSymbol"`
	MatchType    string  `json:"matchType"`    // exact/manifest/wildcard/bm25/contract
	Confidence   float64 `json:"confidence"`
}

// CrossRepoImpactAnalyzer performs cross-repo impact analysis
type CrossRepoImpactAnalyzer struct {
	storage *GroupStorage
}

// NewCrossRepoImpactAnalyzer creates a cross-repo impact analyzer
func NewCrossRepoImpactAnalyzer(storage *GroupStorage) *CrossRepoImpactAnalyzer {
	return &CrossRepoImpactAnalyzer{storage: storage}
}

// Analyze performs two-phase analysis:
// Phase 1: Local impact walk (within member repositories)
// Phase 2: Bridge graph fan-out (cross-repo propagation via ContractLink)
func (a *CrossRepoImpactAnalyzer) Analyze(ctx context.Context, group string, target string, direction string, graphs map[string]*graph.GraphStore) (*CrossRepoImpactResult, error) {
	result := &CrossRepoImpactResult{}

	// Load bridge graph
	bg, err := a.storage.LoadBridgeGraph(group)
	if err != nil {
		return nil, fmt.Errorf("load bridge graph for group %s: %w", group, err)
	}

	// Load group configuration
	config, err := a.storage.LoadConfig(group)
	if err != nil {
		return nil, fmt.Errorf("load config for group %s: %w", group, err)
	}

	// Phase 1: Local impact analysis
	// Determine the repository where the target is located
	var localGS *graph.GraphStore
	var targetRepo string
	for repoName, gs := range graphs {
		if _, ok := config.Repos[repoName]; !ok {
			continue
		}
		// Find target symbol in this repository
		nodes, err := gs.GetNodesByName(gs.Repo(), target)
		if err == nil && len(nodes) > 0 {
			localGS = gs
			targetRepo = repoName
			break
		}
	}

	if localGS != nil {
		localResult, err := collection.RunImpact(ctx, localGS, &collection.ImpactRequest{
			Target:    target,
			Direction: direction,
			MaxDepth:  3,
		})
		if err == nil {
			result.LocalImpact = localResult
			result.Risk = localResult.Risk
		}
	}

	// Phase 2: Bridge graph fan-out — cross-repo propagation via contract links in bridge graph
	crossRefs := a.bridgeWalk(bg, target, targetRepo, direction)
	result.CrossRepoRefs = crossRefs

	// Comprehensive risk rating
	if result.Risk == "" {
		result.Risk = "LOW"
	}
	if len(crossRefs) >= 5 {
		result.Risk = "CRITICAL"
	} else if len(crossRefs) >= 3 {
		if result.Risk != "CRITICAL" {
			result.Risk = "HIGH"
		}
	} else if len(crossRefs) >= 1 {
		if result.Risk == "LOW" {
			result.Risk = "MEDIUM"
		}
	}

	return result, nil
}

// bridgeWalk performs cross-repo propagation via bridge graph
func (a *CrossRepoImpactAnalyzer) bridgeWalk(bg *BridgeGraph, target string, targetRepo string, direction string) []CrossRepoRefInternal {
	var refs []CrossRepoRefInternal

	// Build contract ID to node mapping
	nodeMap := make(map[string]*BridgeContract)
	for i := range bg.Nodes {
		nodeMap[bg.Nodes[i].ContractID] = &bg.Nodes[i]
	}

	// Find contracts associated with target symbol
	targetContracts := a.findTargetContracts(bg, target, targetRepo)

	// Build adjacency list
	// downstream: consumer → provider (along bridge graph edge direction)
	// upstream: provider → consumer (reverse)
	outEdges := make(map[string][]BridgeLink)  // sourceID → []BridgeLink
	inEdges := make(map[string][]BridgeLink)    // targetID → []BridgeLink
	for _, edge := range bg.Edges {
		outEdges[edge.SourceID] = append(outEdges[edge.SourceID], edge)
		inEdges[edge.TargetID] = append(inEdges[edge.TargetID], edge)
	}

	// BFS expansion
	visited := make(map[string]bool)
	queue := make([]string, 0, len(targetContracts))

	for _, tc := range targetContracts {
		visited[tc] = true
		queue = append(queue, tc)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		var edges []BridgeLink
		if direction == "downstream" || direction == "" {
			edges = append(edges, outEdges[cur]...)
		}
		if direction == "upstream" || direction == "" {
			edges = append(edges, inEdges[cur]...)
		}

		for _, edge := range edges {
			nextID := edge.TargetID
			if direction == "upstream" {
				nextID = edge.SourceID
			}

			nextNode, ok := nodeMap[nextID]
			if !ok || visited[nextID] {
				continue
			}
			visited[nextID] = true

			curNode := nodeMap[cur]
			if curNode == nil {
				continue
			}

			// Only record cross-repo references
			if curNode.Repo != nextNode.Repo {
				ref := CrossRepoRefInternal{
					SourceRepo:   curNode.Repo,
					SourceSymbol: curNode.SymbolUID,
					TargetRepo:   nextNode.Repo,
					TargetSymbol: nextNode.SymbolUID,
					MatchType:    edge.MatchType,
					Confidence:   edge.Confidence,
				}
				refs = append(refs, ref)
			}

			queue = append(queue, nextID)
		}
	}

	return refs
}

// findTargetContracts finds bridge graph contracts associated with target symbol
func (a *CrossRepoImpactAnalyzer) findTargetContracts(bg *BridgeGraph, target string, targetRepo string) []string {
	var contractIDs []string

	for _, node := range bg.Nodes {
		// Match by SymbolUID
		if node.SymbolUID != "" {
			// SymbolUID format: repo:filePath:label:name
			parts := strings.SplitN(node.SymbolUID, ":", 4)
			if len(parts) >= 4 && parts[3] == target {
				if targetRepo == "" || parts[0] == targetRepo {
					contractIDs = append(contractIDs, node.ContractID)
					continue
				}
			}
		}

		// Match by ContractID
		if node.ContractID == target {
			contractIDs = append(contractIDs, node.ContractID)
			continue
		}

		// Match by name in Meta
		if node.Meta != nil {
			if name, ok := node.Meta["name"].(string); ok && name == target {
				if targetRepo == "" || node.Repo == targetRepo {
					contractIDs = append(contractIDs, node.ContractID)
				}
			}
		}
	}

	return contractIDs
}