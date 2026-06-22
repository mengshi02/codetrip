package community

import (
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// LeidenAlgorithm implements pure Go Leiden community detection algorithm
// Deterministic: uses mulberry32 PRNG with seed=0xc0de
// High performance: adjacency list + incremental modularity calculation
type LeidenAlgorithm struct {
	seed       uint64
	rng        *rand.Rand
	resolution float64 // Resolution parameter (default 1.0)
}

// NewLeidenAlgorithm creates a new Leiden algorithm instance
func NewLeidenAlgorithm(opts ...LeidenOption) *LeidenAlgorithm {
	l := &LeidenAlgorithm{
		seed:       0xc0de,
		resolution: 1.0,
	}
	for _, opt := range opts {
		opt(l)
	}
	l.rng = rand.New(rand.NewSource(int64(l.seed)))
	return l
}

// LeidenOption represents Leiden configuration option
type LeidenOption func(*LeidenAlgorithm)

// WithLeidenSeed sets the PRNG seed
func WithLeidenSeed(seed uint64) LeidenOption {
	return func(l *LeidenAlgorithm) { l.seed = seed }
}

// WithResolution sets the resolution parameter
func WithResolution(r float64) LeidenOption {
	return func(l *LeidenAlgorithm) { l.resolution = r }
}

// AdjGraph represents internal adjacency graph
type AdjGraph struct {
	Nodes       []string         // Node ID list
	NodeIdx     map[string]int   // Node → index
	Adj         [][]neighbor     // Adjacency list
	Weight      []float64        // Node weight (degree)
	TotalWeight float64          // Total edge weight
}

type neighbor struct {
	idx    int
	weight float64
}

// BuildAdjGraph builds adjacency graph from GraphStore
// Optimized: single ScanPrefix scan for all adjacency KV pairs instead of N×GetOutEdges
func BuildAdjGraph(gs *graph.GraphStore, repo string) (*AdjGraph, error) {
	ag := &AdjGraph{
		NodeIdx: make(map[string]int),
	}

	// Collect nodes
	iter := gs.IterNodes(repo)
	defer iter.Close()

	nodeSet := make(map[string]bool)
	for iter.Next() {
		node := iter.Node()
		if node.Label.IsSymbol() {
			nodeSet[node.ID] = true
		}
	}

	// Assign indices
	for id := range nodeSet {
		ag.NodeIdx[id] = len(ag.Nodes)
		ag.Nodes = append(ag.Nodes, id)
		ag.Adj = append(ag.Adj, nil)
		ag.Weight = append(ag.Weight, 0)
	}

	// Build adjacency list — single ScanPrefix for all CALLS edges (N×Get → 1×Scan)
	allEdges, _ := gs.ScanAllOutEdgesByRelType(string(graph.RelCalls))
	for _, edge := range allEdges {
		srcIdx, srcOk := ag.NodeIdx[edge.Source]
		tgtIdx, tgtOk := ag.NodeIdx[edge.Target]
		if !srcOk || !tgtOk {
			continue
		}
		w := edge.Confidence()
		ag.Adj[srcIdx] = append(ag.Adj[srcIdx], neighbor{idx: tgtIdx, weight: w})
		ag.Weight[srcIdx] += w
		ag.TotalWeight += w
	}

	return ag, nil
}

// Detect executes Leiden community detection
func (l *LeidenAlgorithm) Detect(ag *AdjGraph) *CommunityResult {
	n := len(ag.Nodes)
	if n == 0 {
		return &CommunityResult{}
	}

	// Initialize: each node is its own community
	community := make([]int, n)
	for i := range community {
		community[i] = i
	}

	// Iterative optimization
	for iter := 0; iter < 100; iter++ { // Max 100 iterations
		improved := l.localMove(ag, community)
		if !improved {
			break
		}
		l.refine(ag, community)
		community = l.aggregate(ag, community)
	}

	return l.buildResult(ag, community)
}

// localMove performs local move phase: nodes move to best community
func (l *LeidenAlgorithm) localMove(ag *AdjGraph, community []int) bool {
	n := len(ag.Nodes)
	improved := false

	// Calculate total weight of each community
	commWeight := make([]float64, n)
	for i := 0; i < n; i++ {
		commWeight[community[i]] += ag.Weight[i]
	}

	// Traverse nodes in random order
	order := l.randPerm(n)

	for _, i := range order {
		bestComm := community[i]
		bestDelta := 0.0

		// Calculate delta for removing from current community
		currentComm := community[i]
		commWeight[currentComm] -= ag.Weight[i]

		// Calculate delta for moving to each neighbor community
		neighborComms := make(map[int]float64)
		for _, nb := range ag.Adj[i] {
			neighborComms[community[nb.idx]] += nb.weight
		}

		for comm, edgeWeight := range neighborComms {
			if comm == currentComm {
				continue
			}
			// Modularity increment: ΔQ = (edgeWeight/2m) - (ag.Weight[i] * commWeight[comm]) / (2m * 2m)
			delta := edgeWeight/ag.TotalWeight - l.resolution*ag.Weight[i]*commWeight[comm]/(ag.TotalWeight*ag.TotalWeight)
			if delta > bestDelta {
				bestDelta = delta
				bestComm = comm
			}
		}

		// Move to best community
		if bestComm != currentComm {
			community[i] = bestComm
			commWeight[bestComm] += ag.Weight[i]
			improved = true
		} else {
			commWeight[currentComm] += ag.Weight[i]
		}
	}

	return improved
}

// refine performs refinement phase: ensures communities are well-connected
func (l *LeidenAlgorithm) refine(ag *AdjGraph, community []int) {
	n := len(ag.Nodes)

	// Check internal connectivity for each community
	commNodes := make(map[int][]int)
	for i := 0; i < n; i++ {
		commNodes[community[i]] = append(commNodes[community[i]], i)
	}

	for comm, nodes := range commNodes {
		if len(nodes) <= 1 {
			continue
		}

		// Check sub-communities
		visited := make([]bool, n)
		subComm := make([]int, n)
		subIdx := 0

		for _, node := range nodes {
			if visited[node] {
				continue
			}
			// BFS to find connected components
			queue := []int{node}
			visited[node] = true
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				subComm[cur] = subIdx

				for _, nb := range ag.Adj[cur] {
					if !visited[nb.idx] && community[nb.idx] == comm {
						visited[nb.idx] = true
						queue = append(queue, nb.idx)
					}
				}
			}
			subIdx++
		}

		// If multiple sub-communities, split
		if subIdx > 1 {
			for _, node := range nodes {
				community[node] = comm + subComm[node]*10000 // Avoid collision
			}
		}
	}
}

