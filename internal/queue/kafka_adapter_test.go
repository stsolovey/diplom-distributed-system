package queue

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

func TestKafkaAdapter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Kafka container
	kafkaContainer, err := kafka.Run(ctx,
		"confluentinc/cp-kafka:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		t.Fatalf("Failed to start Kafka container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(kafkaContainer); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}()

	// Get Kafka brokers
	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		t.Fatalf("Failed to get Kafka brokers: %v", err)
	}

	// Wait for Kafka to stabilize
	time.Sleep(5 * time.Second)

	// Create Kafka adapter
	topic := "test-topic"
	consumerGroup := "test-group"

	adapter, err := NewKafkaAdapter(brokers, topic, consumerGroup)
	if err != nil {
		t.Fatalf("Failed to create Kafka adapter: %v", err)
	}
	defer adapter.Close()

	// Subscribe FIRST (before publishing)
	msgChan, err := adapter.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Wait for consumer to be ready
	time.Sleep(2 * time.Second)

	// Test message
	testMsg := &models.DataMessage{
		Id:        uuid.New().String(),
		Timestamp: time.Now().Unix(),
		Source:    "test-source",
		Payload:   []byte("test payload"),
		Metadata:  map[string]string{"key": "value"},
	}

	// Publish AFTER subscribing
	err = adapter.Publish(ctx, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Wait for message
	select {
	case receivedMsg := <-msgChan:
		if receivedMsg == nil {
			t.Fatal("Received nil message")
		}

		if receivedMsg.GetId() != testMsg.GetId() {
			t.Errorf("Expected message ID %s, got %s", testMsg.GetId(), receivedMsg.GetId())
		}

		if receivedMsg.GetSource() != testMsg.GetSource() {
			t.Errorf("Expected source %s, got %s", testMsg.GetSource(), receivedMsg.GetSource())
		}

		if string(receivedMsg.GetPayload()) != string(testMsg.GetPayload()) {
			t.Errorf("Expected payload %s, got %s", testMsg.GetPayload(), receivedMsg.GetPayload())
		}

	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Test stats
	stats := adapter.Stats()
	if stats.TotalEnqueued != 1 {
		t.Errorf("Expected 1 enqueued message, got %d", stats.TotalEnqueued)
	}

	if stats.TotalDequeued != 1 {
		t.Errorf("Expected 1 dequeued message, got %d", stats.TotalDequeued)
	}
}

func TestKafkaAdapter_PublishMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Kafka container
	kafkaContainer, err := kafka.Run(ctx,
		"confluentinc/cp-kafka:7.5.0",
		kafka.WithClusterID("test-cluster-multiple"),
	)
	if err != nil {
		t.Fatalf("Failed to start Kafka container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(kafkaContainer); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}()

	// Get Kafka brokers
	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		t.Fatalf("Failed to get Kafka brokers: %v", err)
	}

	// Wait for Kafka to stabilize
	time.Sleep(5 * time.Second)

	// Create Kafka adapter
	topic := "test-topic-multiple"
	consumerGroup := "test-group-multiple"

	adapter, err := NewKafkaAdapter(brokers, topic, consumerGroup)
	if err != nil {
		t.Fatalf("Failed to create Kafka adapter: %v", err)
	}
	defer adapter.Close()

	// Subscribe FIRST (before publishing)
	msgChan, err := adapter.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Wait for consumer to be ready
	time.Sleep(2 * time.Second)

	// Test multiple messages
	messageCount := 5
	messages := make([]*models.DataMessage, messageCount)

	for i := 0; i < messageCount; i++ {
		messages[i] = &models.DataMessage{
			Id:        uuid.New().String(),
			Timestamp: time.Now().Unix(),
			Source:    "test-source",
			Payload:   []byte("test payload " + strconv.Itoa(i)),
			Metadata:  map[string]string{"index": strconv.Itoa(i)},
		}

		err = adapter.Publish(ctx, messages[i])
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Collect received messages
	receivedCount := 0
	timeout := time.After(30 * time.Second)

	for receivedCount < messageCount {
		select {
		case receivedMsg := <-msgChan:
			if receivedMsg != nil {
				receivedCount++
			}

		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d of %d", receivedCount, messageCount)
		}
	}

	// Test stats
	stats := adapter.Stats()
	if stats.TotalEnqueued != int64(messageCount) {
		t.Errorf("Expected %d enqueued messages, got %d", messageCount, stats.TotalEnqueued)
	}

	if stats.TotalDequeued != int64(messageCount) {
		t.Errorf("Expected %d dequeued messages, got %d", messageCount, stats.TotalDequeued)
	}
}
