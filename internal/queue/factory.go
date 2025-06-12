package queue

import (
	"fmt"
	"log"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
)

// QueueProviderType определяет тип провайдера очереди
type QueueProviderType string

const (
	MemoryQueueType QueueProviderType = "memory"
	NATSQueueType   QueueProviderType = "nats"
)

// QueueFactory создает провайдеры очередей
type QueueFactory struct {
	config *config.Config
}

// NewQueueFactory создает новую фабрику очередей
func NewQueueFactory(cfg *config.Config) *QueueFactory {
	return &QueueFactory{
		config: cfg,
	}
}

// CreateQueueProvider создает провайдер очереди на основе конфигурации
func (f *QueueFactory) CreateQueueProvider() (QueueProvider, error) {
	queueType := QueueProviderType(f.config.QueueType)

	log.Printf("Creating queue provider of type: %s", queueType)

	switch queueType {
	case MemoryQueueType:
		return f.createMemoryQueueProvider()
	case NATSQueueType:
		return f.createNATSQueueProvider()
	default:
		return nil, fmt.Errorf("unsupported queue type: %s", queueType)
	}
}

// createMemoryQueueProvider создает провайдер для in-memory очереди
func (f *QueueFactory) createMemoryQueueProvider() (QueueProvider, error) {
	log.Printf("Creating memory queue with size: %d", f.config.QueueSize)
	return NewMemoryQueueAdapter(f.config.QueueSize), nil
}

// createNATSQueueProvider создает провайдер для NATS очереди
func (f *QueueFactory) createNATSQueueProvider() (QueueProvider, error) {
	log.Printf("Creating NATS queue with URL: %s", f.config.NATSURL)

	// Используем стандартный subject "messages" для всех сообщений
	adapter, err := NewNATSAdapter(f.config.NATSURL, "messages")
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS adapter: %w", err)
	}

	log.Printf("NATS queue provider created successfully")
	return adapter, nil
}

// ValidateQueueType проверяет, поддерживается ли данный тип очереди
func ValidateQueueType(queueType string) error {
	switch QueueProviderType(queueType) {
	case MemoryQueueType, NATSQueueType:
		return nil
	default:
		return fmt.Errorf("unsupported queue type: %s. Supported types: %s, %s",
			queueType, MemoryQueueType, NATSQueueType)
	}
}
