package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkAPIGateway_Ingest(b *testing.B) {
	// Мокаем Ingest сервис
	ingestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"messageId": "test-123", "status": "accepted"})
	}))
	defer ingestServer.Close()

	// Устанавливаем URL мока
	b.Setenv("INGEST_URL", ingestServer.URL)

	// Создаем тестовый сервер API Gateway
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", proxyToIngest)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Подготавливаем payload
	payload := []byte(`{"source":"bench","data":"test data"}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := http.Post(
			server.URL+"/api/v1/ingest",
			"application/json",
			bytes.NewReader(payload),
		)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkAPIGateway_SystemStatus(b *testing.B) {
	// Мокаем сервисы
	ingestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/stats":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total_processed": 1000,
				"current_load":    50,
			})
		}
	}))
	defer ingestServer.Close()

	processorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/stats":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workers_active": 4,
				"queue_size":     10,
			})
		}
	}))
	defer processorServer.Close()

	// Устанавливаем URLs моков
	b.Setenv("INGEST_URL", ingestServer.URL)
	b.Setenv("PROCESSOR_URL", processorServer.URL)

	// Создаем тестовый сервер
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", handleSystemStatus)

	server := httptest.NewServer(mux)
	defer server.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := http.Get(server.URL + "/api/v1/status")
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
