package webhook

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Dispatcher processes webhook deliveries in the background
type Dispatcher struct {
	service  *Service
	interval time.Duration
	batchSize int
	stop     chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewDispatcher creates a new webhook dispatcher
func NewDispatcher(service *Service, interval time.Duration, batchSize int) *Dispatcher {
	if interval == 0 {
		interval = 5 * time.Second
	}
	if batchSize == 0 {
		batchSize = 50
	}

	return &Dispatcher{
		service:   service,
		interval:  interval,
		batchSize: batchSize,
		stop:      make(chan struct{}),
	}
}

// Start begins processing webhook deliveries
func (d *Dispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = true
	d.mu.Unlock()

	d.wg.Add(1)
	go d.run(ctx)

	log.Info().
		Dur("interval", d.interval).
		Int("batch_size", d.batchSize).
		Msg("webhook dispatcher started")

	return nil
}

// Stop stops the dispatcher
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	d.mu.Unlock()

	close(d.stop)
	d.wg.Wait()

	log.Info().Msg("webhook dispatcher stopped")
}

// run is the main dispatcher loop
func (d *Dispatcher) run(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	// Process immediately on start
	d.processBatch(ctx)

	for {
		select {
		case <-d.stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processBatch(ctx)
		}
	}
}

// processBatch processes a batch of pending deliveries
func (d *Dispatcher) processBatch(ctx context.Context) {
	processed, err := d.service.ProcessPendingDeliveries(ctx, d.batchSize)
	if err != nil {
		log.Error().Err(err).Msg("failed to process webhook deliveries")
		return
	}

	if processed > 0 {
		log.Debug().
			Int("processed", processed).
			Msg("processed webhook deliveries")
	}
}

// IsRunning returns whether the dispatcher is running
func (d *Dispatcher) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}
