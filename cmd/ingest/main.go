package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stsolovey/diplom-distributed-system/internal/client"
	"github.com/stsolovey/diplom-distributed-system/internal/config"
	"github.com/stsolovey/diplom-distributed-system/internal/metrics"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	shutdownTimeout = 5 * time.Second
	readTimeout     = 10 * time.Second
	writeTimeout    = 10 * time.Second
	requestTimeout  = 5 * time.Second
)

type IngestStats struct {
	TotalReceived atomic.Int64
	TotalSent     atomic.Int64
	TotalFailed   atomic.Int64
}

// GetStats возвращает текущую статистику в JSON-совместимом формате.
func (s *IngestStats) GetStats() map[string]int64 {
	return map[string]int64{
		"TotalReceived": s.TotalReceived.Load(),
		"TotalSent":     s.TotalSent.Load(),
		"TotalFailed":   s.TotalFailed.Load(),
	}
}

// IngestRequest представляет входящий запрос.
type IngestRequest struct {
	Source   string            `json:"source"`
	Data     string            `json:"data"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IngestResponse представляет ответ сервиса.
type IngestResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
}

type App struct {
	processorClient *client.ProcessorClient
	stats           *IngestStats
}

func main() {
	cfg := config.LoadConfig()

	app := &App{
		processorClient: client.NewProcessorClient(cfg.ProcessorURL),
		stats:           &IngestStats{},
	}

	// HTTP сервер
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", app.handleIngest)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/stats", app.handleStats)
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         ":" + cfg.IngestPort,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down ingest service...")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)

		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()

	log.Printf("Ingest service starting on port %s", cfg.IngestPort)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}

// handleIngest обрабатывает входящие данные.
func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodPost {
		metrics.IngestRequestsTotal.WithLabelValues("method_not_allowed").Inc()
		metrics.IngestRequestDuration.WithLabelValues("method_not_allowed").Observe(time.Since(start).Seconds())
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.IngestRequestsTotal.WithLabelValues("bad_request").Inc()
		metrics.IngestRequestDuration.WithLabelValues("bad_request").Observe(time.Since(start).Seconds())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	app.stats.TotalReceived.Add(1)
	metrics.IngestMessagesProcessed.WithLabelValues("received").Inc()

	// Создаем сообщение
	msg := &models.DataMessage{
		Id:        uuid.New().String(),
		Timestamp: time.Now().Unix(),
		Source:    req.Source,
		Payload:   []byte(req.Data),
		Metadata:  req.Metadata,
	}

	// Отправляем в Processor
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := app.processorClient.SendMessage(ctx, msg); err != nil {
		app.stats.TotalFailed.Add(1)
		metrics.IngestRequestsTotal.WithLabelValues("service_unavailable").Inc()
		metrics.IngestRequestDuration.WithLabelValues("service_unavailable").Observe(time.Since(start).Seconds())
		metrics.IngestMessagesProcessed.WithLabelValues("failed").Inc()
		log.Printf("Failed to send message to processor: %v", err)
		http.Error(w, "Failed to process message", http.StatusServiceUnavailable)
		return
	}

	app.stats.TotalSent.Add(1)
	metrics.IngestRequestsTotal.WithLabelValues("success").Inc()
	metrics.IngestRequestDuration.WithLabelValues("success").Observe(time.Since(start).Seconds())
	metrics.IngestMessagesProcessed.WithLabelValues("sent").Inc()

	// Отправляем ответ
	resp := IngestResponse{
		MessageID: msg.GetId(),
		Status:    "accepted",
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleHealth проверка здоровья сервиса.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]bool{"healthy": true}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// handleStats возвращает статистику.
func (app *App) handleStats(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(app.stats.GetStats()); err != nil {
		log.Printf("Failed to encode stats response: %v", err)
	}
}
