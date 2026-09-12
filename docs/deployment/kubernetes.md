# Kubernetes Deployment

v0.8.0 includes the Kubernetes work originally planned for v0.7.0.

The manifests live in:

```text
deployments/kubernetes/base/
```

They are a Kustomize base rather than a cloud-specific production distribution.

## Upstream version baseline

The manifests are reviewed against upstream Kubernetes **1.37** APIs.

The included AWS Terraform foundation targets EKS **1.36**, because that is the
newest minor currently offered by Amazon EKS standard support.

## Components

The base contains:

- namespace
- shared non-secret ConfigMap
- active/shadow policy ConfigMap
- gateway Deployment + Service
- edge StatefulSet + Service with persistent WAL/replica storage
- worker Deployment
- dashboard Deployment + Service
- HorizontalPodAutoscalers
- PodDisruptionBudgets
- example Ingress
- example Secret template

## Secret setup

Do not commit real credentials.

```bash
cp deployments/kubernetes/base/secret.example.yaml \
   deployments/kubernetes/base/secret.yaml
```

Replace the placeholder database URL, then apply the secret using an
organization-approved secret workflow.

`secret.yaml` is ignored by Git.

## Render

```bash
kubectl kustomize deployments/kubernetes/base
```

## Apply

After the secret exists:

```bash
kubectl apply -f deployments/kubernetes/base/secret.yaml
kubectl apply -k deployments/kubernetes/base
```

## Health checks

### Gateway

```text
GET :8080/health
GET :8080/ready
```

Readiness verifies Kafka and PostgreSQL.


### Edge

```text
GET :8083/health
GET :8083/ready
GET :8083/edge/status
```

The base manifest keeps `TELEMETRYFORGE_EDGE_DURABILITY_MODE=local` so it can render without environment-specific topology metadata. v2.2 non-local durability requires operators to provide node cloud/region/zone identity, peer URLs, quorum, and `TELEMETRYFORGE_EDGE_REPLICATION_TOKEN`. Incoming replica logs use the persistent edge PVC under `/var/lib/telemetryforge/wal/replicas`.

For regional/cross-region/cross-cloud production placement, use stable per-edge identities and peer DNS names that map to independent failure domains; do not label multiple replicas as different zones unless the underlying scheduling placement actually provides that separation.

### Worker

```text
GET :8081/health
GET :8081/ready
```

The worker readiness endpoint verifies Kafka and PostgreSQL independently of the
main consumer loop.

## Horizontal scaling

Gateway/dashboard replicas can scale independently.

Worker scaling is constrained by Kafka partition count:

```text
useful active consumer replicas <= assigned partitions
```

The local telemetry topics currently have six partitions. The example worker
HPA therefore caps at six replicas.

Increasing the HPA to 20 without increasing topic partitions would mostly
create idle consumer-group members.

In-process `TELEMETRYFORGE_WORKER_COUNT` is a separate concurrency control.

## Resource settings

The checked-in requests/limits are starting examples, not benchmark-derived
production recommendations.

They exist so scheduling/HPA behavior is explicit. v0.9.0 load testing will
produce measured sizing guidance.


## Production overlay

v1.0.0 adds:

```text
deployments/kubernetes/overlays/production/
```

Render:

```bash
kubectl kustomize deployments/kubernetes/overlays/production
```

The overlay enables API-key authentication, payload redaction, Kafka
TLS/SCRAM, and secret-mounted credentials.

It also disables anonymous scrape annotations because `/metrics` requires
`admin` when authentication is enabled.

See [Production Profile](production-profile.md).


## Router Deployment

The base now includes `router.yaml` with two replicas and admin port `8082`.

Routing configuration is generated as `telemetryforge-routing` from:

```text
deployments/kubernetes/base/routing/active.json
deployments/kubernetes/base/routing/shadow.json
```

Multiple router replicas safely share the PostgreSQL outbox through leased
`FOR UPDATE SKIP LOCKED` claims.

The production overlay mounts the API-key hash document so `/metrics` can use
the same admin-scope authentication model as worker/gateway.


## v1.9 HA rollout defaults

Gateway, router, and dashboard use rolling updates with `maxUnavailable: 0`; worker permits one unavailable replica. All four Deployments set `maxSurge: 1`, `minReadySeconds`, `progressDeadlineSeconds`, and hostname topology spreading. Router now has a PodDisruptionBudget and HPA in addition to gateway/worker/dashboard controls.

To generate evidence from a real cluster, use `proof/k8s-rollout.sh --execute --ack-cluster-change` or `scripts/proof-k8s-rollout.py --execute --acknowledge-cluster-change`.


## v2.4 lineage key

The base StatefulSet persists its default development lineage key in the WAL PVC. For production, mount an Ed25519 private/public key pair from your external-secret workflow and set `TELEMETRYFORGE_EDGE_LINEAGE_PRIVATE_KEY` / `TELEMETRYFORGE_EDGE_LINEAGE_PUBLIC_KEY` to those mounted paths. Do not place the private PEM directly in Git-managed manifests.
