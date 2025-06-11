package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/stsolovey/diplom-distributed-system/internal/client"
	"github.com/stsolovey/diplom-distributed-system/internal/config"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

var (
	processorClient *client.ProcessorClient
	stats           IngestStats
)

type IngestStats struct {
	TotalReceived atomic.Int64
	TotalSent     atomic.Int64
	TotalFailed   atomic.Int64
}

// GetStats возвращает текущую статистику в JSON-совместимом формате
func (s *IngestStats) GetStats() map[string]int64 {
	return map[string]int64{
		"TotalReceived": s.TotalReceived.Load(),
		"TotalSent":     s.TotalSent.Load(),
		"TotalFailed":   s.TotalFailed.Load(),
	}
}

// IngestRequest представляет входящий запрос
type IngestRequest struct {
	Source   string            `json:"source"`
	Data     string            `json:"data"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IngestResponse представляет ответ сервиса
type IngestResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

func main() {
	cfg := config.LoadConfig()

	// Создаем клиент для отправки в Processor
	processorClient = client.NewProcessorClient(cfg.ProcessorURL)

	// HTTP сервер
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", handleIngest)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/stats", handleStats)

	srv := &http.Server{
		Addr:         ":" + cfg.IngestPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down ingest service...")
		srv.Shutdown(context.Background())
	}()

	log.Printf("Ingest service starting on port %s", cfg.IngestPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleIngest обрабатывает входящие данные
func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	stats.TotalReceived.Add(1)

	// Создаем сообщение
	msg := &models.DataMessage{
		Id:        uuid.New().String(),
		Timestamp: time.Now().Unix(),
		Source:    req.Source,
		Payload:   []byte(req.Data),
		Metadata:  req.Metadata,
	}

	// Отправляем в Processor
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := processorClient.SendMessage(ctx, msg); err != nil {
		stats.TotalFailed.Add(1)
		log.Printf("Failed to send message to processor: %v", err)
		http.Error(w, "Failed to process message", http.StatusServiceUnavailable)
		return
	}

	stats.TotalSent.Add(1)

	// Отправляем ответ
	resp := IngestResponse{
		MessageID: msg.Id,
		Status:    "accepted",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleHealth проверка здоровья сервиса
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
}

// handleStats возвращает статистику
func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.GetStats())
}
