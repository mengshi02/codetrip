package codetrip

import (
	"time"

	"github.com/mengshi02/codetrip/internal/store"
)

type options struct {
	cacheSize          int64
	maxConcurrentIndex int
	nodeCacheSize      int
	traversalLimit     int
	scalePreset        store.ScalePreset
}

// Option configures the storage engine.
type Option func(*options)

// ScalePreset selects storage tuning by expected graph size without exposing
// the internal storage implementation.
type ScalePreset int

const (
	ScaleSmall ScalePreset = iota
	ScaleMedium
	ScaleLarge
)

func defaultOptions() options {
	return options{
		cacheSize:          256 << 20,
		maxConcurrentIndex: 2,
		nodeCacheSize:      500000,
		traversalLimit:     100000,
		scalePreset:        store.ScaleSmall,
	}
}

func WithCacheSize(size int64) Option {
	return func(o *options) { o.cacheSize = size }
}

func WithMaxConcurrentIndex(n int) Option {
	return func(o *options) { o.maxConcurrentIndex = n }
}

func WithNodeCacheSize(size int) Option {
	return func(o *options) { o.nodeCacheSize = size }
}

func WithTraversalLimit(limit int) Option {
	return func(o *options) { o.traversalLimit = limit }
}

func WithScalePreset(preset ScalePreset) Option {
	return func(o *options) { o.scalePreset = store.ScalePreset(preset) }
}

type indexOptions struct {
	repoName      string
	exportCSVPath string
	exportStrict  bool
	timeout       time.Duration
	replace       bool
}

// IndexOption configures one repository indexing operation.
type IndexOption func(*indexOptions)

func defaultIndexOptions() indexOptions {
	return indexOptions{timeout: 30 * time.Minute}
}

func WithRepoName(name string) IndexOption {
	return func(o *indexOptions) { o.repoName = name }
}

// WithCSVExport writes deterministic validation CSVs after parsing.
func WithCSVExport(path string) IndexOption {
	return func(o *indexOptions) { o.exportCSVPath = path }
}

// WithCSVExportStrict makes a CSV export error fail the indexing operation.
func WithCSVExportStrict(enabled bool) IndexOption {
	return func(o *indexOptions) { o.exportStrict = enabled }
}

func WithIndexTimeout(timeout time.Duration) IndexOption {
	return func(o *indexOptions) { o.timeout = timeout }
}

// WithReplaceExisting atomically replaces an existing logical repository.
func WithReplaceExisting(enabled bool) IndexOption {
	return func(o *indexOptions) { o.replace = enabled }
}
