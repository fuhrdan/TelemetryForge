# High Availability

TelemetryForge v1.9 hardens the Kubernetes baseline for rolling maintenance and
replica disruption without claiming a multi-region active/active architecture.

Gateway, router, and dashboard use `maxUnavailable: 0`; worker permits one
unavailable replica. All workloads use `maxSurge: 1`, `minReadySeconds: 5`, a
600-second progress deadline, and hostname topology spreading. Gateway, worker,
router, and dashboard have PodDisruptionBudgets; router joins the existing HPA
set with a 2-6 replica range.

These settings are deployment defaults, not proof of zero request loss. Use the
rolling-restart proof harness under representative traffic if that claim matters.
