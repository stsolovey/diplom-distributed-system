#!/bin/bash
set -e

PROFILE_DIR="results/profiling/complete"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_DIR="$PROFILE_DIR/$TIMESTAMP"

echo "=== Complete Performance Profiling ==="
echo "Output: $OUTPUT_DIR"

mkdir -p "$OUTPUT_DIR"

# 1. CPU профилирование всех компонентов
echo "🔥 CPU Profiling..."
go test -bench=. -benchtime=30s -cpuprofile="$OUTPUT_DIR/cpu_queue.prof" ./internal/queue
go test -bench=. -benchtime=30s -cpuprofile="$OUTPUT_DIR/cpu_processor.prof" ./internal/processor
go test -bench=. -benchtime=30s -cpuprofile="$OUTPUT_DIR/cpu_client.prof" ./internal/client

# 2. Memory профилирование
echo "🧠 Memory Profiling..."
go test -bench=. -benchtime=30s -memprofile="$OUTPUT_DIR/mem_queue.prof" ./internal/queue
go test -bench=. -benchtime=30s -memprofile="$OUTPUT_DIR/mem_processor.prof" ./internal/processor

# 3. Block профилирование (конкуренция)
echo "🔒 Block Profiling..."
go test -bench=. -benchtime=10s -blockprofile="$OUTPUT_DIR/block_queue.prof" ./internal/queue

# 4. Mutex профилирование
echo "🔐 Mutex Profiling..."
go test -bench=. -benchtime=10s -mutexprofile="$OUTPUT_DIR/mutex_processor.prof" ./internal/processor

# 5. Генерация отчетов
echo "📊 Generating reports..."

# CPU hot paths
go tool pprof -top -nodecount=20 "$OUTPUT_DIR/cpu_queue.prof" > "$OUTPUT_DIR/cpu_queue_top.txt"
go tool pprof -top -nodecount=20 "$OUTPUT_DIR/cpu_processor.prof" > "$OUTPUT_DIR/cpu_processor_top.txt"

# Memory allocations
go tool pprof -top -alloc_space "$OUTPUT_DIR/mem_queue.prof" > "$OUTPUT_DIR/mem_queue_allocs.txt"

# Генерация flame graphs
echo "🔥 Generating flame graphs..."
go tool pprof -svg "$OUTPUT_DIR/cpu_queue.prof" > "$OUTPUT_DIR/cpu_queue_flame.svg"
go tool pprof -svg "$OUTPUT_DIR/cpu_processor.prof" > "$OUTPUT_DIR/cpu_processor_flame.svg"

echo "✅ Profiling complete!"
echo "Results saved to: $OUTPUT_DIR"
echo ""
echo "To view interactive CPU profile:"
echo "  go tool pprof $OUTPUT_DIR/cpu_processor.prof"
echo ""
echo "To view flame graphs, open SVG files in browser:"
echo "  firefox $OUTPUT_DIR/cpu_processor_flame.svg" 