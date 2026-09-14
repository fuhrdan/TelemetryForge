# TelemetryForge GCP Terraform Foundation

This module creates the GCP Kubernetes/network foundation used by v2.9 multi-cloud deployments:

- custom VPC and regional subnet;
- regional GKE control plane;
- managed GKE node pool.

It does not provision Kafka or PostgreSQL/TimescaleDB. Supply organization-approved managed or self-hosted endpoints through the normal TelemetryForge deployment configuration.

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -check
terraform validate
terraform plan
```

After apply, run the `kubectl_config_command` output and deploy the Kubernetes manifests. This creates billable GCP resources; review the plan and current pricing first.
