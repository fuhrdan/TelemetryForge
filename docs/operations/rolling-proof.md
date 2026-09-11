# Kubernetes Rolling-Upgrade Proof

The Kubernetes proof harness changes a live cluster and therefore requires two
explicit acknowledgements:

```bash
proof/k8s-rollout.sh --execute --ack-cluster-change
```

or:

```bash
scripts/proof-k8s-rollout.py --execute --acknowledge-cluster-change
```

The harness restarts each TelemetryForge Deployment and waits for rollout status.
For a zero-request-loss claim, run concurrent representative traffic and add an
explicit assertion/evidence source; rollout completion alone is insufficient.
