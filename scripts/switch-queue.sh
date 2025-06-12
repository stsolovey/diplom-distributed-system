#!/bin/bash

# Script to switch queue implementation between memory and NATS
# Usage: ./switch-queue.sh [memory|nats]

set -e

QUEUE_TYPE=${1:-memory}

if [ "$QUEUE_TYPE" != "memory" ] && [ "$QUEUE_TYPE" != "nats" ]; then
    echo "Error: Queue type must be 'memory' or 'nats'"
    echo "Usage: $0 [memory|nats]"
    exit 1
fi

echo "Switching to $QUEUE_TYPE queue implementation..."

# Export environment variables
export QUEUE_TYPE=$QUEUE_TYPE
export NATS_URL="nats://nats:4222"

echo "Updating services with $QUEUE_TYPE queue (minimal downtime)..."

if [ "$QUEUE_TYPE" = "nats" ]; then
    # Start with NATS - recreate containers with new env
    echo "Using NATS JetStream queue"
    docker-compose -f docker/docker-compose.yml up -d --force-recreate
else
    # Start with memory queue - recreate only needed services
    echo "Using in-memory queue"
    docker-compose -f docker/docker-compose.yml up -d --force-recreate api-gateway ingest processor
    # Stop NATS if running
    docker-compose -f docker/docker-compose.yml stop nats || true
fi

echo "Waiting for services to be healthy..."
sleep 10

# Check service status
echo "Service status:"
docker-compose -f docker/docker-compose.yml ps

echo ""
echo "Queue switch completed! Current configuration:"
echo "  Queue Type: $QUEUE_TYPE"
if [ "$QUEUE_TYPE" = "nats" ]; then
    echo "  NATS URL: $NATS_URL"
fi

echo ""
echo "Test the setup:"
echo "  Health check: curl http://localhost:8080/api/v1/status"
echo "  Send message: curl -X POST http://localhost:8080/api/v1/ingest -H 'Content-Type: application/json' -d '{\"source\":\"test\",\"payload\":\"hello world\"}'" 