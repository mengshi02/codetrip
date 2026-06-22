package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/cockroachdb/pebble/v2"
	"github.com/vmihailenco/msgpack/v5"
)

// Store wraps Pebble KV storage
type Store struct {
	db   *pebble.DB
	path string
}

// Config represents Pebble storage configuration
type Config struct {
	Path        string
	CacheSize   int64 // bytes, default 256MB
	MaxOpenDocs int   // default 1024
}

// DefaultConfig returns default configuration
func DefaultConfig(path string) Config {
	return Config{
		Path:        path,
		CacheSize:   256 << 20, // 256MB
		MaxOpenDocs: 1024,
	}
}

// adjMerger is the adjacency index Merge operator
// Implements atomic append: new values are deserialized and appended to the existing list
var adjMerger = &pebble.Merger{
	Name: "adj_merge",
	Merge: func(key, value []byte) (pebble.ValueMerger, error) {
		// Copy value to avoid Pebble internal buffer reuse.
		// The value byte slice may reference an internal Pebble buffer that can be
		// overwritten by subsequent operations, especially under -race mode where
		// memory layout changes expose this issue.
		base := make([]byte, len(value))
		copy(base, value)
		return &adjValueMerger{base: base}, nil
	},
}

// adjValueMerger is the adjacency index value merger
// Handles Pebble Merge operations for adjacency index entries ([]AdjEntry).
// Merge order: older → base → pending (newer), ensuring correct time ordering.
type adjValueMerger struct {
	base    []byte
	pending [][]byte // newer values appended after base
	older   [][]byte // older values prepended before base
}

func (m *adjValueMerger) MergeNewer(value []byte) error {
	// Copy value to avoid Pebble internal buffer reuse
	v := make([]byte, len(value))
	copy(v, value)
	m.pending = append(m.pending, v)
	return nil
}

func (m *adjValueMerger) MergeOlder(value []byte) error {
	// Copy value to avoid Pebble internal buffer reuse
	v := make([]byte, len(value))
	copy(v, value)
	m.older = append(m.older, v)
	return nil
}

func (m *adjValueMerger) Finish(includesBase bool) ([]byte, io.Closer, error) {
	var allEntries []adjEntryForMerge

	for _, o := range m.older {
		decoded := decodeAdjEntriesRaw(o)
		allEntries = append(allEntries, decoded...)
	}

	if len(m.base) > 0 {
		allEntries = append(allEntries, decodeAdjEntriesRaw(m.base)...)
	}

	for _, p := range m.pending {
		decoded := decodeAdjEntriesRaw(p)
		allEntries = append(allEntries, decoded...)
	}

	if allEntries == nil {
		allEntries = make([]adjEntryForMerge, 0)
	}

	data, err := msgpack.Marshal(allEntries)
	if err != nil {
		return nil, nil, err
	}
	return data, nil, nil
}

// adjEntryForMerge mirrors graph.AdjEntry to avoid circular import.
// Must be kept in sync with graph.AdjEntry.
type adjEntryForMerge struct {
	Target string            `msgpack:"target" json:"target"`
	Props  edgePropsForMerge `msgpack:"props,omitempty" json:"props,omitempty"`
}

// edgePropsForMerge mirrors graph.EdgeProps to avoid circular import.
// Must be kept in sync with graph.EdgeProps.
type edgePropsForMerge struct {
	Confidence         float64        `json:"confidence,omitempty" msgpack:"confidence,omitempty"`
	Line               int            `json:"line,omitempty" msgpack:"line,omitempty"`
	File               string         `json:"file,omitempty" msgpack:"file,omitempty"`
	ImportPath         string         `json:"importPath,omitempty" msgpack:"importPath,omitempty"`
	Alias              string         `json:"alias,omitempty" msgpack:"alias,omitempty"`
	Category           string         `json:"category,omitempty" msgpack:"category,omitempty"`
	SourceName         string         `json:"sourceName,omitempty" msgpack:"sourceName,omitempty"`
	SinkName           string         `json:"sinkName,omitempty" msgpack:"sinkName,omitempty"`
	Sanitized          bool           `json:"sanitized,omitempty" msgpack:"sanitized,omitempty"`
	HopIndex           int            `json:"hopIndex,omitempty" msgpack:"hopIndex,omitempty"`
	EdgeType           string         `json:"edgeType,omitempty" msgpack:"edgeType,omitempty"`
	Condition          string         `json:"condition,omitempty" msgpack:"condition,omitempty"`
	FuncID             string         `json:"funcID,omitempty" msgpack:"funcID,omitempty"`
	ReturnType         string         `json:"returnType,omitempty" msgpack:"returnType,omitempty"`
	SameFile           bool           `json:"sameFile,omitempty" msgpack:"sameFile,omitempty"`
	OverloadResolution string         `json:"overloadResolution,omitempty" msgpack:"overloadResolution,omitempty"`
	Order              int            `json:"order,omitempty" msgpack:"order,omitempty"`
	Weight             float64        `json:"weight,omitempty" msgpack:"weight,omitempty"`
	Operation          string         `json:"operation,omitempty" msgpack:"operation,omitempty"`
	Model              string         `json:"model,omitempty" msgpack:"model,omitempty"`
	Topic              string         `json:"topic,omitempty" msgpack:"topic,omitempty"`
	Extra              map[string]any `json:"extra,omitempty" msgpack:"extra,omitempty"`
}

