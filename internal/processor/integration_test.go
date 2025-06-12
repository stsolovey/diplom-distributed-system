package processor

import (
	"context"
	"testing"
	"time"

	"github.com/stsolovey/diplom-distributed-system/internal/models"
	"github.com/stsolovey/diplom-distributed-system/internal/queue"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupNATSContainer запускает контейнер NATS для тестов и возвращает URL для подключения.
func setupNATSContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "nats:2.10-alpine",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForLog("Starting JetStream"),
	}

	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("could not start nats container: %s", err)
	}

	// Добавляем Cleanup для автоматической остановки контейнера после теста.
	t.Cleanup(func() {
		if err := natsContainer.Terminate(ctx); err != nil {
			t.Fatalf("could not terminate nats container: %s", err)
		}
	})

	endpoint, err := natsContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("could not get nats container endpoint: %s", err)
	}

	return endpoint
}

// TestWorkerPool_WithNATS тестирует интеграцию WorkerPool с NATS JetStream
func TestWorkerPool_WithNATS(t *testing.T) {
	// Запускаем NATS контейнер для этого теста
	natsEndpoint := setupNATSContainer(t)
	t.Logf("NATS container started at: %s", natsEndpoint)

	// Создаем NATS адаптер для тестирования
	adapter, err := queue.NewNATSAdapter(natsEndpoint, "test")
	if err != nil {
		// Если контейнер запущен, здесь не должно быть ошибки подключения
		t.Fatalf("Failed to create NATS adapter with endpoint %s: %v", natsEndpoint, err)
	}
	defer adapter.Close()

	// Создаем WorkerPool с NATS
	workerPool := NewWorkerPool(2, adapter)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Запускаем WorkerPool
	err = workerPool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	// Тестовые сообщения
	testMessages := []*models.DataMessage{
		{Id: "test1", Source: "integration_test", Payload: []byte("message 1")},
		{Id: "test2", Source: "integration_test", Payload: []byte("message 2")},
		{Id: "test3", Source: "integration_test", Payload: []byte("message 3")},
		{Id: "test4", Source: "integration_test", Payload: []byte("message 4")},
		{Id: "test5", Source: "integration_test", Payload: []byte("message 5")},
	}

	// Публикуем сообщения
	for _, msg := range testMessages {
		err := adapter.Publish(ctx, msg)
		if err != nil {
			t.Fatalf("Failed to publish message %s: %v", msg.Id, err)
		}
	}

	// Собираем результаты обработки
	results := make([]*models.ProcessingResult, 0, len(testMessages))
	timeout := time.After(20 * time.Second)

	for i := 0; i < len(testMessages); i++ {
		select {
		case result := <-workerPool.Results():
			results = append(results, result)
			t.Logf("Processed message: %s", result.MessageId)
		case <-timeout:
			t.Fatalf("Timeout waiting for results. Got %d/%d results", len(results), len(testMessages))
		}
	}

	// Проверяем результаты
	if len(results) != len(testMessages) {
		t.Fatalf("Expected %d results, got %d", len(testMessages), len(results))
	}

	// Проверяем, что все сообщения обработаны успешно
	processedIds := make(map[string]bool)
	for _, result := range results {
		if !result.Success {
			t.Errorf("Message %s was not processed successfully", result.MessageId)
		}
		processedIds[result.MessageId] = true
	}

	// Проверяем, что все исходные сообщения обработаны
	for _, msg := range testMessages {
		if !processedIds[msg.Id] {
			t.Errorf("Message %s was not processed", msg.Id)
		}
	}

	// Проверяем статистику WorkerPool
	stats := workerPool.GetStats()
	if stats.ProcessedCount != int64(len(testMessages)) {
		t.Errorf("Expected ProcessedCount=%d, got %d", len(testMessages), stats.ProcessedCount)
	}

	if stats.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount=0, got %d", stats.ErrorCount)
	}

	// Проверяем статистику адаптера
	adapterStats := adapter.Stats()
	if adapterStats.TotalEnqueued != int64(len(testMessages)) {
		t.Errorf("Expected TotalEnqueued=%d, got %d", len(testMessages), adapterStats.TotalEnqueued)
	}

	if adapterStats.TotalDequeued != int64(len(testMessages)) {
		t.Errorf("Expected TotalDequeued=%d, got %d", len(testMessages), adapterStats.TotalDequeued)
	}

	t.Logf("NATS integration test completed successfully")
	t.Logf("WorkerPool stats: Processed=%d, Errors=%d", stats.ProcessedCount, stats.ErrorCount)
	t.Logf("Adapter stats: Enqueued=%d, Dequeued=%d, CurrentSize=%d",
		adapterStats.TotalEnqueued, adapterStats.TotalDequeued, adapterStats.CurrentSize)
}

// TestWorkerPool_WithMemory базовый тест для MemoryQueue для сравнения
func TestWorkerPool_WithMemory(t *testing.T) {
	// Создаем MemoryQueue
	memQueue := queue.NewMemoryQueue(100)
	defer memQueue.Close()

	// Создаем WorkerPool с MemoryQueue
	workerPool := NewWorkerPool(2, memQueue)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Запускаем WorkerPool
	err := workerPool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	// Тестовые сообщения
	testMessages := []*models.DataMessage{
		{Id: "mem1", Source: "memory_test", Payload: []byte("memory message 1")},
		{Id: "mem2", Source: "memory_test", Payload: []byte("memory message 2")},
		{Id: "mem3", Source: "memory_test", Payload: []byte("memory message 3")},
	}

	// Публикуем сообщения в MemoryQueue
	for _, msg := range testMessages {
		err := memQueue.Publish(ctx, msg)
		if err != nil {
			t.Fatalf("Failed to publish message %s: %v", msg.Id, err)
		}
	}

	// Собираем результаты
	results := make([]*models.ProcessingResult, 0, len(testMessages))
	timeout := time.After(5 * time.Second)

	for i := 0; i < len(testMessages); i++ {
		select {
		case result := <-workerPool.Results():
			results = append(results, result)
		case <-timeout:
			t.Fatalf("Timeout waiting for results. Got %d/%d results", len(results), len(testMessages))
		}
	}

	// Проверяем результаты
	if len(results) != len(testMessages) {
		t.Fatalf("Expected %d results, got %d", len(testMessages), len(results))
	}

	t.Logf("Memory queue integration test completed successfully")
}
