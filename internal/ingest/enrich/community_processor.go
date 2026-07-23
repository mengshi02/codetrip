package enrich

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
)

// Community Detection Processor.
//
// Uses the Leiden algorithm to detect communities/clusters in the code graph
// based on CALLS, EXTENDS, and IMPLEMENTS relationships.
//
// Communities represent groups of code that work together frequently,
// helping agents navigate the codebase by functional area rather than file structure.
//
// The Go implementation includes a simplified Leiden algorithm (no external dependency)
// that provides equivalent community detection quality.

// ─────────────────────────────────────────────────────────────────────────────
// Public types
// ─────────────────────────────────────────────────────────────────────────────

// CommunityNode represents a detected community.
type CommunityNode struct {
	ID             string
	Label          string
	HeuristicLabel string
	Cohesion       float64
	SymbolCount    int
}

// CommunityMembership represents a node's membership in a community.
type CommunityMembership struct {
	NodeID      string
	CommunityID string
}

// CommunityDetectionResult holds the complete community detection result.
type CommunityDetectionResult struct {
	Communities []CommunityNode
	Memberships []CommunityMembership
	Stats       CommunityStats
}

// CommunityStats holds statistics about community detection.
type CommunityStats struct {
	TotalCommunities int
	Modularity       float64
	NodesProcessed   int
}

// ─────────────────────────────────────────────────────────────────────────────
// Leiden algorithm implementation (simplified, pure Go)
// ─────────────────────────────────────────────────────────────────────────────

// leidenGraph is an undirected adjacency structure for Leiden.
type leidenGraph struct {
	nodes    []string            // node IDs
	adj      map[string][]string // adjacency list
	nodeAttr map[string]leidenNodeAttr
}

type leidenNodeAttr struct {
	Name     string
	FilePath string
	Type     string
}

// leidenCommunity represents community assignments.
type leidenResult struct {
	communities map[string]int // nodeID → community number
	count       int            // number of communities
	modularity  float64
}

// ─────────────────────────────────────────────────────────────────────────────
// Build Leiden input graph from KnowledgeGraph
// ─────────────────────────────────────────────────────────────────────────────

var clusteringRelTypes = map[graph.RelationshipType]bool{
	graph.RelCALLS:      true,
	graph.RelEXTENDS:    true,
	graph.RelIMPLEMENTS: true,
}

var symbolNodeLabels = map[graph.NodeLabel]bool{
	graph.LabelFunction:  true,
	graph.LabelClass:     true,
	graph.LabelMethod:    true,
	graph.LabelInterface: true,
}

// Generic folder names to skip for heuristic labels
var genericFolders = map[string]bool{
	"src": true, "lib": true, "core": true, "utils": true,
	"common": true, "shared": true, "helpers": true,
}

const minConfidenceLarge = 0.5

