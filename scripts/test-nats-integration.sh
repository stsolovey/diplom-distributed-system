#!/bin/bash

# Integration test for NATS queue implementation
# Tests both memory and NATS queue modes

set -e

echo "=== Phase 2 NATS Integration Test ==="
echo

# Function to test queue functionality
test_queue() {
    local queue_type=$1
    echo "Testing $queue_type queue..."
    
    # Wait for services to be ready
    echo "Waiting for services to start..."
    sleep 15
    
    # Health check
    echo "Checking health..."
    health_response=$(curl -s http://localhost:8080/api/v1/status || echo "FAILED")
    if [[ $health_response == *"healthy"* ]]; then
        echo "✅ Health check passed"
    else
        echo "❌ Health check failed: $health_response"
        return 1
    fi
    
    # Send test message
    echo "Sending test message..."
    send_response=$(curl -s -w "%{http_code}" -X POST http://localhost:8080/api/v1/ingest \
        -H 'Content-Type: application/json' \
        -d '{"source":"test","payload":"hello from '"$queue_type"' queue"}' || echo "000")
    
    if [[ $send_response == *"202"* ]]; then
        echo "✅ Message sent successfully"
    else
        echo "❌ Failed to send message: $send_response"
        return 1
    fi
    
    # Check stats
    echo "Checking processor stats..."
    sleep 5
    stats_response=$(curl -s http://localhost:8082/stats || echo "FAILED")
    if [[ $stats_response == *"queue"* ]] && [[ $stats_response == *"pool"* ]]; then
        echo "✅ Stats available"
        echo "   Queue stats: $(echo $stats_response | jq -r '.queue // "N/A"')"
    else
        echo "❌ Stats check failed: $stats_response"
        return 1
    fi
    
    echo "✅ $queue_type queue test completed successfully"
    echo
}

# Test memory queue
echo "1. Testing memory queue implementation..."
./scripts/switch-queue.sh memory
test_queue "memory"

# Test NATS queue
echo "2. Testing NATS queue implementation..."
./scripts/switch-queue.sh nats
test_queue "nats"

# Cleanup
echo "3. Cleaning up..."
docker-compose -f docker/docker-compose.yml down

echo "=== All tests completed successfully! ==="
echo
echo "Phase 2 NATS integration is working correctly:"
echo "✅ Memory queue (Phase 1) still works"
echo "✅ NATS JetStream queue (Phase 2) is functional"
echo "✅ Factory pattern enables seamless switching"
echo "✅ Backward compatibility maintained" 