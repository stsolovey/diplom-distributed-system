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
make docker-build           # сборка образов
make docker-up              # запуск compose (wait ~15s)
make integration-test       # сквозной тест 10 сообщений
make docker-down            # остановка
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
| **make** | any | Автоматизация сборки |

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

## ⚙️ Конфигурация

### Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `API_PORT` | `8080` | Порт API Gateway |
| `INGEST_PORT` | `8081` | Порт Ingest сервиса |
| `PROCESSOR_PORT` | `8082` | Порт Processor сервиса |
| `PROCESSOR_WORKERS` | `4` | Количество worker'ов в pool |
| `PROCESSOR_URL` | `http://localhost:8082` | URL Processor для Ingest |
| `QUEUE_SIZE` | `1000` | Размер in-memory очереди |
| `QUEUE_TYPE` | `memory` | Тип очереди (`memory` \| `nats`) |
| `NATS_URL` | `nats://localhost:4222` | URL для подключения к NATS |

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

# Очередь NATS (опционально)
QUEUE_TYPE=nats
NATS_URL=nats://localhost:4222
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

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Client      │───▶│ API Gateway │───▶│ Ingest      │───▶│ Processor   │
│             │    │ :8080       │    │ :8081       │    │ :8082       │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
                   │ Load        │    │ HTTP        │    │ Worker Pool │
                   │ Balancer    │    │ Client      │    │ (4 workers) │
                   │ (future)    │    │             │    │             │
                   └─────────────┘    └─────────────┘    └─────────────┘
                                                                 │
                                                                 ▼
                                                     ┌─────────────────────┐
                                                     │ Queue               │
                                                     │ ┌─────────────────┐ │
                                                     │ │ Memory (Phase1) │ │
                                                     │ │ NATS (Phase2)   │ │
                                                     │ └─────────────────┘ │
                                                     └─────────────────────┘
```

### Поток данных

1. **Client** отправляет HTTP POST запрос в **API Gateway**
2. **API Gateway** проксирует запрос в **Ingest** сервис
3. **Ingest** создает сообщение и отправляет в **Processor** через HTTP
4. **Processor** добавляет сообщение в очередь
5. **Worker Pool** обрабатывает сообщения из очереди асинхронно
6. Статистики и health checks доступны на всех уровнях

## 🛠️ Команды Make

| Команда | Описание |
|---------|----------|
| `make all` | Генерация protobuf + сборка |
| `make build` | Сборка всех сервисов в `bin/` |
| `make proto` | Генерация `.pb.go` файлов |
| `make clean` | Очистка артефактов сборки |
| `make run-local` | Запуск всех сервисов локально |
| **Тестирование** |
| `make test` | Запуск всех тестов |
| `make test-coverage` | Тесты + HTML отчет покрытия |
| `make bench` | Бенчмарки производительности |
| `make integration-test` | Сквозной тест (Docker) |
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
# Результат: охват 84.8% (processor), 13.0% (queue)
```

### Интеграционные тесты  
```bash
make integration-test
# Тестирует полный цикл через Docker с 10 сообщениями
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

### Тестирование с NATS
```bash
QUEUE_TYPE=nats make docker-up
./scripts/test-nats-integration.sh
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
git commit -m "feat: add NATS queue support

- Implement NatsAdapter with JetStream
- Add queue factory pattern
- Update configuration for NATS_URL
- Add integration tests with testcontainers

Closes #123"
```

## 📊 Производительность

### Базовые метрики (Phase 1)

| Метрика | Значение | Условия |
|---------|----------|---------|
| **P95 latency** | ~10ms | Memory queue, 4 workers |
| **Throughput** | ~4k RPS | Локально, 4 vCPU |
| **Memory usage** | ~50MB | Per service |
| **Queue capacity** | 1000 msgs | In-memory buffer |

### Мониторинг
```bash
# Получение метрик в реальном времени
watch -n 1 'curl -s http://localhost:8080/api/v1/status | jq .'

# Нагрузочный тест с мониторингом
make load-test && curl -s http://localhost:8082/stats | jq .
```

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

### Kubernetes (будущее)
Планируется поддержка Helm charts и Kubernetes deployments в Phase 2.

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
│   └── queue/            # Очереди (Memory + NATS)
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

### Phase 2 (🚧 В планах)
- [ ] NATS JetStream интеграция
- [ ] Горизонтальное масштабирование
- [ ] Metrics (Prometheus)
- [ ] Tracing (Jaeger)
- [ ] Kubernetes deployment

Подробности в [docs/ФАЗА_2.md](docs/ФАЗА_2.md)

## 🤝 Contributing

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'feat: add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Создайте Pull Request

### Перед отправкой PR
```bash
make lint test         # Проверка качества + тесты
make integration-test  # Полный интеграционный тест
```

## 📜 License

Распространяется под лицензией MIT. См. [LICENSE](LICENSE) для деталей.

---

<div align="center">

**[⭐ Star this repo](https://github.com/stsolovey/diplom-distributed-system)** • **[🐛 Report Bug](https://github.com/stsolovey/diplom-distributed-system/issues)** • **[💡 Request Feature](https://github.com/stsolovey/diplom-distributed-system/issues)**

Made with ❤️ for distributed systems learning

</div>