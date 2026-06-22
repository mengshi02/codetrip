package group

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/pebble/v2"
	"github.com/mengshi02/codetrip/internal/store"
)

// GroupStorage is the group storage (Pebble persistence)
//
// Key space:
//   group:{groupName}:config     → JSON(GroupConfig)
//   group:{groupName}:contracts  → JSON([]Contract)
//   group:{groupName}:bridge     → JSON(BridgeGraph)
type GroupStorage struct {
	store *store.Store
}

// NewGroupStorage creates a group storage
func NewGroupStorage(store *store.Store) *GroupStorage {
	return &GroupStorage{store: store}
}

// ============ Configuration operations ============

// SaveConfig saves group configuration
func (s *GroupStorage) SaveConfig(group string, config *GroupConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal group config: %w", err)
	}
	key := fmt.Sprintf("group:%s:config", group)
	return s.store.Set([]byte(key), data)
}

// LoadConfig loads group configuration
func (s *GroupStorage) LoadConfig(group string) (*GroupConfig, error) {
	key := fmt.Sprintf("group:%s:config", group)
	data, err := s.store.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("load group config %s: %w", group, err)
	}
	var config GroupConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal group config: %w", err)
	}
	return &config, nil
}

// ============ Contract operations ============

// SaveContracts saves group contracts
func (s *GroupStorage) SaveContracts(group string, contracts []Contract) error {
	data, err := json.Marshal(contracts)
	if err != nil {
		return fmt.Errorf("marshal contracts: %w", err)
	}
	key := fmt.Sprintf("group:%s:contracts", group)
	return s.store.Set([]byte(key), data)
}

// LoadContracts loads group contracts
func (s *GroupStorage) LoadContracts(group string) ([]Contract, error) {
	key := fmt.Sprintf("group:%s:contracts", group)
	data, err := s.store.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("load contracts for group %s: %w", group, err)
	}
	var contracts []Contract
	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, fmt.Errorf("unmarshal contracts: %w", err)
	}
	return contracts, nil
}

// ============ Bridge graph operations ============

// SaveBridgeGraph saves bridge graph (using Batch write)
func (s *GroupStorage) SaveBridgeGraph(group string, bg *BridgeGraph) error {
	data, err := json.Marshal(bg)
	if err != nil {
		return fmt.Errorf("marshal bridge graph: %w", err)
	}
	key := fmt.Sprintf("group:%s:bridge", group)
	return s.store.Batch(func(b *pebble.Batch) error {
		return b.Set([]byte(key), data, nil)
	})
}

// LoadBridgeGraph loads bridge graph
func (s *GroupStorage) LoadBridgeGraph(group string) (*BridgeGraph, error) {
	key := fmt.Sprintf("group:%s:bridge", group)
	data, err := s.store.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("load bridge graph for group %s: %w", group, err)
	}
	var bg BridgeGraph
	if err := json.Unmarshal(data, &bg); err != nil {
		return nil, fmt.Errorf("unmarshal bridge graph: %w", err)
	}
	return &bg, nil
}

// ============ List operations ============

// ListGroups lists all group names
func (s *GroupStorage) ListGroups() ([]string, error) {
	prefix := []byte("group:")
	seen := make(map[string]bool)

	err := s.store.ScanPrefix(prefix, func(key, _ []byte) error {
		// Parse group:{name}:xxx
		parts := strings.SplitN(string(key), ":", 3)
		if len(parts) >= 2 {
			name := parts[1]
			if !seen[name] {
				seen[name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	groups := make([]string, 0, len(seen))
	for name := range seen {
		groups = append(groups, name)
	}
	sort.Strings(groups)
	return groups, nil
}

// DeleteGroup deletes a group (all related keys)
func (s *GroupStorage) DeleteGroup(group string) error {
	prefix := fmt.Sprintf("group:%s:", group)
	return s.store.Batch(func(b *pebble.Batch) error {
		return s.store.ScanPrefix([]byte(prefix), func(key, _ []byte) error {
			return b.Delete(key, nil)
		})
	})
}