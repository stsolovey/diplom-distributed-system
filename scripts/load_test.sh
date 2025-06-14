#!/bin/bash
set -e

REQUESTS=${1:-1000}
CONCURRENCY=${2:-10}
OUTPUT_DIR=${3:-"results/load_test/$(date +%Y%m%d_%H%M%S)"}

echo "=== Load Test Started ==="
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"  
echo "Output: $OUTPUT_DIR"

mkdir -p "$OUTPUT_DIR"

# Create payload
PAYLOAD='{"source":"load_test","data":"performance test after removing time.Sleep"}'

# Function to send requests
send_requests() {
    local start_time=$(date +%s.%N)
    local success_count=0
    local error_count=0
    
    for ((i=1; i<=REQUESTS/CONCURRENCY; i++)); do
        pids=()
        
        # Launch concurrent requests
        for ((j=1; j<=CONCURRENCY; j++)); do
            {
                local req_start=$(date +%s.%N)
                if response=$(curl -s -w "%{http_code}" -X POST \
                    -H "Content-Type: application/json" \
                    -d "$PAYLOAD" \
                    http://localhost:8080/api/v1/ingest 2>/dev/null); then
                    local req_end=$(date +%s.%N)
                    local latency=$(echo "$req_end - $req_start" | bc -l)
                    
                    if [[ "${response: -3}" == "200" ]]; then
                        echo "SUCCESS,$latency" >> "$OUTPUT_DIR/results.csv"
                    else
                        echo "ERROR,$latency,${response: -3}" >> "$OUTPUT_DIR/errors.csv"
                    fi
                else
                    echo "FAILED,0,connection" >> "$OUTPUT_DIR/errors.csv"
                fi
            } &
            pids+=($!)
        done
        
        # Wait for this batch
        for pid in "${pids[@]}"; do
            wait $pid
        done
        
        # Progress
        if (( i % 10 == 0 )); then
            echo "Completed $((i * CONCURRENCY))/$REQUESTS requests"
        fi
    done
    
    local end_time=$(date +%s.%N)
    local total_time=$(echo "$end_time - $start_time" | bc -l)
    
    # Calculate stats
    local success_count=$(wc -l < "$OUTPUT_DIR/results.csv" 2>/dev/null || echo 0)
    local error_count=$(wc -l < "$OUTPUT_DIR/errors.csv" 2>/dev/null || echo 0)
    local rps=$(echo "scale=2; $success_count / $total_time" | bc -l)
    
    # Average latency
    local avg_latency=$(awk -F',' '{sum+=$2; count++} END {print sum/count}' "$OUTPUT_DIR/results.csv" 2>/dev/null || echo 0)
    
    echo "=== Results ==="
    echo "Total time: ${total_time}s"
    echo "Successful requests: $success_count"
    echo "Failed requests: $error_count"
    echo "Requests per second: $rps"
    echo "Average latency: ${avg_latency}ms"
    
    # Save summary
    cat > "$OUTPUT_DIR/summary.txt" << EOF
Load Test Results
================
Date: $(date)
Requests: $REQUESTS
Concurrency: $CONCURRENCY
Total time: ${total_time}s
Successful requests: $success_count
Failed requests: $error_count
Requests per second: $rps
Average latency: ${avg_latency}ms
EOF
}

send_requests 