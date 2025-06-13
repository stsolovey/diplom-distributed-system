package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/stsolovey/diplom-distributed-system/internal/processor"
	"github.com/stsolovey/diplom-distributed-system/internal/queue"
)

const (
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 10 * time.Second
	serverReadHeaderTimeout = 5 * time.Second
)

type App struct {
	queueProvider queue.Provider
	pool          *processor.WorkerPool
}

func main() { //nolint:funlen
	cfg := config.LoadConfig()

	// Создаем провайдер очереди через фабрику.
	factory := queue.NewFactory(cfg)

	queueProvider, err := factory.CreateProvider()
	if err != nil {
		log.Printf("Failed to create queue provider: %v", err)
		os.Exit(1)
	}
	defer queueProvider.Close()

	// Создаем worker pool с унифицированным интерфейсом.
	pool := processor.NewWorkerPool(cfg.ProcessorWorkers, queueProvider)

	app := &App{
		queueProvider: queueProvider,
		pool:          pool,
	}

	// Контекст для graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())

	// Запускаем воркеры.
	if err := pool.Start(ctx); err != nil {
		if closeErr := queueProvider.Close(); closeErr != nil {
			log.Printf("Error closing queue provider: %v", closeErr)
		}

		log.Printf("Failed to start worker pool: %v", err)
		os.Exit(1) //nolint:gocritic
	}

	// HTTP сервер.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.handleHealth)
	mux.HandleFunc("/stats", app.handleStats)
	mux.HandleFunc("/enqueue", app.handleEnqueue) // Новый эндпоинт для приема сообщений.

	srv := &http.Server{
		Addr:              ":" + cfg.ProcessorPort,
		Handler:           mux,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}

	// Обработка результатов в отдельной горутине.
	go func() {
		for result := range pool.Results() {
			log.Printf("Processed message %s: success=%v", result.GetMessageId(), result.GetSuccess())
		}
	}()

	// Graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down processor service...")
		cancel()
		pool.Stop()

		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()

	log.Printf(
		"Processor service starting on port %s with %d workers using %s queue",
		cfg.ProcessorPort,
		cfg.ProcessorWorkers,
		cfg.QueueType,
	)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}

// handleEnqueue принимает сообщения от Ingest сервиса.
func (a *App) handleEnqueue(w http.ResponseWriter, r *http.Request) {
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
	if err := a.queueProvider.Publish(ctx, &msg); err != nil {
		if errors.Is(err, queue.ErrQueueFull) {
			http.Error(w, "Queue is full", http.StatusServiceUnavailable)
		} else {
			log.Printf("Failed to enqueue message: %v", err)
			http.Error(w, "Failed to enqueue message", http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleHealth проверка здоровья сервиса.
func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]bool{"healthy": true}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// handleStats возвращает статистику.
func (a *App) handleStats(w http.ResponseWriter, _ *http.Request) {
	poolStats := a.pool.GetStats()
	queueStats := a.queueProvider.Stats()

	stats := map[string]interface{}{
		"queue": queueStats,
		"pool":  poolStats,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Failed to encode stats response: %v", err)
	}
}
