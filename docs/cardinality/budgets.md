# Cardinality Budgets

v1.2.0 adds versioned hourly unique-series budgets to policy-as-code.

Budgets answer a different question from dimension rules:

- **dimension rule:** is one label such as `request_id` dangerous?
- **series budget:** how many distinct full tag combinations is this scope
  creating?

## Example

```json
{
  "name": "tenant-hourly-series",
  "source": "*",
  "type": "*",
  "series_limit": 50000,
  "warning_percent": 70,
  "critical_percent": 90
}
```

A narrower source budget can coexist:

```json
{
  "name": "checkout-hourly-series",
  "source": "checkout-*",
  "type": "*",
  "series_limit": 10000,
  "warning_percent": 65,
  "critical_percent": 85
}
```

## Series identity

For budget estimation TelemetryForge hashes:

```text
source
+
event type
+
sorted non-TelemetryForge tag key/value pairs
```

This means a budget estimates actual distinct telemetry series shapes rather
than adding individual label-cardinality estimates together.

`telemetryforge.*` control tags are intentionally excluded from the identity.

The raw assembled series identity is hashed immediately and is not persisted.

## Aggregation scope

Budget state is keyed by the **budget name**, not by one concrete event source.

Therefore:

```text
source="*", type="*"
```

aggregates matching series across the whole tenant, while:

```text
source="checkout-*", type="*"
```

aggregates only the matching checkout scope.

## Status calculation

The risk value is:

```text
max(observed_unique, projected_unique)
```

Consumption is:

```text
risk / series_limit * 100
```

Statuses:

```text
healthy
warning
critical
exceeded
```

The configured percentages determine warning/critical transitions; 100% is
always `exceeded`.

## Non-destructive by design

Budgets do not automatically drop, sample, or quarantine telemetry.

A budget can reveal that a scope is economically or operationally risky, but it
does not identify which specific dimension should be destroyed.

Dimension rules remain the explicit mutation mechanism.

This separation will also give the later Adaptive Sampling milestone a clean
input without letting v1.2 silently change visibility.

## Active and shadow budgets

Active and shadow policy documents can define different budgets.

The dashboard/API stores policy name/version/mode with each status, so a
candidate policy can be compared before promotion.

## API

```text
GET /api/v1/cardinality/budgets?limit=100
```

Only recent budget observations are returned, preventing an old previous-hour
status from looking current after traffic stops.

## CLI

```bash
telemetryctl cardinality budgets --tenant default
```

## Dashboard

The **Series Budgets** panel shows:

- scope;
- active/shadow mode;
- current status;
- observed unique series;
- projected hourly unique series;
- configured limit; and
- percentage consumed.
