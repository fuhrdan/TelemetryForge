# TelemetryForge v1.5.0 Development Release Notes

## Portable Incident Archive

v1.5.0 turns frozen incidents into portable, verifiable investigation
artifacts instead of leaving them tied to one PostgreSQL database.

## `.tfincident`

Unencrypted archives are standard ZIP files with a versioned
`manifest.json`.

The package can preserve:

- incident metadata;
- frozen canonical events and first Flight Recorder capture time;
- Evidence Graph;
- replay history;
- cost simulations;
- relevant Schema Intelligence history/drift;
- exact validated active/shadow Cardinality, shaping, and routing config files.

Every member except the manifest has a recorded SHA-256 and byte size.

The reader rejects changed, extra, missing, duplicate, unsafe-path, oversized,
or incompatible archive members.

## Encryption

Optional encrypted archives wrap the complete ZIP with:

```text
AES-256-GCM
```

The CLI accepts a random 32-byte key encoded as 64 hex characters.

v1.5 intentionally does not accept human passwords or invent a weak
password-to-key scheme.

## CLI

Added:

```bash
telemetryctl incident export
telemetryctl incident verify
telemetryctl incident inspect
telemetryctl incident report
telemetryctl incident import
```

`incident report` creates a standalone HTML investigation report with no
JavaScript, external assets, analytics, or network requests.

## Import safety

Import restores **frozen evidence**, not production traffic.

It does not:

- invoke ingestion;
- populate normal telemetry;
- run sampling/cardinality/routing;
- send to destinations;
- activate archived configuration.

Cross-tenant import requires `--allow-tenant-remap`.

Existing target incident IDs are never overwritten.

The same archive ID cannot be silently imported twice into one tenant.

## Import provenance

Migration `012_incident_archive.sql` stores:

- archive ID;
- source tenant and incident;
- imported incident ID;
- archive/product version;
- complete-file SHA-256;
- encrypted/unencrypted source flag;
- import time.

Added:

```text
GET /api/v1/archive-imports
```

and a dashboard provenance panel.

## v1.4 included cumulatively

The v1.5 development tree includes the adaptive sampling/shaping work introduced
in the v1.4 development milestone: protected telemetry, deterministic/pressure
sampling, safe shaping, shadow shaping, visibility statistics, and frozen-
incident preview.

## Tests

Added unit coverage for:

- unencrypted round trip;
- AES-GCM wrong-key/tamper rejection;
- per-member checksum mismatch;
- ZIP path traversal;
- cross-tenant bundle rejection;
- HTML escaping.

Added a TimescaleDB integration scenario covering:

```text
freeze
 -> export
 -> verify
 -> tenant remap
 -> import
 -> provenance
 -> duplicate import rejection
```

## Known limitations

- Archive encryption uses symmetric operator-managed keys; public-key recipient
  encryption/signatures are not yet implemented.
- Archive export/import is currently a `telemetryctl` administrative workflow,
  not a browser download/upload feature.
- Import does not copy archived replay/cost rows into live history.
- Configuration snapshots are evidence only and are never auto-activated.
- Schema drift export is bounded to the newest 500 tenant findings before
  incident source/type filtering.
- Offline HTML reports can contain sensitive evidence and must be protected.
