package graph

import (
	"hash/fnv"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

const shardCount = 64

// shardedLRU is a sharded LRU cache that reduces lock contention
// under high-concurrency access patterns. Each shard has its own
// mutex and LRU cache, so operations on different keys can proceed
// concurrently without blocking.
//
// Key assignment: shard = fnv32(key) % shardCount
// Per-shard capacity = totalCapacity / shardCount
type shardedLRU[K comparable, V any] struct {
	shards [shardCount]struct {
		mu  sync.RWMutex
		lru *lru.Cache[K, V]
	}
}

// newShardedLRU creates a sharded LRU cache with the given total capacity.
// The capacity is divided evenly across shards (each shard gets total/shardCount entries).
func newShardedLRU[K comparable, V any](totalCapacity int) *shardedLRU[K, V] {
	perShard := totalCapacity / shardCount
	if perShard < 1 {
		perShard = 1
	}

	s := &shardedLRU[K, V]{}
	for i := 0; i < shardCount; i++ {
		cache, _ := lru.New[K, V](perShard)
		s.shards[i].lru = cache
	}
	return s
}

// shardIdx computes the shard index for a string key using FNV-1a hash.
// This is the primary hash function for node ID keys, providing good
// distribution across 64 shards.
func shardIdx(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % shardCount
}

// Get retrieves a value from the cache. Returns the value and true if found,
// or the zero value and false if not found.
func (s *shardedLRU[K, V]) Get(key K) (V, bool) {
	idx := keyShard(key)
	s.shards[idx].mu.RLock()
	defer s.shards[idx].mu.RUnlock()
	return s.shards[idx].lru.Get(key)
}

// Add adds a key-value pair to the cache. If the key already exists,
// the value is updated and the entry is moved to the front (most recently used).
func (s *shardedLRU[K, V]) Add(key K, value V) {
	idx := keyShard(key)
	s.shards[idx].mu.Lock()
	defer s.shards[idx].mu.Unlock()
	s.shards[idx].lru.Add(key, value)
}

// Remove removes a key from the cache.
func (s *shardedLRU[K, V]) Remove(key K) {
	idx := keyShard(key)
	s.shards[idx].mu.Lock()
	defer s.shards[idx].mu.Unlock()
	s.shards[idx].lru.Remove(key)
}

// Keys returns all keys in the cache, across all shards.
func (s *shardedLRU[K, V]) Keys() []K {
	var allKeys []K
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		keys := s.shards[i].lru.Keys()
		s.shards[i].mu.RUnlock()
		allKeys = append(allKeys, keys...)
	}
	return allKeys
}

// Len returns the total number of entries in the cache.
func (s *shardedLRU[K, V]) Len() int {
	total := 0
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		total += s.shards[i].lru.Len()
		s.shards[i].mu.RUnlock()
	}
	return total
}

// Purge clears all entries from all shards.
func (s *shardedLRU[K, V]) Purge() {
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.Lock()
		s.shards[i].lru.Purge()
		s.shards[i].mu.Unlock()
	}
}

// keyShard returns the shard index for a given key.
// For string keys (the primary use case for node IDs), this uses FNV-1a
// for excellent distribution. For non-string keys, it falls back to
// fmt.Sprintf hashing.
func keyShard[K comparable](key K) uint32 {
	if s, ok := any(key).(string); ok {
		return shardIdx(s)
	}
	// Fallback for non-string keys: use fmt.Sprint for a stable string representation
	h := fnv.New32a()
	// Simple numeric hash for int types
	switch v := any(key).(type) {
	case int:
		var buf [8]byte
		for i := 0; i < 8; i++ {
			buf[i] = byte(v >> (i * 8))
		}
		h.Write(buf[:])
	case int64:
		var buf [8]byte
		for i := 0; i < 8; i++ {
			buf[i] = byte(v >> (i * 8))
		}
		h.Write(buf[:])
	}
	return h.Sum32() % shardCount
}