package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	grpcservice "github.com/stsolovey/diplom-distributed-system/internal/grpc"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	grpcTimeoutSeconds   = 10
	gatewayReadTimeout   = 15 * time.Second
	gatewayWriteTimeout  = 15 * time.Second
	gatewayIdleTimeout   = 60 * time.Second
	maxConcurrentStreams = 1000
	http2IdleTimeout     = 300 * time.Second
)

type HTTP2Gateway struct {
	grpcClient grpcservice.IngestServiceClient
	server     *http.Server
}

type IngestRequest struct {
	Source   string            `json:"source"`
	Data     string            `json:"data"`
	Metadata map[string]string `json:"metadata"`
}

type IngestResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func NewHTTP2Gateway(grpcAddr string) *HTTP2Gateway {
	// Подключение к gRPC сервису
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to gRPC: %v", err)

		return nil
	}

	client := grpcservice.NewIngestServiceClient(conn)

	return &HTTP2Gateway{
		grpcClient: client,
	}
}

func (g *HTTP2Gateway) ingestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.sendError(w, "Failed to read request body", http.StatusBadRequest)

		return
	}
	defer r.Body.Close()

	// Парсим JSON
	var req IngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		g.sendError(w, "Invalid JSON", http.StatusBadRequest)

		return
	}

	// Преобразуем в gRPC запрос
	grpcReq := &grpcservice.IngestRequest{
		Source:   req.Source,
		Data:     []byte(req.Data),
		Metadata: req.Metadata,
	}

	// Вызываем gRPC сервис с timeout
	ctx, cancel := context.WithTimeout(r.Context(), grpcTimeoutSeconds*time.Second)
	defer cancel()

	resp, err := g.grpcClient.Ingest(ctx, grpcReq)
	if err != nil {
		g.sendError(w, err.Error(), http.StatusInternalServerError)

		return
	}

	// Отправляем ответ
	response := IngestResponse{
		MessageID: resp.GetMessageId(),
		Status:    resp.GetStatus(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (g *HTTP2Gateway) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(IngestResponse{
		Status: "error",
		Error:  message,
	}); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}

func (g *HTTP2Gateway) Start(addr string, tlsCert, tlsKey string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", g.ingestHandler)

	// Настройка HTTP/2 сервера
	g.server = &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadTimeout:  gatewayReadTimeout,
		WriteTimeout: gatewayWriteTimeout,
		IdleTimeout:  gatewayIdleTimeout,
	}

	// Настраиваем HTTP/2
	if err := http2.ConfigureServer(g.server, &http2.Server{
		MaxConcurrentStreams: maxConcurrentStreams,
		IdleTimeout:          http2IdleTimeout,
	}); err != nil {
		return fmt.Errorf("failed to configure HTTP/2 server: %w", err)
	}

	log.Printf("Starting HTTP/2 Gateway on %s", addr)

	return fmt.Errorf("HTTP/2 gateway failed: %w", g.server.ListenAndServeTLS(tlsCert, tlsKey))
}

func (g *HTTP2Gateway) Stop(ctx context.Context) error {
	if g.server != nil {
		return fmt.Errorf("failed to shutdown server: %w", g.server.Shutdown(ctx))
	}

	return nil
}