// decodeAdjEntriesRaw decodes a merge operand into []adjEntryForMerge.
// Supports both msgpack and JSON formats.
func decodeAdjEntriesRaw(data []byte) []adjEntryForMerge {
	if len(data) == 0 {
		return nil
	}

	// Check for Pebble internal tombstone (all 0xff bytes)
	allFF := true
	for _, b := range data {
		if b != 0xff {
			allFF = false
			break
		}
	}
	if allFF {
		return nil
	}

	// Check for empty msgpack array (0x90) — produced by previous merger with no entries
	if len(data) == 1 && data[0] == 0x90 {
		return nil
	}

	// Try msgpack first (new format)
	var entries []adjEntryForMerge
	if err := msgpack.Unmarshal(data, &entries); err == nil {
		return entries
	}

	// Fallback to JSON (legacy format)
	var jsonEntries []adjEntryForMerge
	if err := json.Unmarshal(data, &jsonEntries); err == nil {
		return jsonEntries
	}

	// Last resort: try decode as raw map entries
	var rawEntries []map[string]any
	if err := msgpack.Unmarshal(data, &rawEntries); err == nil && len(rawEntries) > 0 {
		result := make([]adjEntryForMerge, 0, len(rawEntries))
		for _, m := range rawEntries {
			entry := adjEntryForMerge{}
			if v, ok := m["target"]; ok {
				entry.Target, _ = v.(string)
			}
			if v, ok := m["Target"]; ok {
				entry.Target, _ = v.(string)
			}
			result = append(result, entry)
		}
		return result
	}

	return nil
}

func (m *adjValueMerger) Deletable() bool     { return false }
func (m *adjValueMerger) FirstToFinish() bool { return false }

// ScalePreset defines a preset configuration for different scale scenarios
type ScalePreset int

const (
	ScaleSmall  ScalePreset = iota // <10K nodes (default)
	ScaleMedium                    // 10K-100K nodes
	ScaleLarge                     // 100K-1M+ nodes
)

// Open opens a Pebble database
func Open(cfg Config) (*Store, error) {
	return OpenWithScale(cfg, ScaleSmall)
}

// OpenWithScale opens a Pebble database with scale-specific optimizations
func OpenWithScale(cfg Config, scale ScalePreset) (*Store, error) {
	opts := &pebble.Options{
		MaxOpenFiles: cfg.MaxOpenDocs,
		Levels:       [7]pebble.LevelOptions{},
		Comparer:     pebble.DefaultComparer,
		Merger:       adjMerger,
		Logger:       pebbleLogger{},
	}

	// Apply scale-specific configuration
	switch scale {
	case ScaleLarge: // 100K-1M+ nodes
		opts.Cache = pebble.NewCache(1 << 30) // 1GB block cache
		opts.MemTableSize = 256 << 20         // 256MB memtable
		opts.L0CompactionThreshold = 4
		opts.L0StopWritesThreshold = 12
		for i := range opts.Levels {
			l := &pebble.LevelOptions{}
			l.BlockSize = 128 << 10 // 128KB blocks
			opts.Levels[i] = *l
		}
	case ScaleMedium: // 10K-100K nodes
		opts.Cache = pebble.NewCache(512 << 20) // 512MB block cache
		opts.MemTableSize = 128 << 20           // 128MB memtable
		for i := range opts.Levels {
			l := &pebble.LevelOptions{}
			l.BlockSize = 64 << 10 // 64KB blocks
			opts.Levels[i] = *l
		}
	default: // ScaleSmall: <10K nodes
		opts.Cache = pebble.NewCache(cfg.CacheSize) // default from config (256MB)
		opts.MemTableSize = 64 << 20                // 64MB memtable
		for i := range opts.Levels {
			l := &pebble.LevelOptions{}
			if i > 0 {
				l.BlockSize = 64 << 10
			}
			opts.Levels[i] = *l
		}
		opts.Levels[0].BlockSize = 64 << 10
	}

	db, err := pebble.Open(cfg.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("db open %s: %w", cfg.Path, err)
	}

	return &Store{
		db:   db,
		path: cfg.Path,
	}, nil
}

// pebbleLogger forwards Pebble logs to slog with appropriate levels.
// Info and Debug messages from Pebble are mapped to slog.Debug to reduce noise
// while remaining observable when debug-level logging is enabled.
// Error and Fatal messages are always forwarded at their respective slog levels.
type pebbleLogger struct{}

