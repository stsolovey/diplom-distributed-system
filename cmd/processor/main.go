package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/stsolovey/diplom-distributed-system/internal/processor"
	"github.com/stsolovey/diplom-distributed-system/internal/queue"
)

var (
	messageQueue *queue.MemoryQueue
	pool         *processor.WorkerPool
)

func main() {
	cfg := config.LoadConfig()

	// Создаем очередь
	messageQueue = queue.NewMemoryQueue(cfg.QueueSize)

	// Создаем worker pool
	pool = processor.NewWorkerPool(cfg.ProcessorWorkers, messageQueue)

	// Контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем воркеры
	pool.Start(ctx)

	// HTTP сервер
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/stats", handleStats)
	mux.HandleFunc("/enqueue", handleEnqueue) // Новый эндпоинт для приема сообщений

	srv := &http.Server{
		Addr:    ":" + cfg.ProcessorPort,
		Handler: mux,
	}

	// Обработка результатов в отдельной горутине
	go func() {
		for result := range pool.Results() {
			// В фазе 1 просто логируем результаты
			log.Printf("Processed message %s: success=%v",
				result.MessageId, result.Success)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down processor service...")
		cancel()
		pool.Stop()
		srv.Shutdown(context.Background())
	}()

	log.Printf("Processor service starting on port %s with %d workers",
		cfg.ProcessorPort, cfg.ProcessorWorkers)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleEnqueue принимает сообщения от Ingest сервиса
func handleEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg models.DataMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := messageQueue.Enqueue(ctx, &msg); err != nil {
		if err == queue.ErrQueueFull {
			http.Error(w, "Queue is full", http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Failed to enqueue message", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleHealth проверка здоровья сервиса
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
}

// handleStats возвращает статистику
func handleStats(w http.ResponseWriter, r *http.Request) {
	poolStats := pool.GetStats()
	queueStats := messageQueue.Stats()

	stats := map[string]interface{}{
		"queue": queueStats,
		"pool":  poolStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
