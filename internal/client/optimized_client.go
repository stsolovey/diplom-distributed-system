package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 10
	defaultIdleConnTimeout     = 90 * time.Second
	defaultDialTimeout         = 5 * time.Second
	defaultKeepAlive           = 30 * time.Second
	defaultResponseTimeout     = 10 * time.Second
	defaultTLSTimeout          = 10 * time.Second
	defaultClientTimeout       = 30 * time.Second
	defaultSimulationDelay     = 50 * time.Millisecond
	averagingFactor            = 2
)

// OptimizedClient - клиент с оптимизированным connection pooling.
type OptimizedClient struct {
	client *http.Client
	mu     sync.RWMutex
	stats  Stats
}

type Stats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
}

func NewOptimizedClient() *OptimizedClient {
	// Настройка transport с connection pooling
	transport := &http.Transport{
		// Connection pooling settings
		MaxIdleConns:        defaultMaxIdleConns,
		MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
		IdleConnTimeout:     defaultIdleConnTimeout,

		// Диaler настройки
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultKeepAlive,
		}).DialContext,

		// TCP настройки
		DisableKeepAlives:     false,
		DisableCompression:    false,
		MaxConnsPerHost:       0, // unlimited
		ResponseHeaderTimeout: defaultResponseTimeout,
		ExpectContinueTimeout: 1 * time.Second,

		// TLS настройки
		TLSHandshakeTimeout: defaultTLSTimeout,
		ForceAttemptHTTP2:   true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultClientTimeout,
	}

	return &OptimizedClient{
		client: client,
	}
}

func (c *OptimizedClient) SendMessage(ctx context.Context, _ *models.DataMessage) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		c.updateStats(latency)
	}()

	c.mu.Lock()
	c.stats.TotalRequests++
	c.mu.Unlock()

	// Симуляция отправки сообщения
	// В реальной реализации здесь был бы HTTP запрос
	select {
	case <-ctx.Done():
		c.mu.Lock()
		c.stats.FailedRequests++
		c.mu.Unlock()

		return fmt.Errorf("optimized client context cancelled: %w", ctx.Err())
	case <-time.After(defaultSimulationDelay): // симуляция сетевой задержки
		c.mu.Lock()
		c.stats.SuccessfulRequests++
		c.mu.Unlock()

		return nil
	}
}

func (c *OptimizedClient) updateStats(latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Простое скользящее среднее
	if c.stats.AverageLatency == 0 {
		c.stats.AverageLatency = latency
	} else {
		c.stats.AverageLatency = (c.stats.AverageLatency + latency) / averagingFactor
	}
}

func (c *OptimizedClient) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.stats
}

func (c *OptimizedClient) Close() error {
	c.client.CloseIdleConnections()

	return nil
}
