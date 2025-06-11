#!/bin/bash

echo "Running load test..."

# Убедимся, что сервисы запущены
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "Services are not running. Start them first with 'make docker-up'"
    exit 1
fi

# Создаем файл с payload
cat > /tmp/test-payload.json <<EOF
{
    "source": "load-test",
    "data": "Lorem ipsum dolor sit amet, consectetur adipiscing elit",
    "metadata": {"test": "load", "timestamp": "$(date +%s)"}
}
EOF

# Запускаем нагрузку: 1000 запросов, 10 параллельных
echo "Sending 1000 requests with 10 concurrent connections..."
ab -n 1000 -c 10 -p /tmp/test-payload.json \
   -T application/json \
   http://localhost:8080/api/v1/ingest

# Проверяем статистику после теста
echo -e "\nChecking statistics after load test..."
sleep 2
curl -s http://localhost:8080/api/v1/status | jq .

# Cleanup
rm /tmp/test-payload.json 