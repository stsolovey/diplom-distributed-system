# Profiling Memory Bank

## Useful Commands

### CPU Profiling
```bash
# Basic CPU profile
go test -bench=. -cpuprofile=cpu.prof ./...

# With specific duration
go test -bench=. -benchtime=30s -cpuprofile=cpu.prof ./...

# Exclude integration tests
go test -bench=BenchmarkMemoryQueue -cpuprofile=cpu.prof ./internal/queue -run=^$ -timeout=30s

# Interactive analysis
go tool pprof -http=:8080 cpu.prof

# Top 20 functions
go tool pprof -top -cum cpu.prof | head -20

# Flame graph (requires graphviz)
go tool pprof -web cpu.prof

# Text output
go tool pprof -text cpu.prof > cpu_report.txt
```

### Memory Profiling
```bash
# Memory allocations
go test -bench=. -memprofile=mem.prof ./...

# With specific benchmark
go test -bench=BenchmarkMemoryQueue -memprofile=mem.prof ./internal/queue

# Analyze inuse memory (what's currently in memory)
go tool pprof -sample_index=inuse_space mem.prof

# Analyze allocations (all allocations over time)
go tool pprof -sample_index=alloc_space mem.prof

# Interactive memory analysis
go tool pprof -http=:8081 -sample_index=alloc_space mem.prof

# Top allocators
go tool pprof -top -alloc_space mem.prof
```

### Trace Analysis
```bash
# Generate trace
go test -bench=. -trace=trace.out ./...

# Specific benchmark with trace
go test -bench=BenchmarkMemoryQueue -trace=trace.out ./internal/queue -benchtime=5s

# View trace
go tool trace trace.out

# Export trace data
go tool trace -pprof=net trace.out > net.prof
go tool trace -pprof=sync trace.out > sync.prof
go tool trace -pprof=syscall trace.out > syscall.prof
```

### Comparing Profiles
```bash
# Compare two CPU profiles
go tool pprof -base=baseline.prof optimized.prof

# Compare with percentage difference
go tool pprof -base=baseline.prof -http=:8080 optimized.prof

# Diff memory profiles
go tool pprof -base=mem_before.prof -sample_index=alloc_space mem_after.prof
```

### Continuous Profiling
```bash
# Script for continuous profiling
#!/bin/bash
for i in {1..10}; do
    go test -bench=. -cpuprofile=cpu_${i}.prof ./internal/queue
    sleep 60
done

# Analyze trend
for prof in cpu_*.prof; do
    echo "=== $prof ===" 
    go tool pprof -top $prof | head -5
done
```

## Common Gotchas

### 1. Always Warm Up Before Profiling
```bash
# Wrong - cold start affects results
go test -bench=. -cpuprofile=cpu.prof ./...

# Right - warm up first
./scripts/warmup.sh
go test -bench=. -cpuprofile=cpu.prof ./...
```

### 2. Profile One Thing at a Time
```bash
# Wrong - mixed profiles are hard to analyze
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof ./...

# Right - separate profiles
go test -bench=. -cpuprofile=cpu.prof ./...
go test -bench=. -memprofile=mem.prof ./...
```

### 3. Use Appropriate Benchmark Time
```bash
# Too short - unreliable results
go test -bench=. -benchtime=1s -cpuprofile=cpu.prof ./...

# Too long - may hang on problematic code
go test -bench=. -benchtime=60s -cpuprofile=cpu.prof ./...

# Right - reasonable duration
go test -bench=. -benchtime=5s -cpuprofile=cpu.prof ./...
```

### 4. Save Baseline Before Optimizing
```bash
# Create baseline
mkdir -p profiles/baseline
go test -bench=. -cpuprofile=profiles/baseline/cpu.prof ./...

# After optimization
mkdir -p profiles/optimized  
go test -bench=. -cpuprofile=profiles/optimized/cpu.prof ./...

# Compare
go tool pprof -base=profiles/baseline/cpu.prof profiles/optimized/cpu.prof
```

### 5. Check Both CPU and Memory
```bash
# CPU might look good but memory terrible
go test -bench=. -cpuprofile=cpu.prof ./...      # 200ns/op ✅
go test -bench=. -memprofile=mem.prof ./...      # 1MB/op ❌

# Always check both
go test -bench=. -cpuprofile=cpu.prof ./...
go test -bench=. -memprofile=mem.prof ./...
```

## Optimization Patterns

### sync.Pool Pattern
```go
// Problem: frequent object allocation
func processMessage(data []byte) *Message {
    return &Message{             // Allocation!
        ID:   generateID(),
        Data: data,
    }
}

// Solution: object pool
var messagePool = sync.Pool{
    New: func() interface{} {
        return &Message{
            Data: make([]byte, 0, 1024), // Pre-allocated capacity
        }
    },
}

func processMessage(data []byte) *Message {
    msg := messagePool.Get().(*Message)
    
    // Reset for reuse
    msg.ID = generateID()
    msg.Data = msg.Data[:0]                    // Reset slice but keep capacity
    msg.Data = append(msg.Data, data...)       // Reuse underlying array
    
    return msg
}

func releaseMessage(msg *Message) {
    messagePool.Put(msg)
}
```

