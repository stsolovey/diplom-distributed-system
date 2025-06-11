package processor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/stsolovey/diplom-distributed-system/internal/queue"
)

func TestWorkerPool_ProcessMessages(t *testing.T) {
	q := queue.NewMemoryQueue(100)
	pool := NewWorkerPool(2, q)

	// Создаем контекст для воркеров
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	pool.Start(workerCtx)

	// Создаем горутину для чтения результатов чтобы воркеры не блокировались
	go func() {
		for range pool.Results() {
			// Просто читаем результаты
		}
	}()

	// Send test messages
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		msg := &models.DataMessage{
			Id:      fmt.Sprintf("test-%d", i),
			Payload: []byte(fmt.Sprintf("data-%d", i)),
		}
		if err := q.Enqueue(context.Background(), msg); err != nil {
			t.Fatalf("Failed to enqueue: %v", err)
		}
	}

	// Wait for processing - проверяем статистику
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			stats := pool.GetStats()
			queueStats := q.Stats()
			t.Fatalf("Timeout waiting for processing. Expected %d, got %d. Queue stats: %+v", messageCount, stats.ProcessedCount, queueStats)
		case <-ticker.C:
			stats := pool.GetStats()
			if stats.ProcessedCount >= int64(messageCount) {
				return // Успешно завершаем тест
			}
		}
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	q := queue.NewMemoryQueue(1000)
	pool := NewWorkerPool(4, q)

	// Создаем контекст для воркеров
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Создаем горутину для чтения результатов чтобы воркеры не блокировались
	go func() {
		for range pool.Results() {
			// Просто читаем результаты для предотвращения блокировки
		}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &models.DataMessage{
			Id:      fmt.Sprintf("bench-%d", i),
			Payload: []byte("benchmark data"),
		}
		q.Enqueue(ctx, msg)
	}

	// Wait for all messages to be processed
	for pool.GetStats().ProcessedCount < int64(b.N) {
		time.Sleep(time.Millisecond)
	}

	b.StopTimer()
	cancel() // Отменяем контекст для graceful shutdown
	pool.Stop()
}
