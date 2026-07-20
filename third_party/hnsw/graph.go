package hnsw

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"

	"github.com/coder/hnsw/heap"
	"golang.org/x/exp/maps"
)

type Vector = []float32

// Node is a node in the graph.
type Node[K cmp.Ordered] struct {
	Key   K
	Value Vector
}

func MakeNode[K cmp.Ordered](key K, vec Vector) Node[K] {
	return Node[K]{Key: key, Value: vec}
}

// layerNode is a node in a layer of the graph.
type layerNode[K cmp.Ordered] struct {
	Node[K]

	// QVec holds the int8 quantized vector. When non-nil, distance computation
	// uses QVec with QuantParams instead of Value. This reduces per-node memory
	// from dim*4B (float32) to dim*1B (int8), a 4x compression for 384-dim vectors.
	QVec []byte

	// neighbors is map of neighbor keys to neighbor nodes.
	// It is a map and not a slice to allow for efficient deletes, esp.
	// when M is high.
	neighbors map[K]*layerNode[K]
}

// nodeDistFunc computes the distance between two layerNodes.
// It dispatches to the appropriate distance function based on quantization mode.
type nodeDistFunc[K cmp.Ordered] func(a, b *layerNode[K]) float32

// makeDistFunc creates a nodeDistFunc from the graph's configuration.
func makeDistFunc[K cmp.Ordered](g *Graph[K]) nodeDistFunc[K] {
	if g.QuantType == QuantInt8 {
		qdist := g.QDist
		if qdist == nil {
			qdist = Int8CosineDistance
		}
		params := g.QuantParams
		return func(a, b *layerNode[K]) float32 {
			return qdist(a.QVec, b.QVec, params)
		}
	}
	dist := g.Distance
	return func(a, b *layerNode[K]) float32 {
		return dist(a.Value, b.Value)
	}
}

// addNeighbor adds a neighbor to the node, replacing the neighbor
// with the worst distance if the neighbor set is full.
func (n *layerNode[K]) addNeighbor(newNode *layerNode[K], m int, dist nodeDistFunc[K]) {
	if n.neighbors == nil {
		n.neighbors = make(map[K]*layerNode[K], m)
	}

	n.neighbors[newNode.Key] = newNode
	if len(n.neighbors) <= m {
		return
	}

	// Find the neighbor with the worst distance.
	var (
		worstDist = float32(math.Inf(-1))
		worst     *layerNode[K]
	)
	for _, neighbor := range n.neighbors {
		d := dist(neighbor, n)
		// d > worstDist may always be false if the distance function
		// returns NaN, e.g., when the embeddings are zero.
		if d > worstDist || worst == nil {
			worstDist = d
			worst = neighbor
		}
	}

	delete(n.neighbors, worst.Key)
	// Delete backlink from the worst neighbor.
	delete(worst.neighbors, n.Key)
	worst.replenish(m, dist)
}

type searchCandidate[K cmp.Ordered] struct {
	node *layerNode[K]
	dist float32
}

func (s searchCandidate[K]) Less(o searchCandidate[K]) bool {
	return s.dist < o.dist
}

// search returns the layer node closest to the target node
// within the same layer.
func (n *layerNode[K]) search(
	// k is the number of candidates in the result set.
	k int,
	efSearch int,
	targetQVec []byte,
	targetVec Vector,
	distFn nodeDistFunc[K],
) []searchCandidate[K] {
	// This is a basic greedy algorithm to find the entry point at the given level
	// that is closest to the target node.
	candidates := heap.Heap[searchCandidate[K]]{}
	candidates.Init(make([]searchCandidate[K], 0, efSearch))

	// Compute initial distance: use QVec if available, else float32
	var entryDist float32
	if len(n.QVec) > 0 && len(targetQVec) > 0 {
		entryDist = distFn(n, &layerNode[K]{QVec: targetQVec})
	} else {
		entryDist = distFn(n, &layerNode[K]{Node: Node[K]{Value: targetVec}})
	}

	candidates.Push(
		searchCandidate[K]{
			node: n,
			dist: entryDist,
		},
	)
	var (
		result  = heap.Heap[searchCandidate[K]]{}
		visited = make(map[K]bool)
	)
	result.Init(make([]searchCandidate[K], 0, k))

	// Begin with the entry node in the result set.
	result.Push(candidates.Min())
	visited[n.Key] = true

	for candidates.Len() > 0 {
		var (
			current  = candidates.Pop().node
			improved = false
		)

		// We iterate the map in a sorted, deterministic fashion for
		// tests.
		neighborKeys := maps.Keys(current.neighbors)
		slices.Sort(neighborKeys)
		for _, neighborID := range neighborKeys {
			neighbor := current.neighbors[neighborID]
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true

			var dist float32
			if len(neighbor.QVec) > 0 && len(targetQVec) > 0 {
				dist = distFn(neighbor, &layerNode[K]{QVec: targetQVec})
			} else {
				dist = distFn(neighbor, &layerNode[K]{Node: Node[K]{Value: targetVec}})
			}

			improved = improved || dist < result.Min().dist
			if result.Len() < k {
				result.Push(searchCandidate[K]{node: neighbor, dist: dist})
			} else if dist < result.Max().dist {
				result.PopLast()
				result.Push(searchCandidate[K]{node: neighbor, dist: dist})
			}

			candidates.Push(searchCandidate[K]{node: neighbor, dist: dist})
			// Always store candidates if we haven't reached the limit.
			if candidates.Len() > efSearch {
				candidates.PopLast()
			}
		}

		// Termination condition: no improvement in distance and at least
		// kMin candidates in the result set.
		if !improved && result.Len() >= k {
			break
		}
	}

	return result.Slice()
}

