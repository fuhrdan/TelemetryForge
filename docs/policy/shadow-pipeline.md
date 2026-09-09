# Shadow Policy Pipeline

Changing telemetry policy is risky. A rule intended to save cost can also
remove the exact dimension engineers need during an incident.

The shadow pipeline lets a candidate policy evaluate real production-shaped
events without changing the active event path.

## Data flow

```text
normalized event
      |
      +--------------------+
      |                    |
      v                    v
active policy          shadow policy
      |                    |
      v                    v
action applied       action calculated only
      |                    |
      +---------+----------+
                |
                v
       compare decisions
                |
                v
      store disagreements
```

The shadow policy never removes a tag, quarantines an event, or changes normal
persistence.

## What gets recorded?

When active and shadow actions differ, TelemetryForge stores:

- timestamp
- source
- event type
- dimension
- active policy/version/action
- shadow policy/version/action
- reason

API:

```text
GET /api/v1/policy/shadow-diffs
```

The dashboard shows the most recent disagreements.

## Promotion workflow

A practical policy change can follow:

1. edit `policies/shadow.json`;
2. run policy validation;
3. deploy the candidate only as shadow;
4. observe differences;
5. investigate false positives;
6. copy the reviewed candidate into `active.json`;
7. increment the active policy version.

This makes telemetry policy changes observable before they become destructive.

## Disable shadow mode

Set:

```text
TELEMETRYFORGE_SHADOW_POLICY_FILE=disabled
```

The active policy continues normally.
