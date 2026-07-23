package graph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/util"
)

const (
	defaultNodeCacheSize  = 500000 // 500K entries (~200MB)
	defaultTraversalLimit = 100000 // 100K nodes max per traversal
)

var (
	// ErrTraversalLimitExceeded indicates the traversal exceeded the maximum node visit limit
	ErrTraversalLimitExceeded = errors.New("traversal exceeded maximum node visit limit")
)

// sync.Pool instances for GC-optimized slice reuse during traversals and batch reads.
// These pools reduce allocation pressure when processing large graphs (1M+ nodes).

// adjEntrySlicePool reuses []*Edge slices returned by GetAllOutEdges/GetAllInEdges.
var adjEntrySlicePool = sync.Pool{
	New: func() any { value := make([]*Edge, 0, 32); return &value },
}

// nodeSlicePool reuses []*Node slices for BFS/ShortestPath results.
var nodeSlicePool = sync.Pool{
	New: func() any { value := make([]*Node, 0, 64); return &value },
}

// AdjEntry represents an adjacency index entry (Scheme A: properties embedded in adjacency index value)
// A single ScanPrefix retrieves complete edge data, no additional KV lookups needed
type AdjEntry struct {
	ID     string    `json:"id,omitempty" msgpack:"id,omitempty"`
	Target string    `json:"target" msgpack:"target"`
	Props  EdgeProps `json:"props,omitempty" msgpack:"props,omitempty"`
}

// GraphStore is a Pebble-backed graph store
// High-performance design:
// - Pre-built adjacency index, O(1) neighbor lookup
// - Batch writes reduce IO
// - Pebble is inherently thread-safe, no additional locks needed
// - Node cache (LRU) avoids repeated Get+Unmarshal for hot-path traversals (BFS, ShortestPath)
// - Traversal limit prevents runaway queries on large graphs
// - WriteBuffer + FlushBuffer for Pipeline batch commit pattern
type GraphStore struct {
	store          *store.Store
	repo           string
	nodeCache      *shardedLRU[string, *Node] // Sharded LRU cache for GetNode hot path (64 shards, reduces lock contention)
	traversalLimit int                        // max nodes visited during traversal (0 = unlimited)
	buf            WriteBuffer                // buffered writes for Pipeline batch commit
	bufMu          sync.Mutex                 // protects buf for concurrent Pipeline workers
}

// WriteBuffer accumulates nodes and edges for batched Pebble commits.
// Pipeline phases use BufferNode/BufferEdge to accumulate items, then
// FlushBuffer to commit them in a single Pebble Batch transaction.
// This reduces ~6M individual BatchNoSync calls to ~100 batched commits.
type WriteBuffer struct {
	nodes    []*Node
	edges    []*Edge
	count    int // total items buffered (nodes + edges)
	capacity int // flush threshold
}

// DefaultFlushInterval is the number of buffered items that triggers a flush.
const DefaultFlushInterval = 1000

// NewGraphStore creates a graph store instance
func NewGraphStore(store *store.Store, repo string) *GraphStore {
	return &GraphStore{
		store:          store,
		repo:           repo,
		nodeCache:      newShardedLRU[string, *Node](defaultNodeCacheSize),
		traversalLimit: defaultTraversalLimit,
		buf: WriteBuffer{
			nodes:    make([]*Node, 0, DefaultFlushInterval),
			edges:    make([]*Edge, 0, DefaultFlushInterval),
			count:    0,
			capacity: DefaultFlushInterval,
		},
	}
}

// SetTraversalLimit sets the maximum number of nodes visited during traversal.
// 0 means unlimited. Default is 100000.
func (s *GraphStore) SetTraversalLimit(limit int) {
	s.traversalLimit = limit
}

// SetNodeCacheSize resizes the sharded LRU node cache.
// Existing entries are preserved up to the new capacity.
func (s *GraphStore) SetNodeCacheSize(size int) {
	if size <= 0 {
		return
	}
	oldCache := s.nodeCache
	newCache := newShardedLRU[string, *Node](size)
	// Copy existing entries
	for _, key := range oldCache.Keys() {
		if val, ok := oldCache.Get(key); ok {
			newCache.Add(key, val)
		}
	}
	s.nodeCache = newCache
}

// SetFlushInterval sets the write buffer flush threshold.
// When the buffer accumulates this many items, the next BufferNode/BufferEdge
// call will automatically flush. Default is 1000.
func (s *GraphStore) SetFlushInterval(interval int) {
	if interval <= 0 {
		return
	}
	s.bufMu.Lock()
	s.buf.capacity = interval
	s.bufMu.Unlock()
}

// BufferNode adds a node to the write buffer. If the buffer reaches capacity,
// it is automatically flushed to Pebble. Thread-safe.
func (s *GraphStore) BufferNode(node *Node) error {
	if node.ID == "" {
		node.ID = util.GenerateID(node.Repo, string(node.Label), node.Name)
	}
	if node.Repo == "" {
		node.Repo = s.repo
	}

	s.bufMu.Lock()
	s.buf.nodes = append(s.buf.nodes, node)
	s.buf.count++
	shouldFlush := s.buf.count >= s.buf.capacity
	s.bufMu.Unlock()

	if shouldFlush {
		return s.FlushBuffer()
	}
	return nil
}

// BufferEdge adds an edge to the write buffer. If the buffer reaches capacity,
// it is automatically flushed to Pebble. Thread-safe.
func (s *GraphStore) BufferEdge(edge *Edge) error {
	if edge.ID == "" {
		edge.ID = util.GenerateEdgeID(s.repo, edge.Source, string(edge.Type), edge.Target)
	}
	if edge.Repo == "" {
		edge.Repo = s.repo
	}

	s.bufMu.Lock()
	s.buf.edges = append(s.buf.edges, edge)
	s.buf.count++
	shouldFlush := s.buf.count >= s.buf.capacity
	s.bufMu.Unlock()

	if shouldFlush {
		return s.FlushBuffer()
	}
	return nil
}

