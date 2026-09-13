# WebAssembly Predictor Plugin Boundary

v2.7.0 defines a capability-free WebAssembly predictor boundary. A plugin package consists of an immutable `.wasm` module plus a strict JSON manifest containing its SHA-256 digest, exported entrypoint, timeout, and input/output byte limits.

`telemetryforge-plugincheck` validates:

- WebAssembly binary format version 1;
- manifest schema and SHA-256 identity;
- the declared exported function entrypoint;
- capability-free v2.7 policy.

The Go host supplies only JSON input/output and a deadline. It deliberately does **not** expose database, network, filesystem, lifecycle, routing, secret, or production-mutation handles. Execution is delegated to an explicitly configured sandbox backend; v2.7 does not bundle a third-party Wasm engine and therefore makes no claim that module validation alone is an execution sandbox.

The checked-in `plugins/examples/noop-predictor.wasm` is a minimal validation fixture, not a production predictor.
