.PHONY: all build clean test proto docker-up docker-down bench lint golangci-lint tidy fmt

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

# Дополнительные команды
tidy:
	go mod tidy

fmt:
	go fmt ./... 