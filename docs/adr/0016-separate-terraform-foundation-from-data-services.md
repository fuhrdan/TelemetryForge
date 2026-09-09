# ADR 0016: Separate Terraform Compute Foundation from Data Services

**Status:** Accepted  
**Date:** 2026-09-09

## Context

A Terraform example that provisions EKS, Kafka, PostgreSQL/TimescaleDB, DNS,
certificates, and every vendor-specific service would be large and would lock
TelemetryForge to one cloud architecture.

## Decision

The first AWS Terraform module provisions networking plus EKS compute only.

Kafka and PostgreSQL/TimescaleDB remain external service contracts supplied
through configuration/secrets.

## Alternatives considered

### Provision Amazon MSK and RDS immediately

More complete AWS stack, but it hardcodes data-platform choices and RDS does not
by itself represent the TimescaleDB extension/service model used by the project.

### No Terraform until every cloud service is decided

Would delay infrastructure-as-code and leave the Kubernetes deployment without a
reproducible cloud foundation.

## Consequences

The Terraform example is useful but intentionally incomplete as an
all-in-one deployment. It demonstrates infrastructure boundaries clearly and
lets organizations plug in their existing data services.
