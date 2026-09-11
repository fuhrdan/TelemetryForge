# Incident Comparison

v2.0.0 can compare two Evidence Graphs using a small reproducible similarity model.

```bash
telemetryctl intelligence compare --left INC-42 --right INC-17
```

API:

```text
GET /api/v1/incidents/INC-42/compare?other=INC-17
```

## Similarity inputs

The score is a weighted Jaccard comparison of captured graph features:

```text
source overlap        30%
event-type overlap    25%
hypothesis overlap    35%
change version/SHA    10%
```

Status bands are:

```text
strong_overlap   >= 0.70
partial_overlap  >= 0.40
limited_overlap  > 0
insufficient_evidence when comparison is not meaningful
```

The result includes the matching node IDs from both graphs.

## Safety meaning

Similarity means only that captured evidence overlaps. It does not mean the incidents share a root cause, failure mechanism, owner, or remediation. A high score is a prompt for operator comparison, not an automated causal conclusion.

The scoring weights and thresholds are source-controlled so repeated comparisons are reproducible.
