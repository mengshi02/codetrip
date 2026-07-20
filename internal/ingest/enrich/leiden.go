package enrich

// ─────────────────────────────────────────────────────────────────────────────
// Complete Leiden Algorithm Implementation
// ─────────────────────────────────────────────────────────────────────────────
//
// Faithful port of graphology's Leiden algorithm, vendored from:
//   https://github.com/graphology/graphology/tree/master/src/communities-leiden
//
// Reference:
//   Traag, V. A., et al. "From Louvain to Leiden: Guaranteeing
//   Well-Connected Communities". Scientific Reports, vol. 9, no 1, 2019.
//   https://arxiv.org/abs/1810.08473
//
// Key differences from the previous simplified Louvain:
//   - Refinement phase (mergeNodesSubset) ensures community internal connectivity
//   - Aggregation/zoom-out creates super-nodes for multi-level iteration
//   - Random walk traversal order
//   - Isolate operation (bestDelta < 0 → singleton)
//   - SparseMap / SparseQueueSet for O(1) community lookups
//
// ─────────────────────────────────────────────────────────────────────────────

import (
	"math"
	"math/rand"
)

// ─────────────────────────────────────────────────────────────────────────────
// SparseMap — port of mnemonist/sparse-map
// ─────────────────────────────────────────────────────────────────────────────

// sparseMap is a sparse integer → float64 map using dense/sparse arrays.
// Supports O(1) set/get/delete/clear with integer keys in [0, capacity).
type sparseMap struct {
	capacity int
	vals     []float64 // dense values, indexed by sparse position
	dense    []int     // dense keys (actual keys in the map)
	sparse   []int     // sparse[key] = position in dense, or -1 if absent
	size     int       // number of entries
}

func newSparseMap(capacity int) *sparseMap {
	sm := &sparseMap{
		capacity: capacity,
		vals:     make([]float64, capacity),
		dense:    make([]int, capacity),
		sparse:   make([]int, capacity),
		size:     0,
	}
	for i := range sm.sparse {
		sm.sparse[i] = -1
	}
	return sm
}

func (sm *sparseMap) clear() {
	for i := 0; i < sm.size; i++ {
		sm.sparse[sm.dense[i]] = -1
	}
	sm.size = 0
}

func (sm *sparseMap) get(key int) (float64, bool) {
	if key < 0 || key >= sm.capacity {
		return 0, false
	}
	pos := sm.sparse[key]
	if pos < 0 || pos >= sm.size {
		return 0, false
	}
	return sm.vals[pos], true
}

func (sm *sparseMap) set(key int, val float64) {
	if key < 0 || key >= sm.capacity {
		return
	}
	pos := sm.sparse[key]
	if pos >= 0 && pos < sm.size {
		sm.vals[pos] = val
		return
	}
	sm.dense[sm.size] = key
	sm.vals[sm.size] = val
	sm.sparse[key] = sm.size
	sm.size++
}

