.PHONY: all build clean test proto docker-up docker-down bench lint golangci-lint tidy fmt phase4-install phase4-check phase4-run phase4-demo phase4-monitoring phase4-analyze

# Переменные
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard *.go)

# Команды сборки
all: proto build

build:
	@echo "Building api-gateway..."
	@go build -o $(GOBIN)/api-gateway ./cmd/api-gateway
	@echo "Building processor..."
	@go build -o $(GOBIN)/processor ./cmd/processor
	@echo "Building ingest..."
	@go build -o $(GOBIN)/ingest ./cmd/ingest

proto:
	@echo "Generating protobuf code..."
	@mkdir -p internal/models
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--proto_path=api/proto \
		api/proto/*.proto

test:
	@go test -v -race ./...

test-coverage:
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

bench:
	@go test -bench=. -benchmem ./...

clean:
	@rm -rf $(GOBIN)
	@rm -f coverage.out coverage.html
	@go clean

docker-build:
	@docker-compose -f docker/docker-compose.yml build

docker-up:
	@docker-compose -f docker/docker-compose.yml up -d

docker-down:
	@docker-compose -f docker/docker-compose.yml down

docker-logs:
	@docker-compose -f docker/docker-compose.yml logs -f

run-local:
	@echo "Starting services locally..."
	@go run ./cmd/processor &
	@sleep 1
	@go run ./cmd/ingest &
	@sleep 1
	@go run ./cmd/api-gateway

integration-test:
	@chmod +x scripts/integration-test.sh
	@./scripts/integration-test.sh

load-test:
	@chmod +x scripts/load-test.sh
	@./scripts/load-test.sh

# Линтер
lint: golangci-lint

golangci-lint:
	$(HOME)/go/bin/golangci-lint version
	$(HOME)/go/bin/golangci-lint run

# Профилирование
profile-complete:
	@chmod +x scripts/complete_profiling.sh
	@./scripts/complete_profiling.sh

# gRPC команды
grpc-server:
	@echo "Building gRPC server..."
	@go build -o $(GOBIN)/grpc-server ./cmd/grpc-server

# Сетевые тесты
test-network:
	@echo "Testing network optimizations..."
	@go test -v -run TestOptimizedClient ./internal/client
	@go test -v -run TestTracedClient ./internal/client

# Дополнительные команды
tidy:
	go mod tidy

fmt:
	go fmt ./... 

# ============================================================================
# PHASE 4: LOAD TESTING & MONITORING
# ============================================================================

# Установка k6 и зависимостей
phase4-install:
	@echo "🔧 Installing Phase 4 dependencies..."
	@chmod +x scripts/install-k6.sh
	@./scripts/install-k6.sh
	@echo "Installing Python dependencies..."
	@pip3 install -r requirements-phase4.txt
	@echo "✅ Phase 4 dependencies installed"

# Проверка готовности системы
phase4-check:
	@echo "🔍 Checking Phase 4 readiness..."
	@chmod +x scripts/check-phase4-readiness.sh
	@./scripts/check-phase4-readiness.sh

# Запуск полного цикла нагрузочного тестирования
phase4-run:
	@echo "🚀 Starting Phase 4 comprehensive load testing..."
	@chmod +x scripts/run-all-tests.sh
	@./scripts/run-all-tests.sh

# Быстрая демонстрация с мониторингом
phase4-demo:
	@echo "🎯 Running Phase 4 demo (smoke + load test)..."
	@chmod +x scripts/run-all-tests.sh
	@SKIP_SOAK=true ./scripts/run-all-tests.sh smoke load

# Запуск только мониторинга
phase4-monitoring:
	@echo "📊 Starting monitoring infrastructure..."
	@docker-compose -f docker/monitoring-compose.yml up -d
	@echo "Monitoring services started:"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana: http://localhost:3000 (admin/admin123)"
	@echo "  AlertManager: http://localhost:9093"

# Остановка мониторинга
phase4-monitoring-down:
	@echo "🛑 Stopping monitoring infrastructure..."
	@docker-compose -f docker/monitoring-compose.yml down

# Анализ результатов тестирования
phase4-analyze:
	@echo "📈 Analyzing test results..."
	@if [ -z "$(RESULTS_DIR)" ]; then \
		echo "Usage: make phase4-analyze RESULTS_DIR=path/to/results"; \
		echo "Example: make phase4-analyze RESULTS_DIR=results/load_testing/20240101_120000"; \
	else \
		chmod +x scripts/analyze-results.py; \
		python3 scripts/analyze-results.py $(RESULTS_DIR) --all; \
	fi

# Отдельные тесты
phase4-smoke:
	@echo "💨 Running smoke test..."
	@chmod +x scripts/run-all-tests.sh
	@./scripts/run-all-tests.sh smoke

phase4-load:
	@echo "⚡ Running load test..."
	@chmod +x scripts/run-all-tests.sh
	@./scripts/run-all-tests.sh load

phase4-spike:
	@echo "📈 Running spike test..."
	@chmod +x scripts/run-all-tests.sh
	@./scripts/run-all-tests.sh spike

phase4-soak:
	@echo "🕐 Running soak test (2 hours)..."
	@chmod +x scripts/run-all-tests.sh
	@./scripts/run-all-tests.sh soak

# Очистка результатов тестирования
phase4-clean:
	@echo "🧹 Cleaning Phase 4 test results..."
	@rm -rf results/load_testing/*
	@echo "✅ Test results cleaned"

# Справка по Phase 4
phase4-help:
	@echo "📚 Phase 4 Load Testing Commands:"
	@echo ""
	@echo "Setup & Preparation:"
	@echo "  make phase4-install     - Install k6 and Python dependencies"
	@echo "  make phase4-check       - Check system readiness"
	@echo ""
	@echo "Testing:"
	@echo "  make phase4-run         - Run complete test suite"
	@echo "  make phase4-demo        - Quick demo (smoke + load)"
	@echo "  make phase4-smoke       - Smoke test only"
	@echo "  make phase4-load        - Load test only"
	@echo "  make phase4-spike       - Spike test only"
	@echo "  make phase4-soak        - Soak test only (2 hours)"
	@echo ""
	@echo "Monitoring:"
	@echo "  make phase4-monitoring  - Start monitoring stack"
	@echo "  make phase4-monitoring-down - Stop monitoring"
	@echo ""
	@echo "Analysis:"
	@echo "  make phase4-analyze RESULTS_DIR=path - Analyze test results"
	@echo ""
	@echo "Maintenance:"
	@echo "  make phase4-clean       - Clean test results"
	@echo "  make phase4-help        - Show this help"

# ============================================================================
# DEMO & PRODUCTION COMMANDS
# ============================================================================

# Полная демонстрация системы за 30 секунд
demo:
	@echo "🎯 Starting 30-second system demo..."
	@echo "1/4 Building system..."
	@make proto build
	@echo "2/4 Starting services with monitoring..."
	@make docker-up-full
	@sleep 10
	@echo "3/4 Running health checks..."
	@make health-check
	@echo "4/4 Running quick load test..."
	@make load-test-quick
	@echo "✅ Demo complete! Check http://localhost:3000 for Grafana"

# Production-ready запуск с полным мониторингом
docker-up-full:
	@echo "🚀 Starting production-ready system..."
	@docker-compose -f docker/docker-compose.yml up -d
	@docker-compose -f docker/monitoring-compose.yml up -d
	@echo "Services started:"
	@echo "  API Gateway: http://localhost:8080"
	@echo "  Grafana: http://localhost:3000 (admin/admin)"
	@echo "  Prometheus: http://localhost:9090"

# Проверка работоспособности всех сервисов
health-check:
	@echo "🏥 Checking system health..."
	@curl -f http://localhost:8080/api/v1/status || echo "❌ API Gateway not ready"
	@curl -f http://localhost:8081/health || echo "❌ Ingest not ready"
	@curl -f http://localhost:8082/health || echo "❌ Processor not ready"
	@echo "✅ Health check complete"

# Быстрое нагрузочное тестирование
load-test-quick:
	@echo "⚡ Running quick load test..."
	@if command -v k6 >/dev/null 2>&1; then \
		k6 run --duration 30s --vus 10 k6/scenarios/quick-demo.js; \
	else \
		echo "k6 not found, using curl..."; \
		for i in {1..50}; do \
			curl -X POST http://localhost:8080/api/v1/ingest \
				-H "Content-Type: application/json" \
				-d '{"source":"demo","data":"test message '$$i'"}' & \
		done; \
		wait; \
	fi
	@echo "✅ Load test complete" 