package processor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
)

const (
	// resultsBufferMultiplier defines how many extra slots the results channel has per worker.
	resultsBufferMultiplier = 2
)

// Subscriber интерфейс для получения сообщений.
type Subscriber interface {
	Subscribe(ctx context.Context) (<-chan *models.DataMessage, error)
}

// WorkerPool управляет пулом воркеров для обработки сообщений.
type WorkerPool struct {
	workers    int
	subscriber Subscriber // унифицированный интерфейс для всех типов очередей
	wg         sync.WaitGroup
	results    chan *models.ProcessingResult
	stats      Stats
	statsMu    sync.RWMutex
	msgChan    <-chan *models.DataMessage // канал для получения сообщений
}

type Stats struct {
	ProcessedCount int64
	ErrorCount     int64
	TotalDuration  time.Duration
}

// NewWorkerPool создает новый пул воркеров с унифицированным интерфейсом.
func NewWorkerPool(workers int, subscriber Subscriber) *WorkerPool {
	return &WorkerPool{
		workers:    workers,
		subscriber: subscriber,
		results:    make(chan *models.ProcessingResult, workers*resultsBufferMultiplier),
	}
}

// Start запускает воркеры.
func (wp *WorkerPool) Start(ctx context.Context) error {
	log.Printf("Starting worker pool with %d workers", wp.workers)

	var err error

	wp.msgChan, err = wp.subscriber.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	for i := range make([]struct{}, wp.workers) {
		wp.wg.Add(1)
		go wp.runWorker(ctx, i)
	}

	return nil
}

// runWorker - основной цикл воркера.
func (wp *WorkerPool) runWorker(ctx context.Context, workerID int) {
	defer wp.wg.Done()
	log.Printf("Worker %d started", workerID)

	for {
		var msg *models.DataMessage

		// Получаем сообщение из канала подписки
		select {
		case msg = <-wp.msgChan:
			if msg == nil {
				log.Printf("Worker %d stopping - channel closed", workerID)

				return
			}
		case <-ctx.Done():
			log.Printf("Worker %d stopping", workerID)

			return
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

// processMessage обрабатывает одно сообщение.
func (wp *WorkerPool) processMessage(msg *models.DataMessage) *models.ProcessingResult {
	start := time.Now()

	// Имитация обработки (в реальной системе здесь будет бизнес-логика)
	// time.Sleep(workSimulationDelay) // REMOVED FOR PERFORMANCE - artificial delay.

	// Простая обработка: добавляем префикс к payload
	processedData := "PROCESSED_" + string(msg.GetPayload())

	result := &models.ProcessingResult{
		MessageId:   msg.GetId(),
		ProcessedAt: time.Now().Unix(),
		Success:     true,
		Result:      []byte(processedData),
	}

	// Обновляем статистику
	wp.updateStats(true, time.Since(start))

	return result
}

// updateStats обновляет статистику обработки.
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

// GetStats возвращает текущую статистику.
func (wp *WorkerPool) GetStats() Stats {
	wp.statsMu.RLock()
	defer wp.statsMu.RUnlock()

	return wp.stats
}

// Stop останавливает все воркеры.
func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	wp.wg.Wait()

	// Дренируем канал результатов перед закрытием, чтобы не блокировать возможных
	// отправителей.
	go func() {
		for res := range wp.results {
			_ = res // drain to avoid blocking producers.
		}
	}()

	close(wp.results)
}

// Results возвращает канал с результатами обработки.
func (wp *WorkerPool) Results() <-chan *models.ProcessingResult {
	return wp.results
}