func (n *layerNode[K]) replenish(m int, distFn nodeDistFunc[K]) {
	if len(n.neighbors) >= m {
		return
	}

	// Restore connectivity by adding new neighbors.
	// This is a naive implementation that could be improved by
	// using a priority queue to find the best candidates.
	for _, neighbor := range n.neighbors {
		for key, candidate := range neighbor.neighbors {
			if _, ok := n.neighbors[key]; ok {
				// do not add duplicates
				continue
			}
			if candidate == n {
				continue
			}
			n.addNeighbor(candidate, m, distFn)
			if len(n.neighbors) >= m {
				return
			}
		}
	}
}

// isolates remove the node from the graph by removing all connections
// to neighbors.
func (n *layerNode[K]) isolate(m int, distFn nodeDistFunc[K]) {
	for _, neighbor := range n.neighbors {
		delete(neighbor.neighbors, n.Key)
	}

	for _, neighbor := range n.neighbors {
		neighbor.replenish(m, distFn)
	}
}

type layer[K cmp.Ordered] struct {
	// nodes is a map of nodes IDs to nodes.
	// All nodes in a higher layer are also in the lower layers, an essential
	// property of the graph.
	//
	// nodes is exported for interop with encoding/gob.
	nodes map[K]*layerNode[K]
}

// entry returns the entry node of the layer.
// It doesn't matter which node is returned, even that the
// entry node is consistent, so we just return the first node
// in the map to avoid tracking extra state.
func (l *layer[K]) entry() *layerNode[K] {
	if l == nil {
		return nil
	}
	for _, node := range l.nodes {
		return node
	}
	return nil
}

func (l *layer[K]) size() int {
	if l == nil {
		return 0
	}
	return len(l.nodes)
}

// Graph is a Hierarchical Navigable Small World graph.
// All public parameters must be set before adding nodes to the graph.
// K is cmp.Ordered instead of of comparable so that they can be sorted.
type Graph[K cmp.Ordered] struct {
	// Distance is the distance function used to compare embeddings.
	Distance DistanceFunc

	// Rng is used for level generation. It may be set to a deterministic value
	// for reproducibility. Note that deterministic number generation can lead to
	// degenerate graphs when exposed to adversarial inputs.
	Rng *rand.Rand

	// M is the maximum number of neighbors to keep for each node.
	// A good default for OpenAI embeddings is 16.
	M int

	// Ml is the level generation factor.
	// E.g., for Ml = 0.25, each layer is 1/4 the size of the previous layer.
	Ml float64

	// EfSearch is the number of nodes to consider in the search phase.
	// 20 is a reasonable default. Higher values improve search accuracy at
	// the expense of memory.
	EfSearch int

	// QuantType controls vector quantization. When set to QuantInt8,
	// each node stores QVec (int8 quantized bytes) instead of float32 Value,
	// reducing memory usage by ~4x. Distance computation uses Int8CosineDistance.
	QuantType QuantType

	// QuantParams holds the scale and offset for scalar quantization.
	// Must be set before adding nodes when QuantType is QuantInt8.
	// Use TrainQuantParams to compute from training vectors.
	QuantParams QuantParams

	// QDist is the quantized distance function used when QuantType is QuantInt8.
	// Defaults to Int8CosineDistance if not set.
	QDist QuantizedDistFunc

	// layers is a slice of layers in the graph.
	layers []*layer[K]
}

func defaultRand() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

