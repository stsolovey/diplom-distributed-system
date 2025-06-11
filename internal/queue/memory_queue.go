package queue

import (
	"context"
	"errors"
	"sync"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

var (
	ErrQueueFull   = errors.New("queue is full")
	ErrQueueClosed = errors.New("queue is closed")
)

// MemoryQueue - потокобезопасная in-memory очередь
type MemoryQueue struct {
	messages chan *models.DataMessage
	mu       sync.RWMutex
	closed   bool
	stats    QueueStats
}

type QueueStats struct {
	TotalEnqueued int64
	TotalDequeued int64
	CurrentSize   int
}

// NewMemoryQueue создает новую очередь заданного размера
func NewMemoryQueue(size int) *MemoryQueue {
	return &MemoryQueue{
		messages: make(chan *models.DataMessage, size),
	}
}

// Publish реализует интерфейс Publisher (алиас для Enqueue)
func (q *MemoryQueue) Publish(ctx context.Context, msg *models.DataMessage) error {
	return q.Enqueue(ctx, msg)
}

// Enqueue добавляет сообщение в очередь (неблокирующий)
func (q *MemoryQueue) Enqueue(ctx context.Context, msg *models.DataMessage) error {
	q.mu.RLock()
	if q.closed {
		q.mu.RUnlock()
		return ErrQueueClosed
	}
	q.mu.RUnlock()

	select {
	case q.messages <- msg:
		q.mu.Lock()
		q.stats.TotalEnqueued++
		q.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrQueueFull
	}
}

// Dequeue извлекает сообщение из очереди (блокирующий)
func (q *MemoryQueue) Dequeue(ctx context.Context) (*models.DataMessage, error) {
	select {
	case msg := <-q.messages:
		if msg == nil {
			return nil, ErrQueueClosed
		}
		q.mu.Lock()
		q.stats.TotalDequeued++
		q.mu.Unlock()
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stats возвращает статистику очереди
func (q *MemoryQueue) Stats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	stats := q.stats
	stats.CurrentSize = len(q.messages)
	return stats
}

// Close закрывает очередь
func (q *MemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.messages)
	}
	return nil
}
