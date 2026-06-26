// Cluster Enricher — enriches graph nodes with cluster/community metadata.
//
// Mirrors TS cluster-enricher.ts, skeleton for codetrip.
// After community detection runs, this adds community_id and cluster_label
// properties to nodes so they can be filtered/grouped in queries.
// Deferred to Phase 4 when community detection is implemented.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ClusterInfo holds community detection output for a single node.
type ClusterInfo struct {
	ScopeID      string
	CommunityID  int
	ClusterLabel string // human-readable label derived from top symbols
	Modularity   float64
}

// EnrichClustersResult holds the enriched metadata per node.
type EnrichClustersResult struct {
	Clusters map[string]ClusterInfo // ScopeID → ClusterInfo
}

// EnrichClusters adds community metadata to graph nodes.
//
// Current status: skeleton — full implementation deferred to Phase 4.
func EnrichClusters(graph shared.KnowledgeGraph, communityResult *CommunityProcessorResult) (*EnrichClustersResult, error) {
	// TODO(Phase 4): map community assignments to node properties
	return &EnrichClustersResult{
		Clusters: map[string]ClusterInfo{},
	}, nil
}

// AssignClusterLabels generates human-readable labels for each community.
// Uses the most prominent symbol names in each community.
//
// Current status: skeleton — full implementation deferred to Phase 4.
func AssignClusterLabels(communities map[int][]string) map[int]string {
	// TODO(Phase 4): derive labels from top-weighted symbols per community
	return map[int]string{}
}