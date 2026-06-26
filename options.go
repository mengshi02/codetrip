package codetrip

import (
	"time"

	"github.com/mengshi02/codetrip/internal/ingestion"
	"github.com/mengshi02/codetrip/internal/store"
)

// options represents engine options
type options struct {
	cacheSize          int64
	phases             []ingestion.Phase
	maxConcurrentIndex int
	autoMigrate        bool              // automatically migrate schema on version mismatch
	nodeCacheSize      int               // LRU node cache capacity (default: 500000)
	traversalLimit     int               // max nodes visited during traversal (default: 100000)
	scalePreset        store.ScalePreset // scale preset for Pebble optimization
	quantInt8          bool              // enable int8 vector quantization (default: false)
	twoStageSearch     bool              // enable two-stage search: int8 coarse + float32 refine (default: false)
	bm25ChunkSize      int               // BM25 batch chunk size for large repos (default: 0 = auto 10000)
}

// Option is an engine configuration option function
type Option func(*options)

func defaultOptions() options {
	return options{
		cacheSize:          256 << 20, // 256MB
		maxConcurrentIndex: 2,
		autoMigrate:        true,   // auto-migrate by default
		nodeCacheSize:      500000, // 500K entries (~200MB)
		traversalLimit:     100000, // 100K nodes max
		scalePreset:        store.ScaleSmall,
	}
}

// WithCacheSize sets the Pebble cache size
func WithCacheSize(size int64) Option {
	return func(o *options) {
		o.cacheSize = size
	}
}

// WithMaxConcurrentIndex sets the maximum number of concurrent IndexRepo operations.
// Default is 2. Set to 0 for unlimited.
func WithMaxConcurrentIndex(n int) Option {
	return func(o *options) {
		o.maxConcurrentIndex = n
	}
}

// WithAutoMigrate controls automatic schema migration on startup.
// Default is true. Set to false to fail on schema version mismatch instead of migrating.
func WithAutoMigrate(enabled bool) Option {
	return func(o *options) {
		o.autoMigrate = enabled
	}
}

// WithPhase adds a custom Phase
func WithPhase(phase ingestion.Phase) Option {
	return func(o *options) {
		o.phases = append(o.phases, phase)
	}
}

// WithNodeCacheSize sets the LRU node cache capacity.
// Default is 500000 entries (~200MB). Set to 0 to disable node caching.
func WithNodeCacheSize(size int) Option {
	return func(o *options) {
		o.nodeCacheSize = size
	}
}

// WithTraversalLimit sets the maximum number of nodes visited during graph traversal.
// Default is 100000. When exceeded, traversal returns ErrTraversalLimitExceeded.
func WithTraversalLimit(limit int) Option {
	return func(o *options) {
		o.traversalLimit = limit
	}
}

// WithScalePreset sets the scale preset for Pebble storage optimization.
// Default is ScaleSmall. Use ScaleLarge for 100K-1M+ node repositories.
func WithScalePreset(preset store.ScalePreset) Option {
	return func(o *options) {
		o.scalePreset = preset
	}
}

// WithQuantization enables int8 vector quantization for HNSW index.
// When enabled, vectors are quantized from float32 to int8, reducing HNSW memory
// usage by ~4x (384-dim: 1536B → 384B per node). A quantized vector file is
// generated during indexing and mmap'd at search time.
// Requires WithEmbeddings to be effective.
func WithQuantization(enabled bool) Option {
	return func(o *options) {
		o.quantInt8 = enabled
	}
}

// WithTwoStageSearch enables two-stage vector search: int8 coarse search + float32 precise reranking.
// When enabled, the first stage uses int8 quantized vectors for fast HNSW search (taking top-K×3),
// then the second stage reads float32 vectors from mmap for precise cosine similarity reranking.
// This recovers ~2% recall lost from quantization while keeping search latency <200ms.
// Requires WithQuantization to be effective.
func WithTwoStageSearch(enabled bool) Option {
	return func(o *options) {
		o.twoStageSearch = enabled
	}
}

// WithBM25ChunkSize sets the chunk size for BM25 batch indexing.
// For large repos (1M+ nodes), batch indexing is broken into chunks to limit memory usage.
// Default is 0 (auto: 10000 nodes per chunk). Set to a larger value for faster indexing
// with more memory, or smaller for memory-constrained environments.
func WithBM25ChunkSize(size int) Option {
	return func(o *options) {
		o.bm25ChunkSize = size
	}
}