func (sm *sparseMap) add(key int, val float64) {
	if key < 0 || key >= sm.capacity {
		return
	}
	pos := sm.sparse[key]
	if pos >= 0 && pos < sm.size {
		sm.vals[pos] += val
	} else {
		sm.dense[sm.size] = key
		sm.vals[sm.size] = val
		sm.sparse[key] = sm.size
		sm.size++
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SparseQueueSet — port of mnemonist/sparse-queue-set
// ─────────────────────────────────────────────────────────────────────────────

// sparseQueueSet is a circular-buffer queue that deduplicates items (each item enqueued at most once).
// Port of mnemonist/sparse-queue-set.
type sparseQueueSet struct {
	capacity int
	dense    []int // items in queue order (circular buffer)
	sparse   []int // sparse[key] = position in dense, or capacity if absent
	size     int   // number of items currently in queue
	start    int   // dequeue pointer (circular)
}

func newSparseQueueSet(capacity int) *sparseQueueSet {
	sq := &sparseQueueSet{
		capacity: capacity,
		dense:    make([]int, capacity),
		sparse:   make([]int, capacity),
		size:     0,
		start:    0,
	}
	for i := range sq.sparse {
		sq.sparse[i] = capacity // sentinel: not in queue
	}
	return sq
}

// has checks whether the member is currently in the queue.
func (sq *sparseQueueSet) has(member int) bool {
	if sq.size == 0 || member < 0 || member >= sq.capacity {
		return false
	}
	index := sq.sparse[member]
	if index >= sq.capacity {
		return false
	}
	// Check if index is within the circular window [start, start+size)
	if sq.start+sq.size <= sq.capacity {
		return index >= sq.start && index < sq.start+sq.size && sq.dense[index] == member
	}
	// Wraps around
	return (index >= sq.start || index < (sq.start+sq.size)%sq.capacity) && sq.dense[index] == member
}

func (sq *sparseQueueSet) enqueue(item int) {
	if item < 0 || item >= sq.capacity {
		return
	}
	if sq.has(item) {
		return // already in queue
	}
	index := (sq.start + sq.size) % sq.capacity
	sq.dense[index] = item
	sq.sparse[item] = index
	sq.size++
}

func (sq *sparseQueueSet) dequeue() (int, bool) {
	if sq.size == 0 {
		return 0, false
	}
	index := sq.start
	item := sq.dense[index]
	sq.sparse[item] = sq.capacity // mark as absent
	sq.size--
	sq.start++
	if sq.start == sq.capacity {
		sq.start = 0
	}
	return item, true
}

func (sq *sparseQueueSet) clear() {
	sq.start = 0
	sq.size = 0
	// Don't need to clear dense/sparse since has() uses size==0 as fast path
}

// ─────────────────────────────────────────────────────────────────────────────
// UndirectedLouvainIndex — port of graphology-indices/louvain
// ─────────────────────────────────────────────────────────────────────────────

// undirectedLouvainIndex is the CSR-format adjacency index for Louvain/Leiden.
type undirectedLouvainIndex struct {
	C          int      // current number of nodes (communities at current level)
	M          float64  // total edge weight (sum of all edge weights)
	E          int      // current number of directed edges in CSR
	U          int      // number of unused community slots
	resolution float64  // resolution parameter (gamma)
	level      int      // current zoom-out level
	nodes      []string // original node IDs (length = original order)
	N          int      // original number of nodes (does not change with zoomOut)

	// Edge-level (CSR format)
	neighborhood []int     // neighbor indices
	weights      []float64 // edge weights

	// Node-level
	loops      []float64 // self-loop weight for each node
	starts     []int     // CSR row pointers (starts[i]..starts[i+1] are neighbors of i)
	belongings []int     // community assignment for each node
	mapping    []int     // maps original node → final community (updated on zoomOut)

	// Community-level
	counts       []int     // number of nodes in each community
	unused       []int     // stack of unused community IDs
	totalWeights []float64 // total degree weight for each community
}

// newUndirectedLouvainIndex builds the CSR index from a leidenGraph.
func newUndirectedLouvainIndex(lg *leidenGraph, resolution float64) *undirectedLouvainIndex {
	order := len(lg.nodes)
	if order == 0 {
		return &undirectedLouvainIndex{resolution: resolution}
	}

	// Count total directed edges (each undirected edge → 2 directed)
	totalDirectedEdges := 0
	for _, nodeID := range lg.nodes {
		totalDirectedEdges += len(lg.adj[nodeID])
	}

	idx := &undirectedLouvainIndex{
		C:            order,
		M:            0,
		E:            totalDirectedEdges,
		U:            0,
		resolution:   resolution,
		level:        0,
		nodes:        make([]string, order),
		N:            order,
		neighborhood: make([]int, totalDirectedEdges),
		weights:      make([]float64, totalDirectedEdges),
		loops:        make([]float64, order),
		starts:       make([]int, order+1),
		belongings:   make([]int, order),
		mapping:      make([]int, order),
		counts:       make([]int, order),
		unused:       make([]int, order),
		totalWeights: make([]float64, order),
	}

	// Map nodeID → index
	nodeIndex := make(map[string]int, order)
	for i, nodeID := range lg.nodes {
		idx.nodes[i] = nodeID
		nodeIndex[nodeID] = i
		idx.belongings[i] = i
		idx.counts[i] = 1
	}

	// Build starts: count neighbors per node, then prefix sum
	// starts[i] will point to the beginning of i's neighbors in neighborhood[]
	// We use a decrement approach like the JS code
	n := 0
	for i, nodeID := range lg.nodes {
		n += len(lg.adj[nodeID])
		idx.starts[i] = n
	}
	idx.starts[order] = totalDirectedEdges

	// Fill neighborhood and weights by decrementing starts
	for i, nodeID := range lg.nodes {
		degree := len(lg.adj[nodeID])
		for _, nbrID := range lg.adj[nodeID] {
			j := nodeIndex[nbrID]
			weight := 1.0 // unweighted

			idx.M += weight
			idx.totalWeights[i] += weight

			pos := idx.starts[i] - 1
			idx.starts[i] = pos
			idx.neighborhood[pos] = j
			idx.weights[pos] = weight
		}
		// After decrement loop, starts[i] should be back to the correct position
		// We need to fix starts: they were decremented degree times
		// The JS code decrements starts[source] and starts[target] per edge
		// But since we're iterating per-node, we need a different approach
		_ = degree
	}

	// Actually, the JS code iterates edges, not nodes. Let's rebuild properly.
	// Reset and rebuild using edge iteration (matching JS forEachEdge pattern)
	idx.M = 0
	for i := range idx.totalWeights {
		idx.totalWeights[i] = 0
	}
	for i := range idx.starts {
		idx.starts[i] = 0
	}

	// First pass: compute starts (prefix sum of degrees)
	for i, nodeID := range lg.nodes {
		idx.starts[i+1] = idx.starts[i] + len(lg.adj[nodeID])
	}

	// Second pass: fill neighborhood/weights using a running offset per node
	offset := make([]int, order)
	copy(offset, idx.starts[:order])

	for i, nodeID := range lg.nodes {
		for _, nbrID := range lg.adj[nodeID] {
			j := nodeIndex[nbrID]
			weight := 1.0

			if i == j {
				// self-loop: JS does self.M += weight before the if(source===target) check
				idx.M += weight
				idx.totalWeights[i] += weight * 2
				idx.loops[i] = weight * 2
				continue
			}

			// Only count M once per undirected edge (i < j) to match JS forEachEdge
			if i < j {
				idx.M += weight
			}
			idx.totalWeights[i] += weight

			pos := offset[i]
			idx.neighborhood[pos] = j
			idx.weights[pos] = weight
			offset[i]++
		}
	}

	// Initialize mapping
	copy(idx.mapping, idx.belongings)

	return idx
}

// fastDelta computes a fast modularity delta (off by a constant factor from true delta).
// This matches graphology's UndirectedLouvainIndex.prototype.fastDelta.
func (idx *undirectedLouvainIndex) fastDelta(i int, degree float64, targetCommunityDegree float64, targetCommunity int) float64 {
	M := idx.M
	targetCommunityTotalWeight := idx.totalWeights[targetCommunity]
	degree += idx.loops[i]
	return targetCommunityDegree - (degree*targetCommunityTotalWeight*idx.resolution)/(2*M)
}

// fastDeltaWithOwnCommunity computes fast delta for the node's own community.
func (idx *undirectedLouvainIndex) fastDeltaWithOwnCommunity(i int, degree float64, targetCommunityDegree float64, targetCommunity int) float64 {
	M := idx.M
	targetCommunityTotalWeight := idx.totalWeights[targetCommunity]
	degree += idx.loops[i]
	return targetCommunityDegree - (degree*(targetCommunityTotalWeight-degree)*idx.resolution)/(2*M)
}

// move moves node i from its current community to targetCommunity.
func (idx *undirectedLouvainIndex) move(i int, degree float64, targetCommunity int) {
	currentCommunity := idx.belongings[i]
	loops := idx.loops[i]

	idx.totalWeights[currentCommunity] -= degree + loops
	idx.totalWeights[targetCommunity] += degree + loops

	idx.belongings[i] = targetCommunity

	nowEmpty := idx.counts[currentCommunity] == 1
	idx.counts[currentCommunity]--
	idx.counts[targetCommunity]++

	if nowEmpty {
		idx.unused[idx.U] = currentCommunity
		idx.U++
	}
}

// isolate creates a new singleton community for node i.
func (idx *undirectedLouvainIndex) isolate(i int, degree float64) int {
	currentCommunity := idx.belongings[i]

	// Already a singleton
	if idx.counts[currentCommunity] == 1 {
		return currentCommunity
	}

	idx.U--
	newCommunity := idx.unused[idx.U]

	loops := idx.loops[i]

	idx.totalWeights[currentCommunity] -= degree + loops
	idx.totalWeights[newCommunity] += degree + loops

	idx.belongings[i] = newCommunity

	idx.counts[currentCommunity]--
	idx.counts[newCommunity]++

	return newCommunity
}

// expensiveMove moves a node, computing its degree first.
func (idx *undirectedLouvainIndex) expensiveMove(i int, ci int) {
	degree := idx.computeNodeDegree(i)
	idx.move(i, degree, ci)
}

// computeNodeDegree computes the degree of node i.
func (idx *undirectedLouvainIndex) computeNodeDegree(i int) float64 {
	degree := 0.0
	for o := idx.starts[i]; o < idx.starts[i+1]; o++ {
		degree += idx.weights[o]
	}
	return degree
}

// zoomOut aggregates the current partition into a coarser graph for the next level.
// Returns a mapping from old community IDs to new community IDs.
func (idx *undirectedLouvainIndex) zoomOut() map[int]int {
	// Renumber communities contiguously
	inducedGraph := make([]struct {
		adj             map[int]float64
		totalWeights    float64
		internalWeights float64
	}, idx.C-idx.U)

	newLabels := make(map[int]int)
	C := 0

	for i := 0; i < idx.C; i++ {
		ci := idx.belongings[i]
		if _, ok := newLabels[ci]; !ok {
			newLabels[ci] = C
			inducedGraph[C].adj = make(map[int]float64)
			inducedGraph[C].totalWeights = idx.totalWeights[ci]
			inducedGraph[C].internalWeights = 0
			C++
		}
		idx.belongings[i] = newLabels[ci]
	}

	// Update mapping (equivalent to JS: mapping[i] = belongings[mapping[i]])
	for i := 0; i < idx.N; i++ {
		idx.mapping[i] = idx.belongings[idx.mapping[i]]
	}

	// Build induced graph
	for i := 0; i < idx.C; i++ {
		ci := idx.belongings[i]
		inducedGraph[ci].internalWeights += idx.loops[i]

		for j := idx.starts[i]; j < idx.starts[i+1]; j++ {
			n := idx.neighborhood[j]
			cj := idx.belongings[n]

			if ci == cj {
				inducedGraph[ci].internalWeights += idx.weights[j]
				continue
			}

			inducedGraph[ci].adj[cj] += idx.weights[j]
		}
	}

	// Rewrite neighborhood
	idx.C = C
	n := 0
	E := 0

	for ci := 0; ci < C; ci++ {
		idx.totalWeights[ci] = inducedGraph[ci].totalWeights
		idx.loops[ci] = inducedGraph[ci].internalWeights
		idx.counts[ci] = 1
		idx.starts[ci] = n
		idx.belongings[ci] = ci

		for cj, w := range inducedGraph[ci].adj {
			idx.neighborhood[n] = cj
			idx.weights[n] = w
			E++
			n++
		}
	}

	idx.starts[C] = E
	idx.E = E
	idx.U = 0
	idx.level++

	return newLabels
}

// modularity computes the modularity Q of the current partition.
func (idx *undirectedLouvainIndex) modularity() float64 {
	Q := 0.0
	M2 := idx.M * 2
	internalWeights := make([]float64, idx.C)

	for i := 0; i < idx.C; i++ {
		ci := idx.belongings[i]
		internalWeights[ci] += idx.loops[i]

		for j := idx.starts[i]; j < idx.starts[i+1]; j++ {
			cj := idx.belongings[idx.neighborhood[j]]
			if ci != cj {
				continue
			}
			internalWeights[ci] += idx.weights[j]
		}
	}

	for i := 0; i < idx.C; i++ {
		if idx.counts[i] == 0 {
			continue
		}
		Q += internalWeights[i]/M2 - math.Pow(idx.totalWeights[i]/M2, 2)*idx.resolution
	}

	return Q
}

// collect returns the node→community mapping for all original nodes.
func (idx *undirectedLouvainIndex) collect() map[string]int {
	result := make(map[string]int, idx.N)
	for i := 0; i < idx.N; i++ {
		result[idx.nodes[i]] = idx.mapping[i]
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// UndirectedLeidenAddenda — port of leiden/utils.cjs
// ─────────────────────────────────────────────────────────────────────────────

// undirectedLeidenAddenda implements the refinement phase of the Leiden algorithm.
type undirectedLeidenAddenda struct {
	index      *undirectedLouvainIndex
	randomness float64
	rng        *rand.Rand
	resolution float64

	// Used to group nodes by communities
	B int // number of non-empty communities
	C int // snapshot of index.C at groupByCommunities time

	communitiesOffsets       []int
	nodesSortedByCommunities []int
	communitiesBounds        []int

	// Used to merge nodes subsets
	communityWeights          []float64
	degrees                   []float64
	nonSingleton              []uint8
	externalEdgeWeightPerComm []float64
	belongings                []int
	neighboringCommunities    *sparseMap
	cumulativeIncrement       []float64
	macroCommunities          [][]int
}

func newUndirectedLeidenAddenda(idx *undirectedLouvainIndex, randomness float64, rng *rand.Rand) *undirectedLeidenAddenda {
	order := idx.C
	return &undirectedLeidenAddenda{
		index:                     idx,
		randomness:                randomness,
		rng:                       rng,
		resolution:                idx.resolution,
		B:                         order,
		C:                         0,
		communitiesOffsets:        make([]int, order),
		nodesSortedByCommunities:  make([]int, order),
		communitiesBounds:         make([]int, order+1),
		communityWeights:          make([]float64, order),
		degrees:                   make([]float64, order),
		nonSingleton:              make([]uint8, order),
		externalEdgeWeightPerComm: make([]float64, order),
		belongings:                make([]int, order),
		neighboringCommunities:    newSparseMap(order),
		cumulativeIncrement:       make([]float64, order),
	}
}

// groupByCommunities sorts nodes by their community assignment.
func (a *undirectedLeidenAddenda) groupByCommunities() {
	idx := a.index

	n := 0
	o := 0

	for i := 0; i < idx.C; i++ {
		c := idx.counts[i]
		if c != 0 {
			a.communitiesBounds[o] = n
			o++
			n += c
			a.communitiesOffsets[i] = n
		}
	}
	a.communitiesBounds[o] = n

	o = 0
	for i := 0; i < idx.C; i++ {
		b := idx.belongings[i]
		o = a.communitiesOffsets[b] - 1
		a.communitiesOffsets[b] = o
		a.nodesSortedByCommunities[o] = i
	}

	a.B = idx.C - idx.U
	a.C = idx.C
}

// mergeNodesSubset implements the Leiden refinement for a single community.
// It ensures the community is well-connected by potentially splitting it.
func (a *undirectedLeidenAddenda) mergeNodesSubset(start, stop int) []int {
	idx := a.index
	currentMacroCommunity := idx.belongings[a.nodesSortedByCommunities[start]]
	neighboringComms := a.neighboringCommunities

	var totalNodeWeight float64

	// Initializing singletons
	for j := start; j < stop; j++ {
		i := a.nodesSortedByCommunities[j]
		a.belongings[i] = i
		a.nonSingleton[i] = 0
		a.degrees[i] = 0
		totalNodeWeight += idx.loops[i] / 2

		a.communityWeights[i] = idx.loops[i]
		a.externalEdgeWeightPerComm[i] = 0

		ei := idx.starts[i]
		el := idx.starts[i+1]

		for ; ei < el; ei++ {
			et := idx.neighborhood[ei]
			w := idx.weights[ei]

			a.degrees[i] += w

			if idx.belongings[et] != currentMacroCommunity {
				continue
			}

			totalNodeWeight += w
			a.externalEdgeWeightPerComm[i] += w
			a.communityWeights[i] += w
		}
	}

	// Copy micro degrees
	microDegrees := make([]float64, len(a.externalEdgeWeightPerComm))
	copy(microDegrees, a.externalEdgeWeightPerComm)

	order := stop - start

	if order <= 0 {
		return nil
	}

	ri := start + a.rng.Intn(order)

	for s := start; s < stop; s++ {
		j := start + (ri % order)
		ri++

		i := a.nodesSortedByCommunities[j]

		if a.nonSingleton[i] == 1 {
			continue
		}

		// Well-connected check
		if a.externalEdgeWeightPerComm[i] <
			a.communityWeights[i]*(totalNodeWeight/2-a.communityWeights[i])*a.resolution {
			continue
		}

		a.communityWeights[i] = 0
		a.externalEdgeWeightPerComm[i] = 0

		neighboringComms.clear()
		neighboringComms.set(i, 0)

		degree := 0.0

		ei := idx.starts[i]
		el := idx.starts[i+1]

		for ; ei < el; ei++ {
			et := idx.neighborhood[ei]

			if idx.belongings[et] != currentMacroCommunity {
				continue
			}

			w := idx.weights[ei]
			degree += w
			a.addWeightToCommunity(neighboringComms, a.belongings[et], w)
		}

		bestCommunity := i
		maxQualityValueIncrement := 0.0
		totalTransformedQualityValueIncrement := 0.0

		for ci := 0; ci < neighboringComms.size; ci++ {
			targetCommunity := neighboringComms.dense[ci]
			targetCommunityDegree := neighboringComms.vals[ci]
			targetCommunityWeight := a.communityWeights[targetCommunity]

			if a.externalEdgeWeightPerComm[targetCommunity] >=
				targetCommunityWeight*(totalNodeWeight/2-targetCommunityWeight)*a.resolution {

				qualityValueIncrement :=
					targetCommunityDegree -
						((degree+idx.loops[i])*targetCommunityWeight*a.resolution)/totalNodeWeight

				if qualityValueIncrement > maxQualityValueIncrement {
					bestCommunity = targetCommunity
					maxQualityValueIncrement = qualityValueIncrement
				}

				if qualityValueIncrement >= 0 {
					totalTransformedQualityValueIncrement += math.Exp(qualityValueIncrement / a.randomness)
				}
			}

			a.cumulativeIncrement[ci] = totalTransformedQualityValueIncrement
		}

		var chosenCommunity int
		if totalTransformedQualityValueIncrement < math.MaxFloat64 && !math.IsInf(totalTransformedQualityValueIncrement, 1) {
			r := totalTransformedQualityValueIncrement * a.rng.Float64()
			lo := -1
			hi := neighboringComms.size + 1

			for lo < hi-1 {
				mid := (lo + hi) >> 1
				if a.cumulativeIncrement[mid] >= r {
					hi = mid
				} else {
					lo = mid
				}
			}

			chosenCommunity = neighboringComms.dense[hi]
		} else {
			chosenCommunity = bestCommunity
		}

		a.communityWeights[chosenCommunity] += degree + idx.loops[i]

		ei = idx.starts[i]
		el = idx.starts[i+1]

		for ; ei < el; ei++ {
			et := idx.neighborhood[ei]

			if idx.belongings[et] != currentMacroCommunity {
				continue
			}

			targetCommunity := a.belongings[et]

			if targetCommunity == chosenCommunity {
				a.externalEdgeWeightPerComm[chosenCommunity] -= microDegrees[et]
			} else {
				a.externalEdgeWeightPerComm[chosenCommunity] += microDegrees[et]
			}
		}

		if chosenCommunity != i {
			a.belongings[i] = chosenCommunity
			a.nonSingleton[chosenCommunity] = 1
			a.C--
		}
	}

	// Collect resulting micro-communities
	neighboringComms.clear()
	for j := start; j < stop; j++ {
		i := a.nodesSortedByCommunities[j]
		neighboringComms.set(a.belongings[i], 1)
	}

	result := make([]int, neighboringComms.size)
	for ci := 0; ci < neighboringComms.size; ci++ {
		result[ci] = neighboringComms.dense[ci]
	}
	return result
}

func (a *undirectedLeidenAddenda) addWeightToCommunity(sm *sparseMap, community int, weight float64) {
	currentWeight, _ := sm.get(community)
	sm.set(community, currentWeight+weight)
}

// refinePartition runs the refinement phase on all communities.
func (a *undirectedLeidenAddenda) refinePartition() {
	a.groupByCommunities()

	a.macroCommunities = make([][]int, a.B)

	for i := 0; i < a.B; i++ {
		start := a.communitiesBounds[i]
		stop := a.communitiesBounds[i+1]
		a.macroCommunities[i] = a.mergeNodesSubset(start, stop)
	}
}

// split isolates all community leaders and moves followers to their refined communities.
func (a *undirectedLeidenAddenda) split() {
	idx := a.index
	isolates := a.neighboringCommunities
	isolates.clear()

	for i := 0; i < idx.C; i++ {
		community := a.belongings[i]
		if i != community {
			continue
		}
		isolated := idx.isolate(i, a.degrees[i])
		isolates.set(community, float64(isolated))
	}

	for i := 0; i < idx.C; i++ {
		community := a.belongings[i]
		if i == community {
			continue
		}
		isolated, _ := isolates.get(community)
		idx.move(i, a.degrees[i], int(isolated))
	}

	for i := 0; i < len(a.macroCommunities); i++ {
		macro := a.macroCommunities[i]
		for j := 0; j < len(macro); j++ {
			if macro[j] < 0 || macro[j] >= isolates.capacity {
				continue
			}
			isolated, _ := isolates.get(macro[j])
			macro[j] = int(isolated)
		}
	}
}

// zoomOut refines the partition and aggregates to the next level.
func (a *undirectedLeidenAddenda) zoomOut() {
	idx := a.index
	a.refinePartition()
	a.split()

	newLabels := idx.zoomOut()

	for i := 0; i < len(a.macroCommunities); i++ {
		macro := a.macroCommunities[i]
		if len(macro) == 0 {
			continue
		}
		leader := newLabels[macro[0]]

		for j := 1; j < len(macro); j++ {
			follower := newLabels[macro[j]]
			idx.expensiveMove(follower, leader)
		}
	}
}

// onlySingletons checks if all communities have exactly 1 member.
func (a *undirectedLeidenAddenda) onlySingletons() bool {
	idx := a.index
	for i := 0; i < idx.C; i++ {
		if idx.counts[i] > 1 {
			return false
		}
	}
	return true
}

// ─────────────────────────────────────────────────────────────────────────────
// undirectedLeiden — main algorithm loop
// ─────────────────────────────────────────────────────────────────────────────

const epsilon = 1e-10

// tieBreaker decides whether a new delta is better than the current best.
// Matches graphology's tieBreaker function exactly.
func tieBreaker(bestCommunity, currentCommunity, targetCommunity int, delta, bestDelta float64) bool {
	if math.Abs(delta-bestDelta) < epsilon {
		if bestCommunity == currentCommunity {
			return false
		}
		return targetCommunity > bestCommunity
	}
	return delta > bestDelta
}

// undirectedLeidenMain is the core Leiden algorithm, ported from graphology.
func undirectedLeidenMain(idx *undirectedLouvainIndex, addenda *undirectedLeidenAddenda, rng *rand.Rand, randomWalk bool) {
	communities := newSparseMap(idx.C)
	queue := newSparseQueueSet(idx.C)

	for {
		l := idx.C

		// Clear queue for this iteration
		queue.clear()

		// Traversal: random walk or sequential
		ri := 0
		if randomWalk && l > 0 {
			ri = rng.Intn(l)
		}

		for s := 0; s < l; s++ {
			i := ri % l
			ri++
			queue.enqueue(i)
		}

		currentMoves := 0

		for queue.size > 0 {
			i, _ := queue.dequeue()

			degree := 0.0
			communities.clear()

			currentCommunity := idx.belongings[i]

			start := idx.starts[i]
			end := idx.starts[i+1]

			// Traverse neighbors
			for ; start < end; start++ {
				j := idx.neighborhood[start]
				weight := idx.weights[start]
				targetCommunity := idx.belongings[j]

				degree += weight
				communities.add(targetCommunity, weight)
			}

			// Compute delta for current community
			ownDegree, _ := communities.get(currentCommunity)
			bestDelta := idx.fastDeltaWithOwnCommunity(i, degree, ownDegree, currentCommunity)
			bestCommunity := currentCommunity

			// Find best community to move to
			for ci := 0; ci < communities.size; ci++ {
				targetCommunity := communities.dense[ci]

				if targetCommunity == currentCommunity {
					continue
				}

				targetCommunityDegree := communities.vals[ci]

				delta := idx.fastDelta(i, degree, targetCommunityDegree, targetCommunity)

				if tieBreaker(bestCommunity, currentCommunity, targetCommunity, delta, bestDelta) {
					bestDelta = delta
					bestCommunity = targetCommunity
				}
			}

			if bestDelta < 0 {
				bestCommunity = idx.isolate(i, degree)
				if bestCommunity == currentCommunity {
					continue
				}
			} else {
				if bestCommunity == currentCommunity {
					continue
				}
				idx.move(i, degree, bestCommunity)
			}

			currentMoves++

			// Add neighbors from other communities to the queue
			start = idx.starts[i]
			end = idx.starts[i+1]

			for ; start < end; start++ {
				j := idx.neighborhood[start]
				targetCommunity := idx.belongings[j]
				if targetCommunity != bestCommunity {
					queue.enqueue(j)
				}
			}
		}

		if currentMoves == 0 {
			idx.zoomOut()
			break
		}

		if !addenda.onlySingletons() {
			addenda.zoomOut()
			continue
		}

		break
	}
}