func (pebbleLogger) Infof(format string, args ...interface{}) {
	slog.Debug("pebble", "msg", fmt.Sprintf(format, args...))
}
func (pebbleLogger) Debugf(format string, args ...interface{}) {
	slog.Debug("pebble", "msg", fmt.Sprintf(format, args...))
}
func (pebbleLogger) Errorf(format string, args ...interface{}) {
	slog.Error("pebble", "msg", fmt.Sprintf(format, args...))
}
func (pebbleLogger) Fatalf(format string, args ...interface{}) {
	slog.Error("pebble fatal", "msg", fmt.Sprintf(format, args...))
}

// Close closes the database
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the database path
func (s *Store) Path() string {
	return s.path
}

// Trip returns the underlying Pebble Trip instance
func (s *Store) Trip() *pebble.DB {
	return s.db
}

// Get reads a single key
func (s *Store) Get(key []byte) ([]byte, error) {

	val, closer, err := s.db.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Copy data to avoid underlying buffer reuse
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

// Set writes a single key-value pair
func (s *Store) Set(key, value []byte) error {
	return s.db.Set(key, value, pebble.Sync)
}

// SetNoSync writes a single key-value pair (no flush, used for batch writes)
func (s *Store) SetNoSync(key, value []byte) error {
	return s.db.Set(key, value, pebble.NoSync)
}

// Delete deletes a key
func (s *Store) Delete(key []byte) error {
	return s.db.Delete(key, pebble.Sync)
}

// DeleteNoSync deletes a key (no flush)
func (s *Store) DeleteNoSync(key []byte) error {
	return s.db.Delete(key, pebble.NoSync)
}

// Batch performs batch operations
func (s *Store) Batch(fn func(b *pebble.Batch) error) error {

	batch := s.db.NewBatch()
	defer batch.Close()

	if err := fn(batch); err != nil {
		return err
	}

	return batch.Commit(pebble.Sync)
}

// BatchNoSync performs batch operations (no flush)
func (s *Store) BatchNoSync(fn func(b *pebble.Batch) error) error {

	batch := s.db.NewBatch()
	defer batch.Close()

	if err := fn(batch); err != nil {
		return err
	}

	return batch.Commit(pebble.NoSync)
}

// IterOptions represents iterator options
type IterOptions struct {
	LowerBound []byte
	UpperBound []byte
}

// NewIterator creates an iterator
func (s *Store) NewIterator(opts *IterOptions) *pebble.Iterator {

	var pebbleOpts *pebble.IterOptions
	if opts != nil {
		pebbleOpts = &pebble.IterOptions{
			LowerBound: opts.LowerBound,
			UpperBound: opts.UpperBound,
		}
	}
	iter, err := s.db.NewIter(pebbleOpts)
	if err != nil {
		return nil
	}
	return iter
}

// ScanPrefix scans all key-value pairs with the specified prefix
func (s *Store) ScanPrefix(prefix []byte, fn func(key, val []byte) error) error {
	return s.ScanPrefixLimit(prefix, 0, fn)
}

// ScanPrefixLimit scans key-value pairs with the specified prefix, maxItems=0 means no limit
// Prevents large repos from returning tens of thousands of results causing OOM
func (s *Store) ScanPrefixLimit(prefix []byte, maxItems int, fn func(key, val []byte) error) error {
	upperBound := make([]byte, len(prefix))
	copy(upperBound, prefix)
	for i := len(upperBound) - 1; i >= 0; i-- {
		upperBound[i]++
		if upperBound[i] != 0 {
			break
		}
	}

	iter := s.NewIterator(&IterOptions{
		LowerBound: prefix,
		UpperBound: upperBound,
	})
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		if err := fn(iter.Key(), iter.Value()); err != nil {
			return err
		}
		count++
		if maxItems > 0 && count >= maxItems {
			break
		}
	}
	return iter.Error()
}

// HasKey checks if a key exists
func (s *Store) HasKey(key []byte) (bool, error) {

	_, closer, err := s.db.Get(key)
	if err != nil {
		if err == pebble.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	closer.Close()
	return true, nil
}

// Compact triggers manual compaction
func (s *Store) Compact() error {
	// Need to provide a valid key range, use first and last bytes as range
	return s.db.Compact(context.Background(), []byte{0}, []byte{0xFF, 0xFF}, true)
}

// Flush flushes (sync, ensures data is visible to iterators)
func (s *Store) Flush() error {
	return s.db.Flush()
}

// Metrics returns storage metrics
func (s *Store) Metrics() *pebble.Metrics {
	return s.db.Metrics()
}

// Checkpoint creates a point-in-time snapshot of the database in the given directory.
// This is the basis for backup/restore functionality.
func (s *Store) Checkpoint(dest string) error {
	return s.db.Checkpoint(dest)
}