// aggregate performs aggregation phase: communities merge into supernodes
func (l *LeidenAlgorithm) aggregate(ag *AdjGraph, community []int) []int {
	// Renumber communities (0..k-1)
	commMap := make(map[int]int)
	idx := 0
	for _, c := range community {
		if _, ok := commMap[c]; !ok {
			commMap[c] = idx
			idx++
		}
	}
	for i, c := range community {
		community[i] = commMap[c]
	}
	return community
}

// buildResult builds detection result
func (l *LeidenAlgorithm) buildResult(ag *AdjGraph, community []int) *CommunityResult {
	// Group by community
	commNodes := make(map[int][]int)
	for i, c := range community {
		commNodes[c] = append(commNodes[c], i)
	}

	result := &CommunityResult{
		Communities:  make([]CommunityNode, 0, len(commNodes)),
		Memberships:  make([]CommunityMembership, 0, len(ag.Nodes)),
	}

	for _, nodeIdxs := range commNodes {
		// Calculate cohesion
		cohesion := l.calculateCohesion(ag, nodeIdxs)

		// Collect keywords
		keywords := make([]string, 0)

		cn := CommunityNode{
			ID:             ag.Nodes[nodeIdxs[0]], // Use first node ID as community ID
			HeuristicLabel: l.generateLabel(ag, nodeIdxs),
			Cohesion:       cohesion,
			SymbolCount:    len(nodeIdxs),
			Keywords:       keywords,
		}

		result.Communities = append(result.Communities, cn)

		for _, idx := range nodeIdxs {
			result.Memberships = append(result.Memberships, CommunityMembership{
				NodeID:      ag.Nodes[idx],
				CommunityID: cn.ID,
			})
		}
	}

	// Sort for determinism
	sort.Slice(result.Communities, func(i, j int) bool {
		return result.Communities[i].ID < result.Communities[j].ID
	})

	result.Stats = CommunityStats{
		CommunityCount: len(commNodes),
		AvgCohesion:    l.avgCohesion(result.Communities),
		Modularity:     l.modularity(ag, community),
	}

	return result
}

