# Policy as Code

TelemetryForge policies are versioned JSON documents committed alongside the
application code.

## Why JSON?

The first policy format uses JSON because:

- Go can parse it with the standard library;
- pull requests show ordinary text diffs;
- strict unknown-field rejection catches misspelled settings;
- no sidecar/runtime policy dependency is needed; and
- the format is easy to generate or translate later.

Open Policy Agent was considered. OPA is a strong option for organization-wide
policy platforms, but adding another runtime service before TelemetryForge has a
need for arbitrary Rego would increase operational surface without improving the
current Cardinality Firewall use case.

## Example

```json
{
  "name": "default-cardinality-policy",
  "version": "1.2.0",
  "default_unique_threshold": 1000,
  "default_action": "allow",
  "dangerous_keys": [
    "request_id",
    "session_id",
    "trace_id"
  ],
  "rules": [
    {
      "source": "*",
      "type": "*",
      "dimension": "request_id",
      "unique_threshold": 100,
      "action": "drop_tag"
    }
  ],
  "budgets": [
    {
      "name": "tenant-hourly-series",
      "source": "*",
      "type": "*",
      "series_limit": 50000,
      "warning_percent": 70,
      "critical_percent": 90
    }
  ]
}
```

## Matching

Rules contain:

- `source`
- `type`
- `dimension`

Each field accepts Go `path.Match`-style glob syntax. Empty or `*` source/type
patterns match all values.

The first matching rule wins.

## Validation

Worker startup fails if its active or shadow policy is invalid.

Operators can validate explicitly:

```bash
go run ./cmd/telemetryctl policy validate \
  --file policies/active.json
```

Validation checks:

- policy name/version
- minimum cardinality thresholds
- supported action values
- rule dimensions
- unknown JSON fields
- trailing JSON documents

## Deployment

Local Docker images include default policy files under:

```text
/etc/telemetryforge/policies/
```

Kubernetes generates a `telemetryforge-policies` ConfigMap from the same
reviewed JSON files.

Production teams can replace those files through their normal GitOps process.


## v1.2 series budgets

Budgets match `source` and `type` using the same glob behavior as dimension
rules, but they estimate the number of distinct **full series identities** in a
one-hour window.

They are deliberately advisory. Budget status never performs `drop_tag`,
`quarantine`, or sampling by itself.

Validation also checks:

- unique budget names;
- minimum series limit;
- warning percentage between 0 and 100; and
- critical percentage greater than warning and no greater than 100.

See [Cardinality Budgets](../cardinality/budgets.md).


## v1.2 budget validation hardening

Budget and rule glob patterns are parsed during policy validation. Malformed glob
patterns are rejected at startup/`telemetryctl policy validate` rather than
silently matching nothing.

Cardinality thresholds and series budgets are also capped at PostgreSQL's signed
64-bit storage range so a syntactically valid policy cannot fail later when its
status is persisted.
