#!/bin/bash

echo "Testing system..."

# Проверка здоровья
echo "Checking health..."
curl -s http://localhost:8080/health | jq .

# Отправка тестового сообщения
echo -e "\nSending test message..."
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "source": "test-client",
    "data": "Hello, distributed system!",
    "metadata": {
      "priority": "high",
      "type": "test"
    }
  }')

echo "Response: $RESPONSE"

# Проверка статуса системы
echo -e "\nChecking system status..."
sleep 2
curl -s http://localhost:8080/api/v1/status | jq . 