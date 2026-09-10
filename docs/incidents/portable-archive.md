# Portable `.tfincident` Incident Archives

TelemetryForge v1.5.0 can export a frozen incident into one portable,
self-verifying file.

```text
INC-42.tfincident
```

The unencrypted format is an ordinary ZIP archive so operators can inspect it
with standard tools. `manifest.json` is the compatibility/integrity root.

## Contents

A typical archive contains:

```text
manifest.json
README.txt
incident/metadata.json
incident/events.jsonl
evidence/graph.json
analysis/replay-runs.json
analysis/cost-simulations.json
schema/registry.json
schema/drift.json
configuration/policy-active.json
configuration/policy-shadow.json
configuration/shaping-active.json
configuration/shaping-shadow.json
configuration/routing-active.json
configuration/routing-shadow.json
```

Optional members are omitted when no corresponding evidence exists.

## Integrity

Every member except `manifest.json` is listed in the manifest with:

```json
{
  "sha256": "...",
  "bytes": 1234
}
```

Verification rejects:

- changed member bytes;
- extra unlisted members;
- manifest-listed missing members;
- duplicate ZIP member names;
- path traversal / unsafe member paths;
- incompatible format versions;
- manifest/event/schema/replay/cost count mismatches.

The CLI also prints the SHA-256 of the **complete archive file**. Keep that
digest in tickets/postmortems when chain-of-custody matters.

## Bounds

The parser enforces:

```text
128 archive members maximum
512 MiB total uncompressed archive content
128 MiB maximum individual member
100,000 frozen event records maximum
8 MiB maximum single JSONL event line
```

These are defensive parser limits, not performance claims.

## Export

```bash
telemetryctl incident export \
  --id INC-42 \
  --out INC-42.tfincident \
  --tenant production
```

By default the exporter snapshots the checked-in active/shadow:

- Cardinality Firewall policy
- adaptive shaping configuration
- routing configuration

Each configuration is validated with its real TelemetryForge parser before it
is archived.

Use `disabled` for a snapshot you intentionally want to omit.

## Verify

```bash
telemetryctl incident verify --file INC-42.tfincident
```

Verification parses and checks the complete archive.

## Inspect offline

```bash
telemetryctl incident inspect --file INC-42.tfincident
```

This prints archive/incident/evidence/schema/configuration summary without a
database or Kafka connection.

## Standalone HTML report

```bash
telemetryctl incident report \
  --file INC-42.tfincident \
  --out INC-42.html
```

The report is self-contained:

- no JavaScript;
- no external fonts;
- no trackers;
- no network requests.

Telemetry-controlled strings are rendered through Go `html/template` and are
HTML-escaped.

The HTML is still sensitive because it intentionally summarizes full-fidelity
incident evidence.

## Import

```bash
telemetryctl incident import \
  --file INC-42.tfincident
```

Import creates a frozen incident only. It does **not** replay archive events
through normal ingestion, primary telemetry persistence, automatic incident
detection, or routing.

Configuration snapshots are evidence only. Import never activates archived
policy/shaping/routing configuration.

Existing incident IDs are never overwritten.

See [Archive Workflow](../operations/archive-workflow.md).