// FlushBuffer commits all buffered nodes and edges to Pebble in a single Batch.
// After flushing, the buffer is reset. Thread-safe.
func (s *GraphStore) FlushBuffer() error {
	s.bufMu.Lock()
	// Swap buffer contents to local variables for batch processing
	nodes := s.buf.nodes
	edges := s.buf.edges
	s.buf.nodes = make([]*Node, 0, s.buf.capacity)
	s.buf.edges = make([]*Edge, 0, s.buf.capacity)
	s.buf.count = 0
	s.bufMu.Unlock()

	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	return s.Batch(func(b *Batch) error {
		for _, node := range nodes {
			if err := b.AddNode(node); err != nil {
				return err
			}
		}
		for _, edge := range edges {
			if err := b.AddEdge(edge); err != nil {
				return err
			}
		}
		return nil
	})
}

// BufferedCount returns the number of items currently in the write buffer.
func (s *GraphStore) BufferedCount() int {
	s.bufMu.Lock()
	defer s.bufMu.Unlock()
	return s.buf.count
}

// ============ Node Operations ============

// AddNode adds a node (auto-generates ID, maintains indexes)
// Cache is updated only after the Pebble batch commits successfully,
// to prevent other goroutines from reading uncommitted data.
func (s *GraphStore) AddNode(node *Node) error {
	if node.ID == "" {
		node.ID = util.GenerateID(node.Repo, string(node.Label), node.Name)
	}
	if node.Repo == "" {
		node.Repo = s.repo
	}

	data, err := Encode(node)
	if err != nil {
		return fmt.Errorf("encode node %s: %w", node.ID, err)
	}

	err = s.store.BatchNoSync(func(b *pebble.Batch) error {
		// Node KV
		if err := b.Set([]byte(node.Key()), data, nil); err != nil {
			return err
		}
		// Type index
		if err := b.Set([]byte(typeKey(node.Repo, string(node.Label), node.ID)), []byte(node.ID), nil); err != nil {
			return err
		}
		// Name index
		if err := b.Set([]byte(nameKey(node.Repo, node.Name, node.ID)), []byte(node.ID), nil); err != nil {
			return err
		}
		// File index
		if node.FilePath != "" {
			if err := b.Set([]byte(fileKey(node.Repo, node.FilePath, node.ID)), []byte(node.ID), nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Update cache only after successful commit
	s.nodeCache.Add(node.ID, node)
	return nil
}

// GetNode retrieves a node (with read-through cache for hot-path optimization)
func (s *GraphStore) GetNode(id string) (*Node, error) {
	// Check cache first
	if v, ok := s.nodeCache.Get(id); ok {
		return v, nil
	}
	key := nodeKey(s.repo, id)
	data, err := s.store.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	var node Node
	if err := Decode(data, &node); err != nil {
		return nil, fmt.Errorf("decode node %s: %w", id, err)
	}
	s.nodeCache.Add(id, &node)
	return &node, nil
}

// GetNodesByLabel retrieves nodes by label
// Optimization: collect all nodeIDs first, then batch retrieve to avoid context switching overhead from individual GetNode calls in ScanPrefix
func (s *GraphStore) GetNodesByLabel(repo, label string) ([]*Node, error) {
	prefix := typePrefix(repo, label)
	var nodeIDs []string

	err := s.store.ScanPrefix([]byte(prefix), func(key, val []byte) error {
		nodeIDs = append(nodeIDs, string(val))
		return nil
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		node, err := s.GetNode(nodeID)
		if err != nil {
			continue // Skip corrupted data
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// BatchGetNodes retrieves multiple nodes by IDs in a single call.
// Cache hits are served from LRU; cache misses are batched and fetched from Pebble.
// Results are returned in the same order as the input IDs.
// Missing nodes are silently skipped (the returned slice may be shorter than ids).
func (s *GraphStore) BatchGetNodes(ids []string) ([]*Node, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	results := make([]*Node, 0, len(ids))
	var missIDs []string
	var missIndices []int // maps missIDs index → results position

	for _, id := range ids {
		if v, ok := s.nodeCache.Get(id); ok {
			results = append(results, v)
		} else {
			results = append(results, nil) // placeholder
			missIDs = append(missIDs, id)
			missIndices = append(missIndices, len(results)-1)
		}
	}

	// Batch fetch cache misses from Pebble
	for idx, id := range missIDs {
		key := nodeKey(s.repo, id)
		data, err := s.store.Get([]byte(key))
		if err != nil {
			continue // skip missing/corrupted nodes
		}
		var node Node
		if err := Decode(data, &node); err != nil {
			continue
		}
		s.nodeCache.Add(id, &node)
		results[missIndices[idx]] = &node
	}

	// Compact: remove nil entries
	filtered := results[:0]
	for _, n := range results {
		if n != nil {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}

// GetNodesByName retrieves nodes by name
func (s *GraphStore) GetNodesByName(repo, name string) ([]*Node, error) {
	prefix := namePrefix(repo, name)
	var nodeIDs []string

	err := s.store.ScanPrefix([]byte(prefix), func(key, val []byte) error {
		nodeIDs = append(nodeIDs, string(val))
		return nil
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		node, err := s.GetNode(nodeID)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// GetNodesByFile retrieves nodes by file path
func (s *GraphStore) GetNodesByFile(repo, filePath string) ([]*Node, error) {
	prefix := filePrefix(repo, filePath)
	var nodeIDs []string

	err := s.store.ScanPrefix([]byte(prefix), func(key, val []byte) error {
		nodeIDs = append(nodeIDs, string(val))
		return nil
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		node, err := s.GetNode(nodeID)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// DeleteNode deletes a node (also deletes indexes)
func (s *GraphStore) DeleteNode(id string) error {
	node, err := s.GetNode(id)
	if err != nil {
		return err
	}

	// Collect all edges involving this node before modifying adjacency indexes.
	// This must be done before BatchNoSync because GetAllOutEdges/GetAllInEdges
	// read from Pebble directly (not from the batch buffer).
	outEdges, _ := s.GetAllOutEdges(id)
	inEdges, _ := s.GetAllInEdges(id)

	// Collect edge KV keys to delete.
	// Edge KV format: "e:e:{repo}:{src}:{type}:{target}:{shortHash}"
	// Outgoing edges: prefix-scan "e:e:{repo}:{nodeID}:" finds edges where this node is source.
	// Incoming edges: prefix-scan "e:e:{repo}:{source}:{type}:{nodeID}:" for each inEdge.
	var edgeKVKeysToDelete []string
	outEdgePrefix := "e:e:" + node.Repo + ":" + id + ":"
	s.store.ScanPrefix([]byte(outEdgePrefix), func(key, _ []byte) error {
		edgeKVKeysToDelete = append(edgeKVKeysToDelete, string(key))
		return nil
	})
	for _, edge := range inEdges {
		// For each incoming edge (source→this node), prefix-scan by source+type+target
		inEdgePrefix := "e:e:" + node.Repo + ":" + edge.Source + ":" + string(edge.Type) + ":" + id + ":"
		s.store.ScanPrefix([]byte(inEdgePrefix), func(key, _ []byte) error {
			edgeKVKeysToDelete = append(edgeKVKeysToDelete, string(key))
			return nil
		})
	}

	return s.store.BatchNoSync(func(b *pebble.Batch) error {
		// 1. Clean up reverse adjacency: for each outgoing edge, remove this node
		//    from the target node's incoming adjacency index.
		for _, edge := range outEdges {
			inAdjKey := adjKey(node.Repo, edge.Target, "in", string(edge.Type))
			if err := s.removeAdjEntry(b, inAdjKey, id); err != nil {
				slog.Warn("delete_node: remove reverse in-adj failed",
					"node", id, "target", edge.Target, "relType", edge.Type, "error", err)
			}
		}

		// 2. Clean up reverse adjacency: for each incoming edge, remove this node
		//    from the source node's outgoing adjacency index.
		for _, edge := range inEdges {
			outAdjKey := adjKey(node.Repo, edge.Source, "out", string(edge.Type))
			if err := s.removeAdjEntry(b, outAdjKey, id); err != nil {
				slog.Warn("delete_node: remove reverse out-adj failed",
					"node", id, "source", edge.Source, "relType", edge.Type, "error", err)
			}
		}

		// 3. Delete all edge KV records involving this node
		for _, key := range edgeKVKeysToDelete {
			if err := b.Delete([]byte(key), nil); err != nil {
				slog.Warn("delete_node: delete edge KV failed", "key", key, "error", err)
			}
		}

		// 4. Delete node KV
		if err := b.Delete([]byte(node.Key()), nil); err != nil {
			return err
		}
		// 5. Delete type index
		if err := b.Delete([]byte(typeKey(node.Repo, string(node.Label), node.ID)), nil); err != nil {
			return err
		}
		// 6. Delete name index
		if err := b.Delete([]byte(nameKey(node.Repo, node.Name, node.ID)), nil); err != nil {
			return err
		}
		// 7. Delete file index
		if node.FilePath != "" {
			if err := b.Delete([]byte(fileKey(node.Repo, node.FilePath, node.ID)), nil); err != nil {
				return err
			}
		}
		// 8. Delete this node's own adjacency index
		if err := s.deleteAdjIndex(b, node.Repo, id); err != nil {
			return err
		}
		// 9. Invalidate caches
		s.nodeCache.Remove(id)
		return nil
	})
}

// IterNodes iterates over all nodes in a repository
func (s *GraphStore) IterNodes(repo string) NodeIterator {
	prefix := string(nodePrefix(repo))
	iter := s.store.NewIterator(&store.IterOptions{LowerBound: []byte(prefix)})
	iter.First() // Position at first element
	return &pebbleNodeIterator{
		store:  s.store,
		prefix: prefix,
		iter:   iter,
	}
}

// ============ Edge Operations ============

// AddEdge adds an edge (maintains adjacency index)
func (s *GraphStore) AddEdge(edge *Edge) error {
	if edge.ID == "" {
		edge.ID = util.GenerateEdgeID(s.repo, edge.Source, string(edge.Type), edge.Target)
	}
	if edge.Repo == "" {
		edge.Repo = s.repo
	}

	data, err := Encode(edge)
	if err != nil {
		return fmt.Errorf("encode edge %s: %w", edge.ID, err)
	}

	return s.store.BatchNoSync(func(b *pebble.Batch) error {
		// Edge KV
		if err := b.Set([]byte(edge.Key()), data, nil); err != nil {
			return err
		}
		// Outgoing edge adjacency index: adj:{repo}:{srcID}:out:{relType} → []AdjEntry
		outAdjKey := adjKey(s.repo, edge.Source, "out", string(edge.Type))
		if err := s.appendAdjEntry(b, outAdjKey, AdjEntry{ID: edge.ID, Target: edge.Target, Props: edge.Props}); err != nil {
			return err
		}
		// Incoming edge adjacency index: adj:{repo}:{tgtID}:in:{relType} → []AdjEntry
		inAdjKey := adjKey(s.repo, edge.Target, "in", string(edge.Type))
		if err := s.appendAdjEntry(b, inAdjKey, AdjEntry{ID: edge.ID, Target: edge.Source, Props: edge.Props}); err != nil {
			return err
		}
		// Invalidate adjacency caches for both source and target nodes
		// (adjCache removed — rely on Pebble block cache instead)
		return nil
	})
}

// GetEdge retrieves an edge
func (s *GraphStore) GetEdge(id string) (*Edge, error) {
	key := edgeKey(s.repo, id)
	data, err := s.store.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("get edge %s: %w", id, err)
	}
	var edge Edge
	if err := Decode(data, &edge); err != nil {
		return nil, fmt.Errorf("decode edge %s: %w", id, err)
	}
	return &edge, nil
}

// GetOutEdges retrieves outgoing edges
func (s *GraphStore) GetOutEdges(nodeID, relType string) ([]*Edge, error) {
	targets, err := s.getAdjTargets(nodeID, "out", relType)
	if err != nil {
		return nil, err
	}
	return s.resolveEdges(nodeID, targets, RelType(relType), true)
}

// GetInEdges retrieves incoming edges
func (s *GraphStore) GetInEdges(nodeID, relType string) ([]*Edge, error) {
	sources, err := s.getAdjTargets(nodeID, "in", relType)
	if err != nil {
		return nil, err
	}
	return s.resolveEdges(nodeID, sources, RelType(relType), false)
}

// DeleteEdge deletes an edge (updates adjacency index)
func (s *GraphStore) DeleteEdge(id string) error {
	edge, err := s.GetEdge(id)
	if err != nil {
		return err
	}

	return s.store.BatchNoSync(func(b *pebble.Batch) error {
		// Delete edge KV
		if err := b.Delete([]byte(edge.Key()), nil); err != nil {
			return err
		}
		// Update outgoing edge adjacency index
		outAdjKey := adjKey(s.repo, edge.Source, "out", string(edge.Type))
		if err := s.removeAdjEntry(b, outAdjKey, edge.Target); err != nil {
			return err
		}
		// Update incoming edge adjacency index
		inAdjKey := adjKey(s.repo, edge.Target, "in", string(edge.Type))
		if err := s.removeAdjEntry(b, inAdjKey, edge.Source); err != nil {
			return err
		}
		// Invalidate adjacency caches
		// (adjCache removed — rely on Pebble block cache instead)
		return nil
	})
}

// ============ Batch Operations ============

// Batch executes a batch operation
// Cache updates are deferred until after the Pebble batch commits successfully,
// preventing other goroutines from reading uncommitted data from the cache.
func (s *GraphStore) Batch(fn func(b *Batch) error) error {
	batch := &Batch{
		store:      s.store,
		graphStore: s,
		pebBatch:   nil,
		nodes:      make(map[string]*Node),
		edges:      make(map[string]*Edge),
		adjBuffer:  make(map[string][]AdjEntry),
	}

	err := s.store.BatchNoSync(func(pb *pebble.Batch) error {
		batch.pebBatch = pb
		if err := fn(batch); err != nil {
			return err
		}
		return batch.writeToPebble()
	})
	if err != nil {
		return err
	}

	// Only update caches after successful Pebble batch commit
	batch.updateCaches()
	return nil
}

// ============ Traversal Primitives ============

// BFS performs breadth-first search with context cancellation and traversal limit.
// If ctx is nil, context.Background() is used. The traversal is bounded by maxNodesVisited
// (from traversalLimit) to prevent runaway queries on large graphs.
func (s *GraphStore) BFS(ctx context.Context, startID string, direction TraverseDir, maxDepth int, filter EdgeFilter) ([]*Node, error) {
	nodes, _, err := s.BFSWithEdges(ctx, startID, direction, maxDepth, filter)
	return nodes, err
}

// BFSWithEdges returns the nodes discovered by BFS and the exact edge used to
// discover each node. Returning discovery edges keeps the result bounded and
// explains relation type and direction without emitting the full induced graph.
func (s *GraphStore) BFSWithEdges(ctx context.Context, startID string, direction TraverseDir, maxDepth int, filter EdgeFilter) ([]*Node, []*Edge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	maxNodes := s.traversalLimit
	if maxNodes <= 0 {
		maxNodes = defaultTraversalLimit
	}

	visited := make(map[string]bool)
	result := make([]*Node, 0)
	traversed := make([]*Edge, 0)
	queue := []bfsEntry{{id: startID, depth: 0}}
	visited[startID] = true
	nodesVisited := 0

	for len(queue) > 0 {
		// Periodic context check (every 1000 nodes)
		if nodesVisited%1000 == 0 {
			select {
			case <-ctx.Done():
				return result, traversed, ctx.Err()
			default:
			}
		}
		// Traversal limit check
		if nodesVisited >= maxNodes {
			return result, traversed, ErrTraversalLimitExceeded
		}

		entry := queue[0]
		queue = queue[1:]

		if entry.depth >= maxDepth {
			continue
		}

		var edges []*Edge

		if direction == TraverseOut || direction == TraverseBoth {
			outEdges, _ := s.GetAllOutEdges(entry.id)
			edges = append(edges, outEdges...)
		}
		if direction == TraverseIn || direction == TraverseBoth {
			inEdges, _ := s.GetAllInEdges(entry.id)
			edges = append(edges, inEdges...)
		}

		for _, edge := range edges {
			if filter != nil && !filter(edge) {
				continue
			}

			nextID := edge.Target
			if edge.Target == entry.id {
				nextID = edge.Source
			}

			if visited[nextID] {
				continue
			}
			visited[nextID] = true

			node, err := s.GetNode(nextID)
			if err != nil {
				continue
			}
			result = append(result, node)
			traversed = append(traversed, edge)
			queue = append(queue, bfsEntry{id: nextID, depth: entry.depth + 1})
			nodesVisited++
		}
	}

	return result, traversed, nil
}

type bfsEntry struct {
	id    string
	depth int
}

// ShortestPath finds the shortest path (BFS) with context cancellation and traversal limit.
// If ctx is nil, context.Background() is used.
func (s *GraphStore) ShortestPath(ctx context.Context, srcID, dstID string) ([]*Edge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	maxNodes := s.traversalLimit
	if maxNodes <= 0 {
		maxNodes = defaultTraversalLimit
	}

	type pathEntry struct {
		id   string
		path []*Edge
	}

	visited := make(map[string]bool)
	queue := []pathEntry{{id: srcID, path: nil}}
	visited[srcID] = true
	nodesVisited := 0

	for len(queue) > 0 {
		// Periodic context check
		if nodesVisited%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
		// Traversal limit check
		if nodesVisited >= maxNodes {
			return nil, ErrTraversalLimitExceeded
		}

		entry := queue[0]
		queue = queue[1:]

		edges, err := s.GetAllOutEdges(entry.id)
		if err != nil {
			continue
		}

		for _, edge := range edges {
			if visited[edge.Target] {
				continue
			}
			visited[edge.Target] = true
			nodesVisited++

			newPath := make([]*Edge, len(entry.path)+1)
			copy(newPath, entry.path)
			newPath[len(entry.path)] = edge

			if edge.Target == dstID {
				return newPath, nil
			}

			queue = append(queue, pathEntry{id: edge.Target, path: newPath})
		}
	}

	return nil, fmt.Errorf("no path from %s to %s", srcID, dstID)
}

// DetectCycles detects circular dependencies using iterative DFS to avoid stack overflow.
// If ctx is nil, context.Background() is used. The traversal is bounded by traversalLimit.
func (s *GraphStore) DetectCycles(ctx context.Context, repo string) ([][]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	maxNodes := s.traversalLimit
	if maxNodes <= 0 {
		maxNodes = defaultTraversalLimit
	}

	var cycles [][]string
	visited := make(map[string]int) // 0=white, 1=gray, 2=black
	// path tracks the current DFS path for cycle extraction
	path := make([]string, 0)

	nodes := s.IterNodes(repo)
	defer nodes.Close()

	// Collect all nodes
	var allNodes []*Node
	for nodes.Next() {
		allNodes = append(allNodes, nodes.Node())
	}

	// dfsFrame represents a stack frame for iterative DFS
	type dfsFrame struct {
		nodeID    string
		edgeIdx   int     // current edge index being processed
		edges     []*Edge // pre-fetched edges for this node
		returning bool    // true = returning from a child, false = first visit
	}

	totalVisited := 0

	for _, startNode := range allNodes {
		if visited[startNode.ID] != 0 {
			continue
		}

		// Start DFS from this node
		stack := []dfsFrame{{
			nodeID:    startNode.ID,
			edgeIdx:   0,
			edges:     nil,
			returning: false,
		}}

		for len(stack) > 0 {
			// Context check
			if totalVisited%1000 == 0 {
				select {
				case <-ctx.Done():
					return cycles, ctx.Err()
				default:
				}
			}
			if totalVisited >= maxNodes {
				return cycles, ErrTraversalLimitExceeded
			}

			frame := &stack[len(stack)-1]

			// First visit: mark gray
			if !frame.returning {
				visited[frame.nodeID] = 1
				path = append(path, frame.nodeID)
				// Cycle-detection edges: only consider semantic (dependency) edges.
				// Structural edges like DEFINES, CONTAINS, MEMBER_OF create spurious cycles.
				edges, _ := s.GetAllOutEdges(frame.nodeID)
				var semanticEdges []*Edge
				for _, e := range edges {
					switch e.Type {
					case RelCalls, RelImplements, RelExtends, RelInherits, RelAccesses,
						RelUses, RelMethodOverrides, RelMethodImplements, RelQueries,
						RelFetches, RelWraps, RelDecorates, RelBindsEventHandler, RelEmitsEvent:
						if e.Source == e.Target {
							continue // skip self-loops
						}
						semanticEdges = append(semanticEdges, e)
					default:
						continue // skip structural/organizational edges (DEFINES, CONTAINS, MEMBER_OF, etc.)
					}
				}
				frame.edges = semanticEdges
				frame.edgeIdx = 0
				frame.returning = true
				totalVisited++
			}

			// Process edges
			if frame.edgeIdx < len(frame.edges) {
				edge := frame.edges[frame.edgeIdx]
				frame.edgeIdx++
				targetID := edge.Target

				switch visited[targetID] {
				case 0: // white — recurse
					stack = append(stack, dfsFrame{
						nodeID:    targetID,
						edgeIdx:   0,
						edges:     nil,
						returning: false,
					})
				case 1: // gray — cycle detected
					cycleStart := -1
					for i, id := range path {
						if id == targetID {
							cycleStart = i
							break
						}
					}
					if cycleStart >= 0 {
						cycle := make([]string, len(path)-cycleStart)
						copy(cycle, path[cycleStart:])
						cycles = append(cycles, cycle)
					}
				case 2: // black — already processed, skip
				}
				continue
			}

			// All edges processed — backtrack
			visited[frame.nodeID] = 2
			path = path[:len(path)-1]
			stack = stack[:len(stack)-1]
		}
	}

	return deduplicateAndSortCycles(cycles), nil
}

// deduplicateAndSortCycles removes duplicate cycles (same set of nodes, different start point)
// and sorts remaining cycles by length (shortest first).
// A cycle A→B→C→A is the same as B→C→A→B — we normalize by starting from the
// lexicographically smallest node ID.
// Also removes supersets: if cycle A→B→C exists, A→B→C→D is a superset and is removed.
func deduplicateAndSortCycles(cycles [][]string) [][]string {
	if len(cycles) == 0 {
		return cycles
	}

	seen := make(map[string]bool)
	var result [][]string

	for _, cycle := range cycles {
		if len(cycle) == 0 {
			continue
		}
		// Normalize: find the rotation starting from the smallest node ID
		minIdx := 0
		for i := 1; i < len(cycle); i++ {
			if cycle[i] < cycle[minIdx] {
				minIdx = i
			}
		}
		// Rotate cycle to start from minIdx
		normalized := make([]string, len(cycle))
		copy(normalized, cycle[minIdx:])
		copy(normalized[len(cycle)-minIdx:], cycle[:minIdx])

		// Use the normalized cycle as a dedup key
		key := strings.Join(normalized, "|")
		if !seen[key] {
			seen[key] = true
			result = append(result, normalized)
		}
	}

	// Remove superset cycles: if a shorter cycle's node set is a subset of
	// a longer cycle's node set, the longer one is redundant (it's just the
	// shorter cycle with detours). Only keep the minimal (shortest) representation.
	result = removeSupersetCycles(result)

	// Sort by cycle length (shortest first — most actionable)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if len(result[i]) > len(result[j]) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// removeSupersetCycles removes cycles whose node set is a superset of
// another cycle's node set. For example, if A→B→C and A→B→D→C both exist,
// the latter contains A,B,C plus D, so it's a superset of {A,B,C} and is removed.
func removeSupersetCycles(cycles [][]string) [][]string {
	if len(cycles) <= 1 {
		return cycles
	}

	// Build node sets for each cycle
	type cycleSet struct {
		cycle   []string
		nodeSet map[string]bool
	}
	sets := make([]cycleSet, len(cycles))
	for i, c := range cycles {
		ns := make(map[string]bool, len(c))
		for _, id := range c {
			ns[id] = true
		}
		sets[i] = cycleSet{cycle: c, nodeSet: ns}
	}

	keep := make([]bool, len(cycles))
	for i := range keep {
		keep[i] = true
	}

	for i := 0; i < len(sets); i++ {
		if !keep[i] {
			continue
		}
		for j := 0; j < len(sets); j++ {
			if i == j || !keep[j] {
				continue
			}
			// If set[i] ⊆ set[j] and len(i) < len(j), then j is a superset of i → remove j
			if len(sets[i].cycle) < len(sets[j].cycle) && isSubset(sets[i].nodeSet, sets[j].nodeSet) {
				keep[j] = false
			}
		}
	}

	var result [][]string
	for i, k := range keep {
		if k {
			result = append(result, sets[i].cycle)
		}
	}
	return result
}

// isSubset returns true if every key in a is also in b.
func isSubset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// ============ Helper Methods ============

// ReleaseEdges returns an edge slice to the sync.Pool for reuse.
// Callers must not reference the slice after calling this function.
func ReleaseEdges(edges []*Edge) {
	if edges == nil {
		return
	}
	edges = edges[:0]
	adjEntrySlicePool.Put(&edges)
}

// ReleaseNodes returns a node slice to the sync.Pool for reuse.
// Callers must not reference the slice after calling this function.
func ReleaseNodes(nodes []*Node) {
	if nodes == nil {
		return
	}
	nodes = nodes[:0]
	nodeSlicePool.Put(&nodes)
}

// getAdjTargets retrieves target ID list from adjacency index
func (s *GraphStore) getAdjTargets(nodeID, direction, relType string) ([]string, error) {
	adjKey := adjKey(s.repo, nodeID, direction, relType)
	data, err := s.store.Get([]byte(adjKey))
	if err != nil {
		return nil, nil // No adjacency data is not an error
	}
	entries, err := DecodeAdjEntries(data)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(entries))
	for _, e := range entries {
		targets = append(targets, e.Target)
	}
	return targets, nil
}

// GetAllOutEdges retrieves all types of outgoing edges (Scheme A: properties retrieved directly from adjacency index)
// Adjacency data is read from Pebble (with block cache) instead of a separate application-level cache.
// Uses sync.Pool for edge slice reuse to reduce GC pressure on large graphs.
func (s *GraphStore) GetAllOutEdges(nodeID string) ([]*Edge, error) {
	prefix := adjScanPrefix(s.repo, nodeID, "out")
	edges := (*adjEntrySlicePool.Get().(*[]*Edge))[:0]

	err := s.store.ScanPrefix([]byte(prefix), func(key, val []byte) error {
		entries, err := DecodeAdjEntries(val)
		if err != nil {
			return nil
		}

		// Parse relationship type: adj:{repo}:{id}:out:{relType}
		keyStr := string(key)
		relTypeStr := ""
		if idx := strings.LastIndex(keyStr, ":out:"); idx >= 0 {
			relTypeStr = keyStr[idx+5:] // skip ":out:"
		}

		for _, entry := range entries {
			edges = append(edges, &Edge{
				ID:     entry.ID,
				Type:   RelType(relTypeStr),
				Source: nodeID,
				Target: entry.Target,
				Props:  entry.Props,
			})
		}
		return nil
	})
	return edges, err
}

// ScanAllOutEdgesByRelType scans ALL outgoing edges of a specific relType across all nodes in the repo.
// Uses a single ScanPrefix over the entire adj:{repo}: prefix, filtering by ":out:{relType}" suffix.
// This replaces N×GetOutEdges calls with a single scan, reducing IO from N to 1.
func (s *GraphStore) ScanAllOutEdgesByRelType(relType string) ([]*Edge, error) {
	var edges []*Edge
	prefix := AdjRepoPrefix(s.repo)
	suffix := ":out:" + relType

	err := s.store.ScanPrefix(prefix, func(key, val []byte) error {
		keyStr := string(key)
		if !strings.HasSuffix(keyStr, suffix) {
			return nil
		}
		// Extract source nodeID: adj:{repo}:{nodeID}:out:{relType}
		prefixStr := "adj:" + s.repo + ":"
		nodeID := keyStr[len(prefixStr) : len(keyStr)-len(suffix)]

		entries, err := DecodeAdjEntries(val)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			edges = append(edges, &Edge{
				ID:     entry.ID,
				Type:   RelType(relType),
				Source: nodeID,
				Target: entry.Target,
				Props:  entry.Props,
			})
		}
		return nil
	})
	return edges, err
}

// GetAllInEdges retrieves all types of incoming edges (Scheme A: properties retrieved directly from adjacency index)
// Adjacency data is read from Pebble (with block cache) instead of a separate application-level cache.
// Uses sync.Pool for edge slice reuse to reduce GC pressure on large graphs.
func (s *GraphStore) GetAllInEdges(nodeID string) ([]*Edge, error) {
	prefix := adjScanPrefix(s.repo, nodeID, "in")
	edges := (*adjEntrySlicePool.Get().(*[]*Edge))[:0]

	err := s.store.ScanPrefix([]byte(prefix), func(key, val []byte) error {
		entries, err := DecodeAdjEntries(val)
		if err != nil {
			return nil
		}

		// Extract relType from key tail
		keyStr := string(key)
		relTypeStr := ""
		if idx := strings.LastIndex(keyStr, ":in:"); idx >= 0 {
			relTypeStr = keyStr[idx+4:] // skip ":in:"
		}

		for _, entry := range entries {
			edges = append(edges, &Edge{
				ID:     entry.ID,
				Type:   RelType(relTypeStr),
				Source: entry.Target,
				Target: nodeID,
				Props:  entry.Props,
			})
		}
		return nil
	})
	return edges, err
}

// resolveEdges resolves complete edge information from adjacency index results
func (s *GraphStore) resolveEdges(nodeID string, targets []string, relType RelType, isOut bool) ([]*Edge, error) {
	edges := make([]*Edge, 0, len(targets))
	for _, targetID := range targets {
		var src, tgt string
		if isOut {
			src, tgt = nodeID, targetID
		} else {
			src, tgt = targetID, nodeID
		}
		edges = append(edges, &Edge{
			Type:   relType,
			Source: src,
			Target: tgt,
		})
	}
	return edges, nil
}

// appendAdjEntry appends an adjacency index entry using Pebble Merge operation.
// This avoids the Read-Modify-Write pattern (read old → unmarshal → append → marshal → write)
// by leveraging Pebble's built-in Merger to atomically combine new entries with existing data.
// The adjValueMerger in store.go handles the actual merge logic during compaction/read.
func (s *GraphStore) appendAdjEntry(b *pebble.Batch, key string, entry AdjEntry) error {
	data, err := EncodeAdjEntry(entry)
	if err != nil {
		return fmt.Errorf("encode adj entry: %w", err)
	}
	return b.Merge([]byte(key), data, nil)
}

// removeAdjEntry removes an entry from the adjacency index
// Note: Merge doesn't support delete operations, still requires RMW, but delete operations are rare and acceptable
func (s *GraphStore) removeAdjEntry(b *pebble.Batch, key string, targetID string) error {
	existing, err := s.store.Get([]byte(key))
	if err != nil {
		return nil
	}
	entries, err := DecodeAdjEntries(existing)
	if err != nil {
		return nil
	}

	filtered := make([]AdjEntry, 0, len(entries))
	for _, e := range entries {
		if e.Target != targetID {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) == 0 {
		return b.Delete([]byte(key), nil)
	}
	data, err := EncodeAdjEntries(filtered)
	if err != nil {
		return fmt.Errorf("encode filtered adj entries: %w", err)
	}
	return b.Set([]byte(key), data, nil)
}

// deleteAdjIndex deletes all adjacency indexes for a node
func (s *GraphStore) deleteAdjIndex(b *pebble.Batch, repo, nodeID string) error {
	for _, dir := range []string{"out", "in"} {
		prefix := adjScanPrefix(repo, nodeID, dir)
		if err := s.store.ScanPrefixLimit([]byte(prefix), 10000, func(key, _ []byte) error {
			return b.Delete(key, nil)
		}); err != nil {
			return fmt.Errorf("scan adj prefix %s: %w", prefix, err)
		}
	}
	return nil
}

// ============ Pebble Iterator Adapter ============

type pebbleNodeIterator struct {
	store   *store.Store
	prefix  string
	iter    *pebble.Iterator
	current *Node
}

func (it *pebbleNodeIterator) Next() bool {
	if !it.iter.Valid() {
		return false
	}
	if !strings.HasPrefix(string(it.iter.Key()), it.prefix) {
		return false
	}

	var node Node
	if err := Decode(it.iter.Value(), &node); err != nil {
		it.iter.Next()
		return it.Next()
	}
	it.current = &node
	it.iter.Next()
	return true
}

func (it *pebbleNodeIterator) Node() *Node {
	return it.current
}

func (it *pebbleNodeIterator) Close() error {
	return it.iter.Close()
}

// Repo returns the current repository name
func (s *GraphStore) Repo() string {
	return s.repo
}

// Store returns the underlying Pebble Store (for use by LexicalIndex and other modules)
func (s *GraphStore) Store() *store.Store {
	return s.store
}

// GetAllNodes retrieves all nodes in a repository (with limit)
func (s *GraphStore) GetAllNodes(repo string, limit int) []*Node {
	prefix := nodePrefix(repo)
	var nodes []*Node

	s.store.ScanPrefixLimit([]byte(prefix), limit, func(key, val []byte) error {
		var node Node
		if err := Decode(val, &node); err != nil {
			return nil
		}
		nodes = append(nodes, &node)
		return nil
	})

	return nodes
}

// Flush flushes data to disk
func (s *GraphStore) Flush() error {
	return s.store.Flush()
}

// Compact compacts the storage
func (s *GraphStore) Compact() error {
	return s.store.Compact()
}

// GraphNodeLookup is a graph node lookup interface (for scope resolution)
type GraphNodeLookup interface {
	FindByName(repo, name string) ([]*Node, error)
	FindByUID(uid string) (*Node, error)
}

// LookupByName performs lookup by name (implements GraphNodeLookup)
func (s *GraphStore) FindByName(repo, name string) ([]*Node, error) {
	return s.GetNodesByName(repo, name)
}

// FindByUID performs lookup by UID
func (s *GraphStore) FindByUID(uid string) (*Node, error) {
	parts := strings.SplitN(uid, ":", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid UID: %s", uid)
	}
	repo, filePath, label, name := parts[0], parts[1], parts[2], parts[3]
	nodes, err := s.GetNodesByName(repo, name)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.FilePath == filePath && string(n.Label) == label {
			return n, nil
		}
	}
	return nil, fmt.Errorf("node not found: %s", uid)
}

// Ensure GraphStore implements GraphNodeLookup
var _ GraphNodeLookup = (*GraphStore)(nil)

// Batch represents a batch operation context
type Batch struct {
	store      *store.Store
	graphStore *GraphStore // Reference to parent GraphStore for accessing repo and appendAdjEntry
	pebBatch   *pebble.Batch
	nodes      map[string]*Node
	edges      map[string]*Edge
	adjBuffer  map[string][]AdjEntry // Coalesced adjacency entries per adjKey, reducing Merge calls
}

// AddNode adds a node in batch
func (b *Batch) AddNode(node *Node) error {
	if node.ID == "" {
		node.ID = util.GenerateID(node.Repo, string(node.Label), node.Name)
	}
	b.nodes[node.ID] = node
	return nil
}

// AddEdge adds an edge in batch.
// Adjacency entries are buffered in adjBuffer and coalesced by key during writeToPebble,
// reducing Pebble Merge calls from N-per-edge to N-per-unique-key.
func (b *Batch) AddEdge(edge *Edge) error {
	if edge.ID == "" {
		edge.ID = util.GenerateEdgeID(b.graphStore.repo, edge.Source, string(edge.Type), edge.Target)
	}
	if edge.Repo == "" {
		edge.Repo = b.graphStore.repo
	}
	b.edges[edge.ID] = edge

	// Buffer outgoing adjacency entry
	outKey := adjKey(b.graphStore.repo, edge.Source, "out", string(edge.Type))
	b.adjBuffer[outKey] = append(b.adjBuffer[outKey], AdjEntry{ID: edge.ID, Target: edge.Target, Props: edge.Props})

	// Buffer incoming adjacency entry
	inKey := adjKey(b.graphStore.repo, edge.Target, "in", string(edge.Type))
	b.adjBuffer[inKey] = append(b.adjBuffer[inKey], AdjEntry{ID: edge.ID, Target: edge.Source, Props: edge.Props})

	return nil
}

// writeToPebble writes batch operations to Pebble (without updating caches).
// Cache updates are deferred until after the Pebble batch is committed,
// to prevent other goroutines from reading uncommitted data from the cache.
func (b *Batch) writeToPebble() error {
	start := time.Now()
	nodeCount := len(b.nodes)
	edgeCount := len(b.edges)

	for _, node := range b.nodes {
		data, err := Encode(node)
		if err != nil {
			return err
		}
		if err := b.pebBatch.Set([]byte(node.Key()), data, nil); err != nil {
			return err
		}
		// Indexes
		if err := b.pebBatch.Set([]byte(typeKey(node.Repo, string(node.Label), node.ID)), []byte(node.ID), nil); err != nil {
			return err
		}
		if err := b.pebBatch.Set([]byte(nameKey(node.Repo, node.Name, node.ID)), []byte(node.ID), nil); err != nil {
			return err
		}
		if node.FilePath != "" {
			if err := b.pebBatch.Set([]byte(fileKey(node.Repo, node.FilePath, node.ID)), []byte(node.ID), nil); err != nil {
				return err
			}
		}
	}

	for _, edge := range b.edges {
		data, err := Encode(edge)
		if err != nil {
			return err
		}
		if err := b.pebBatch.Set([]byte(edge.Key()), data, nil); err != nil {
			return err
		}
	}

	// Coalesced adjacency writes: one Merge per unique adjKey instead of one per edge.
	// This reduces ~5M individual Merge calls to ~2M (same-direction same-type edges
	// to the same node are merged in-memory first).
	for key, entries := range b.adjBuffer {
		mergedData, err := EncodeAdjEntries(entries)
		if err != nil {
			return fmt.Errorf("encode coalesced adj entries for key %s: %w", key, err)
		}
		if err := b.pebBatch.Merge([]byte(key), mergedData, nil); err != nil {
			return fmt.Errorf("merge adj entries for key %s: %w", key, err)
		}
	}

	slog.Info("batch flush",
		"repo", b.graphStore.repo,
		"nodes", nodeCount,
		"edges", edgeCount,
		"adjKeys", len(b.adjBuffer),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

// updateCaches updates node and adjacency caches after the Pebble batch has been committed.
// This must only be called after writeToPebble succeeds and the batch is committed,
// so that cached data reflects actually persisted state.
func (b *Batch) updateCaches() {
	for _, node := range b.nodes {
		b.graphStore.nodeCache.Add(node.ID, node)
	}
	// adjCache removed — adjacency data now served by Pebble block cache
}

// ContextKey represents a context key
type ContextKey string

const (
	// CtxGraphStore is the graph store context key
	CtxGraphStore ContextKey = "graphStore"
)

// GraphFromContext retrieves GraphStore from context
func GraphFromContext(ctx context.Context) *GraphStore {
	if v, ok := ctx.Value(CtxGraphStore).(*GraphStore); ok {
		return v
	}
	return nil
}

// WithGraphStore injects GraphStore into context
func WithGraphStore(ctx context.Context, store *GraphStore) context.Context {
	return context.WithValue(ctx, CtxGraphStore, store)
}
