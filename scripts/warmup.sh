#!/bin/bash
set -e

echo "🔥 Warming up system with 1000 messages..."

for i in {1..1000}; do
    curl -s -X POST http://localhost:8080/api/v1/ingest \
        -H "Content-Type: application/json" \
        -d '{"source":"warmup","data":"test"}' > /dev/null &
    
    # Каждые 100 запросов делаем паузу
    if [ $((i % 100)) -eq 0 ]; then
        wait
        echo "Warmup: $i/1000"
    fi
done
wait
echo "✅ Warmup completed!" 