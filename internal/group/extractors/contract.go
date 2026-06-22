package extractors

import (
	"context"

	"github.com/mengshi02/codetrip/internal/graph"
)

// Contract represents a contract node
type Contract struct {
	ID         string         `json:"id"`
	ContractID string         `json:"contractId"` // unique contract identifier
	Type       string         `json:"type"`       // http/grpc/thrift/topic/lib/include/custom
	Role       string         `json:"role"`       // provider | consumer
	Repo       string         `json:"repo"`
	SymbolUID  string         `json:"symbolUid"` // associated symbol UID
	Confidence float64        `json:"confidence"`
	Meta       map[string]any `json:"meta,omitempty"` // contract metadata (e.g., path, method, service, etc.)
}

// ContractExtractorFn is the contract extraction function signature
type ContractExtractorFn func(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error)