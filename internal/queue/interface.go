package queue

import (
	"context"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// Publisher интерфейс для публикации сообщений.
type Publisher interface {
	Publish(ctx context.Context, msg *models.DataMessage) error
}

// Subscriber интерфейс для подписки на сообщения.
type Subscriber interface {
	Subscribe(ctx context.Context) (<-chan *models.DataMessage, error)
	Close() error
}

// Provider объединяет Publisher и Subscriber.
type Provider interface {
	Publisher
	Subscriber
	Stats() Stats
}

// MessageBroker интерфейс для брокеров сообщений (для будущей интеграции с NATS/Kafka).
type MessageBroker interface {
	Connect(ctx context.Context) error
	Disconnect() error
	CreatePublisher(subject string) (Publisher, error)
	CreateSubscriber(subject string) (Subscriber, error)
}
