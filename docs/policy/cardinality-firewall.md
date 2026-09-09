# Cardinality Firewall

The Cardinality Firewall protects downstream observability systems from
identifier-shaped labels that can create a separate time series for nearly
every request, user, session, trace, or order.

## Why cardinality matters

A metric such as:

```text
request.duration{service="checkout",region="us-west"}
```

has a small number of stable series.

Adding a request ID changes the cost model:

```text
request.duration{service="checkout",request_id="8e3..."}
```

If every request gets a new `request_id`, downstream time-series systems may
need one series per request. Storage, index, query, and vendor-billing costs can
grow much faster than traffic itself.

## Processing order

```text
Flight Recorder
    |
    v
Normalizer
    |
    v
Cardinality Firewall
    |
    +-- allow ------> normal persistence
    |
    +-- drop_tag ---> remove dangerous dimension -> normal persistence
    |
    `-- quarantine -> preserve full evidence
                      remove dangerous dimension
                      mark event quarantined
                      -> normal control-plane persistence
```

The Flight Recorder runs first, so an active policy can remove a dangerous tag
without destroying the original incident evidence.

## Estimator

TelemetryForge uses a hybrid bounded estimator per tracked
`source / event type / dimension`:

- the first 16 distinct SHA-256-derived hashes are counted exactly; then
- a small 64-register HyperLogLog estimate takes over.

The fixed exact window avoids small-threshold approximation surprises without
keeping raw values or an unbounded set.

Benefits:

- fixed memory per tracked dimension;
- no need to keep every raw tag value in memory;
- bounded global dimension count; and
- deterministic enough for operational warnings.

This is **not** a billing-grade cardinality estimator. Its purpose is to answer:

> Is this dimension becoming dangerous enough that policy should intervene?

The raw tag value is never copied into the finding table. Findings store a
short SHA-256-derived fingerprint only.

The fingerprint is a correlation aid, **not an anonymization guarantee**.
Low-entropy values can still be guessable by dictionary attack, so finding
tables remain operationally sensitive.

## Projection

Projected cardinality scales observed unique values to a one-hour horizon, but
only after at least ten samples and 30 seconds of observation.

That minimum window prevents two startup samples arriving milliseconds apart
from producing a meaningless huge projection.

Identifier-shaped dimensions have an additional early-warning path because
keys such as `request_id` are structurally risky even before a long observation
window exists.

That heuristic produces an `allow` finding only. It does **not** activate
`drop_tag` or `quarantine` early. Destructive policy actions require the
configured cardinality threshold itself to be crossed.

## Bounded memory

The default worker tracks at most:

```text
20,000 source/type/dimension combinations
```

When that cap is reached, the least recently observed dimension state is
evicted.

The report-suppression map is bounded separately and repeated findings for the
same mode/source/type/dimension/action are emitted at most once per minute.

## Policy actions

### `allow`

Record a warning if the threshold is crossed but leave the event unchanged.

### `drop_tag`

Remove the dangerous tag from the normal persisted/downstream representation.

The original full-fidelity value remains available in the Flight Recorder.

### `quarantine`

Preserve the original normalized event in `quarantined_events`, remove the
dangerous dimension from the normal event, and add:

```text
telemetryforge.quarantined=true
telemetryforge.quarantine_dimensions=<comma-separated dimensions>
```

No external vendor router exists yet, so v0.8.0 cannot literally block a vendor
forwarder. The marker is the routing contract future output sinks will honor.

## Operational evidence

The dashboard/API exposes:

```text
GET /api/v1/cardinality/findings
```

Each finding includes:

- active or shadow mode
- policy name/version
- source
- event type
- dimension
- observed unique estimate
- one-hour projected estimate
- action
- reason
- short value fingerprint
- first/last seen timestamps