### Pre-allocation Pattern
```go
// Problem: slice growth reallocations
func processItems(items []Item) []Result {
    var results []Result                       // Starts with 0 capacity
    for _, item := range items {
        results = append(results, process(item)) // Multiple reallocations!
    }
    return results
}

// Solution: pre-allocate with known size
func processItems(items []Item) []Result {
    results := make([]Result, 0, len(items))   // Pre-allocate capacity
    for _, item := range items {
        results = append(results, process(item)) // No reallocations
    }
    return results
}

// Advanced: reuse slices
var resultPool = sync.Pool{
    New: func() interface{} {
        return make([]Result, 0, 100)          // Common size
    },
}

func processItems(items []Item) []Result {
    results := resultPool.Get().([]Result)
    results = results[:0]                      // Reset length but keep capacity
    
    for _, item := range items {
        results = append(results, process(item))
    }
    
    // Make copy for return, reuse slice
    ret := make([]Result, len(results))
    copy(ret, results)
    resultPool.Put(results)
    
    return ret
}
```

### String Builder Pattern
```go
// Problem: string concatenation allocations
func buildMessage(parts []string) string {
    var msg string
    for _, part := range parts {
        msg += part                            // Multiple allocations!
    }
    return msg
}

// Solution: strings.Builder
func buildMessage(parts []string) string {
    var builder strings.Builder
    
    // Pre-allocate if size known
    totalLen := 0
    for _, part := range parts {
        totalLen += len(part)
    }
    builder.Grow(totalLen)
    
    for _, part := range parts {
        builder.WriteString(part)              // No allocations
    }
    
    return builder.String()
}
```

### Buffer Pool Pattern
```go
// Problem: frequent buffer allocations
func processData(data []byte) ([]byte, error) {
    buf := bytes.NewBuffer(nil)                // Allocation!
    
    // Process data...
    buf.Write(data)
    
    return buf.Bytes(), nil
}

// Solution: buffer pool
var bufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, 4096))
    },
}

func processData(data []byte) ([]byte, error) {
    buf := bufferPool.Get().(*bytes.Buffer)
    buf.Reset()                                // Clear previous content
    
    // Process data...
    buf.Write(data)
    
    // Copy result before returning buffer to pool
    result := make([]byte, buf.Len())
    copy(result, buf.Bytes())
    
    bufferPool.Put(buf)
    
    return result, nil
}
```

### Batch Processing Pattern
```go
// Problem: high per-operation overhead
func processMessages(msgs []*Message) {
    for _, msg := range msgs {
        db.Save(msg)                           // N database calls
    }
}

// Solution: batch operations
func processMessages(msgs []*Message) {
    const batchSize = 100
    
    for i := 0; i < len(msgs); i += batchSize {
        end := i + batchSize
        if end > len(msgs) {
            end = len(msgs)
        }
        
        batch := msgs[i:end]
        db.SaveBatch(batch)                    // N/100 database calls
    }
}
```

## Performance Analysis Checklist

### Before Profiling
- [ ] System is warmed up
- [ ] Realistic test data
- [ ] Consistent load
- [ ] Stable environment
- [ ] No other processes consuming resources

### During Profiling
- [ ] Profile duration sufficient (5-30s)
- [ ] Single component focus
- [ ] Baseline measurements saved
- [ ] Multiple runs for consistency
- [ ] Both CPU and memory profiled

### After Profiling
- [ ] Hot paths identified
- [ ] Allocation sources found
- [ ] Optimization plan created
- [ ] Changes implemented incrementally
- [ ] Results validated with new profiles

## Troubleshooting

### Profile Files Empty or Small
```bash
# Check if benchmarks are running
go test -bench=. -v ./...

# Increase benchmark time
go test -bench=. -benchtime=10s -cpuprofile=cpu.prof ./...

# Disable optimizations that hide allocations
go test -bench=. -gcflags="-N -l" -cpuprofile=cpu.prof ./...
```

### Can't See Custom Functions in Profile
```bash
# Disable inlining
go test -bench=. -gcflags="-l" -cpuprofile=cpu.prof ./...

# Increase sample rate (for very fast functions)
CPUPROFILE_HZ=1000 go test -bench=. -cpuprofile=cpu.prof ./...
```

### Benchmark Hangs or Takes Too Long
```bash
# Add timeout
go test -bench=. -timeout=60s -cpuprofile=cpu.prof ./...

# Profile only specific benchmarks
go test -bench=BenchmarkSpecific -cpuprofile=cpu.prof ./...

# Exclude problematic tests
go test -bench=. -run=^$ -cpuprofile=cpu.prof ./...
```

## Production Profiling

### HTTP pprof endpoints
```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Your application code...
}
```

### Collecting production profiles
```bash
# CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# Memory profile
curl http://localhost:6060/debug/pprof/heap > mem.prof

# Goroutine profile
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof

# Analyze production profile
go tool pprof cpu.prof
```

### Automated profiling script
```bash
#!/bin/bash
# collect-profiles.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PROFILE_DIR="profiles/production/$TIMESTAMP"
mkdir -p "$PROFILE_DIR"

echo "Collecting production profiles..."

# CPU profile
curl -s "http://localhost:6060/debug/pprof/profile?seconds=30" > "$PROFILE_DIR/cpu.prof"

# Memory profile  
curl -s "http://localhost:6060/debug/pprof/heap" > "$PROFILE_DIR/heap.prof"

# Goroutines
curl -s "http://localhost:6060/debug/pprof/goroutine" > "$PROFILE_DIR/goroutine.prof"

echo "Profiles saved to $PROFILE_DIR"

# Quick analysis
echo "=== CPU Top Functions ==="
go tool pprof -top "$PROFILE_DIR/cpu.prof" | head -10

echo "=== Memory Top Allocators ==="
go tool pprof -top "$PROFILE_DIR/heap.prof" | head -10
``` 