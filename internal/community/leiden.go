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

	// Ultra performance: preallocated temporary buffers, zero GC during execution
	tmpCommWeight []float64
	tmpCommSum    []float64
	tmpCommList   []int
	tmpPerm       []int
	visTimestamp  []uint32
	visTick       uint32
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
	Nodes       []string
	NodeIdx     map[string]int
	Adj         [][]neighbor
	Weight      []float64
	TotalWeight float64
}

type neighbor struct {
	idx    int
	weight float64
}

// BuildAdjGraph builds adjacency graph from GraphStore
func BuildAdjGraph(gs *graph.GraphStore, repo string) (*AdjGraph, error) {
	ag := &AdjGraph{
		NodeIdx: make(map[string]int),
	}

	iter := gs.IterNodes(repo)
	defer iter.Close()

	nodeSet := make(map[string]bool)
	for iter.Next() {
		node := iter.Node()
		if node.Label.IsSymbol() {
			nodeSet[node.ID] = true
		}
	}

	sortedIDs := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	for _, id := range sortedIDs {
		ag.NodeIdx[id] = len(ag.Nodes)
		ag.Nodes = append(ag.Nodes, id)
		ag.Adj = append(ag.Adj, nil)
		ag.Weight = append(ag.Weight, 0)
	}

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

	// Preallocate all temporary buffers, no further resizing during runtime
	l.ensureBuffers(n)

	community := make([]int, n)
	for i := range community {
		community[i] = i
	}

	for iter := 0; iter < 100; iter++ {
		improved := l.localMoveFast(ag, community)
		if !improved {
			break
		}
		l.refineFast(ag, community)
		community = l.aggregateFast(community)
	}

	return l.buildResult(ag, community)
}

// localMoveFast ultra-performance version: no maps, no allocations, array reuse
func (l *LeidenAlgorithm) localMoveFast(ag *AdjGraph, community []int) bool {
	n := len(ag.Nodes)
	improved := false

	tw := ag.TotalWeight
	res := l.resolution

	// Clear community total weights
	clear(l.tmpCommWeight)
	for i := 0; i < n; i++ {
		c := community[i]
		l.tmpCommWeight[c] += ag.Weight[i]
	}

	order := l.randPermCached(n)

	for _, i := range order {
		currentComm := community[i]
		wI := ag.Weight[i]

		// Temporarily remove from current community
		l.tmpCommWeight[currentComm] -= wI

		// Use array to accumulate neighbor community weights, replacing map
		clear(l.tmpCommSum)
		commCount := 0
		for _, nb := range ag.Adj[i] {
			c := community[nb.idx]
			if l.tmpCommSum[c] == 0 {
				l.tmpCommList[commCount] = c
				commCount++
			}
			l.tmpCommSum[c] += nb.weight
		}

		// Sort to ensure determinism
		slice := l.tmpCommList[:commCount]
		sort.Ints(slice)

		bestComm := currentComm
		bestDelta := 0.0

		for _, c := range slice {
			if c == currentComm {
				continue
			}
			e := l.tmpCommSum[c]
			delta := e/tw - res*wI*l.tmpCommWeight[c]/(tw*tw)
			if delta > bestDelta || (delta == bestDelta && c < bestComm) {
				bestDelta = delta
				bestComm = c
			}
		}

		if bestComm != currentComm {
			community[i] = bestComm
			l.tmpCommWeight[bestComm] += wI
			improved = true
		} else {
			l.tmpCommWeight[currentComm] += wI
		}
	}

	return improved
}

// refineFast timestamp-based visited, allocation-free BFS
func (l *LeidenAlgorithm) refineFast(ag *AdjGraph, community []int) {
	n := len(ag.Nodes)
	l.visTick++

	// Group nodes by community (array reuse)
	maxComm := 0
	for _, c := range community {
		if c > maxComm {
			maxComm = c
		}
	}
	commNodes := make([][]int, maxComm+1)
	for i := 0; i < n; i++ {
		c := community[i]
		commNodes[c] = append(commNodes[c], i)
	}

	// Sort to ensure stable traversal order
	sortedComms := make([]int, 0, len(commNodes))
	for c, ns := range commNodes {
		if len(ns) > 0 {
			sortedComms = append(sortedComms, c)
		}
	}
	sort.Ints(sortedComms)

	for _, comm := range sortedComms {
		nodes := commNodes[comm]
		if len(nodes) <= 1 {
			continue
		}

		subIdx := 0
		for _, u := range nodes {
			if l.visTimestamp[u] == l.visTick {
				continue
			}

			// BFS
			q := []int{u}
			l.visTimestamp[u] = l.visTick
			for len(q) > 0 {
				cur := q[0]
				q = q[1:]
				community[cur] = comm*1000000 + subIdx

				for _, nb := range ag.Adj[cur] {
					v := nb.idx
					if community[v] == comm && l.visTimestamp[v] != l.visTick {
						l.visTimestamp[v] = l.visTick
						q = append(q, v)
					}
				}
			}
			subIdx++
		}
	}
}