// NewGraph returns a new graph with default parameters, roughly designed for
// storing OpenAI embeddings.
func NewGraph[K cmp.Ordered]() *Graph[K] {
	return &Graph[K]{
		M:        16,
		Ml:       0.25,
		Distance: CosineDistance,
		EfSearch: 20,
		Rng:      defaultRand(),
	}
}

// maxLevel returns an upper-bound on the number of levels in the graph
// based on the size of the base layer.
func maxLevel(ml float64, numNodes int) (int, error) {
	if ml == 0 {
		return 0, fmt.Errorf("ml must be greater than 0")
	}

	if numNodes == 0 {
		return 1, nil
	}

	l := math.Log(float64(numNodes))
	l /= math.Log(1 / ml)

	m := int(math.Round(l)) + 1

	return m, nil
}

// randomLevel generates a random level for a new node.
func (h *Graph[K]) randomLevel() (int, error) {
	// max avoids having to accept an additional parameter for the maximum level
	// by calculating a probably good one from the size of the base layer.
	max := 1
	if len(h.layers) > 0 {
		if h.Ml == 0 {
			return 0, fmt.Errorf("(*Graph).Ml must be greater than 0")
		}
		var err error
		max, err = maxLevel(h.Ml, h.layers[0].size())
		if err != nil {
			return 0, err
		}
	}

	for level := 0; level < max; level++ {
		if h.Rng == nil {
			h.Rng = defaultRand()
		}
		r := h.Rng.Float64()
		if r > h.Ml {
			return level, nil
		}
	}

	return max, nil
}

func (g *Graph[K]) assertDims(n Vector) error {
	if len(g.layers) == 0 {
		return nil
	}
	hasDims := g.Dims()
	if hasDims != len(n) {
		return fmt.Errorf("embedding dimension mismatch: %d != %d", hasDims, len(n))
	}
	return nil
}

// Dims returns the number of dimensions in the graph, or
// 0 if the graph is empty.
func (g *Graph[K]) Dims() int {
	if len(g.layers) == 0 {
		return 0
	}
	entry := g.layers[0].entry()
	if len(entry.QVec) > 0 {
		return len(entry.QVec)
	}
	return len(entry.Value)
}

func ptr[T any](v T) *T {
	return &v
}

// Add inserts nodes into the graph.
// If another node with the same ID exists, it is replaced.
// Returns an error if graph configuration is invalid (e.g., Ml=0, Distance=nil)
// or if embedding dimensions mismatch.
func (g *Graph[K]) Add(nodes ...Node[K]) error {
	distFn := makeDistFunc(g)
	for _, node := range nodes {
		key := node.Key
		vec := node.Value

		if err := g.assertDims(vec); err != nil {
			return fmt.Errorf("node %v: %w", key, err)
		}

		// Quantize the vector if QuantInt8 mode is active
		var qvec []byte
		if g.QuantType == QuantInt8 {
			qvec = Quantize(vec, g.QuantParams)
		}

		insertLevel, err := g.randomLevel()
		if err != nil {
			return fmt.Errorf("node %v: %w", key, err)
		}
		// Create layers that don't exist yet.
		for insertLevel >= len(g.layers) {
			g.layers = append(g.layers, &layer[K]{})
		}

		if insertLevel < 0 {
			return fmt.Errorf("node %v: invalid level %d", key, insertLevel)
		}

		var elevator *K

		preLen := g.Len()

		// Insert node at each layer, beginning with the highest.
		for i := len(g.layers) - 1; i >= 0; i-- {
			layer := g.layers[i]
			newNode := &layerNode[K]{
				Node: Node[K]{
					Key:   key,
					Value: vec,
				},
				QVec: qvec,
			}

			// Insert the new node into the layer.
			if layer.entry() == nil {
				layer.nodes = map[K]*layerNode[K]{key: newNode}
				continue
			}

			// Now at the highest layer with more than one node, so we can begin
			// searching for the best way to enter the graph.
			searchPoint := layer.entry()

			// On subsequent layers, we use the elevator node to enter the graph
			// at the best point.
			if elevator != nil {
				searchPoint = layer.nodes[*elevator]
			}

			if g.Distance == nil && g.QuantType == QuantNone {
				return fmt.Errorf("(*Graph).Distance must be set")
			}
			if g.QuantType == QuantInt8 && len(g.QuantParams.Scale) == 0 {
				return fmt.Errorf("(*Graph).QuantParams must be set for QuantInt8 mode")
			}

			neighborhood := searchPoint.search(g.M, g.EfSearch, qvec, vec, distFn)
			if len(neighborhood) == 0 {
				// This should never happen because the searchPoint itself
				// should be in the result set.
				return fmt.Errorf("node %v: no nodes found during search", key)
			}

			// Re-set the elevator node for the next layer.
			elevator = ptr(neighborhood[0].node.Key)

			if insertLevel >= i {
				if _, ok := layer.nodes[key]; ok {
					g.Delete(key)
				}
				// Insert the new node into the layer.
				layer.nodes[key] = newNode
				for _, node := range neighborhood {
					// Create a bi-directional edge between the new node and the best node.
					node.node.addNeighbor(newNode, g.M, distFn)
					newNode.addNeighbor(node.node, g.M, distFn)
				}
			}
		}

		// Invariant check: the node should have been added to the graph.
		if g.Len() != preLen+1 {
			if len(g.layers) > 0 && g.layers[len(g.layers)-1].entry() == nil {
				g.layers = g.layers[:len(g.layers)-1]
			}
		}
	}
	return nil
}

