package queue

import (
	"errors"
	"fmt"
	"log"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
)

// ProviderType определяет тип провайдера очереди.
type ProviderType string

const (
	MemoryProviderType ProviderType = "memory"
	NATSProviderType   ProviderType = "nats"
)

// Factory создает провайдеры очередей.
type Factory struct {
	config *config.Config
}

// NewFactory создает новую фабрику очередей.
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		config: cfg,
	}
}

// revive:disable:ireturn
// CreateProvider создает провайдер очереди на основе конфигурации.
func (f *Factory) CreateProvider() (Provider, error) { //nolint:ireturn
	queueType := ProviderType(f.config.QueueType)

	log.Printf("Creating queue provider of type: %s", queueType)

	switch queueType {
	case MemoryProviderType:
		return f.createMemoryProvider()
	case NATSProviderType:
		return f.createNATSProvider()
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedQueueType, queueType)
	}
}

// revive:enable:ireturn

// createMemoryProvider создает провайдер для in-memory очереди.
// revive:disable:ireturn
func (f *Factory) createMemoryProvider() (Provider, error) { //nolint:ireturn
	log.Printf("Creating memory queue with size: %d", f.config.QueueSize)

	return NewMemoryAdapter(f.config.QueueSize), nil
}

// revive:enable:ireturn

// createNATSProvider создает провайдер для NATS очереди.
// revive:disable:ireturn
func (f *Factory) createNATSProvider() (Provider, error) { //nolint:ireturn
	log.Printf("Creating NATS queue with URL: %s", f.config.NATSURL)

	// Используем стандартный subject "messages" для всех сообщений.
	adapter, err := NewNATSAdapter(f.config.NATSURL, "messages")
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS adapter: %w", err)
	}

	log.Printf("NATS queue provider created successfully")

	return adapter, nil
}

// revive:enable:ireturn

// ValidateProviderType проверяет, поддерживается ли данный тип очереди.
func ValidateProviderType(queueType string) error {
	switch ProviderType(queueType) {
	case MemoryProviderType, NATSProviderType:
		return nil
	default:
		return fmt.Errorf("%w: %s. Supported types: %s, %s",
			ErrUnsupportedQueueType, queueType, MemoryProviderType, NATSProviderType)
	}
}

var ErrUnsupportedQueueType = errors.New("unsupported queue type")
