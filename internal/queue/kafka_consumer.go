package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/IBM/sarama"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	kafkaConsumerChanSize = 100
)

type KafkaConsumer struct {
	consumerGroup sarama.ConsumerGroup
	topic         string
	handler       *kafkaConsumerHandler
	wg            sync.WaitGroup
	cancel        context.CancelFunc
}

type kafkaConsumerHandler struct {
	msgChan chan *models.DataMessage
	ready   chan bool
}

func NewKafkaConsumer(brokers []string, topic, groupID string) (*KafkaConsumer, error) {
	config := getKafkaConsumerConfig()

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &KafkaConsumer{
		consumerGroup: consumerGroup,
		topic:         topic,
		handler: &kafkaConsumerHandler{
			msgChan: make(chan *models.DataMessage, kafkaConsumerChanSize),
			ready:   make(chan bool),
		},
	}, nil
}

func (c *KafkaConsumer) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	// Create cancellable context for proper shutdown
	consumeCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)

	go func() {
		defer c.wg.Done()
		defer close(c.handler.msgChan)

		for {
			// Consumer group will handle reconnections automatically
			err := c.consumerGroup.Consume(consumeCtx, []string{c.topic}, c.handler)
			if err != nil {
				log.Printf("Error from consumer: %v", err)
			}

			// Check if context was cancelled (shutdown requested)
			if consumeCtx.Err() != nil {
				return
			}

			c.handler.ready = make(chan bool)
		}
	}()

	// Wait for consumer to be ready
	<-c.handler.ready

	return c.handler.msgChan, nil
}

func (c *KafkaConsumer) Close() error {
	// Cancel context first to stop the consume loop
	if c.cancel != nil {
		c.cancel()
	}

	// Then close the consumer group
	if err := c.consumerGroup.Close(); err != nil {
		c.wg.Wait() // Wait for consume goroutine to finish

		return fmt.Errorf("failed to close Kafka consumer group: %w", err)
	}

	c.wg.Wait() // Wait for consume goroutine to finish

	return nil
}

// ConsumerGroupHandler interface implementation.
func (h *kafkaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)

	return nil
}

func (h *kafkaConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *kafkaConsumerHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			var msg models.DataMessage
			if err := json.Unmarshal(message.Value, &msg); err != nil {
				log.Printf("Failed to unmarshal Kafka message: %v", err)
				session.MarkMessage(message, "")

				continue
			}

			select {
			case h.msgChan <- &msg:
				session.MarkMessage(message, "")
			case <-session.Context().Done():
				return nil
			}

		case <-session.Context().Done():
			return nil
		}
	}
}