// Search finds the k nearest neighbors from the target node.
// Returns an error if the embedding dimensions mismatch or the graph
// is misconfigured.
func (h *Graph[K]) Search(near Vector, k int) ([]Node[K], error) {
	sr, err := h.search(near, k)
	if err != nil {
		return nil, err
	}
	out := make([]Node[K], len(sr))
	for i, node := range sr {
		out[i] = node.Node
	}
	return out, nil
}

// SearchWithDistance finds the k nearest neighbors from the target node
// and returns the distance.
func (h *Graph[K]) SearchWithDistance(near Vector, k int) ([]SearchResult[K], error) {
	return h.search(near, k)
}

type SearchResult[T cmp.Ordered] struct {
	Node[T]
	Distance float32
}

func (h *Graph[K]) search(near Vector, k int) ([]SearchResult[K], error) {
	if err := h.assertDims(near); err != nil {
		return nil, err
	}
	if len(h.layers) == 0 {
		return nil, nil
	}

	distFn := makeDistFunc(h)

	// Quantize query vector if in quantized mode
	var nearQVec []byte
	if h.QuantType == QuantInt8 {
		nearQVec = Quantize(near, h.QuantParams)
	}

	var (
		efSearch = h.EfSearch

		elevator *K
	)

	for layer := len(h.layers) - 1; layer >= 0; layer-- {
		searchPoint := h.layers[layer].entry()
		if elevator != nil {
			searchPoint = h.layers[layer].nodes[*elevator]
		}

		// Descending hierarchies
		if layer > 0 {
			nodes := searchPoint.search(1, efSearch, nearQVec, near, distFn)
			if len(nodes) == 0 {
				return nil, fmt.Errorf("search: no entry found at layer %d", layer)
			}
			elevator = ptr(nodes[0].node.Key)
			continue
		}

		nodes := searchPoint.search(k, efSearch, nearQVec, near, distFn)
		out := make([]SearchResult[K], 0, len(nodes))

		for _, node := range nodes {
			out = append(out, SearchResult[K]{
				Node:     node.node.Node,
				Distance: node.dist,
			})
		}

		return out, nil
	}

	// Unreachable: the loop above always has at least one layer and returns
	// from the layer==0 case.
	return nil, fmt.Errorf("search: unexpected empty layer stack")
}

// Len returns the number of nodes in the graph.
func (h *Graph[K]) Len() int {
	if len(h.layers) == 0 {
		return 0
	}
	return h.layers[0].size()
}

// Delete removes a node from the graph by key.
// It tries to preserve the clustering properties of the graph by
// replenishing connectivity in the affected neighborhoods.
func (h *Graph[K]) Delete(key K) bool {
	if len(h.layers) == 0 {
		return false
	}

	distFn := makeDistFunc(h)

	deleteLayer := map[int]struct{}{}
	var deleted bool
	for i, layer := range h.layers {
		node, ok := layer.nodes[key]
		if !ok {
			continue
		}
		delete(layer.nodes, key)
		if len(layer.nodes) == 0 {
			deleteLayer[i] = struct{}{}
		}
		node.isolate(h.M, distFn)
		deleted = true
	}

	if len(deleteLayer) > 0 {
		newLayers := make([]*layer[K], 0, len(h.layers)-len(deleteLayer))
		for i, layer := range h.layers {
			if _, ok := deleteLayer[i]; ok {
				continue
			}
			newLayers = append(newLayers, layer)
		}

		h.layers = newLayers
	}

	return deleted
}

// Lookup returns the vector with the given key.
func (h *Graph[K]) Lookup(key K) (Vector, bool) {
	if len(h.layers) == 0 {
		return nil, false
	}

	node, ok := h.layers[0].nodes[key]
	if !ok {
		return nil, false
	}
	return node.Value, ok
}
