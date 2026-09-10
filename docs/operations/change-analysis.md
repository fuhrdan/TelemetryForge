# Change Analysis Runbook

## Record changes from CI/CD

Prefer the authenticated gateway endpoint rather than inserting database rows.

```bash
curl -X POST "$TELEMETRYFORGE_URL/api/v1/changes" \
  -H "Authorization: Bearer $TELEMETRYFORGE_INGEST_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "change_id":"DEP-1042",
    "source":"checkout-api",
    "kind":"deployment",
    "environment":"production",
    "version":"4.12.7",
    "previous_version":"4.12.6",
    "git_sha":"'"$GIT_SHA"'",
    "build_id":"'"$BUILD_ID"'",
    "actor":"ci"
  }'
```

## List changes

```bash
telemetryctl change list --tenant production
```

## Analyze one change

```bash
telemetryctl change analyze \
  --id DEP-1042 \
  --before 15m \
  --after 15m \
  --tenant production
```

The output includes source-level before/after statistics and an observed blast
radius.

## Inspect cached analysis

```bash
telemetryctl change show --id DEP-1042 --tenant production
```

## Demonstration

```bash
make demo-change
```

The local scenario emits:

```text
healthy baseline
  -> deployment marker
  -> checkout + payments regression
  -> explicit rollback
  -> recovery telemetry
```

Then select the deployment in the dashboard Change Intelligence panel.

## Interpretation

Treat `regression-associated` as a prioritized investigation lead, not a root
cause verdict. Check the Evidence Graph, traces, deployment diff, and rollback
behavior before making a causal postmortem statement.
