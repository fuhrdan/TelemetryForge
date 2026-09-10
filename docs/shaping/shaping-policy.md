# Shaping Policy

Active configuration:

```text
shaping/active.json
```

Candidate configuration:

```text
shaping/shadow.json
```

## Rule matching

Rules can match:

- authenticated tenant;
- source;
- event type;
- severity; and
- tag patterns.

Tenant/source/type/tag patterns use Go `path.Match` glob syntax and are
validated at startup/CLI validation time.

Rules are evaluated in document order. Matching rules compose until `stop` is
true.

## Sampling fields

```json
{
  "sample_rate": 0.5,
  "min_sample_rate": 0.1
}
```

Rates are between 0 and 1.

`sample_rate` is the normal rate. `min_sample_rate` is the lowest pressure
adaptation can reduce that rule to.

## Tag shaping

Drop tags:

```json
{"drop_tags": ["debug_id"]}
```

Rename tags:

```json
{"rename_tags": {"http.method": "http.request.method"}}
```

A rename never overwrites an already-existing destination key. If both keys
exist, the destination value wins and the old key is removed.

Configuration is deliberately bounded to 256 rules. Config names/versions and
rule names are length-capped, and each rule has bounded tag matchers/transforms so
configured metric labels and policy work cannot grow without limit.

Configuration validation rejects:

- an empty tag key;
- the same tag being both dropped and renamed;
- rename-to-self;
- multiple source keys targeting the same destination key.

## Payload shaping

A rule can bound payload size:

```json
{
  "payload_max_bytes": 32768,
  "payload_action": "drop"
}
```

When an oversized payload is dropped, TelemetryForge adds:

```text
telemetryforge.payload_oversize=true
```

to the shaped event.

TelemetryForge does not byte-truncate arbitrary JSON because that can create an
invalid document.

The full pre-shaping envelope remains available in the Flight Recorder.

## Shadow shaping

The shadow configuration is evaluated against the same pre-shaped event and
same current queue pressure.

It never mutates the active output.

Only differences are stored, including:

- keep vs sample-out changes;
- effective-rate changes; and
- transformation-effect changes.


## Protected-event transformations

Protected telemetry is kept **and preserves its tags/payload by default**. A
matching rule must explicitly set:

```json
{"shape_protected": true}
```

before tag/payload transformations are applied to a protected event. Sampling
can never exclude a protected event.
