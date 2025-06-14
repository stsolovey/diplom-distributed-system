package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"sort"
	"sync"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	tracedClientTimeout = 30 * time.Second
	traceBufferSize     = 1000
	simulationDelayMs   = 100
	maxTracesLimit      = 1000
	percentile95        = 0.95
	percentile99        = 0.99
)

// TracedClient - клиент с детальным трейсингом запросов.
type TracedClient struct {
	client *http.Client
	mu     sync.RWMutex
	traces []RequestTrace
}

type RequestTrace struct {
	RequestID    string
	DNSLookup    time.Duration
	TCPConnect   time.Duration
	TLSHandshake time.Duration
	FirstByte    time.Duration
	TotalTime    time.Duration
	Timestamp    time.Time
}

type LatencyMetrics struct {
	AverageDNS       time.Duration
	AverageTCP       time.Duration
	AverageTLS       time.Duration
	AverageFirstByte time.Duration
	AverageTotal     time.Duration
	P95Total         time.Duration
	P99Total         time.Duration
}

func NewTracedClient() *TracedClient {
	client := &http.Client{
		Timeout: tracedClientTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // testing/development only
		},
	}

	return &TracedClient{
		client: client,
		traces: make([]RequestTrace, 0, traceBufferSize),
	}
}

func (c *TracedClient) SendMessage(ctx context.Context, msg *models.DataMessage) error {
	trace := RequestTrace{
		RequestID: msg.GetId(),
		Timestamp: time.Now(),
	}

	var (
		dnsStart       time.Time
		connectStart   time.Time
		tlsStart       time.Time
		firstByteStart time.Time
	)

	// Создаем traced context
	clientTrace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			trace.DNSLookup = time.Since(dnsStart)
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			trace.TCPConnect = time.Since(connectStart)
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			trace.TLSHandshake = time.Since(tlsStart)
		},
		GotFirstResponseByte: func() {
			trace.FirstByte = time.Since(firstByteStart)
		},
	}

	ctx = httptrace.WithClientTrace(ctx, clientTrace)
	requestStart := time.Now()
	firstByteStart = requestStart

	// Симуляция HTTP запроса
	select {
	case <-ctx.Done():
		return fmt.Errorf("traced client context cancelled: %w", ctx.Err())
	case <-time.After(time.Millisecond * simulationDelayMs):
		// симуляция успешного запроса
	} //nolint:wsl // complex case with select and assignments
	trace.TotalTime = time.Since(requestStart) //nolint:wsl // assignment after select block
	c.recordTrace(trace)

	return nil
}

func (c *TracedClient) recordTrace(trace RequestTrace) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.traces = append(c.traces, trace)

	// Ограничиваем размер буфера
	if len(c.traces) > maxTracesLimit {
		c.traces = c.traces[1:]
	}
}

func (c *TracedClient) GetLatencyMetrics() LatencyMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.traces) == 0 {
		return LatencyMetrics{}
	}

	var (
		totalDNS       time.Duration
		totalTCP       time.Duration
		totalTLS       time.Duration
		totalFirstByte time.Duration
		totalTime      time.Duration
	)

	allTotal := make([]time.Duration, 0, len(c.traces))

	for _, trace := range c.traces {
		totalDNS += trace.DNSLookup
		totalTCP += trace.TCPConnect
		totalTLS += trace.TLSHandshake
		totalFirstByte += trace.FirstByte
		totalTime += trace.TotalTime
		allTotal = append(allTotal, trace.TotalTime)
	}

	count := len(c.traces)
	metrics := LatencyMetrics{
		AverageDNS:       totalDNS / time.Duration(count),
		AverageTCP:       totalTCP / time.Duration(count),
		AverageTLS:       totalTLS / time.Duration(count),
		AverageFirstByte: totalFirstByte / time.Duration(count),
		AverageTotal:     totalTime / time.Duration(count),
	}

	// Вычисляем перцентили
	if count > 0 {
		// Простая сортировка для перцентилей
		sort.Slice(allTotal, func(i, j int) bool {
			return allTotal[i] < allTotal[j]
		})

		p95Index := int(float64(count) * percentile95)
		p99Index := int(float64(count) * percentile99)

		if p95Index < count {
			metrics.P95Total = allTotal[p95Index]
		}

		if p99Index < count {
			metrics.P99Total = allTotal[p99Index]
		}
	}

	return metrics
}

func (c *TracedClient) GetTraces() []RequestTrace {
	c.mu.RLock()
	defer c.mu.RUnlock()

	traces := make([]RequestTrace, len(c.traces))
	copy(traces, c.traces)

	return traces
}

func (c *TracedClient) ClearTraces() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.traces = c.traces[:0]
}
