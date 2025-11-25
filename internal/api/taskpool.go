package api

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

// TaskPool provides a bounded pool for executing concurrent tasks.
// It limits the number of concurrent goroutines to prevent resource exhaustion.
type TaskPool struct {
	sem    chan struct{}
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTaskPool creates a new task pool with the specified maximum concurrency.
// If maxConcurrency is <= 0, it defaults to 5.
func NewTaskPool(maxConcurrency int) *TaskPool {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &TaskPool{
		sem:    make(chan struct{}, maxConcurrency),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Submit queues a task for execution. If the pool is at capacity,
// it blocks until a slot becomes available or the pool is closed.
// Returns false if the pool is closed.
func (p *TaskPool) Submit(task func()) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.sem <- struct{}{}:
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() { <-p.sem }()
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("task panicked")
				}
			}()
			task()
		}()
		return true
	}
}

// TrySubmit attempts to queue a task without blocking.
// Returns false if the pool is at capacity or closed.
func (p *TaskPool) TrySubmit(task func()) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.sem <- struct{}{}:
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() { <-p.sem }()
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("task panicked")
				}
			}()
			task()
		}()
		return true
	default:
		return false
	}
}

// Wait blocks until all submitted tasks complete.
func (p *TaskPool) Wait() {
	p.wg.Wait()
}

// Close stops accepting new tasks and waits for existing tasks to complete.
func (p *TaskPool) Close() {
	p.cancel()
	p.wg.Wait()
}

// Pending returns the number of tasks currently in the queue waiting for execution.
func (p *TaskPool) Pending() int {
	return len(p.sem)
}
