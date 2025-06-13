package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// MockProvider for testing purposes.
type MockProvider struct {
	publishError error
	stats        Stats
	messages     []*models.DataMessage
	closed       bool
}

func (m *MockProvider) Publish(ctx context.Context, msg *models.DataMessage) error {
	if m.publishError != nil {
		return m.publishError
	}

	m.messages = append(m.messages, msg)
	m.stats.TotalEnqueued++

	return nil
}

func (m *MockProvider) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	ch := make(chan *models.DataMessage, 10)
	// Send all stored messages
	go func() {
		defer close(ch)
		for _, msg := range m.messages {
			select {
			case ch <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (m *MockProvider) Stats() Stats {
	return m.stats
}

func (m *MockProvider) Close() error {
	m.closed = true

	return nil
}

func TestCompositeAdapter_PublishFailFast(t *testing.T) {
	// Create 2 memory adapters.
	adapter1 := NewMemoryAdapter(10)
	adapter2 := NewMemoryAdapter(10)

	composite := NewCompositeAdapter(
		[]Provider{adapter1, adapter2},
		FailFast,
	)

	// Send message.
	msg := &models.DataMessage{
		Id:        "test-fail-fast",
		Timestamp: time.Now().Unix(),
		Source:    "test",
		Payload:   []byte("test payload"),
	}

	err := composite.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that message is in both queues.
	stats1 := adapter1.Stats()
	stats2 := adapter2.Stats()

	if stats1.TotalEnqueued != 1 {
		t.Errorf("Expected 1 enqueued message in adapter1, got %d", stats1.TotalEnqueued)
	}

	if stats2.TotalEnqueued != 1 {
		t.Errorf("Expected 1 enqueued message in adapter2, got %d", stats2.TotalEnqueued)
	}

	// Check composite stats.
	compositeStats := composite.Stats()
	if compositeStats.TotalEnqueued != 2 {
		t.Errorf("Expected 2 total enqueued messages, got %d", compositeStats.TotalEnqueued)
	}
}

func TestCompositeAdapter_PublishFailFastWithError(t *testing.T) {
	// Create one successful and one failing provider.
	adapter1 := NewMemoryAdapter(10)
	mockProvider := &MockProvider{
		publishError: errors.New("mock error"),
	}

	composite := NewCompositeAdapter(
		[]Provider{adapter1, mockProvider},
		FailFast,
	)

	msg := &models.DataMessage{
		Id:        "test-fail-fast-error",
		Timestamp: time.Now().Unix(),
		Source:    "test",
		Payload:   []byte("test payload"),
	}

	err := composite.Publish(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error in FailFast mode when one provider fails")
	}

	if !errors.Is(err, errors.New("mock error")) && err.Error() != "failed to publish to all providers: mock error" {
		t.Errorf("Expected error containing 'mock error', got: %v", err)
	}
}

func TestCompositeAdapter_PublishBestEffort(t *testing.T) {
	// Create one successful and one failing provider.
	adapter1 := NewMemoryAdapter(10)
	mockProvider := &MockProvider{
		publishError: errors.New("mock error"),
	}

	composite := NewCompositeAdapter(
		[]Provider{adapter1, mockProvider},
		BestEffort,
	)

	msg := &models.DataMessage{
		Id:        "test-best-effort",
		Timestamp: time.Now().Unix(),
		Source:    "test",
		Payload:   []byte("test payload"),
	}

	err := composite.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("Expected no error in BestEffort mode, got: %v", err)
	}

	// Check that successful adapter got the message.
	stats1 := adapter1.Stats()
	if stats1.TotalEnqueued != 1 {
		t.Errorf("Expected 1 enqueued message in successful adapter, got %d", stats1.TotalEnqueued)
	}
}

func TestCompositeAdapter_Subscribe(t *testing.T) {
	adapter1 := NewMemoryAdapter(10)
	adapter2 := NewMemoryAdapter(10)

	composite := NewCompositeAdapter(
		[]Provider{adapter1, adapter2},
		FailFast,
	)

	// Add message to first adapter.
	msg := &models.DataMessage{
		Id:        "test-subscribe",
		Timestamp: time.Now().Unix(),
		Source:    "test",
		Payload:   []byte("test payload"),
	}

	err := adapter1.Publish(context.Background(), msg)
	if err != nil {
		t.Fatalf("Failed to publish to adapter1: %v", err)
	}

	// Subscribe through composite.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgChan, err := composite.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Should receive message from first adapter.
	select {
	case receivedMsg := <-msgChan:
		if receivedMsg.GetId() != msg.GetId() {
			t.Errorf("Expected message ID %s, got %s", msg.GetId(), receivedMsg.GetId())
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for message")
	}
}

func TestCompositeAdapter_NoProviders(t *testing.T) {
	composite := NewCompositeAdapter([]Provider{}, FailFast)

	msg := &models.DataMessage{
		Id:        "test-no-providers",
		Timestamp: time.Now().Unix(),
		Source:    "test",
		Payload:   []byte("test payload"),
	}

	err := composite.Publish(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error when no providers configured")
	}

	_, err = composite.Subscribe(context.Background())
	if err == nil {
		t.Fatal("Expected error when no providers configured for subscribe")
	}
}

func TestCompositeAdapter_Close(t *testing.T) {
	mockProvider1 := &MockProvider{}
	mockProvider2 := &MockProvider{}

	composite := NewCompositeAdapter(
		[]Provider{mockProvider1, mockProvider2},
		FailFast,
	)

	err := composite.Close()
	if err != nil {
		t.Fatalf("Expected no error on close, got: %v", err)
	}

	if !mockProvider1.closed {
		t.Error("Expected mockProvider1 to be closed")
	}

	if !mockProvider2.closed {
		t.Error("Expected mockProvider2 to be closed")
	}
}
