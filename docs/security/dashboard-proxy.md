# Dashboard Server-Side API Proxy

Browsers cannot safely hold a long-lived tenant API key in JavaScript.

v1.0 replaces the old direct Next.js rewrite with a server-side route:

```text
/telemetry-api/*
```

## Flow

```text
Browser
  |
  | same-origin request / EventSource
  v
Next.js server route
  |
  | Authorization: Bearer <read-only key>
  v
TelemetryForge gateway
```

The raw read key lives only in the dashboard server environment:

```text
TELEMETRYFORGE_DASHBOARD_API_KEY
```

The gateway validates it like every other API key and derives the tenant from
that credential.

## Why this matters for SSE

Browser `EventSource` does not provide a normal way to attach an arbitrary
Authorization header.

The server proxy keeps SSE same-origin for the browser while authenticating the
server-to-gateway connection.

## Header handling

The proxy strips:

- `Authorization` supplied by the browser
- `Host`
- hop-by-hop headers

and injects its configured read-only credential.

## Production principle

Use a read-scoped key for the dashboard.

Do not give the dashboard an ingest/admin credential simply because the server
could technically hold one.
