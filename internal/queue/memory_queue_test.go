package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

func TestMemoryQueue_EnqueueDequeue(t *testing.T) {
	q := NewMemoryQueue(10)
	defer q.Close()

	msg := &models.DataMessage{
		Id:      "test-123",
		Payload: []byte("test data"),
	}

	// Test enqueue
	err := q.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Test dequeue
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	received, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}

	if received.Id != msg.Id {
		t.Errorf("Expected ID %s, got %s", msg.Id, received.Id)
	}
}

func TestMemoryQueue_FullQueue(t *testing.T) {
	q := NewMemoryQueue(1)
	defer q.Close()

	// Fill queue
	msg1 := &models.DataMessage{Id: "1"}
	msg2 := &models.DataMessage{Id: "2"}

	err := q.Enqueue(context.Background(), msg1)
	if err != nil {
		t.Fatalf("Failed first enqueue: %v", err)
	}

	// Should fail - queue is full
	err = q.Enqueue(context.Background(), msg2)
	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}
}

func TestMemoryQueue_Stats(t *testing.T) {
	q := NewMemoryQueue(10)
	defer q.Close()

	// Initial stats
	stats := q.Stats()
	if stats.TotalEnqueued != 0 || stats.TotalDequeued != 0 {
		t.Error("Initial stats should be zero")
	}

	// Enqueue and check
	msg := &models.DataMessage{Id: "test"}
	q.Enqueue(context.Background(), msg)

	stats = q.Stats()
	if stats.TotalEnqueued != 1 || stats.CurrentSize != 1 {
		t.Error("Stats not updated after enqueue")
	}
}
