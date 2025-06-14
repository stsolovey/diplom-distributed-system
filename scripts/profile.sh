#!/bin/bash
set -e

PROFILE_DIR="results/profiling/baseline"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "=== Go Performance Profiling ==="
echo "Timestamp: $TIMESTAMP"
echo "Output dir: $PROFILE_DIR"

# Создаем директорию для текущего прогона
mkdir -p "$PROFILE_DIR/$TIMESTAMP"

echo "Current working directory: $(pwd)"

# Функция для профилирования
profile_component() {
    local component=$1
    local package=$2
    
    echo -e "\n📊 Profiling $component..."
    
    # CPU профиль
    echo "  🔥 CPU profiling..."
    go test -bench=. -benchtime=30s -cpuprofile="$PROFILE_DIR/$TIMESTAMP/cpu_${component}.prof" $package || echo "No benchmarks found for $component CPU"
    
    # Memory профиль
    echo "  🧠 Memory profiling..."
    go test -bench=. -benchtime=30s -memprofile="$PROFILE_DIR/$TIMESTAMP/mem_${component}.prof" $package || echo "No benchmarks found for $component Memory"
    
    # Trace (для анализа latency)
    echo "  📈 Trace profiling..."
    go test -bench=. -benchtime=10s -trace="$PROFILE_DIR/$TIMESTAMP/trace_${component}.out" $package || echo "No benchmarks found for $component Trace"
}

# Профилируем компоненты
profile_component "processor" "./internal/processor"
profile_component "queue" "./internal/queue"
profile_component "api-gateway" "./cmd/api-gateway"

echo -e "\n✅ Profiling completed!"
echo "Results saved in: $PROFILE_DIR/$TIMESTAMP"
echo "To analyze CPU profile: go tool pprof -http=:8080 $PROFILE_DIR/$TIMESTAMP/cpu_processor.prof"
echo "To analyze Memory profile: go tool pprof -http=:8081 $PROFILE_DIR/$TIMESTAMP/mem_processor.prof"
echo "To analyze Trace: go tool trace $PROFILE_DIR/$TIMESTAMP/trace_processor.out" 