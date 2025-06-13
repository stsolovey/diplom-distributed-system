package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	kafkaAdapterChanSize = 100
)

var (
	ErrKafkaProducerCreate  = errors.New("failed to create Kafka producer")
	ErrKafkaConsumerCreate  = errors.New("failed to create Kafka consumer")
	ErrKafkaPublishFailed   = errors.New("kafka publish failed")
	ErrKafkaSubscribeFailed = errors.New("kafka subscribe failed")
	ErrKafkaAdapterClose    = errors.New("kafka adapter close errors")
)

type KafkaAdapter struct {
	producer *KafkaProducer
	consumer *KafkaConsumer
	stats    *kafkaStats
}

type kafkaStats struct {
	published int64
	consumed  int64
}

func NewKafkaAdapter(brokers []string, topic, consumerGroup string) (*KafkaAdapter, error) {
	log.Printf("Creating Kafka adapter with brokers: %v, topic: %s, consumer group: %s", brokers, topic, consumerGroup)

	producer, err := NewKafkaProducer(brokers, topic)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKafkaProducerCreate, err)
	}

	consumer, err := NewKafkaConsumer(brokers, topic, consumerGroup)
	if err != nil {
		producer.Close() // Clean up producer if consumer creation fails

		return nil, fmt.Errorf("%w: %w", ErrKafkaConsumerCreate, err)
	}

	log.Printf("Kafka adapter created successfully")

	return &KafkaAdapter{
		producer: producer,
		consumer: consumer,
		stats:    &kafkaStats{},
	}, nil
}

func (a *KafkaAdapter) Publish(ctx context.Context, msg *models.DataMessage) error {
	err := a.producer.Publish(ctx, msg)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrKafkaPublishFailed, err)
	}

	atomic.AddInt64(&a.stats.published, 1)

	return nil
}

func (a *KafkaAdapter) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan, err := a.consumer.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKafkaSubscribeFailed, err)
	}

	// Wrap channel to count consumed messages
	countedChan := make(chan *models.DataMessage, kafkaAdapterChanSize)

	go func() {
		defer close(countedChan)

		for msg := range msgChan {
			if msg != nil {
				atomic.AddInt64(&a.stats.consumed, 1)
				select {
				case countedChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return countedChan, nil
}

func (a *KafkaAdapter) Close() error {
	var errs []error

	if err := a.producer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("producer close error: %w", err))
	}

	if err := a.consumer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("consumer close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %v", ErrKafkaAdapterClose, errs)
	}

	log.Printf("Kafka adapter closed successfully")

	return nil
}

func (a *KafkaAdapter) Stats() Stats {
	return Stats{
		TotalEnqueued: atomic.LoadInt64(&a.stats.published),
		TotalDequeued: atomic.LoadInt64(&a.stats.consumed),
		CurrentSize:   0, // Kafka queue size is not directly measurable
	}
}
