package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// defaultHTTPClient - переиспользуемый HTTP клиент с connection pooling
var defaultHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	},
}

// ProcessorClient - клиент для отправки сообщений в Processor
type ProcessorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewProcessorClient создает новый клиент с переиспользуемым HTTP client
func NewProcessorClient(baseURL string) *ProcessorClient {
	return &ProcessorClient{
		baseURL:    baseURL,
		httpClient: defaultHTTPClient,
	}
}

// SendMessage отправляет сообщение в Processor для обработки
func (c *ProcessorClient) SendMessage(ctx context.Context, msg *models.DataMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/enqueue", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
