# 🚀 Diplom Distributed System

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-24.x+-2496ED?style=flat&logo=docker)](https://docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Высокопроизводительная Go-платформа для обработки данных в реальном времени**

Учебный прототип распределенной системы с архитектурой `API Gateway → Ingest → Processor (worker-pool)`, демонстрирующий сквозной поток данных, health-checks, метрики и покрытие тестами.

## 📋 Содержание

- [Быстрый старт](#-быстрый-старт)
- [Требования](#-требования) 
- [Установка](#-установка)
- [Поддерживаемые очереди](#-поддерживаемые-очереди)
- [Конфигурация](#-конфигурация)
- [API](#-api)
- [Архитектура](#-архитектура)
- [Команды Make](#-команды-make)
- [Тестирование](#-тестирование)
- [Качество кода](#-качество-кода)
- [Производительность](#-производительность)
- [Развертывание](#-развертывание)

## 🚀 Быстрый старт

### Локально (без Docker)
```bash
git clone https://github.com/stsolovey/diplom-distributed-system.git
cd diplom-distributed-system
make proto build            # генерация protobuf + сборка
make run-local              # запуск всех сервисов
```

### Через Docker
```bash
# Memory очередь (по умолчанию)
make docker-build docker-up

# NATS JetStream
QUEUE_TYPE=nats make docker-up

# Apache Kafka
QUEUE_TYPE=kafka make docker-up

# Composite (dual-write в NATS + Kafka)
QUEUE_TYPE=composite COMPOSITE_PROVIDERS=nats,kafka make docker-up
```

### Быстрая проверка
```bash
# Отправка тестового сообщения
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"source":"test","data":"Hello World","metadata":{"type":"demo"}}'

# Проверка статуса системы
curl http://localhost:8080/api/v1/status | jq .
```

## 🔧 Требования

| Компонент | Версия | Назначение |
|-----------|--------|------------|
| **Go** | 1.24+ | Основной язык разработки |
| **Docker** | 24.x+ | Контейнеризация |
| **Docker Compose** | v2+ | Оркестрация сервисов |
| **protoc** | 3.21+ | Компиляция protobuf |
| **protoc-gen-go** | latest | Go генератор для protobuf |
| **make** | 4.3+ | Автоматизация сборки |

### Дополнительные инструменты
- **jq** - для обработки JSON в скриптах
- **ab** (ApacheBench) - для нагрузочного тестирования
- **golangci-lint** - для проверки качества кода

## 📦 Установка

```bash
# 1. Клонирование репозитория
git clone https://github.com/stsolovey/diplom-distributed-system.git
cd diplom-distributed-system

# 2. Установка зависимостей Go
go mod tidy

# 3. Генерация protobuf кода
make proto

# 4. Сборка всех сервисов
make build

# 5. Проверка установки
./bin/api-gateway --help || echo "API Gateway ready"
./bin/ingest --help || echo "Ingest ready"  
./bin/processor --help || echo "Processor ready"
```

## 🔄 Поддерживаемые очереди

Система поддерживает четыре типа очередей сообщений:

### 1. Memory (фаза 1)
In-memory очередь для разработки и тестирования.
```bash
QUEUE_TYPE=memory make docker-up
```

### 2. NATS JetStream (фаза 2-а)
Высокопроизводительный message broker для production.
```bash
QUEUE_TYPE=nats NATS_URL=nats://localhost:4222 make docker-up
```

### 3. Apache Kafka (фаза 2-б)
Enterprise-grade очередь с персистентностью и масштабируемостью.
```bash
QUEUE_TYPE=kafka \
KAFKA_BROKERS=localhost:29092 \
KAFKA_TOPIC=diplom-messages \
make docker-up
```

### 4. Composite (Dual-Write)
Позволяет писать одновременно в несколько брокеров — полезно для миграций, репликации и A/B-тестов.

| Переменная          | Пример                | Что делает |
|---------------------|-----------------------|------------|
| `QUEUE_TYPE`        | `composite`           | Включает адаптер |
| `COMPOSITE_PROVIDERS` | `nats,kafka`          | Список провайдеров |
| `COMPOSITE_STRATEGY`  | `fail-fast` \| `best-effort` | Стратегия обработки ошибок |

**Примеры запуска:**

```bash
# Fail-Fast migration (NATS+Kafka) - останавливается при первой ошибке
QUEUE_TYPE=composite \
COMPOSITE_PROVIDERS=nats,kafka \
COMPOSITE_STRATEGY=fail-fast \
make docker-up
```

```bash
# Best-Effort репликация - логирует ошибки, но продолжает работу
QUEUE_TYPE=composite \
COMPOSITE_PROVIDERS=nats,kafka \
COMPOSITE_STRATEGY=best-effort \
make docker-up
```

## ⚙️ Конфигурация

### Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| **Основные сервисы** |
| `API_PORT` | `8080` | Порт API Gateway |
| `INGEST_PORT` | `8081` | Порт Ingest сервиса |
| `PROCESSOR_PORT` | `8082` | Порт Processor сервиса |
| `PROCESSOR_WORKERS` | `4` | Количество worker'ов в pool |
| `PROCESSOR_URL` | `http://localhost:8082` | URL Processor для Ingest |
| **Очереди** |
| `QUEUE_SIZE` | `1000` | Размер in-memory очереди |
| `QUEUE_TYPE` | `memory` | Тип очереди (`memory` \| `nats` \| `kafka` \| `composite`) |
| **NATS** |
| `NATS_URL` | `nats://localhost:4222` | URL для подключения к NATS |
| **Kafka** |
| `KAFKA_BROKERS` | `kafka:29092` | Список брокеров через "," |
| `KAFKA_TOPIC` | `diplom-messages` | Топик для публикации |
| `KAFKA_CONSUMER_GROUP` | `processor-group` | Группа консьюмеров |
| **Composite (Dual-Write)** |
| `COMPOSITE_PROVIDERS` | `nats,kafka` | Очередь(и) для dual-write |
| `COMPOSITE_STRATEGY` | `fail-fast` | **fail-fast** / **best-effort** |

### Пример конфигурации

Создайте файл `.env`:
```bash
# Базовая конфигурация
API_PORT=8080
INGEST_PORT=8081
PROCESSOR_PORT=8082

# Настройки производительности
PROCESSOR_WORKERS=8
QUEUE_SIZE=2000

# Composite dual-write в NATS + Kafka
QUEUE_TYPE=composite
COMPOSITE_PROVIDERS=nats,kafka
COMPOSITE_STRATEGY=fail-fast

# NATS настройки
NATS_URL=nats://localhost:4222

# Kafka настройки
KAFKA_BROKERS=localhost:29092
KAFKA_TOPIC=diplom-messages
KAFKA_CONSUMER_GROUP=processor-group
```

Применение: `source .env && make run-local`

## 🌐 API

### API Gateway (`:8080`)

#### `POST /api/v1/ingest`
Прием данных через прокси к Ingest сервису.

**Запрос:**
```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source": "sensor-01",
    "data": "temperature:23.5",
    "metadata": {
      "location": "warehouse-A",
      "timestamp": "2024-01-15T10:30:00Z"
    }
  }'
```

**Ответ:**
```json
{
  "messageId": "123e4567-e89b-12d3-a456-426614174000",
  "status": "accepted"
}
```

#### `GET /api/v1/status`
Агрегированный статус всех сервисов.

**Ответ:**
```json
{
  "ingest": {
    "healthy": true,
    "stats": {
      "TotalReceived": 150,
      "TotalSent": 148,
      "TotalFailed": 2
    }
  },
  "processor": {
    "healthy": true,
    "stats": {
      "queue": {"size": 5, "capacity": 1000},
      "pool": {"processed": 148, "errors": 0, "workers": 4}
    }
  }
}
```

#### `GET /health`
Health check API Gateway.

### Ingest Service (`:8081`)

#### `POST /ingest`
Прямой прием данных.

#### `GET /stats`
Статистика Ingest сервиса.

#### `GET /health`
Health check Ingest.

### Processor Service (`:8082`)

#### `POST /enqueue`
Прямое добавление сообщений в очередь.

#### `GET /stats`
Статистика Processor и очереди.

#### `GET /health`
Health check Processor.

## 🏗️ Архитектура

```mermaid
flowchart TB
    %% Core chain
    client[Client] --> gateway["API Gateway<br>:8080"]
    gateway --> ingest["Ingest<br>:8081"]
    ingest --> processor["Processor<br>:8082"]

    %% Вертикальные детализирующие ветки
    subgraph Details
        direction TB
        gateway --> lb["Load Balancer<br>(future)"]
        ingest --> http["HTTP Client"]
        processor --> pool["Worker Pool<br>(4 workers)"]
    end

    %% Кластер очередей
    subgraph QueueCluster["Queue Cluster"]
        direction TB
        memory["Memory"]
        nats["NATS JetStream"]
        kafka["Apache Kafka"]
        composite["Composite"]
        
        composite -.-> nats
        composite -.-> kafka
        composite -.-> memory
    end
    
    pool <--> QueueCluster
```

### Поток данных

1. **Client** отправляет HTTP POST запрос в **API Gateway**
2. **API Gateway** проксирует запрос в **Ingest** сервис
3. **Ingest** создает сообщение и отправляет в **Processor** через HTTP
4. **Processor** добавляет сообщение в очередь (Memory/NATS/Kafka/Composite)
5. **Worker Pool** обрабатывает сообщения из очереди асинхронно
6. **Composite Adapter** может дублировать сообщения в несколько очередей
7. Статистики и health checks доступны на всех уровнях

## 🛠️ Команды Make

| Команда | Описание |
|---------|----------|
| `make all` | Генерация protobuf + сборка |
| `make build` | Сборка всех сервисов в `bin/` |
| `make proto` | Генерация `.pb.go` файлов |
| `make clean` | Очистка артефактов сборки |
| `make run-local` | Запуск всех сервисов локально |
| `make switch-queue QUEUE=nats` | Быстрое переключение типа очереди |
| **Тестирование** |
| `make test` | Запуск всех тестов |
| `make test-coverage` | Тесты + HTML отчет покрытия |
| `make bench` | Бенчмарки производительности |
| `make integration-test` | Сквозной тест (все 4 типа очередей) |
| `make load-test` | Нагрузочный тест (ApacheBench) |
| **Docker** |
| `make docker-build` | Сборка Docker образов |
| `make docker-up` | Запуск через docker-compose |
| `make docker-down` | Остановка контейнеров |
| `make docker-logs` | Просмотр логов контейнеров |
| **Качество кода** |
| `make lint` | Запуск golangci-lint |
| `make fmt` | Форматирование кода |
| `make tidy` | Очистка go.mod |

## 🧪 Тестирование

### Юнит тесты
```bash
make test
# Результат: охват 84.8% (processor), включая CompositeAdapter и Kafka
```

### Интеграционные тесты  
```bash
make integration-test
# Тестирует полный цикл через Docker для всех 4 типов очередей:
# - Memory (быстрый тест)
# - NATS JetStream
# - Apache Kafka (с testcontainers)
# - Composite (NATS + Kafka dual-write)
```

### Нагрузочное тестирование
```bash
make load-test  
# 1000 запросов, 10 параллельных соединений через ApacheBench
```

### Покрытие кода
```bash
make test-coverage
# Создает coverage.html с детальным отчетом
```

### Тестирование разных очередей
```bash
# NATS JetStream
QUEUE_TYPE=nats make docker-up
./scripts/test-nats-integration.sh

# Apache Kafka
QUEUE_TYPE=kafka make docker-up
./scripts/test-kafka-integration.sh

# Composite dual-write
QUEUE_TYPE=composite COMPOSITE_PROVIDERS=nats,kafka make docker-up
./scripts/test-composite-integration.sh
```

## 🔍 Качество кода

### Линтеры
Проект использует `golangci-lint` с настроенными правилами:

```bash
make lint
# Проверяет: err113, gochecknoglobals, godot, mnd, wsl, 
# nlreturn, protogetter, tagliatelle, revive, funlen, 
# gocognit, nestif, gocritic, ireturn, forbidigo
```

### Соглашения
- **Conventional Commits**: `feat:`, `fix:`, `docs:`, `refactor:`
- **Error handling**: используются статические ошибки с `errors.Is`
- **Naming**: отсутствие type stuttering
- **Timeouts**: все HTTP операции с явными таймаутами

### Пример коммита
```bash
git commit -m "feat: add Kafka integration and CompositeAdapter

- Implement Kafka provider with SyncProducer and ConsumerGroup
- Add CompositeAdapter for dual-write functionality
- Support fail-fast and best-effort strategies
- Add comprehensive testcontainer-based tests
- Update configuration and factory patterns

Closes #123"
```

## 📊 Производительность

### Метрики по типам очередей

| Тип очереди | P95 latency | Throughput | Memory | Особенности |
|-------------|-------------|------------|---------|-------------|
| **Memory** | ~10ms | ~4k RPS | ~50MB | Быстрая, не персистентная |
| **NATS** | ~15ms | ~3k RPS | ~70MB | At-least-once, clustering |
| **Kafka** | ~25ms | ~2k RPS | ~100MB | Exactly-once, партиционирование |
| **Composite** | ~30ms | ~1.5k RPS | ~120MB | Dual-write overhead |

*Условия: локальное тестирование, 4 vCPU, 4 worker'а*

### Мониторинг
```bash
# Получение метрик в реальном времени
watch -n 1 'curl -s http://localhost:8080/api/v1/status | jq .'

# Нагрузочный тест с мониторингом
make load-test && curl -s http://localhost:8082/stats | jq .

# Composite адаптер показывает агрегированные метрики всех провайдеров
curl -s http://localhost:8082/stats | jq '.queue.composite_stats'
```

### Специфика метрик
- **Kafka адаптер**: не отдаёт `CurrentSize` (размер топика недоступен)
- **Composite stats**: агрегируют метрики всех дочерних брокеров
- **NATS JetStream**: показывает размер stream'а в реальном времени

## 🚀 Развертывание

### Development
```bash
make run-local
```

### Staging/Production
```bash
# С внешним NATS кластером
export NATS_URL="nats://nats-cluster:4222"
export QUEUE_TYPE="nats"
export PROCESSOR_WORKERS=8

make docker-up
```

### Kubernetes (Phase 3)
Планируется поддержка Helm charts и Kubernetes deployments.

### Health Checks
Все сервисы предоставляют endpoints для Kubernetes probes:
- **Readiness**: `GET /health`
- **Liveness**: `GET /health`

## 📁 Структура проекта

```
├── cmd/                    # Точки входа сервисов
│   ├── api-gateway/       # HTTP Gateway (порт 8080)
│   ├── ingest/            # Data Ingest Service (порт 8081)  
│   └── processor/         # Message Processor (порт 8082)
├── internal/              # Внутренние пакеты
│   ├── client/           # HTTP клиенты
│   ├── config/           # Конфигурация
│   ├── models/           # Protobuf модели
│   ├── processor/        # Worker pool implementation
│   └── queue/            # Очереди (Memory, NATS, Kafka, Composite)
│       ├── kafka_*.go    # Kafka provider implementation
│       └── composite_*.go # Composite dual-write adapter
├── api/proto/            # Protobuf определения
├── docker/               # Docker конфигурации
├── scripts/              # Скрипты автоматизации
├── docs/                 # Дополнительная документация
└── bin/                  # Скомпилированные бинарники
```

## 🗺️ Roadmap

### Phase 1 (✅ Завершена)
- [x] MVP с in-memory очередью
- [x] HTTP API и health checks
- [x] Интеграционные тесты
- [x] Docker контейнеризация
- [x] Базовые метрики

### Phase 2 (✅ Завершена)
- [x] NATS JetStream интеграция
- [x] Apache Kafka интеграция (KRaft mode)
- [x] CompositeAdapter для dual-write
- [x] Comprehensive тестирование (testcontainers)
- [x] Factory pattern для всех провайдеров

### Phase 3 (🚧 В планах)
- [ ] Оптимизация и Observability
- [ ] Metrics (Prometheus/Grafana)
- [ ] Distributed Tracing (Jaeger)
- [ ] Kubernetes deployment + Helm
- [ ] Горизонтальное масштабирование
- [ ] Circuit breakers и rate limiting

Подробности в [docs/ФАЗА_2_5.md](docs/ФАЗА_2_5.md)

## 🤝 Contributing

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'feat: add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Создайте Pull Request

### Перед отправкой PR
```bash
make lint test         # Проверка качества + тесты
make integration-test  # Полный интеграционный тест (все очереди)
```

## 📜 License

Распространяется под лицензией MIT. См. [LICENSE](LICENSE) для деталей.

---

<div align="center">

**[⭐ Star this repo](https://github.com/stsolovey/diplom-distributed-system)** • **[🐛 Report Bug](https://github.com/stsolovey/diplom-distributed-system/issues)** • **[💡 Request Feature](https://github.com/stsolovey/diplom-distributed-system/issues)**

Made with ❤️ for distributed systems learning

</div>