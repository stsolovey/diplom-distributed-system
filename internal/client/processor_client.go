package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	defaultTimeout      = 5 * time.Second
	maxIdleConns        = 100
	maxIdleConnsPerHost = 10
	idleConnTimeout     = 90 * time.Second
	enqueueEndpoint     = "/enqueue"
	contentTypeHeader   = "Content-Type"
	jsonContentType     = "application/json"
)

var (
	// ErrUnexpectedStatusCode - ошибка при получении неожиданного HTTP статуса.
	ErrUnexpectedStatusCode = errors.New("unexpected status code")

	// defaultHTTPClient - переиспользуемый HTTP клиент с connection pooling.
	//nolint:gochecknoglobals // reuse of HTTP client for connection pooling is intentional.
	defaultHTTPClient = &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        maxIdleConns,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeout,
			DisableCompression:  false,
		},
	}
)

// ProcessorClient - клиент для отправки сообщений в Processor.
type ProcessorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewProcessorClient создает новый клиент с переиспользуемым HTTP client.
func NewProcessorClient(baseURL string) *ProcessorClient {
	return &ProcessorClient{
		baseURL:    baseURL,
		httpClient: defaultHTTPClient,
	}
}

// SendMessage отправляет сообщение в Processor для обработки.
func (c *ProcessorClient) SendMessage(ctx context.Context, msg *models.DataMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+enqueueEndpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(contentTypeHeader, jsonContentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	return nil
}
