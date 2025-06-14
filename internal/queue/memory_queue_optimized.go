package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// Оптимизированная версия с object pool.
//
//nolint:gochecknoglobals // object pool requires global scope for performance
var messagePool = sync.Pool{
	New: func() interface{} {
		return &models.DataMessage{
			Metadata: make(map[string]string),
		}
	},
}

// OptimizedMemoryQueue - версия с пулом объектов.
type OptimizedMemoryQueue struct {
	messages chan *models.DataMessage
	mu       sync.RWMutex
	closed   bool
	stats    Stats
}

func NewOptimizedMemoryQueue(size int) *OptimizedMemoryQueue {
	return &OptimizedMemoryQueue{
		messages: make(chan *models.DataMessage, size),
	}
}

func (q *OptimizedMemoryQueue) Publish(ctx context.Context, msg *models.DataMessage) error {
	// Копируем в объект из пула
	pooledMsg, ok := messagePool.Get().(*models.DataMessage)
	if !ok {
		return ErrQueueFull
	}

	pooledMsg.Id = msg.GetId()
	pooledMsg.Timestamp = msg.GetTimestamp()
	pooledMsg.Source = msg.GetSource()
	pooledMsg.Payload = append(pooledMsg.Payload[:0], msg.GetPayload()...)

	// Очищаем и копируем metadata
	for k := range pooledMsg.GetMetadata() {
		delete(pooledMsg.GetMetadata(), k)
	}

	for k, v := range msg.GetMetadata() {
		pooledMsg.GetMetadata()[k] = v
	}

	select {
	case q.messages <- pooledMsg:
		q.mu.Lock()
		q.stats.TotalEnqueued++
		q.mu.Unlock()

		return nil
	case <-ctx.Done():
		messagePool.Put(pooledMsg)

		return fmt.Errorf("publish context cancelled: %w", ctx.Err())
	default:
		messagePool.Put(pooledMsg)

		return ErrQueueFull
	}
}

func (q *OptimizedMemoryQueue) Enqueue(ctx context.Context, msg *models.DataMessage) error {
	return q.Publish(ctx, msg)
}

func (q *OptimizedMemoryQueue) Dequeue(ctx context.Context) (*models.DataMessage, error) {
	select {
	case msg := <-q.messages:
		if msg == nil {
			return nil, ErrQueueClosed
		}

		q.mu.Lock()
		q.stats.TotalDequeued++
		q.mu.Unlock()

		// Возвращаем сообщение в пул после использования
		// ВНИМАНИЕ: потребитель должен скопировать данные если нужно их сохранить
		defer messagePool.Put(msg)

		return msg, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("dequeue context cancelled: %w", ctx.Err())
	}
}

func (q *OptimizedMemoryQueue) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan := make(chan *models.DataMessage, memoryQueueBufferSize)

	go func() {
		defer close(msgChan)

		for {
			msg, err := q.Dequeue(ctx)
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, ErrQueueClosed) {
					return
				}

				continue
			}

			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	return msgChan, nil
}

func (q *OptimizedMemoryQueue) Stats() Stats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	stats := q.stats
	stats.CurrentSize = len(q.messages)

	return stats
}

func (q *OptimizedMemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.closed {
		q.closed = true
		close(q.messages)

		// Очищаем очередь и возвращаем объекты в пул
		for msg := range q.messages {
			messagePool.Put(msg)
		}
	}

	return nil
}
