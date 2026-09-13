# TelemetryForge Predictor Plugins

TelemetryForge v2.7 defines a capability-free WebAssembly predictor package format. Modules are digest-pinned, must export the manifest entrypoint, and may exchange only bounded JSON through an explicitly configured sandbox backend.

`examples/noop-predictor.wasm` is a minimal validation fixture only. It proves the package/entrypoint validator; it does not implement a production prediction algorithm.

Validate a package with:

```bash
go run ./cmd/plugincheck --manifest plugins/examples/noop-predictor.manifest.json --module plugins/examples/noop-predictor.wasm
```

See [WebAssembly Predictor Plugin Boundary](../docs/autonomy/wasm-plugins.md).
