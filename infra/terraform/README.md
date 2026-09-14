# Multi-Cloud Kubernetes Foundations

TelemetryForge v2.9 includes provider foundations for three managed Kubernetes services:

- `aws/` — Amazon EKS;
- `gcp/` — Google Kubernetes Engine;
- `azure/` — Azure Kubernetes Service.

Each module creates the network and Kubernetes compute foundation only. Kafka, PostgreSQL/TimescaleDB, DNS, cross-cloud private networking, and organization-specific identity/secrets remain deployment decisions outside these starter modules.

Validate all three with:

```bash
make terraform-check
```

These modules create billable infrastructure when applied. Review each provider plan, quotas, networking model, current service versions, and pricing before deployment.
