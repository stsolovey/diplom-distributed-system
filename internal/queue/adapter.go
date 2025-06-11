package queue

import (
	"context"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// MemoryQueueAdapter адаптирует MemoryQueue для использования через интерфейсы
// Это позволит легко заменить на NATS в Фазе 2
type MemoryQueueAdapter struct {
	queue *MemoryQueue
}

// NewMemoryQueueAdapter создает новый адаптер
func NewMemoryQueueAdapter(size int) QueueProvider {
	return &MemoryQueueAdapter{
		queue: NewMemoryQueue(size),
	}
}

// Publish реализует интерфейс Publisher
func (a *MemoryQueueAdapter) Publish(ctx context.Context, msg *models.DataMessage) error {
	return a.queue.Publish(ctx, msg)
}

// Subscribe реализует интерфейс Subscriber
// Для MemoryQueue используем простой подход через канал
func (a *MemoryQueueAdapter) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan := make(chan *models.DataMessage, 100)

	// Запускаем горутину для чтения из очереди
	go func() {
		defer close(msgChan)
		for {
			msg, err := a.queue.Dequeue(ctx)
			if err != nil {
				return
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

// Stats возвращает статистику
func (a *MemoryQueueAdapter) Stats() QueueStats {
	return a.queue.Stats()
}

// Close закрывает адаптер
func (a *MemoryQueueAdapter) Close() error {
	return a.queue.Close()
}

// GetUnderlyingQueue возвращает базовую очередь для совместимости с существующим кодом
// В Фазе 2 этот метод будет удален
func (a *MemoryQueueAdapter) GetUnderlyingQueue() *MemoryQueue {
	return a.queue
}
