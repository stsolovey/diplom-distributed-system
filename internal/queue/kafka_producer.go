package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

type KafkaProducer struct {
	producer sarama.SyncProducer
	topic    string
}

func NewKafkaProducer(brokers []string, topic string) (*KafkaProducer, error) {
	config := getKafkaProducerConfig()

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return &KafkaProducer{
		producer: producer,
		topic:    topic,
	}, nil
}

func (p *KafkaProducer) Publish(_ context.Context, msg *models.DataMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	kafkaMsg := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       sarama.StringEncoder(msg.GetId()),
		Value:     sarama.ByteEncoder(data),
		Timestamp: time.Now(),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("source"),
				Value: []byte(msg.GetSource()),
			},
			{
				Key:   []byte("message_id"),
				Value: []byte(msg.GetId()),
			},
		},
	}

	partition, offset, err := p.producer.SendMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	// Log successful send for debugging
	_ = partition // avoid unused variable
	_ = offset    // avoid unused variable

	return nil
}

func (p *KafkaProducer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close Kafka producer: %w", err)
	}

	return nil
}
