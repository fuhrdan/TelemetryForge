# TelemetryForge Azure Terraform Foundation

This module creates the Azure Kubernetes/network foundation used by v2.9 multi-cloud deployments:

- resource group;
- virtual network and AKS subnet;
- AKS cluster with a managed system node pool.

Kafka and PostgreSQL/TimescaleDB remain external deployment dependencies so organizations can use their standard managed services or self-hosted platforms.

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -check
terraform validate
terraform plan
```

After apply, run the `kubectl_config_command` output and deploy the Kubernetes manifests. This creates billable Azure resources; review the plan and current pricing first.
