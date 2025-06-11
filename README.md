# Diplom Distributed System – Phase 1 (MVP)

## Назначение
Учебный прототип высокопроизводительной Go-платформы:  
`API Gateway → Ingest → Processor (worker-pool)` с in-memory очередью.  
Фаза 1 демонстрирует сквозной поток данных, health-checks и покрытие тестами.

## Стек
| Компонент | Версия | Роль |
|-----------|--------|------|
| Go | 1.24 | код сервисов |
| Docker / Compose | 24.x / v3.8 | контейнеризация |
| BusyBox (Alpine) | 3.19 | минимальные образы |
| Protobuf / gRPC | v1.36 |IDL & future RPC|

## Быстрый старт (локально)
```bash
git clone <repo>
cd diploma
make proto build            # генерация + бинарники
make run-local              # 3 сервиса в терминалах
./scripts/test-system.sh    # Health + 1 сообщение
````

## Быстрый старт (Docker)

```bash
make docker-build           # собирает образы
make docker-up              # запускает compose; wait ~15 s
make integration-test       # сквозной тест 10 сообщений
make docker-down            # остановка
```

## Структура репозитория

```
cmd/             # main-пакеты сервисов
internal/
  queue/         # MemoryQueue + интерфейсы Publisher/Subscriber
  processor/     # WorkerPool
  client/        # HTTP-клиент Ingest→Processor
docker/          # Dockerfile.* + docker-compose.yml
scripts/         # интеграционный и нагрузочный тесты
```

## Метрики

* P95 latency (MVP): ≈ 10 ms
* Throughput baseline: \~4 k RPS на ноутбуке (4 vCPU)