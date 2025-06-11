package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/stsolovey/diplom-distributed-system/internal/queue"
)

// WorkerPool управляет пулом воркеров для обработки сообщений
type WorkerPool struct {
	workers int
	queue   *queue.MemoryQueue
	wg      sync.WaitGroup
	results chan *models.ProcessingResult
	stats   Stats
	statsMu sync.RWMutex
}

type Stats struct {
	ProcessedCount int64
	ErrorCount     int64
	TotalDuration  time.Duration
}

// NewWorkerPool создает новый пул воркеров
func NewWorkerPool(workers int, q *queue.MemoryQueue) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		queue:   q,
		results: make(chan *models.ProcessingResult, workers*2),
	}
}

// Start запускает воркеры
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Printf("Starting worker pool with %d workers", wp.workers)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.runWorker(ctx, i)
	}
}

// runWorker - основной цикл воркера
func (wp *WorkerPool) runWorker(ctx context.Context, workerID int) {
	defer wp.wg.Done()
	log.Printf("Worker %d started", workerID)

	for {
		// Прямой вызов блокирующей операции без лишнего select
		msg, err := wp.queue.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || err == queue.ErrQueueClosed {
				log.Printf("Worker %d stopping", workerID)
				return
			}
			// Другие ошибки - продолжаем работу
			continue
		}

		result := wp.processMessage(msg)

		select {
		case wp.results <- result:
		case <-ctx.Done():
			log.Printf("Worker %d stopping", workerID)
			return
		}
	}
}

// processMessage обрабатывает одно сообщение
func (wp *WorkerPool) processMessage(msg *models.DataMessage) *models.ProcessingResult {
	start := time.Now()

	// Имитация обработки (в реальной системе здесь будет бизнес-логика)
	time.Sleep(time.Millisecond * 10) // Симуляция работы

	// Простая обработка: добавляем префикс к payload
	processedData := fmt.Sprintf("PROCESSED_%s", string(msg.Payload))

	result := &models.ProcessingResult{
		MessageId:   msg.Id,
		ProcessedAt: time.Now().Unix(),
		Success:     true,
		Result:      []byte(processedData),
	}

	// Обновляем статистику
	wp.updateStats(true, time.Since(start))

	return result
}

// updateStats обновляет статистику обработки
func (wp *WorkerPool) updateStats(success bool, duration time.Duration) {
	wp.statsMu.Lock()
	defer wp.statsMu.Unlock()

	if success {
		wp.stats.ProcessedCount++
	} else {
		wp.stats.ErrorCount++
	}
	wp.stats.TotalDuration += duration
}

// GetStats возвращает текущую статистику
func (wp *WorkerPool) GetStats() Stats {
	wp.statsMu.RLock()
	defer wp.statsMu.RUnlock()
	return wp.stats
}

// Stop останавливает все воркеры
func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	wp.wg.Wait()

	// Дренируем канал результатов перед закрытием
	go func() {
		for range wp.results {
			// Читаем оставшиеся результаты
		}
	}()

	close(wp.results)
}

// Results возвращает канал с результатами обработки
func (wp *WorkerPool) Results() <-chan *models.ProcessingResult {
	return wp.results
}