// buildLeidenGraph constructs the undirected graph for community detection.
func buildLeidenGraph(kg *graph.KnowledgeGraph, isLarge bool) *leidenGraph {
	lg := &leidenGraph{
		adj:      make(map[string][]string),
		nodeAttr: make(map[string]leidenNodeAttr),
	}

	connectedNodes := make(map[string]bool)
	nodeDegree := make(map[string]int)

	// Collect connected nodes from clustering relationships
	kg.ForEachRelationship(func(rel *graph.GraphRelationship) {
		if !clusteringRelTypes[rel.Type] || rel.SourceID == rel.TargetID {
			return
		}
		if isLarge && rel.Confidence < minConfidenceLarge {
			return
		}
		connectedNodes[rel.SourceID] = true
		connectedNodes[rel.TargetID] = true
		nodeDegree[rel.SourceID]++
		nodeDegree[rel.TargetID]++
	})

	// Add symbol nodes that are connected (sorted for deterministic community IDs)
	var symbolNodes []string
	kg.ForEachNode(func(node *graph.GraphNode) {
		if !symbolNodeLabels[node.Label] || !connectedNodes[node.ID] {
			return
		}
		if isLarge && nodeDegree[node.ID] < 2 {
			return
		}
		symbolNodes = append(symbolNodes, node.ID)
		lg.nodeAttr[node.ID] = leidenNodeAttr{
			Name:     node.Properties.Name,
			FilePath: node.Properties.FilePath,
			Type:     string(node.Label),
		}
	})
	sort.Strings(symbolNodes)
	lg.nodes = symbolNodes

	// Build set of nodes that are in the graph
	nodeSet := make(map[string]bool, len(lg.nodes))
	for _, id := range lg.nodes {
		nodeSet[id] = true
	}

	// Add edges (undirected, deduplicated)
	seenEdges := make(map[string]bool)
	kg.ForEachRelationship(func(rel *graph.GraphRelationship) {
		if !clusteringRelTypes[rel.Type] {
			return
		}
		if isLarge && rel.Confidence < minConfidenceLarge {
			return
		}
		if rel.SourceID == rel.TargetID {
			return
		}
		// Only add if both endpoints are symbol nodes in the graph
		if !nodeSet[rel.SourceID] || !nodeSet[rel.TargetID] {
			return
		}
		if isLarge && (nodeDegree[rel.SourceID] < 2 || nodeDegree[rel.TargetID] < 2) {
			return
		}
		// Deduplicate undirected edges
		key := rel.SourceID + "<->" + rel.TargetID
		revKey := rel.TargetID + "<->" + rel.SourceID
		if seenEdges[key] || seenEdges[revKey] {
			return
		}
		seenEdges[key] = true
		lg.adj[rel.SourceID] = append(lg.adj[rel.SourceID], rel.TargetID)
		lg.adj[rel.TargetID] = append(lg.adj[rel.TargetID], rel.SourceID)
	})

	return lg
}

// ─────────────────────────────────────────────────────────────────────────────
// Simplified Leiden algorithm
// ─────────────────────────────────────────────────────────────────────────────

