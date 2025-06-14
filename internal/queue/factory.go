package queue

import (
	"errors"
	"fmt"
	"log"

	"github.com/stsolovey/diplom-distributed-system/internal/config"
)

var (
	ErrUnsupportedQueueType           = errors.New("unsupported queue type")
	ErrNoCompositeProvidersConfigured = errors.New("no composite providers configured")
	ErrUnsupportedCompositeStrategy   = errors.New("unsupported composite strategy")
)

// ProviderType defines the type of queue provider.
type ProviderType string

const (
	MemoryProviderType    ProviderType = "memory"
	NATSProviderType      ProviderType = "nats"
	KafkaProviderType     ProviderType = "kafka"
	CompositeProviderType ProviderType = "composite"
)

// Factory creates queue providers.
type Factory struct {
	config *config.Config
}

// NewFactory creates a new queue factory.
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		config: cfg,
	}
}

// CreateProvider creates a queue provider based on configuration.
func (f *Factory) CreateProvider() (Provider, error) { //nolint:ireturn // factory pattern
	queueType := ProviderType(f.config.QueueType)

	log.Printf("Creating queue provider of type: %s", queueType)

	switch queueType {
	case MemoryProviderType:
		return f.createMemoryProvider()
	case NATSProviderType:
		return f.createNATSProvider()
	case KafkaProviderType:
		return f.createKafkaProvider()
	case CompositeProviderType:
		return f.createCompositeProvider()
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedQueueType, queueType)
	}
}

// createMemoryProvider creates a provider for in-memory queue.
func (f *Factory) createMemoryProvider() (Provider, error) { //nolint:ireturn // factory pattern
	log.Printf("Creating memory queue with size: %d", f.config.QueueSize)

	return NewMemoryAdapter(f.config.QueueSize), nil
}

// createNATSProvider creates a provider for NATS queue.
func (f *Factory) createNATSProvider() (Provider, error) { //nolint:ireturn // factory pattern
	log.Printf("Creating NATS queue with URL: %s", f.config.NATSURL)

	// Use standard subject "messages" for all messages.
	adapter, err := NewNATSAdapter(f.config.NATSURL, "messages")
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS adapter: %w", err)
	}

	log.Printf("NATS queue provider created successfully")

	return adapter, nil
}

// createKafkaProvider creates a provider for Kafka queue.
func (f *Factory) createKafkaProvider() (Provider, error) { //nolint:ireturn // factory pattern
	log.Printf("Creating Kafka queue with brokers: %v, topic: %s, consumer group: %s",
		f.config.KafkaBrokers, f.config.KafkaTopic, f.config.KafkaConsumerGroup)

	adapter, err := NewKafkaAdapter(f.config.KafkaBrokers, f.config.KafkaTopic, f.config.KafkaConsumerGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka adapter: %w", err)
	}

	log.Printf("Kafka queue provider created successfully")

	return adapter, nil
}

// createCompositeProvider creates a provider for composite (dual-write) queue.
func (f *Factory) createCompositeProvider() (Provider, error) { //nolint:ireturn // factory pattern
	log.Printf("Creating composite queue with providers: %v, strategy: %s",
		f.config.CompositeProviders, f.config.CompositeStrategy)

	if len(f.config.CompositeProviders) == 0 {
		return nil, ErrNoCompositeProvidersConfigured
	}

	providers, err := f.createProviders(f.config.CompositeProviders)
	if err != nil {
		return nil, err
	}

	strategy, err := f.parseCompositeStrategy(f.config.CompositeStrategy)
	if err != nil {
		return nil, err
	}

	adapter := NewCompositeAdapter(providers, strategy)
	log.Printf("Composite queue provider created successfully with %d providers", len(providers))

	return adapter, nil
}

func (f *Factory) createProviders(providerTypes []string) ([]Provider, error) {
	providers := make([]Provider, 0, len(providerTypes))

	for _, providerType := range providerTypes {
		provider, err := f.createSingleProvider(ProviderType(providerType))
		if err != nil {
			return nil, err
		}

		providers = append(providers, provider)
	}

	return providers, nil
}

//nolint:ireturn // factory pattern
func (f *Factory) createSingleProvider(providerType ProviderType) (Provider, error) {
	switch providerType {
	case MemoryProviderType:
		return NewMemoryAdapter(f.config.QueueSize), nil
	case NATSProviderType:
		adapter, err := NewNATSAdapter(f.config.NATSURL, "messages")
		if err != nil {
			return nil, fmt.Errorf("failed to create NATS adapter for composite: %w", err)
		}

		return adapter, nil
	case KafkaProviderType:
		adapter, err := NewKafkaAdapter(f.config.KafkaBrokers, f.config.KafkaTopic, f.config.KafkaConsumerGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kafka adapter for composite: %w", err)
		}

		return adapter, nil
	default:
		return nil, fmt.Errorf("unsupported provider type in composite: %s: %w", providerType, ErrUnsupportedQueueType)
	}
}

func (f *Factory) parseCompositeStrategy(strategyStr string) (CompositeStrategy, error) {
	switch strategyStr {
	case "fail-fast":
		return FailFast, nil
	case "best-effort":
		return BestEffort, nil
	default:
		return FailFast, fmt.Errorf("%w: %s", ErrUnsupportedCompositeStrategy, strategyStr)
	}
}

// ValidateProviderType checks if the given queue type is supported.
func ValidateProviderType(queueType string) error {
	switch ProviderType(queueType) {
	case MemoryProviderType, NATSProviderType, KafkaProviderType, CompositeProviderType:
		return nil
	default:
		return fmt.Errorf("%w: %s. Supported types: %s, %s, %s, %s",
			ErrUnsupportedQueueType, queueType, MemoryProviderType, NATSProviderType, KafkaProviderType, CompositeProviderType)
	}
}
