# Connector SDK Contract

The in-process Go connector contract lives in `internal/connectors`. A connector exposes kind, capabilities, `Send`, `Ready`, and `Close`. It receives one canonical post-policy event and never receives a database transaction or outbox row.

Connectors must not implement unbounded retry loops. They return typed delivery errors to the router, which decides retry, destination DLQ, or fallback using durable state.

Connector configuration stores secret references (`header_env`, `bearer_token_env`, `api_key_env`) rather than raw secrets. Static credential-shaped headers and credential-bearing URLs are rejected.

The v1.8 SDK is an in-process extension boundary; it does not load arbitrary unsigned shared libraries or external plugin executables.
