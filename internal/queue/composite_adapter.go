package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"golang.org/x/sync/errgroup"
)

var ErrNoProvidersConfigured = errors.New("no providers configured")

// CompositeStrategy defines behavior on errors.
type CompositeStrategy int

const (
	// FailFast - if at least one write fails, return error.
	FailFast CompositeStrategy = iota
	// BestEffort - try to write everywhere, ignore errors.
	BestEffort
)

// CompositeAdapter allows writing to multiple queues simultaneously.
type CompositeAdapter struct {
	providers []Provider
	strategy  CompositeStrategy
	mu        sync.RWMutex
}

// NewCompositeAdapter creates a new composite adapter.
func NewCompositeAdapter(providers []Provider, strategy CompositeStrategy) *CompositeAdapter {
	return &CompositeAdapter{
		providers: providers,
		strategy:  strategy,
	}
}

// Publish sends message to all configured providers.
func (c *CompositeAdapter) Publish(ctx context.Context, msg *models.DataMessage) error {
	c.mu.RLock()
	providers := make([]Provider, len(c.providers))
	copy(providers, c.providers)
	c.mu.RUnlock()

	if len(providers) == 0 {
		return ErrNoProvidersConfigured
	}

	if c.strategy == FailFast {
		// Use errgroup for parallel writing with error handling.
		g, groupCtx := errgroup.WithContext(ctx)

		for _, provider := range providers {
			g.Go(func() error {
				return provider.Publish(groupCtx, msg)
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("failed to publish to all providers: %w", err)
		}

		return nil
	}

	// BestEffort strategy - try all providers, log errors but don't return them.
	var wg sync.WaitGroup

	errorCount := 0

	var errorMu sync.Mutex

	for _, provider := range providers {
		wg.Add(1)

		go func(p Provider) {
			defer wg.Done()

			if err := p.Publish(ctx, msg); err != nil {
				errorMu.Lock()
				errorCount++

				log.Printf("CompositeAdapter: failed to publish to provider: %v", err)
				errorMu.Unlock()
			}
		}(provider)
	}

	wg.Wait()

	if errorCount > 0 {
		log.Printf("CompositeAdapter: %d/%d providers failed during best-effort publish",
			errorCount, len(providers))
	}

	return nil
}

// Subscribe returns message channel from the first provider.
// In the future, this could merge channels from all providers.
func (c *CompositeAdapter) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.providers) == 0 {
		return nil, ErrNoProvidersConfigured
	}

	// For the first version - use channel from the first provider.
	msgChan, err := c.providers[0].Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to first provider: %w", err)
	}

	return msgChan, nil
}

// Stats returns aggregated statistics from all providers.
func (c *CompositeAdapter) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var aggregated Stats

	for _, provider := range c.providers {
		stats := provider.Stats()
		aggregated.TotalEnqueued += stats.TotalEnqueued
		aggregated.TotalDequeued += stats.TotalDequeued
		aggregated.CurrentSize += stats.CurrentSize
	}

	return aggregated
}

// Close closes all configured providers.
func (c *CompositeAdapter) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errorsList []error

	for _, provider := range c.providers {
		if err := provider.Close(); err != nil {
			errorsList = append(errorsList, err)
		}
	}

	if len(errorsList) > 0 {
		log.Printf("CompositeAdapter: %d errors during close", len(errorsList))

		// Return first error.
		return fmt.Errorf("failed to close providers: %w", errorsList[0])
	}

	return nil
}
