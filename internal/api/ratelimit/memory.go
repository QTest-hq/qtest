package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryStorage implements Storage using in-memory counters with sliding windows
type MemoryStorage struct {
	mu       sync.RWMutex
	counters map[string]*windowCounter
	closed   bool
	stopCh   chan struct{}
}

type windowCounter struct {
	mu        sync.Mutex
	count     int64
	windowEnd time.Time
	window    time.Duration
}

// NewMemoryStorage creates a new in-memory rate limit storage
func NewMemoryStorage() *MemoryStorage {
	m := &MemoryStorage{
		counters: make(map[string]*windowCounter),
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go m.cleanup()

	return m
}

// Increment increments the counter for a key and checks the limit
func (m *MemoryStorage) Increment(ctx context.Context, key string, window time.Duration, limit int) (int64, time.Time, error) {
	m.mu.Lock()
	counter, exists := m.counters[key]
	if !exists {
		counter = &windowCounter{
			window: window,
		}
		m.counters[key] = counter
	}
	m.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	now := time.Now()

	// Reset window if expired
	if now.After(counter.windowEnd) {
		counter.count = 0
		counter.windowEnd = now.Add(window)
	}

	// Increment counter
	counter.count++

	return counter.count, counter.windowEnd, nil
}

// Get returns the current count and reset time without incrementing
func (m *MemoryStorage) Get(ctx context.Context, key string) (int64, time.Time, error) {
	m.mu.RLock()
	counter, exists := m.counters[key]
	m.mu.RUnlock()

	if !exists {
		return 0, time.Now(), nil
	}

	counter.mu.Lock()
	defer counter.mu.Unlock()

	now := time.Now()

	// If window expired, return 0
	if now.After(counter.windowEnd) {
		return 0, now, nil
	}

	return counter.count, counter.windowEnd, nil
}

// Reset clears the counter for a key
func (m *MemoryStorage) Reset(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.counters, key)
	return nil
}

// Close stops the cleanup goroutine and releases resources
func (m *MemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	close(m.stopCh)
	m.counters = nil

	return nil
}

// cleanup periodically removes expired counters
func (m *MemoryStorage) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.removeExpired()
		}
	}
}

func (m *MemoryStorage) removeExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, counter := range m.counters {
		counter.mu.Lock()
		expired := now.After(counter.windowEnd)
		counter.mu.Unlock()

		if expired {
			delete(m.counters, key)
		}
	}
}
