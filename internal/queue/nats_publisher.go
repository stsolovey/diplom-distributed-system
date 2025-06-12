package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

type NATSPublisher struct {
	js      jetstream.JetStream
	subject string
}

// NewNATSPublisher создает publisher для конкретного subject
func NewNATSPublisher(broker *NATSBroker, subject string) *NATSPublisher {
	return &NATSPublisher{
		js:      broker.js,
		subject: broker.config.SubjectPrefix + "." + subject,
	}
}

// Publish отправляет сообщение в NATS JetStream с гарантией доставки
func (p *NATSPublisher) Publish(ctx context.Context, msg *models.DataMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Публикуем с ожиданием ACK для гарантии персистентности
	ack, err := p.js.Publish(ctx, p.subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Проверяем, что сообщение было успешно сохранено в JetStream
	if ack == nil {
		return fmt.Errorf("received nil acknowledgment from JetStream")
	}

	// В современной версии NATS JetStream ACK возвращается синхронно
	// Сообщение гарантированно сохранено в stream
	return nil
}
