package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
)

const (
	clientTimeout      = 3 * time.Second
	idleConnTimeout    = 30 * time.Second
	maxIdleConnections = 10
	serverReadTimeout  = 10 * time.Second
	serverWriteTimeout = 10 * time.Second
	healthTimeout      = 2 * time.Second
)

// ServiceInfo represents a backend service handled by the gateway.
type ServiceInfo struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Healthy  bool   `json:"healthy"`
}

//nolint:gochecknoglobals // dependency injection would create unnecessary boilerplate here.
var services = []ServiceInfo{
	{Name: "ingest", Endpoint: getIngestURL()},
	{Name: "processor", Endpoint: getProcessorURL()},
}

// httpClientWithTimeout is an HTTP client tuned for short health/stats requests.
//
//nolint:gochecknoglobals // global reuse is intentional to leverage connection pooling.
var httpClientWithTimeout = &http.Client{
	Timeout: clientTimeout,
	Transport: &http.Transport{
		MaxIdleConns:    maxIdleConnections,
		IdleConnTimeout: idleConnTimeout,
	},
}

// getIngestURL возвращает URL для Ingest сервиса.
func getIngestURL() string {
	if url := os.Getenv("INGEST_URL"); url != "" {
		return url
	}

	return "http://localhost:8081" // fallback для локальной разработки.
}

// getProcessorURL возвращает URL для Processor сервиса.
func getProcessorURL() string {
	if url := os.Getenv("PROCESSOR_URL"); url != "" {
		return url
	}

	return "http://localhost:8082" // fallback для локальной разработки.
}

func main() {
	cfg := config.LoadConfig()

	mux := http.NewServeMux()

	// Основные эндпоинты.
	mux.HandleFunc("/api/v1/ingest", proxyToIngest)
	mux.HandleFunc("/api/v1/status", handleSystemStatus)
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}

	// Graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down API Gateway...")

		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()

	log.Printf("API Gateway starting on port %s", cfg.APIPort)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}

// proxyToIngest проксирует запросы к Ingest сервису.
func proxyToIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Создаем новый запрос к Ingest сервису.
	ingestURL := getIngestURL() + "/ingest"

	// Копируем тело запроса.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)

		return
	}
	defer r.Body.Close()

	// Отправляем запрос.
	ingestReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ingestURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to create request to ingest: %v", err)
		http.Error(w, "Failed to proxy request", http.StatusInternalServerError)

		return
	}

	ingestReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClientWithTimeout.Do(ingestReq)
	if err != nil {
		log.Printf("Failed to proxy to ingest: %v", err)
		http.Error(w, "Failed to proxy request", http.StatusServiceUnavailable)

		return
	}
	defer resp.Body.Close()

	// Копируем ответ.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	if _, err = io.Copy(w, resp.Body); err != nil {
		log.Printf("Failed to copy response body: %v", err)
	}
}

// handleSystemStatus возвращает статус всех сервисов.
func handleSystemStatus(w http.ResponseWriter, r *http.Request) { //nolint:funlen
	status := make(map[string]interface{})

	for i, svc := range services {
		// Проверяем здоровье каждого сервиса с таймаутом.
		healthURL := svc.Endpoint + "/health"

		ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)

		cancel()

		if err != nil {
			services[i].Healthy = false
		} else {
			resp, err := httpClientWithTimeout.Do(healthReq)
			if err != nil {
				services[i].Healthy = false
			} else {
				services[i].Healthy = resp.StatusCode == http.StatusOK
				resp.Body.Close()
			}
		}

		// Получаем статистику с таймаутом.
		statsURL := svc.Endpoint + "/stats"

		ctx, cancel = context.WithTimeout(r.Context(), healthTimeout)
		statsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, statsURL, nil)

		cancel()

		if err == nil { //nolint:nestif
			statsResp, err := httpClientWithTimeout.Do(statsReq)
			if err == nil {
				var stats interface{}
				if decodeErr := json.NewDecoder(statsResp.Body).Decode(&stats); decodeErr != nil {
					log.Printf("Failed to decode stats response: %v", decodeErr)
				}

				status[svc.Name] = map[string]interface{}{
					"healthy": services[i].Healthy,
					"stats":   stats,
				}

				statsResp.Body.Close()
			} else {
				status[svc.Name] = map[string]interface{}{
					"healthy": services[i].Healthy,
					"stats":   nil,
					"error":   err.Error(),
				}
			}
		} else {
			status[svc.Name] = map[string]interface{}{
				"healthy": services[i].Healthy,
				"stats":   nil,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Failed to encode status response: %v", err)
	}
}

// handleHealth проверка здоровья API Gateway.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]bool{"healthy": true}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// loggingMiddleware логирует все запросы.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Обертка для перехвата статус кода
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
