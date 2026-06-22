package group

import "github.com/mengshi02/codetrip/internal/group/extractors"

// Contract is a contract node (type alias, actual definition in extractors subpackage)
type Contract = extractors.Contract

// BridgeContract is a bridge graph contract node
type BridgeContract struct {
	ContractID string         `json:"contractId"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Repo       string         `json:"repo"`
	SymbolUID  string         `json:"symbolUid"`
	Confidence float64        `json:"confidence"`
	Meta       map[string]any `json:"meta,omitempty"`
}

// BridgeLink is a bridge graph contract link edge
type BridgeLink struct {
	SourceID   string  `json:"sourceId"`   // consumer contract ID
	TargetID   string  `json:"targetId"`   // provider contract ID
	MatchType  string  `json:"matchType"`  // exact/manifest/wildcard/bm25/embedding
	Confidence float64 `json:"confidence"`
	ContractID string  `json:"contractId"`
}

// BridgeGraph is a bridge graph (cross-repo contract link graph)
type BridgeGraph struct {
	Nodes []BridgeContract `json:"nodes"`
	Edges []BridgeLink     `json:"edges"`
}

// AddNode adds a node to bridge graph
func (bg *BridgeGraph) AddNode(node BridgeContract) {
	bg.Nodes = append(bg.Nodes, node)
}

// AddEdge adds an edge to bridge graph
func (bg *BridgeGraph) AddEdge(edge BridgeLink) {
	bg.Edges = append(bg.Edges, edge)
}