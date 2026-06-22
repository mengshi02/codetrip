package group

import (
	"context"
	"fmt"
	"time"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

// GroupInfo is the group information (exposed externally)
type GroupInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Repos       []string `json:"repos"`
}

// GroupSyncResult is the group synchronization result (exposed externally)
type GroupSyncResult struct {
	Group       string  `json:"group"`
	Contracts   int     `json:"contracts"`
	BridgeLinks int     `json:"bridgeLinks"`
	Duration    float64 `json:"duration"`
}

// GroupService is the group service (unified external entry point)
type GroupService struct {
	storage  *GroupStorage
	builder  *BridgeBuilder
	analyzer *CrossRepoImpactAnalyzer
}

// NewGroupService creates a group service
func NewGroupService(store *store.Store) *GroupService {
	storage := NewGroupStorage(store)
	matcher := NewContractMatcher(DefaultMatchingConfig())
	builder := NewBridgeBuilder(storage, matcher)
	analyzer := NewCrossRepoImpactAnalyzer(storage)

	return &GroupService{
		storage:  storage,
		builder:  builder,
		analyzer: analyzer,
	}
}

// ListGroups lists all groups
func (s *GroupService) ListGroups() ([]GroupInfo, error) {
	groupNames, err := s.storage.ListGroups()
	if err != nil {
		return nil, err
	}

	groups := make([]GroupInfo, 0, len(groupNames))
	for _, name := range groupNames {
		config, err := s.storage.LoadConfig(name)
		if err != nil {
			// Config loading failed, skip
			groups = append(groups, GroupInfo{Name: name})
			continue
		}

		repos := make([]string, 0, len(config.Repos))
		for repoName := range config.Repos {
			repos = append(repos, repoName)
		}

		groups = append(groups, GroupInfo{
			Name:        name,
			Description: config.Description,
			Repos:       repos,
		})
	}

	return groups, nil
}

// SyncGroup synchronizes a group (extract contracts + build bridge graph)
func (s *GroupService) SyncGroup(ctx context.Context, config *GroupConfig, extractors map[string]ContractExtractorFn, graphs map[string]*graph.GraphStore) (*GroupSyncResult, error) {
	start := time.Now()

	// Save configuration
	if err := s.storage.SaveConfig(config.Name, config); err != nil {
		return nil, fmt.Errorf("save group config: %w", err)
	}

	// Build bridge graph
	bg, err := s.builder.Build(ctx, config, extractors, graphs)
	if err != nil {
		return nil, fmt.Errorf("build bridge graph: %w", err)
	}

	return &GroupSyncResult{
		Group:       config.Name,
		Contracts:   len(bg.Nodes),
		BridgeLinks: len(bg.Edges),
		Duration:    time.Since(start).Seconds(),
	}, nil
}

// Impact performs cross-repo impact analysis
func (s *GroupService) Impact(ctx context.Context, group string, target string, direction string, graphs map[string]*graph.GraphStore) (*CrossRepoImpactResult, error) {
	return s.analyzer.Analyze(ctx, group, target, direction, graphs)
}

// DeleteGroup deletes a group
func (s *GroupService) DeleteGroup(group string) error {
	return s.storage.DeleteGroup(group)
}

// GetBridgeGraph gets the bridge graph
func (s *GroupService) GetBridgeGraph(group string) (*BridgeGraph, error) {
	return s.storage.LoadBridgeGraph(group)
}

// GetContracts gets group contracts
func (s *GroupService) GetContracts(group string) ([]Contract, error) {
	return s.storage.LoadContracts(group)
}