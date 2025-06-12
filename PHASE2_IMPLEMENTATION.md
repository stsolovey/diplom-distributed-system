# Phase 2 NATS JetStream Implementation

## Overview

Phase 2 adds NATS JetStream as an alternative message queue implementation while maintaining full backward compatibility with the existing in-memory queue. The system can now seamlessly switch between queue types without code changes.

## Key Features

### ✅ Implemented in Phase 2

- **NATS JetStream Integration**: Enterprise-grade message queue with persistence
- **Factory Pattern**: Seamless switching between memory and NATS queues
- **Backward Compatibility**: Existing memory queue functionality unchanged
- **Pull-based Subscription**: Back-pressure and flow control
- **Graceful Error Handling**: Automatic reconnection and retry logic
- **Docker Integration**: Complete containerized deployment
- **Statistics & Monitoring**: Queue metrics for both implementations

## Queue Types

### Memory Queue (Phase 1)
- **Use Case**: Development, testing, low-volume scenarios
- **Persistence**: None (in-memory only)
- **Scaling**: Single instance
- **Configuration**: `QUEUE_TYPE=memory`

### NATS JetStream Queue (Phase 2)
- **Use Case**: Production, high-volume, distributed scenarios
- **Persistence**: Durable storage with configurable retention
- **Scaling**: Multi-instance with automatic load balancing
- **Configuration**: `QUEUE_TYPE=nats`

## Usage

### Quick Start

1. **Start with Memory Queue (default)**:
   ```bash
   make docker-up
   # or
   ./scripts/switch-queue.sh memory
   ```

2. **Switch to NATS Queue**:
   ```bash
   ./scripts/switch-queue.sh nats
   ```

3. **Test Both Implementations**:
   ```bash
   ./scripts/test-nats-integration.sh
   ```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `QUEUE_TYPE` | `memory` | Queue implementation: `memory` or `nats` |
| `NATS_URL` | `nats://nats:4222` | NATS server connection URL |
| `QUEUE_SIZE` | `1000` | Buffer size (memory queue only) |

### Manual Configuration

**Memory Queue**:
```bash
export QUEUE_TYPE=memory
export QUEUE_SIZE=1000
docker-compose -f docker/docker-compose.yml up -d api-gateway ingest processor
```

**NATS Queue**:
```bash
export QUEUE_TYPE=nats
export NATS_URL=nats://nats:4222
docker-compose -f docker/docker-compose.yml up -d
```

## API Endpoints

All existing endpoints remain unchanged. The queue implementation is transparent to clients:

- `POST /api/v1/ingest` - Submit messages
- `GET /api/v1/status` - System health
- `GET /stats` - Queue and processor statistics

## NATS Configuration

The NATS JetStream is configured with:

- **Stream Name**: `DIPLOM_STREAM`
- **Subject Pattern**: `diplom.messages`
- **Retention Policy**: Work queue (messages deleted after acknowledgment)
- **Storage**: File-based persistence
- **Max Age**: 24 hours
- **Acknowledgment**: Explicit acknowledgment required

## Monitoring

### NATS Monitoring
- **Admin UI**: http://localhost:8222
- **Health Check**: Available via Docker healthcheck

### Application Statistics
Both queue types provide unified statistics via `/stats` endpoint:

```json
{
  "queue": {
    "total_enqueued": 1000,
    "total_dequeued": 995,
    "current_size": 5
  },
  "pool": {
    "processed_count": 995,
    "error_count": 0,
    "total_duration": "9.95s"
  }
}
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────────┐
│                 │     │                 │     │                     │
│   API Gateway   │────▶│  Ingest Service │────▶│  Processor Service  │
│   (Port 8080)   │HTTP │   (Port 8081)   │HTTP │    (Port 8082)      │
│                 │     │                 │     │                     │
└─────────────────┘     └─────────────────┘     └──────────┬──────────┘
                                                           │
                        ┌──────────────────────────────────┼──────────────┐
                        │                                  ▼              │
                        │         QueueProvider Factory                   │
                        │                                                 │
                        │  ┌─────────────────┐    ┌─────────────────────┐ │
                        │  │  MemoryQueue    │    │  NATS JetStream     │ │
                        │  │  Adapter        │    │  Adapter            │ │
                        │  │                 │    │                     │ │
                        │  │  • In-memory    │    │  • Persistent       │ │
                        │  │  • Single node  │    │  • Multi-node       │ │
                        │  │  • Fast         │    │  • Scalable         │ │
                        │  └─────────────────┘    └─────────────────────┘ │
                        └─────────────────────────────────────────────────┘
```

## Testing

### Integration Tests
```bash
# Test both queue implementations
./scripts/test-nats-integration.sh

# Test specific implementation
./scripts/switch-queue.sh nats
curl -X POST http://localhost:8080/api/v1/ingest \
  -H 'Content-Type: application/json' \
  -d '{"source":"test","payload":"hello nats"}'
```

### Unit Tests
```bash
# Run existing tests (backward compatibility)
go test ./internal/queue/... -v

# Run all tests
go test ./... -v
```

## Migration Path

**Phase 1 → Phase 2**: Zero-downtime migration
1. Deploy Phase 2 code with `QUEUE_TYPE=memory` (maintains existing behavior)
2. Test functionality with memory queue
3. Switch to `QUEUE_TYPE=nats` when ready for NATS benefits
4. Monitor and validate NATS performance

## Benefits of NATS Implementation

1. **Persistence**: Messages survive service restarts
2. **Durability**: Guaranteed delivery with acknowledgments
3. **Scalability**: Horizontal scaling of consumers
4. **Back-pressure**: Automatic flow control
5. **Monitoring**: Rich metrics and observability
6. **Fault Tolerance**: Automatic reconnection and retry

## Troubleshooting

### Common Issues

1. **NATS Connection Failed**
   - Verify NATS service is running: `docker-compose ps nats`
   - Check NATS logs: `docker-compose logs nats`

2. **Messages Not Processing**
   - Check processor logs: `docker-compose logs processor`
   - Verify queue statistics: `curl http://localhost:8082/stats`

3. **High Memory Usage**
   - Switch to NATS for better memory management
   - Adjust `QUEUE_SIZE` for memory queue

### Debug Commands

```bash
# Service status
docker-compose ps

# View logs
docker-compose logs processor
docker-compose logs nats

# Check NATS streams
docker exec -it $(docker-compose ps -q nats) nats stream ls

# Queue statistics
curl http://localhost:8082/stats | jq
```

## Next Steps (Future Phases)

- **Phase 3**: Kafka integration as additional queue option
- **Phase 4**: Message routing and filtering
- **Phase 5**: Message transformation and enrichment

---

**Implementation completed successfully!** ✅  
All Phase 2 requirements from ФАЗА_2.md have been implemented with full backward compatibility. 