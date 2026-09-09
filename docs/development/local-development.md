# Local Development

## Requirements

- Go 1.23+
- Docker with Docker Compose (optional)

## Run locally

```bash
go run ./cmd/gateway
```

The gateway listens on `:8080` by default.

## Verify health

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Submit an event

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "source":"checkout-api",
    "type":"request.duration",
    "timestamp":"2026-09-09T15:30:00Z",
    "schema_version":"1.0",
    "correlation_id":"order-123"
  }'
```

## Run checks

```bash
make check
```
