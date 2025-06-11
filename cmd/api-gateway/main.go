package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
)

type ServiceInfo struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Healthy  bool   `json:"healthy"`
}

var services = []ServiceInfo{
	{Name: "ingest", Endpoint: getIngestURL()},
	{Name: "processor", Endpoint: getProcessorURL()},
}

// httpClientWithTimeout - клиент с таймаутом для health/stats запросов
var httpClientWithTimeout = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	},
}

// getIngestURL возвращает URL для Ingest сервиса
func getIngestURL() string {
	if url := os.Getenv("INGEST_URL"); url != "" {
		return url
	}
	return "http://localhost:8081" // fallback для локальной разработки
}

// getProcessorURL возвращает URL для Processor сервиса
func getProcessorURL() string {
	if url := os.Getenv("PROCESSOR_URL"); url != "" {
		return url
	}
	return "http://localhost:8082" // fallback для локальной разработки
}

func main() {
	cfg := config.LoadConfig()

	mux := http.NewServeMux()

	// Основные эндпоинты
	mux.HandleFunc("/api/v1/ingest", proxyToIngest)
	mux.HandleFunc("/api/v1/status", handleSystemStatus)
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("Shutting down API Gateway...")
		srv.Shutdown(context.Background())
	}()

	log.Printf("API Gateway starting on port %s", cfg.APIPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// proxyToIngest проксирует запросы к Ingest сервису
func proxyToIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Создаем новый запрос к Ingest сервису
	ingestURL := getIngestURL() + "/ingest"

	// Копируем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Отправляем запрос
	resp, err := http.Post(ingestURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to proxy to ingest: %v", err)
		http.Error(w, "Failed to proxy request", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Копируем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleSystemStatus возвращает статус всех сервисов
func handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := make(map[string]interface{})

	for i, svc := range services {
		// Проверяем здоровье каждого сервиса с таймаутом
		healthURL := fmt.Sprintf("%s/health", svc.Endpoint)

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		healthReq, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
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

		// Получаем статистику с таймаутом
		statsURL := fmt.Sprintf("%s/stats", svc.Endpoint)

		ctx, cancel = context.WithTimeout(r.Context(), 2*time.Second)
		statsReq, err := http.NewRequestWithContext(ctx, "GET", statsURL, nil)
		cancel()

		if err == nil {
			statsResp, err := httpClientWithTimeout.Do(statsReq)
			if err == nil {
				var stats interface{}
				json.NewDecoder(statsResp.Body).Decode(&stats)
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
	json.NewEncoder(w).Encode(status)
}

// handleHealth проверка здоровья API Gateway
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
}

// loggingMiddleware логирует все запросы
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
