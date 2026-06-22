package pipeline

import (
	"context"
	"runtime"
	"sync"
)

// WorkerPool is a high-performance worker pool
// Design: fixed goroutine pool + task channel, avoiding frequent creation and destruction
type WorkerPool struct {
	maxWorkers int
	tasks      chan task
	wg         sync.WaitGroup
}

type task struct {
	fn func() error
}

// NewWorkerPool creates a worker pool
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}
	return &WorkerPool{
		maxWorkers: maxWorkers,
		tasks:      make(chan task, maxWorkers*2), // Buffer 2x to reduce blocking
	}
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker is the worker goroutine
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for t := range p.tasks {
		_ = t.fn()
	}
}

// Submit submits a task
func (p *WorkerPool) Submit(fn func() error) {
	p.tasks <- task{fn: fn}
}

// Wait waits for all tasks to complete
func (p *WorkerPool) Wait() {
	close(p.tasks)
	p.wg.Wait()
}

// ProcessSlice processes a slice in parallel
func ProcessSlice[T any](ctx context.Context, items []T, maxWorkers int, fn func(ctx context.Context, item T) error) error {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	if len(items) <= maxWorkers {
		// Small number of tasks: execute directly in parallel
		errCh := make(chan error, len(items))
		var wg sync.WaitGroup
		for _, item := range items {
			wg.Add(1)
			go func(item T) {
				defer wg.Done()
				if err := fn(ctx, item); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}(item)
		}
		wg.Wait()
		close(errCh)
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	}

	// Large number of tasks: use worker pool
	pool := NewWorkerPool(maxWorkers)
	pool.Start()

	var errOnce sync.Once
	var firstErr error

	for _, item := range items {
		item := item
		pool.Submit(func() error {
			if err := fn(ctx, item); err != nil {
				errOnce.Do(func() { firstErr = err })
			}
			return nil
		})
	}

	pool.Wait()
	return firstErr
}