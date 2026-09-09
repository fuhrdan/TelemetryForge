# Terraform Deployment Foundation

The first Terraform implementation lives at:

```text
infra/terraform/aws/
```

## Baseline

- Terraform 1.16.2
- AWS provider 6.62.0
- Amazon EKS Kubernetes 1.36

## Scope

Terraform creates the AWS compute/network foundation:

- VPC
- public/private subnets across two availability zones
- Internet Gateway
- NAT Gateway
- EKS cluster
- managed EKS node group
- IAM roles/policy attachments

It deliberately does **not** provision Kafka or TimescaleDB.

That boundary allows the same Kubernetes deployment to use:

- organization-managed Kafka
- Amazon MSK
- Confluent Cloud
- self-hosted Kafka
- Timescale Cloud
- organization-managed PostgreSQL/TimescaleDB

without changing the application deployment model.

## Validation

```bash
terraform -chdir=infra/terraform/aws fmt -check
terraform -chdir=infra/terraform/aws init -backend=false
terraform -chdir=infra/terraform/aws validate
```

CI runs those commands.

## Production considerations

The example uses one NAT Gateway to keep the foundation understandable. A
production availability design may require one NAT Gateway per availability
zone, private EKS endpoints, organization-specific IAM/access entries, VPC
endpoints, flow logs, encryption policy, and centralized Terraform state.

Those choices should be made explicitly rather than hidden in a portfolio
example.