// runLeiden executes the complete Leiden community detection algorithm.
// This is a faithful port of graphology's Leiden implementation including:
//   - Local moving phase with random walk
//   - Refinement phase (mergeNodesSubset) ensuring well-connected communities
//   - Aggregation/zoom-out for multi-level iteration
//   - Isolate operation for negative modularity gain
func runLeiden(lg *leidenGraph, resolution float64, maxIterations int) leidenResult {
	if len(lg.nodes) == 0 {
		return leidenResult{communities: map[string]int{}, count: 0, modularity: 0}
	}

	// Build CSR index from the leiden graph
	idx := newUndirectedLouvainIndex(lg, resolution)

	if idx.E == 0 {
		// No edges — everyone is isolated
		communities := make(map[string]int, len(lg.nodes))
		for i, nodeID := range lg.nodes {
			communities[nodeID] = i
		}
		return leidenResult{communities: communities, count: len(lg.nodes), modularity: 0}
	}

	// Set up RNG — use a fixed seed for deterministic results
	// (graphology uses Math.random by default, but for reproducibility we use a seed)
	rng := rand.New(rand.NewSource(42))

	// Create addenda for the refinement phase
	addenda := newUndirectedLeidenAddenda(idx, 0.01, rng)

	// Run the main Leiden algorithm loop
	undirectedLeidenMain(idx, addenda, rng, true)

	// Collect results
	communities := idx.collect()

	// Count unique communities and compute modularity
	uniqueComms := make(map[int]bool)
	for _, c := range communities {
		uniqueComms[c] = true
	}

	mod := idx.modularity()

	log.Printf("[leiden] C=%d uniqueComms=%d modularity=%.6f M=%.1f E=%d", idx.C, len(uniqueComms), mod, idx.M, idx.E)

	return leidenResult{
		communities: communities,
		count:       len(uniqueComms),
		modularity:  mod,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Heuristic label generation
// ─────────────────────────────────────────────────────────────────────────────

// generateHeuristicLabel creates a human-readable label from folder patterns.
func generateHeuristicLabel(memberIDs []string, nodePathMap map[string]string, nodeAttr map[string]leidenNodeAttr, commNum int) string {
	folderCounts := make(map[string]int)

	for _, nodeID := range memberIDs {
		fp := nodePathMap[nodeID]
		if fp == "" {
			fp = nodeAttr[nodeID].FilePath
		}
		parts := strings.Split(filepath.ToSlash(fp), "/")

		if len(parts) >= 2 {
			folder := parts[len(parts)-2]
			lower := strings.ToLower(folder)
			if !genericFolders[lower] {
				folderCounts[folder]++
			}
		}
	}

	// Find most common folder
	bestFolder := ""
	maxCount := 0
	for folder, count := range folderCounts {
		if count > maxCount {
			maxCount = count
			bestFolder = folder
		}
	}

	if bestFolder != "" {
		return capitalize(bestFolder)
	}

	// Fallback: use node names to detect common prefixes
	names := make([]string, 0, len(memberIDs))
	for _, nodeID := range memberIDs {
		name := nodeAttr[nodeID].Name
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) > 2 {
		prefix := findCommonPrefix(names)
		if len(prefix) > 2 {
			return capitalize(prefix)
		}
	}

	// Last resort
	return fmt.Sprintf("Cluster_%d", commNum)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// findCommonPrefix finds the longest common prefix among strings.
func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	sorted := make([]string, len(strs))
	copy(sorted, strs)
	sort.Strings(sorted)

	first := sorted[0]
	last := sorted[len(sorted)-1]

	i := 0
	for i < len(first) && i < len(last) && first[i] == last[i] {
		i++
	}
	return first[:i]
}

// ─────────────────────────────────────────────────────────────────────────────
// Cohesion calculation
// ─────────────────────────────────────────────────────────────────────────────

// calculateCohesion estimates cohesion (0-1) based on internal edge density.
func calculateCohesion(memberIDs []string, adj map[string][]string) float64 {
	if len(memberIDs) <= 1 {
		return 1.0
	}

	memberSet := make(map[string]bool)
	for _, id := range memberIDs {
		memberSet[id] = true
	}

	// Sample up to 50 members for large communities
	sampleSize := 50
	sample := memberIDs
	if len(memberIDs) > sampleSize {
		sample = memberIDs[:sampleSize]
	}

	internalEdges := 0
	totalEdges := 0

	for _, nodeID := range sample {
		neighbors := adj[nodeID]
		for _, nbr := range neighbors {
			totalEdges++
			if memberSet[nbr] {
				internalEdges++
			}
		}
	}

	if totalEdges == 0 {
		return 1.0
	}
	return math.Min(1.0, float64(internalEdges)/float64(totalEdges))
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessCommunities — main entry point.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessCommunities(kg *graph.KnowledgeGraph) CommunityDetectionResult {
	// Pre-check total symbol count to determine large-graph mode
	symbolCount := 0
	kg.ForEachNode(func(node *graph.GraphNode) {
		if symbolNodeLabels[node.Label] {
			symbolCount++
		}
	})
	isLarge := symbolCount > 10000

	// Build Leiden input graph
	lg := buildLeidenGraph(kg, isLarge)

	if len(lg.nodes) == 0 {
		return CommunityDetectionResult{
			Communities: []CommunityNode{},
			Memberships: []CommunityMembership{},
			Stats:       CommunityStats{TotalCommunities: 0, Modularity: 0, NodesProcessed: 0},
		}
	}

	// Run Leiden algorithm
	resolution := 1.0
	maxIter := 0
	if isLarge {
		resolution = 2.0
		maxIter = 3
	}

	result := runLeiden(lg, resolution, maxIter)

	communities := result.communities

	// Create community nodes
	communityNodes := createCommunityNodes(communities, result.count, lg, kg)
	materializedCommunities := make(map[string]struct{}, len(communityNodes))
	for _, community := range communityNodes {
		materializedCommunities[community.ID] = struct{}{}
	}

	// Create membership mappings only for materialized communities. Singleton
	// communities are intentionally omitted above and must not leave dangling
	// MEMBER_OF relationships behind.
	var memberships []CommunityMembership
	for nodeID, commNum := range communities {
		communityID := fmt.Sprintf("comm_%d", commNum)
		if _, ok := materializedCommunities[communityID]; !ok {
			continue
		}
		memberships = append(memberships, CommunityMembership{
			NodeID:      nodeID,
			CommunityID: communityID,
		})
	}

	return CommunityDetectionResult{
		Communities: communityNodes,
		Memberships: memberships,
		Stats: CommunityStats{
			TotalCommunities: len(communityNodes),
			Modularity:       result.modularity,
			NodesProcessed:   len(lg.nodes),
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// createCommunityNodes — creates Community nodes with heuristic labels.
// ─────────────────────────────────────────────────────────────────────────────

func createCommunityNodes(
	communities map[string]int,
	communityCount int,
	lg *leidenGraph,
	kg *graph.KnowledgeGraph,
) []CommunityNode {
	// Group node IDs by community
	communityMembers := make(map[int][]string)
	for nodeID, commNum := range communities {
		communityMembers[commNum] = append(communityMembers[commNum], nodeID)
	}

	// Build node path lookup
	nodePathMap := make(map[string]string)
	kg.ForEachNode(func(node *graph.GraphNode) {
		if node.Properties.FilePath != "" {
			nodePathMap[node.ID] = node.Properties.FilePath
		}
	})

	// Create community nodes — skip singletons
	var communityNodes []CommunityNode

	for commNum, memberIDs := range communityMembers {
		if len(memberIDs) < 2 {
			continue
		}

		heuristicLabel := generateHeuristicLabel(memberIDs, nodePathMap, lg.nodeAttr, commNum)

		communityNodes = append(communityNodes, CommunityNode{
			ID:             fmt.Sprintf("comm_%d", commNum),
			Label:          heuristicLabel,
			HeuristicLabel: heuristicLabel,
			Cohesion:       calculateCohesion(memberIDs, lg.adj),
			SymbolCount:    len(memberIDs),
		})
	}

	// Sort by size descending
	sort.Slice(communityNodes, func(i, j int) bool {
		return communityNodes[i].SymbolCount > communityNodes[j].SymbolCount
	})

	return communityNodes
}

// ─────────────────────────────────────────────────────────────────────────────
// ApplyCommunitiesToGraph — adds Community nodes and MEMBER_OF edges to KG.
// ─────────────────────────────────────────────────────────────────────────────

func ApplyCommunitiesToGraph(kg *graph.KnowledgeGraph, result CommunityDetectionResult) {
	// Add Community nodes
	for _, cn := range result.Communities {
		symbolCount := cn.SymbolCount
		cohesion := cn.Cohesion
		kg.AddNode(&graph.GraphNode{
			ID:    cn.ID,
			Label: graph.LabelCommunity,
			Properties: graph.NodeProperties{
				Name:           cn.HeuristicLabel,
				HeuristicLabel: cn.HeuristicLabel,
				SymbolCount:    floatPtr(float64(symbolCount)),
				Cohesion:       floatPtr(cohesion),
				Keywords:       []string{},
				EnrichedBy:     "heuristic",
			},
		})
	}

	// Add MEMBER_OF relationships
	for _, membership := range result.Memberships {
		if _, sourceExists := kg.GetNode(membership.NodeID); !sourceExists {
			continue
		}
		if _, communityExists := kg.GetNode(membership.CommunityID); !communityExists {
			continue
		}
		relID := graph.GenerateID("MEMBER_OF", membership.NodeID+"->"+membership.CommunityID)
		kg.AddRelationship(&graph.GraphRelationship{
			ID:         relID,
			SourceID:   membership.NodeID,
			TargetID:   membership.CommunityID,
			Type:       graph.RelMEMBER_OF,
			Confidence: 1.0,
			Reason:     "leiden-algorithm",
		})
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
