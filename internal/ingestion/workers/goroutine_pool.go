// GoroutinePool — Go-native concurrent parse dispatch.
//
// Replaces TS WorkerPool (worker_pool.ts). Key differences:
//
//   - TS: Worker Threads → slot allocation → postMessage → structured clone
//   - Go: goroutine → semaphore → channel → shared memory
//
// goroutine 极轻量（2KB 栈），无需 slot/respawn 机制。
// 并发度由 semaphore.Weighted 控制。
// panic 通过 recover 捕获，出错文件被 quarantine 追踪。
package workers

import (
	"context"
	"runtime"
	"sync"

	"golang.org/x/sync/semaphore"
)

// ── FileEntry (re-export for convenience) ────────────────────────────────

// FileEntry represents a file to be parsed.
type FileEntry struct {
	Path    string
	Content string
}

// ── SubBatch splitter ────────────────────────────────────────────────────

// splitIntoSubBatches splits files into sub-batches by byte budget.
// Each sub-batch will be processed by a single goroutine.
// This corresponds to TS WorkerPool's sub-batch logic.
func splitIntoSubBatches(files []FileEntry, budget int64) [][]FileEntry {
	if len(files) == 0 {
		return nil
	}

	var batches [][]FileEntry
	var current []FileEntry
	var currentBytes int64

	for _, f := range files {
		fSize := int64(len(f.Content))
		if len(current) > 0 && currentBytes+fSize > budget {
			batches = append(batches, current)
			current = nil
			currentBytes = 0
		}
		current = append(current, f)
		currentBytes += fSize
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// ── GoroutinePool ────────────────────────────────────────────────────────

// GoroutinePool manages goroutine-based concurrent parsing.
// It replaces TS WorkerPool but leverages Go's native strengths:
//   - goroutines are lightweight (2KB stack) — no slot/respawn needed
//   - channels provide native communication — no postMessage/transferList
//   - recover handles panics — no quarantine/circuit-breaker
//   - shared memory — no V8 structured clone boundary
type GoroutinePool struct {
	maxWorkers   int
	semaphore    *semaphore.Weighted
	resultCh     chan ParseFileSetResult
	errCh        chan PoolError
	wg           sync.WaitGroup
	quarantine   *QuarantineTracker
	ctx          context.Context
	cancel       context.CancelFunc
	subBatchSize int64 // byte budget per sub-batch

	// Statistics (updated atomically or under wg-guaranteed single-reader)
	stats PoolStats
}

// resolveConcurrency determines the effective concurrency level.
// Priority: explicit value > runtime.NumCPU().
func resolveConcurrency(n int) int {
	if n > 0 {
		return n
	}
	return runtime.NumCPU()
}

// NewGoroutinePool creates a new GoroutinePool.
//
// Parameters:
//   - maxWorkers: maximum concurrent goroutines (0 = NumCPU)
//   - subBatchBytes: byte budget per sub-batch (0 = DefaultSubBatchBytes)
func NewGoroutinePool(maxWorkers int, subBatchBytes int64) *GoroutinePool {
	concurrency := resolveConcurrency(maxWorkers)
	if subBatchBytes <= 0 {
		subBatchBytes = DefaultSubBatchBytes
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GoroutinePool{
		maxWorkers:   concurrency,
		semaphore:    semaphore.NewWeighted(int64(concurrency)),
		resultCh:     make(chan ParseFileSetResult, concurrency*2),
		errCh:        make(chan PoolError, concurrency*2),
		quarantine:   NewQuarantineTracker(),
		ctx:          ctx,
		cancel:       cancel,
		subBatchSize: subBatchBytes,
		stats: PoolStats{
			MaxWorkers: concurrency,
		},
	}
}

// ── Dispatch ─────────────────────────────────────────────────────────────

// Dispatch dispatches files for concurrent parsing.
// It splits the files into sub-batches, spawns a goroutine per sub-batch,
// and collects results via channels.
//
// Corresponds to TS WorkerPool.dispatch:
//   - TS: postMessage + transferList → worker → postMessage back
//   - Go: goroutine → shared memory → channel back
func (p *GoroutinePool) Dispatch(
	files []FileEntry,
	parseFn func([]FileEntry) ParseFileSetResult,
	onProgress func(completed int, message string),
) ([]ParseFileSetResult, []PoolError) {
	// Split into sub-batches
	subBatches := splitIntoSubBatches(files, p.subBatchSize)
	if len(subBatches) == 0 {
		return nil, nil
	}

	// Launch goroutines
	for i, batch := range subBatches {
		batch := batch // capture loop variable
		idx := i

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()

			// Acquire semaphore
			if err := p.semaphore.Acquire(p.ctx, 1); err != nil {
				// Context cancelled
				return
			}
			defer p.semaphore.Release(1)

			// Check quarantine — skip already-failed files
			filtered := make([]FileEntry, 0, len(batch))
			for _, f := range batch {
				if p.quarantine.IsQuarantined(f.Path) {
					continue
				}
				filtered = append(filtered, f)
			}
			if len(filtered) == 0 {
				return
			}

			// Parse with recover
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Capture the first file path as the likely culprit
						path := ""
						if len(filtered) > 0 {
							path = filtered[0].Path
						}
						p.quarantine.Mark(path, "goroutine panic")
						p.errCh <- PoolError{
							FilePath:  path,
							Recovered: true,
							Err:       newPanicError(r),
						}
						p.stats.Errors++
					}
				}()

				result := parseFn(filtered)
				result.SubBatchIdx = idx
				p.resultCh <- result
				p.stats.Completed += result.FileCount
			}()

			if onProgress != nil {
				onProgress(p.stats.Completed, "parsing...")
			}
		}()
	}

	// Wait for all goroutines, then close channels
	go func() {
		p.wg.Wait()
		close(p.resultCh)
		close(p.errCh)
	}()

	// Collect results
	var results []ParseFileSetResult
	var errors []PoolError

	for r := range p.resultCh {
		results = append(results, r)
	}
	for e := range p.errCh {
		errors = append(errors, e)
	}

	p.stats.Quarantined = p.quarantine.Count()
	return results, errors
}

// Cancel cancels all in-flight goroutines.
func (p *GoroutinePool) Cancel() {
	p.cancel()
}

// Stats returns a snapshot of the pool statistics.
func (p *GoroutinePool) Stats() PoolStats {
	return p.stats
}

// Quarantine returns the quarantine tracker.
func (p *GoroutinePool) Quarantine() *QuarantineTracker {
	return p.quarantine
}

// ── Panic error helper ───────────────────────────────────────────────────

type panicError struct {
	val interface{}
}

func (e *panicError) Error() string { return "goroutine panic recovered" }

func newPanicError(val interface{}) *panicError { return &panicError{val: val} }