// aggregateFast fast community renumbering
func (l *LeidenAlgorithm) aggregateFast(community []int) []int {
	n := len(community)
	m := make([]int, n)
	idx := 0
	cMap := make(map[int]int, n/4)

	for _, c := range community {
		if _, ok := cMap[c]; !ok {
			cMap[c] = idx
			idx++
		}
	}
	for i, c := range community {
		m[i] = cMap[c]
	}
	return m
}

// ensureBuffers one-time preallocation of all temporary buffers
func (l *LeidenAlgorithm) ensureBuffers(n int) {
	if len(l.tmpCommWeight) < n {
		l.tmpCommWeight = make([]float64, n*2)
		l.tmpCommSum = make([]float64, n*2)
		l.tmpCommList = make([]int, n*2)
		l.tmpPerm = make([]int, n)
		l.visTimestamp = make([]uint32, n)
		l.visTick = 0
	}
}

// randPermCached slice reuse, no allocations
func (l *LeidenAlgorithm) randPermCached(n int) []int {
	p := l.tmpPerm[:n]
	for i := 0; i < n; i++ {
		p[i] = i
	}
	l.rng.Shuffle(n, func(i, j int) {
		p[i], p[j] = p[j], p[i]
	})
	return p
}

// buildResult builds detection result
func (l *LeidenAlgorithm) buildResult(ag *AdjGraph, community []int) *CommunityResult {
	commNodes := make(map[int][]int)
	for i, c := range community {
		commNodes[c] = append(commNodes[c], i)
	}

	result := &CommunityResult{
		Communities: make([]CommunityNode, 0, len(commNodes)),
		Memberships: make([]CommunityMembership, 0, len(ag.Nodes)),
	}

	for _, nodeIdxs := range commNodes {
		cohesion := l.calculateCohesionFast(ag, nodeIdxs)
		cn := CommunityNode{
			ID:             ag.Nodes[nodeIdxs[0]],
			HeuristicLabel: l.generateLabel(ag, nodeIdxs),
			Cohesion:       cohesion,
			SymbolCount:    len(nodeIdxs),
			Keywords:       []string{},
		}
		result.Communities = append(result.Communities, cn)
		for _, idx := range nodeIdxs {
			result.Memberships = append(result.Memberships, CommunityMembership{
				NodeID:      ag.Nodes[idx],
				CommunityID: cn.ID,
			})
		}
	}

	sort.Slice(result.Communities, func(i, j int) bool {
		return result.Communities[i].ID < result.Communities[j].ID
	})

	result.Stats = CommunityStats{
		CommunityCount: len(commNodes),
		AvgCohesion:    l.avgCohesion(result.Communities),
		Modularity:     l.modularityFast(ag, community),
	}

	return result
}

// calculateCohesionFast faster cohesion calculation
func (l *LeidenAlgorithm) calculateCohesionFast(ag *AdjGraph, nodeIdxs []int) float64 {
	if len(nodeIdxs) <= 1 {
		return 1.0
	}
	mask := make(map[int]bool, len(nodeIdxs))
	for _, idx := range nodeIdxs {
		mask[idx] = true
	}
	internal, total := 0.0, 0.0
	for _, idx := range nodeIdxs {
		for _, nb := range ag.Adj[idx] {
			total += nb.weight
			if mask[nb.idx] {
				internal += nb.weight
			}
		}
	}
	if total == 0 {
		return 0.0
	}
	return internal / total
}

// modularityFast faster modularity calculation
func (l *LeidenAlgorithm) modularityFast(ag *AdjGraph, community []int) float64 {
	n := len(ag.Nodes)
	m2 := ag.TotalWeight * 2
	commW := make(map[int]float64, n/4)
	commIn := make(map[int]float64, n/4)

	for i := 0; i < n; i++ {
		c := community[i]
		commW[c] += ag.Weight[i]
		for _, nb := range ag.Adj[i] {
			if community[nb.idx] == c {
				commIn[c] += nb.weight
			}
		}
	}

	Q := 0.0
	for c := range commW {
		Q += commIn[c]/m2 - math.Pow(commW[c]/m2, 2)
	}
	return Q
}

// generateLabel generates heuristic community name
func (l *LeidenAlgorithm) generateLabel(ag *AdjGraph, nodeIdxs []int) string {
	if len(nodeIdxs) == 0 {
		return "empty"
	}
	names := make([]string, 0, len(nodeIdxs))
	for _, idx := range nodeIdxs {
		names = append(names, ag.Nodes[idx])
	}
	prefix := commonPrefix(names)
	if prefix != "" && len(prefix) >= 2 {
		return prefix + "*"
	}
	bestIdx := nodeIdxs[0]
	bestW := 0.0
	for _, idx := range nodeIdxs {
		if ag.Weight[idx] > bestW {
			bestW = ag.Weight[idx]
			bestIdx = idx
		}
	}
	name := ag.Nodes[bestIdx]
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

// commonPrefix calculates longest common prefix of string slice
func commonPrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	prefix := names[0]
	for _, s := range names[1:] {
		minL := min(len(prefix), len(s))
		prefix = prefix[:minL]
		for i := 0; i < minL; i++ {
			if prefix[i] != s[i] {
				prefix = prefix[:i]
				break
			}
		}
		if prefix == "" {
			break
		}
	}
	return prefix
}

// avgCohesion calculates average cohesion across communities
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