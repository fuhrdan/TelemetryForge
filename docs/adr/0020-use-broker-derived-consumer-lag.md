# ADR 0020: Use Broker-Derived Kafka Consumer Lag

**Status:** Accepted  
**Date:** 2026-09-09

## Context

The previous local "lag" value approximated backlog from the most recent fetch.
That is not Kafka consumer lag and can be misleading during scaling or
backpressure.

## Decision

The worker periodically queries Kafka group lag using franz-go's admin client.

For every topic/partition it reports:

```text
broker end offset - committed consumer-group offset
```

as provided by Kafka/franz-go, then sums non-negative partition lag for the
group-level gauge.

The query runs independently of the processing loop on a 15-second cadence.

## Consequences

The Grafana lag panel now represents Kafka's durable backlog rather than local
queue state.

The worker performs periodic admin queries against Kafka. Failure to obtain lag
is logged but does not stop event processing.
