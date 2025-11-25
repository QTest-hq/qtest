package api

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTaskPool_Submit(t *testing.T) {
	pool := NewTaskPool(3)
	defer pool.Close()

	var counter int64
	const numTasks = 10

	for i := 0; i < numTasks; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
	}

	pool.Wait()

	if counter != numTasks {
		t.Errorf("counter = %d, want %d", counter, numTasks)
	}
}

func TestTaskPool_ConcurrencyLimit(t *testing.T) {
	pool := NewTaskPool(2)
	defer pool.Close()

	var maxConcurrent int64
	var current int64

	for i := 0; i < 10; i++ {
		pool.Submit(func() {
			c := atomic.AddInt64(&current, 1)
			// Track max concurrent
			for {
				max := atomic.LoadInt64(&maxConcurrent)
				if c <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, c) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt64(&current, -1)
		})
	}

	pool.Wait()

	if maxConcurrent > 2 {
		t.Errorf("maxConcurrent = %d, want <= 2", maxConcurrent)
	}
}

func TestTaskPool_TrySubmit(t *testing.T) {
	pool := NewTaskPool(1)
	defer pool.Close()

	// First task should succeed
	started := make(chan struct{})
	done := make(chan struct{})

	if !pool.TrySubmit(func() {
		close(started)
		<-done
	}) {
		t.Error("first TrySubmit should succeed")
	}

	<-started // Wait for task to start

	// Second task should fail (pool at capacity)
	if pool.TrySubmit(func() {}) {
		t.Error("second TrySubmit should fail when pool is full")
	}

	close(done) // Let first task complete
	pool.Wait()
}

func TestTaskPool_Close(t *testing.T) {
	pool := NewTaskPool(1)

	var counter int64
	started := make(chan struct{})
	proceed := make(chan struct{})

	// Submit a task that blocks
	pool.Submit(func() {
		close(started)
		<-proceed
		atomic.AddInt64(&counter, 1)
	})

	<-started // Wait for task to start

	// Close the pool (this will block waiting for the task)
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(proceed)
	}()

	pool.Close()

	// Should not accept new tasks after close
	if pool.Submit(func() {}) {
		t.Error("Submit should return false after Close")
	}

	if counter != 1 {
		t.Errorf("counter = %d, want 1", counter)
	}
}

func TestTaskPool_DefaultConcurrency(t *testing.T) {
	pool := NewTaskPool(0)
	defer pool.Close()

	var counter int64
	for i := 0; i < 10; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&counter, 1)
		})
	}

	pool.Wait()

	if counter != 10 {
		t.Errorf("counter = %d, want 10", counter)
	}
}

func TestTaskPool_PanicRecovery(t *testing.T) {
	pool := NewTaskPool(2)
	defer pool.Close()

	var completed int64

	// Submit a task that panics
	pool.Submit(func() {
		panic("test panic")
	})

	// Submit normal tasks
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&completed, 1)
		})
	}

	pool.Wait()

	// All non-panicking tasks should complete
	if completed != 5 {
		t.Errorf("completed = %d, want 5", completed)
	}
}