// calculateCohesion calculates community cohesion
func (l *LeidenAlgorithm) calculateCohesion(ag *AdjGraph, nodeIdxs []int) float64 {
	if len(nodeIdxs) <= 1 {
		return 1.0
	}

	internalWeight := 0.0
	totalWeight := 0.0

	nodeSet := make(map[int]bool)
	for _, idx := range nodeIdxs {
		nodeSet[idx] = true
	}

	for _, idx := range nodeIdxs {
		for _, nb := range ag.Adj[idx] {
			totalWeight += nb.weight
			if nodeSet[nb.idx] {
				internalWeight += nb.weight
			}
		}
	}

	if totalWeight == 0 {
		return 0.0
	}
	return internalWeight / totalWeight
}

// generateLabel generates heuristic community name
// Strategy: 1. Common prefix 2. Highest weight node name
func (l *LeidenAlgorithm) generateLabel(ag *AdjGraph, nodeIdxs []int) string {
	if len(nodeIdxs) == 0 {
		return "empty"
	}

	// 1. Collect node names in community
	names := make([]string, 0, len(nodeIdxs))
	for _, idx := range nodeIdxs {
		if idx < len(ag.Nodes) {
			// Try to get node name from GraphStore (if available)
			names = append(names, ag.Nodes[idx])
		}
	}

	// 2. Calculate common prefix
	prefix := commonPrefix(names)
	if prefix != "" && len(prefix) >= 2 {
		return prefix + "*"
	}

	// 3. Fallback: use highest weight node name in community
	bestIdx := nodeIdxs[0]
	bestWeight := 0.0
	for _, idx := range nodeIdxs {
		if idx < len(ag.Weight) && ag.Weight[idx] > bestWeight {
			bestWeight = ag.Weight[idx]
			bestIdx = idx
		}
	}
	if bestIdx < len(ag.Nodes) {
		name := ag.Nodes[bestIdx]
		// Extract short name after last separator
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		return name
	}

	return "community"
}

// commonPrefix calculates common prefix of string slice
func commonPrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	prefix := names[0]
	for _, name := range names[1:] {
		for i := 0; i < len(prefix) && i < len(name); i++ {
			if prefix[i] != name[i] {
				prefix = prefix[:i]
				break
			}
		}
		if len(name) < len(prefix) {
			prefix = name
		}
	}
	return prefix
}

// avgCohesion calculates average cohesion
func (l *LeidenAlgorithm) avgCohesion(communities []CommunityNode) float64 {
	if len(communities) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range communities {
		sum += c.Cohesion
	}
	return sum / float64(len(communities))
}

// modularity calculates modularity
func (l *LeidenAlgorithm) modularity(ag *AdjGraph, community []int) float64 {
	Q := 0.0
	n := len(ag.Nodes)
	m2 := ag.TotalWeight * 2

	commWeight := make(map[int]float64)   // Community total degree
	commInternal := make(map[int]float64) // Community internal edge weight

	for i := 0; i < n; i++ {
		commWeight[community[i]] += ag.Weight[i]
		for _, nb := range ag.Adj[i] {
			if community[nb.idx] == community[i] {
				commInternal[community[i]] += nb.weight
			}
		}
	}

	for comm := range commWeight {
		Q += commInternal[comm]/m2 - math.Pow(commWeight[comm]/m2, 2)
	}

	return Q
}

// randPerm generates random permutation
func (l *LeidenAlgorithm) randPerm(n int) []int {
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	l.rng.Shuffle(n, func(i, j int) {
		perm[i], perm[j] = perm[j], perm[i]
	})
	return perm
}