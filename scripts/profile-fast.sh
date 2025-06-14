#!/bin/bash
set -e

PROFILE_DIR="results/profiling/baseline"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "=== Fast Go Performance Profiling ==="
echo "Timestamp: $TIMESTAMP"
echo "Output dir: $PROFILE_DIR"

# Создаем директорию для текущего прогона
mkdir -p "$PROFILE_DIR/$TIMESTAMP"

echo "Current working directory: $(pwd)"

# Функция для быстрого профилирования
profile_component_fast() {
    local component=$1
    local package=$2
    
    echo -e "\n📊 Fast profiling $component..."
    
    # CPU профиль (5 секунд)
    echo "  🔥 CPU profiling (5s)..."
    go test -bench=. -benchtime=5s -cpuprofile="$PROFILE_DIR/$TIMESTAMP/cpu_${component}.prof" $package 2>/dev/null || echo "No benchmarks found for $component CPU"
    
    # Memory профиль (3 секунды)
    echo "  🧠 Memory profiling (3s)..."
    go test -bench=. -benchtime=3s -memprofile="$PROFILE_DIR/$TIMESTAMP/mem_${component}.prof" $package 2>/dev/null || echo "No benchmarks found for $component Memory"
    
    # Trace (2 секунды)
    echo "  📈 Trace profiling (2s)..."
    go test -bench=. -benchtime=2s -trace="$PROFILE_DIR/$TIMESTAMP/trace_${component}.out" $package 2>/dev/null || echo "No benchmarks found for $component Trace"
}

# Профилируем только ключевые компоненты
echo "🎯 Profiling key components..."
profile_component_fast "queue" "./internal/queue"
profile_component_fast "processor" "./internal/processor"
profile_component_fast "api-gateway" "./cmd/api-gateway"

echo -e "\n✅ Fast profiling completed!"
echo "Results saved in: $PROFILE_DIR/$TIMESTAMP"

# Проверяем какие файлы созданы
echo -e "\n📁 Generated files:"
ls -la "$PROFILE_DIR/$TIMESTAMP/" || echo "No profile files generated"

echo -e "\n🔧 Analysis commands:"
if [ -f "$PROFILE_DIR/$TIMESTAMP/cpu_queue.prof" ]; then
    echo "CPU Queue: go tool pprof -http=:8080 $PROFILE_DIR/$TIMESTAMP/cpu_queue.prof"
fi
if [ -f "$PROFILE_DIR/$TIMESTAMP/mem_queue.prof" ]; then
    echo "Memory Queue: go tool pprof -http=:8081 $PROFILE_DIR/$TIMESTAMP/mem_queue.prof"
fi
if [ -f "$PROFILE_DIR/$TIMESTAMP/trace_queue.out" ]; then
    echo "Trace Queue: go tool trace $PROFILE_DIR/$TIMESTAMP/trace_queue.out"
fi 