// ============ Index Options ============

// indexOptions represents index options
type indexOptions struct {
	repoName   string
	maxWorkers int
	byteBudget int64
	withCFG    bool
	withPDG    bool
	timeout    time.Duration
}

// IndexOption is an index configuration option function
type IndexOption func(*indexOptions)

func defaultIndexOptions() indexOptions {
	return indexOptions{
		maxWorkers: 0,        // 0 = runtime.NumCPU()
		byteBudget: 20 << 20, // 20MB
		withCFG:    false,
		withPDG:    false,
		timeout:    30 * time.Minute,
	}
}

// WithRepoName sets the repository name
func WithRepoName(name string) IndexOption {
	return func(o *indexOptions) {
		o.repoName = name
	}
}

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(n int) IndexOption {
	return func(o *indexOptions) {
		o.maxWorkers = n
	}
}

// WithByteBudget sets the byte budget per chunk
func WithByteBudget(budget int64) IndexOption {
	return func(o *indexOptions) {
		o.byteBudget = budget
	}
}

// WithCFG enables CFG construction
func WithCFG(enable bool) IndexOption {
	return func(o *indexOptions) {
		o.withCFG = enable
	}
}

// WithPDG enables PDG construction
func WithPDG(enable bool) IndexOption {
	return func(o *indexOptions) {
		o.withPDG = enable
	}
}

// WithIndexTimeout sets the index timeout
func WithIndexTimeout(d time.Duration) IndexOption {
	return func(o *indexOptions) {
		o.timeout = d
	}
}

// ============ Embed Options ============

// embedOptions holds embed configuration
type embedOptions struct {
	endpoint       string        // HTTP embedding service endpoint (required)
	model          string        // model name (default: from endpoint)
	apiKey         string        // API key (optional)
	dimensions     int           // vector dimensions (default: auto-detect)
	batchSize      int           // batch size (default: 16)
	incremental    bool          // incremental embedding (skip unchanged nodes)
	quantInt8      bool          // enable int8 quantization
	twoStageSearch bool          // enable two-stage search
	timeout        time.Duration // embed timeout
}

// EmbedOption is an embed configuration option function
type EmbedOption func(*embedOptions)

func defaultEmbedOptions() embedOptions {
	return embedOptions{
		batchSize: 16,
		timeout:   30 * time.Minute,
	}
}

// WithEmbedEndpoint sets the HTTP embedding service endpoint
func WithEmbedEndpoint(endpoint string) EmbedOption {
	return func(o *embedOptions) {
		o.endpoint = endpoint
	}
}

// WithEmbedModel sets the embedding model name
func WithEmbedModel(model string) EmbedOption {
	return func(o *embedOptions) {
		o.model = model
	}
}

// WithEmbedAPIKey sets the API key for the embedding service
func WithEmbedAPIKey(apiKey string) EmbedOption {
	return func(o *embedOptions) {
		o.apiKey = apiKey
	}
}

// WithEmbedDimensions sets the vector dimensions
func WithEmbedDimensions(dimensions int) EmbedOption {
	return func(o *embedOptions) {
		o.dimensions = dimensions
	}
}

// WithEmbedBatchSize sets the batch size for embedding requests
func WithEmbedBatchSize(batchSize int) EmbedOption {
	return func(o *embedOptions) {
		o.batchSize = batchSize
	}
}

// WithEmbedIncremental enables incremental embedding (skip unchanged nodes)
func WithEmbedIncremental(enabled bool) EmbedOption {
	return func(o *embedOptions) {
		o.incremental = enabled
	}
}

// WithEmbedQuantInt8 enables int8 vector quantization
func WithEmbedQuantInt8(enabled bool) EmbedOption {
	return func(o *embedOptions) {
		o.quantInt8 = enabled
	}
}

// WithEmbedTwoStageSearch enables two-stage search (int8 coarse + float32 refine)
func WithEmbedTwoStageSearch(enabled bool) EmbedOption {
	return func(o *embedOptions) {
		o.twoStageSearch = enabled
	}
}

// WithEmbedTimeout sets the embedding timeout
func WithEmbedTimeout(d time.Duration) EmbedOption {
	return func(o *embedOptions) {
		o.timeout = d
	}
}
