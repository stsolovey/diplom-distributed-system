#!/bin/bash
set -e

echo "=== Integration Test ==="

# 1. Запуск сервисов
echo "Starting services..."
docker-compose -f docker/docker-compose.yml up -d

# Ждем готовности
echo "Waiting for services..."
sleep 5

# 2. Health checks
echo "Checking health..."
for port in 8080 8081 8082; do
    if ! curl -sf http://localhost:$port/health > /dev/null; then
        echo "Service on port $port is not healthy"
        docker-compose -f docker/docker-compose.yml logs
        exit 1
    fi
done
echo "All services are healthy!"

# 3. Отправляем тестовые сообщения
echo -e "\nSending test messages..."
for i in {1..10}; do
    curl -s -X POST http://localhost:8080/api/v1/ingest \
        -H "Content-Type: application/json" \
        -d "{
            \"source\": \"test-$i\",
            \"data\": \"Message $i\",
            \"metadata\": {\"seq\": \"$i\"}
        }" > /dev/null
    echo -n "."
done
echo " Done!"

# 4. Проверяем статистику
echo -e "\nChecking statistics..."
sleep 3

# Processor stats
STATS=$(curl -s http://localhost:8082/stats)
PROCESSED=$(echo $STATS | jq '.pool.ProcessedCount')
echo "Processed messages: $PROCESSED"

if [ "$PROCESSED" -lt 5 ]; then
    echo "ERROR: Expected at least 5 processed messages, got $PROCESSED"
    docker-compose -f docker/docker-compose.yml logs processor
    exit 1
fi

# 5. System status
echo -e "\nSystem status:"
curl -s http://localhost:8080/api/v1/status | jq .

# 6. Cleanup
echo -e "\nCleaning up..."
docker-compose -f docker/docker-compose.yml down

echo -e "\n✅ Integration test passed!" 