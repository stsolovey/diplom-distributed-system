package queue

import (
	"context"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

// MemoryAdapter адаптирует MemoryQueue для использования через интерфейсы.
// Это позволит легко заменить на NATS в Фазе 2.
type MemoryAdapter struct {
	queue *MemoryQueue
}

const memoryAdapterBufferSize = 100

// revive:disable:ireturn
// NewMemoryAdapter создает новый адаптер.
func NewMemoryAdapter(size int) *MemoryAdapter {
	return &MemoryAdapter{
		queue: NewMemoryQueue(size),
	}
}

// revive:enable:ireturn

// Publish реализует интерфейс Publisher.
func (a *MemoryAdapter) Publish(ctx context.Context, msg *models.DataMessage) error {
	return a.queue.Publish(ctx, msg)
}

// Subscribe реализует интерфейс Subscriber.
// Для MemoryQueue используем простой подход через канал.
func (a *MemoryAdapter) Subscribe(ctx context.Context) (<-chan *models.DataMessage, error) {
	msgChan := make(chan *models.DataMessage, memoryAdapterBufferSize)

	// Запускаем горутину для чтения из очереди.
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

// Stats возвращает статистику.
func (a *MemoryAdapter) Stats() Stats {
	return a.queue.Stats()
}

// Close закрывает адаптер.
func (a *MemoryAdapter) Close() error {
	return a.queue.Close()
}

// GetUnderlyingQueue возвращает базовую очередь для совместимости с существующим кодом.
// В Фазе 2 этот метод будет удален.
func (a *MemoryAdapter) GetUnderlyingQueue() *MemoryQueue {
	return a.queue
}
