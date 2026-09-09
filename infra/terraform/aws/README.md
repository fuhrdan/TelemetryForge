# TelemetryForge AWS Terraform Foundation

This directory provisions the **Kubernetes compute/network foundation**, not
every TelemetryForge dependency.

It creates:

- VPC
- two public subnets
- two private subnets
- Internet Gateway
- one NAT Gateway
- Amazon EKS cluster
- managed EKS node group
- required IAM roles/policies

## Version baseline

- Terraform 1.16.2
- AWS provider 6.62.0
- Amazon EKS Kubernetes 1.36

Upstream Kubernetes 1.37 is current, but EKS currently lists 1.36 as its newest
standard-support minor. The Terraform default follows the managed-service
support boundary rather than requesting an unavailable version.

## Why Kafka and TimescaleDB are not provisioned here

The deployment contract accepts external Kafka and PostgreSQL/TimescaleDB
endpoints. That keeps the first Terraform release usable with managed services,
self-hosted clusters, or organization-standard data platforms.

A later cloud-specific module can choose MSK/RDS/Timescale Cloud without
forcing that architecture into the Kubernetes application manifests.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -check
terraform validate
terraform plan
```

After apply:

```bash
terraform output -raw kubectl_config_command
```

Then create the TelemetryForge secret and apply
`deployments/kubernetes/base/`.

## Cost note

This example creates an EKS control plane, EC2 nodes, and a NAT Gateway. It is
not a free local demo. Review the Terraform plan and AWS pricing before apply.
