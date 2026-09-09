# Production Deployment Profile

The base Kubernetes manifests remain understandable development/application
manifests.

v1.0 adds a production-oriented overlay at:

```text
deployments/kubernetes/overlays/production/
```

It is a hardened starting point, not a turnkey claim that every organization's
security requirements are identical.

## Production overlay changes

- API-key authentication enabled
- API-key hash document mounted from Secret
- dashboard read key injected only into the Next.js server
- API payload redaction enabled
- Kafka TLS enabled
- Kafka SCRAM-SHA-512 enabled
- service-account token automount disabled
- `RuntimeDefault` seccomp
- non-root containers
- anonymous Prometheus scrape annotations disabled

## Required secrets

The example documents:

### `telemetryforge-api-keys`

Contains the hash-only API-key document.

### `telemetryforge-dashboard-auth`

Contains the raw **read-only** dashboard key.

### `telemetryforge-secrets`

Contains application dependency secrets such as:

- PostgreSQL URL
- Kafka SASL username/password

Do not commit the example with real values.

Use an External Secrets operator, cloud secret manager, Sealed Secrets, Vault,
or another organization-approved mechanism.

## Kafka TLS material

For a private/custom CA or mTLS client certificate, mount files and configure:

```text
TELEMETRYFORGE_KAFKA_CA_FILE
TELEMETRYFORGE_KAFKA_CERT_FILE
TELEMETRYFORGE_KAFKA_KEY_FILE
```

SASL mechanisms supported:

```text
plain
scram-sha-256
scram-sha-512
```

## Metrics authentication

In `api_key` mode `/metrics` requires `admin`.

The production overlay disables the simple anonymous scrape annotations.

`servicemonitor.example.yaml` demonstrates the Prometheus Operator pattern using
a bearer-token Secret. Adapt namespaces/labels to the real cluster.

## NetworkPolicy

`networkpolicy.example.yaml` is intentionally not included automatically.

Ingress-controller and monitoring namespace labels are cluster-specific. Apply
a reviewed policy for the actual environment rather than shipping a misleading
"universal" NetworkPolicy.

## TLS at the HTTP edge

The Kustomize base does not choose an ingress controller or certificate manager.

Production should terminate HTTPS using the organization's ingress/load-balancer
standard and keep gateway traffic private where possible.
