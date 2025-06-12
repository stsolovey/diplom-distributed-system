package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

type NATSSubscriber struct {
	js       jetstream.JetStream
	subject  string
	consumer jetstream.Consumer
}

// NewNATSSubscriber создает subscriber для конкретного subject
func NewNATSSubscriber(broker *NATSBroker, subject string) (*NATSSubscriber, error) {
	fullSubject := broker.config.SubjectPrefix + "." + subject

	// Создаем durable consumer
	cfg := jetstream.ConsumerConfig{
		Name:          subject + "-consumer",
		Durable:       subject + "-consumer",
		FilterSubject: fullSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       30 * time.Second,
	}

	consumer, err := broker.js.CreateOrUpdateConsumer(
		context.Background(),
		broker.config.StreamName,
		cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &NATSSubscriber{
		js:       broker.js,
		subject:  fullSubject,
		consumer: consumer,
	}, nil
}

// Subscribe создает канал для получения сообщений с правильным управлением ресурсами
func (s *NATSSubscriber) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan := make(chan *models.DataMessage, 100)

	// Создаем pull subscription с back-pressure
	iter, err := s.consumer.Messages(
		jetstream.PullMaxMessages(10), // back-pressure: получаем max 10 сообщений за раз
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message iterator: %w", err)
	}

	// Горутина для чтения сообщений
	go func() {
		defer close(msgChan)
		defer iter.Stop() // КРИТИЧНО: освобождаем ресурсы итератора

		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := iter.Next()
				if err != nil {
					if err == context.Canceled {
						return
					}
					// Проверяем timeout ошибки по строковому представлению
					if err != nil && (err.Error() == "nats: timeout" || err.Error() == "timeout") {
						// При timeout не спамим логи, просто ждем
						time.Sleep(100 * time.Millisecond)
						continue
					}
					log.Printf("Error fetching message: %v", err)
					time.Sleep(time.Second) // back-off при других ошибках
					continue
				}

				// Десериализуем сообщение
				var dataMsg models.DataMessage
				if err := json.Unmarshal(msg.Data(), &dataMsg); err != nil {
					log.Printf("Failed to unmarshal message: %v", err)
					msg.Nak() // negative acknowledgment
					continue
				}

				// Отправляем в канал
				select {
				case msgChan <- &dataMsg:
					// Подтверждаем только после успешной отправки в канал
					if err := msg.Ack(); err != nil {
						log.Printf("Failed to ack message: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return msgChan, nil
}

// Close останавливает подписку
func (s *NATSSubscriber) Close() error {
	// Consumer автоматически очищается при закрытии соединения
	return nil
}
