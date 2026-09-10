# Change Intelligence API

## Ingest a change

```text
POST /api/v1/changes
```

Requires `ingest` scope.

Example:

```json
{
  "change_id": "DEP-1042",
  "source": "checkout-api",
  "kind": "deployment",
  "status": "completed",
  "environment": "production",
  "version": "4.12.7",
  "previous_version": "4.12.6",
  "git_sha": "abc123...",
  "build_id": "build-1042",
  "actor": "github-actions",
  "summary": "Checkout release"
}
```

Supported kinds:

```text
deployment
rollback
release
feature_flag
configuration
infrastructure
```

The gateway assigns tenant identity from authentication and publishes a
canonical event through the durable Kafka path.

## List changes

```text
GET /api/v1/changes?limit=50
```

Requires `read` scope.

## Generate analysis

```text
POST /api/v1/changes/{id}/analyze?before_seconds=900&after_seconds=900
```

Requires `read` scope. Windows must be between 60 and 7200 seconds.

The result is persisted as the latest regenerable analysis snapshot.

## Read latest analysis

```text
GET /api/v1/changes/{id}/analysis
```

Returns `404` until an analysis has been generated.

## Changes near an incident

```text
GET /api/v1/incidents/{id}/changes
```

Returns structured changes from 15 minutes before the first frozen event
through five minutes after the last frozen event.
