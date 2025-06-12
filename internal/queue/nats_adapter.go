package queue

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

type NATSAdapter struct {
	broker     *NATSBroker
	publisher  *NATSPublisher
	subscriber *NATSSubscriber

	// Статистика - только атомарные операции, без мьютекса
	totalEnqueued int64
	totalDequeued int64
	errors        int64
}

// NewNATSAdapter создает адаптер для NATS
func NewNATSAdapter(natsURL, subject string) (*NATSAdapter, error) {
	// Конфигурация NATS
	cfg := NATSConfig{
		URL:           natsURL,
		StreamName:    "DIPLOM_STREAM",
		SubjectPrefix: "diplom",
		MaxReconnects: 5,
		ReconnectWait: 2 * time.Second,
	}

	// Создаем брокер
	broker, err := NewNATSBroker(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS broker: %w", err)
	}

	// Создаем publisher
	publisher := NewNATSPublisher(broker, subject)

	// Создаем subscriber
	subscriber, err := NewNATSSubscriber(broker, subject)
	if err != nil {
		broker.Close()
		return nil, fmt.Errorf("failed to create NATS subscriber: %w", err)
	}

	return &NATSAdapter{
		broker:     broker,
		publisher:  publisher,
		subscriber: subscriber,
	}, nil
}

// Publish реализует интерфейс Publisher
func (a *NATSAdapter) Publish(ctx context.Context, msg *models.DataMessage) error {
	err := a.publisher.Publish(ctx, msg)

	if err != nil {
		atomic.AddInt64(&a.errors, 1)
		return err
	}

	// Увеличиваем счетчик только после успешного ACK от JetStream
	atomic.AddInt64(&a.totalEnqueued, 1)
	return nil
}

// Subscribe реализует интерфейс Subscriber
func (a *NATSAdapter) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan, err := a.subscriber.Subscribe(ctx)
	if err != nil {
		atomic.AddInt64(&a.errors, 1)
		return nil, err
	}

	// Оборачиваем канал для подсчета статистики
	wrappedChan := make(chan *models.DataMessage, 100)

	go func() {
		defer close(wrappedChan)
		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				atomic.AddInt64(&a.totalDequeued, 1)

				select {
				case wrappedChan <- msg:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return wrappedChan, nil
}

// Stats возвращает статистику NATS
func (a *NATSAdapter) Stats() QueueStats {
	return QueueStats{
		TotalEnqueued: atomic.LoadInt64(&a.totalEnqueued),
		TotalDequeued: atomic.LoadInt64(&a.totalDequeued),
		CurrentSize:   -1, // JetStream не предоставляет точный размер очереди
	}
}

// Close закрывает адаптер
func (a *NATSAdapter) Close() error {
	var errs []error

	if err := a.subscriber.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := a.broker.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing NATS adapter: %v", errs)
	}

	return nil
}
