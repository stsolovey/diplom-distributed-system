package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Метрики для Ingest сервиса
var (
	IngestRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ingest_requests_total",
			Help: "Total number of ingest requests",
		},
		[]string{"status"},
	)

	IngestRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ingest_request_duration_seconds",
			Help:    "Duration of ingest requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	IngestMessagesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ingest_messages_processed_total",
			Help: "Total number of messages processed by ingest service",
		},
		[]string{"status"},
	)
)

// Метрики для Processor сервиса
var (
	ProcessorMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "processor_messages_total",
			Help: "Total number of messages processed",
		},
		[]string{"status"},
	)

	ProcessorWorkerPoolSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processor_worker_pool_size",
			Help: "Current size of worker pool",
		},
	)

	ProcessorQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processor_queue_size",
			Help: "Current queue size",
		},
	)

	ProcessorProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "processor_processing_duration_seconds",
			Help:    "Duration of message processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

// Метрики для API Gateway
var (
	GatewayRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of gateway requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	GatewayRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Duration of gateway requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	GatewayUpstreamRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_upstream_requests_total",
			Help: "Total number of upstream requests from gateway",
		},
		[]string{"service", "status"},
	)
)

// HTTP метрики (общие)
var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)
)
