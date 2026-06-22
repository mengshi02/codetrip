package group

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/group/extractors"
)

// ContractExtractorFn is the contract extraction function signature (type alias to extractors subpackage)
type ContractExtractorFn = extractors.ContractExtractorFn

// BridgeBuilder builds bridge graphs
type BridgeBuilder struct {
	storage *GroupStorage
	matcher *ContractMatcher
}

// NewBridgeBuilder creates a bridge graph builder
func NewBridgeBuilder(storage *GroupStorage, matcher *ContractMatcher) *BridgeBuilder {
	return &BridgeBuilder{
		storage: storage,
		matcher: matcher,
	}
}

// Build constructs a bridge graph:
// 1. Collect contracts from all repositories (via ContractExtractorFn)
// 2. Distinguish between providers and consumers
// 3. Run matching pipeline
// 4. Build and persist bridge graph
func (b *BridgeBuilder) Build(ctx context.Context, config *GroupConfig, extractors map[string]ContractExtractorFn, graphs map[string]*graph.GraphStore) (*BridgeGraph, error) {
	// 1. Extract contracts from all repositories in parallel
	type repoContracts struct {
		repo      string
		contracts []Contract
		err       error
	}

	ch := make(chan repoContracts, len(config.Repos))
	var wg sync.WaitGroup

	for repoName := range config.Repos {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			gs, ok := graphs[name]
			if !ok {
				ch <- repoContracts{repo: name, err: fmt.Errorf("graph store not found for repo: %s", name)}
				return
			}

			var allContracts []Contract

			// Use registered extractors
			for contractType, extractorFn := range extractors {
				// Check if type is enabled
				if !isTypeEnabled(contractType, config.Detect.EnabledTypes) {
					continue
				}

				contracts, err := extractorFn(ctx, name, gs)
				if err != nil {
					// Single extractor failure should not block
					continue
				}
				allContracts = append(allContracts, contracts...)
			}

			// Use built-in extractors
			builtinContracts := extractBuiltinContracts(ctx, name, gs, config.Detect.EnabledTypes)
			allContracts = append(allContracts, builtinContracts...)

			ch <- repoContracts{repo: name, contracts: allContracts}
		}(repoName)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect all contracts
	var allContracts []Contract
	for rc := range ch {
		if rc.err != nil {
			continue
		}
		allContracts = append(allContracts, rc.contracts...)
	}

	// 2. Distinguish between providers and consumers
	var consumers, providers []Contract
	for _, c := range allContracts {
		switch c.Role {
		case "consumer":
			consumers = append(consumers, c)
		case "provider":
			providers = append(providers, c)
		}
	}

	// 3. Run matching pipeline
	links := b.matcher.Match(consumers, providers, config.Links)

	// 4. Build bridge graph
	bg := &BridgeGraph{}

	// Add nodes (with deduplication)
	nodeSet := make(map[string]bool)
	for _, c := range allContracts {
		if !nodeSet[c.ContractID] {
			nodeSet[c.ContractID] = true
			bg.AddNode(BridgeContract{
				ContractID: c.ContractID,
				Type:       c.Type,
				Role:       c.Role,
				Repo:       c.Repo,
				SymbolUID:  c.SymbolUID,
				Confidence: c.Confidence,
				Meta:       c.Meta,
			})
		}
	}

	// Add edges
	for _, link := range links {
		bg.AddEdge(link)
	}

	// Persist contracts and bridge graph
	groupName := config.Name
	if err := b.storage.SaveContracts(groupName, allContracts); err != nil {
		return nil, fmt.Errorf("save contracts: %w", err)
	}
	if err := b.storage.SaveBridgeGraph(groupName, bg); err != nil {
		return nil, fmt.Errorf("save bridge graph: %w", err)
	}

	return bg, nil
}

// isTypeEnabled checks if a contract type is enabled
func isTypeEnabled(contractType string, enabledTypes []string) bool {
	if len(enabledTypes) == 0 {
		return true
	}
	for _, t := range enabledTypes {
		if t == contractType {
			return true
		}
	}
	return false
}

// extractBuiltinContracts extracts contracts using built-in extractors
func extractBuiltinContracts(ctx context.Context, repo string, gs *graph.GraphStore, enabledTypes []string) []Contract {
	var contracts []Contract

	builtinExtractors := map[string]ContractExtractorFn{
		"http":    extractors.ExtractHTTPContracts,
		"grpc":    extractors.ExtractGRPCContracts,
		"thrift":  extractors.ExtractThriftContracts,
		"topic":   extractors.ExtractTopicContracts,
		"lib":     extractors.ExtractLibContracts,
		"include": extractors.ExtractIncludeContracts,
	}

	for ctype, fn := range builtinExtractors {
		if !isTypeEnabled(ctype, enabledTypes) {
			continue
		}
		// Set timeout protection
		extractCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := fn(extractCtx, repo, gs)
		cancel()
		if err != nil {
			continue
		}
		contracts = append(contracts, result...)
	}

	return contracts
}