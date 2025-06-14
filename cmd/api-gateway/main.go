package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	clientTimeout          = 3 * time.Second
	idleConnTimeout        = 30 * time.Second
	maxIdleConnections     = 10
	serverReadTimeout      = 10 * time.Second
	serverWriteTimeout     = 10 * time.Second
	healthTimeout          = 2 * time.Second
	shutdownTimeoutSeconds = 5
	rsaKeyBits             = 2048
	localhostIPv4          = 127
	certValidityDays       = 365
)

// ServiceInfo represents a backend service handled by the gateway.
type ServiceInfo struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Healthy  bool   `json:"healthy"`
}

// infrastructure code for future use
//
//nolint:gochecknoglobals,unused // dependency injection would create unnecessary boilerplate here
var services = []ServiceInfo{
	{Name: "ingest", Endpoint: getIngestURL()},
	{Name: "processor", Endpoint: getProcessorURL()},
}

// httpClientWithTimeout is an HTTP client tuned for short health/stats requests.
//
// infrastructure code for future use
//
//nolint:gochecknoglobals,unused // global reuse is intentional to leverage connection pooling
var httpClientWithTimeout = &http.Client{
	Timeout: clientTimeout,
	Transport: &http.Transport{
		MaxIdleConns:    maxIdleConnections,
		IdleConnTimeout: idleConnTimeout,
	},
}

// getIngestURL возвращает URL для Ingest сервиса.
//
//nolint:unused // infrastructure code for future use
func getIngestURL() string {
	if url := os.Getenv("INGEST_URL"); url != "" {
		return url
	}

	return "http://localhost:8081" // fallback для локальной разработки.
}

// getProcessorURL возвращает URL для Processor сервиса.
//
//nolint:unused // infrastructure code for future use
func getProcessorURL() string {
	if url := os.Getenv("PROCESSOR_URL"); url != "" {
		return url
	}

	return "http://localhost:8082" // fallback для локальной разработки.
}

func main() {
	// Создаем самоподписанные сертификаты для демо
	cert, key, err := generateSelfSignedCert()
	if err != nil {
		panic(fmt.Errorf("failed to generate certificates: %w", err))
	}

	// Сохраняем сертификаты во временные файлы
	certFile, keyFile, cleanup, err := saveCerts(cert, key)
	if err != nil {
		panic(fmt.Errorf("failed to save certificates: %w", err))
	}
	defer cleanup()

	// Создаем HTTP/2 Gateway
	gateway := NewHTTP2Gateway("localhost:50052") // gRPC server address
	if gateway == nil {
		panic("failed to create gateway")
	}

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = ctx // используется для контекста приложения

	// Обработка сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down gateway...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeoutSeconds*time.Second)
		defer shutdownCancel()

		if err := gateway.Stop(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Запускаем HTTP/2 gateway
	log.Println("Starting HTTP/2 Gateway on https://localhost:8443")
	log.Println("Testing: curl -k -X POST https://localhost:8443/ingest -d '{\"source\":\"test\",\"data\":\"hello\"}'")

	if err := gateway.Start(":8443", certFile, keyFile); err != nil {
		log.Printf("Gateway error: %v", err)
	}
}

func generateSelfSignedCert() ([]byte, []byte, error) {
	// Генерируем приватный ключ
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Создаем шаблон сертификата
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(certValidityDays * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(localhostIPv4, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	// Создаем сертификат
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Кодируем сертификат в PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Кодируем ключ в PEM
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certPEM, keyPEM, nil
}

func saveCerts(cert, key []byte) (string, string, func(), error) {
	// Создаем временные файлы
	certF, err := os.CreateTemp("", "cert-*.pem")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create cert temp file: %w", err)
	}

	keyF, err := os.CreateTemp("", "key-*.pem")
	if err != nil {
		certF.Close()
		os.Remove(certF.Name())

		return "", "", nil, fmt.Errorf("failed to create key temp file: %w", err)
	}

	// Записываем сертификаты
	if _, err := certF.Write(cert); err != nil {
		certF.Close()
		keyF.Close()
		os.Remove(certF.Name())
		os.Remove(keyF.Name())

		return "", "", nil, fmt.Errorf("failed to write cert: %w", err)
	}

	if _, err := keyF.Write(key); err != nil {
		certF.Close()
		keyF.Close()
		os.Remove(certF.Name())
		os.Remove(keyF.Name())

		return "", "", nil, fmt.Errorf("failed to write key: %w", err)
	}

	certF.Close()
	keyF.Close()

	cleanup := func() {
		os.Remove(certF.Name())
		os.Remove(keyF.Name())
	}

	return certF.Name(), keyF.Name(), cleanup, nil
}

// proxyToIngest проксирует запросы к Ingest сервису.
//
//nolint:unused // infrastructure code for future use
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
//
//nolint:unused,funlen // infrastructure code for future use
func handleSystemStatus(w http.ResponseWriter, r *http.Request) {
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
//
//nolint:unused // infrastructure code for future use
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]bool{"healthy": true}); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// loggingMiddleware логирует все запросы.
//
//nolint:unused // infrastructure code for future use
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Обертка для перехвата статус кода
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

//nolint:unused // infrastructure code for future use
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

//nolint:unused // infrastructure code for future use
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
