package processor

import (
	"context"
	"fmt"
	"sync"
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
	// Создаем контекст с отменой
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем очередь и пул
	queueSize := b.N
	if queueSize < 1000 {
		queueSize = 1000
	}

	memQueue := queue.NewMemoryQueue(queueSize)
	pool := NewWorkerPool(4, memQueue)

	// Предзаполняем очередь
	for i := 0; i < b.N; i++ {
		msg := &models.DataMessage{
			Id:        fmt.Sprintf("bench-%d", i),
			Timestamp: time.Now().Unix(),
			Source:    "benchmark",
			Payload:   []byte("test payload for benchmarking"),
		}
		if err := memQueue.Publish(ctx, msg); err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}
	}

	// Запускаем пул
	if err := pool.Start(ctx); err != nil {
		b.Fatalf("Failed to start pool: %v", err)
	}

	// Сброс таймера после подготовки
	b.ResetTimer()

	// Считаем обработанные сообщения
	var processed int
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		for range pool.Results() {
			mu.Lock()
			processed++
			if processed >= b.N {
				close(done)
				mu.Unlock()
				return
			}
			mu.Unlock()
		}
	}()

	// Ждем завершения с таймаутом
	select {
	case <-done:
		// Успешно обработали все
	case <-time.After(30 * time.Second):
		mu.Lock()
		b.Fatalf("Timeout: processed only %d/%d messages", processed, b.N)
		mu.Unlock()
	}

	b.StopTimer()

	// Останавливаем пул
	cancel()
	pool.Stop()

	// Логируем производительность
	b.Logf("Processed %d messages", b.N)
}
