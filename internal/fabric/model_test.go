package fabric

import "testing"

func healthy() Input {
	return Input{
		WALReady: true, NodeID: "edge-aws-a", WALBytes: 1024, WALMaxBytes: 1 << 20, PendingRecords: 3,
		LineageKeyID: "edge-key-01", SealedSegments: 4,
		DurabilityMode: "cross-cloud", DurabilityQuorum: 2, ReplicationConfiguredPeers: 3, ReplicationReady: true,
		MeshPolicy: "global", MeshConfiguredPeers: 3, MeshHealthyPeers: 3, DeliveryReady: true,
		FastPathBatchSize: 64,
	}
}

func TestHealthyFabricIsReady(t *testing.T) {
	snapshot := Evaluate(healthy())
	if snapshot.State != StateReady || !snapshot.AcceptanceReady || !snapshot.DeliveryReady || !snapshot.AuditReady {
		t.Fatalf("unexpected healthy snapshot: %+v", snapshot)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDownstreamPartitionDegradesWithoutClosingAcceptance(t *testing.T) {
	input := healthy()
	input.DeliveryReady = false
	input.MeshHealthyPeers = 0
	snapshot := Evaluate(input)
	if snapshot.State != StateDegraded || !snapshot.AcceptanceReady || snapshot.DeliveryReady {
		t.Fatalf("downstream partition must be degraded but acceptance-ready: %+v", snapshot)
	}
}

func TestQuorumLossFailsAcceptanceClosed(t *testing.T) {
	input := healthy()
	input.ReplicationReady = false
	snapshot := Evaluate(input)
	if snapshot.State != StateNotReady || snapshot.AcceptanceReady {
		t.Fatalf("quorum loss must close acceptance: %+v", snapshot)
	}
}

func TestWALCapacityFailsAcceptanceClosed(t *testing.T) {
	input := healthy()
	input.WALBytes = input.WALMaxBytes
	snapshot := Evaluate(input)
	if snapshot.State != StateNotReady || snapshot.AcceptanceReady {
		t.Fatalf("full WAL must close acceptance: %+v", snapshot)
	}
}

func TestRequiredEBPFFailsClosed(t *testing.T) {
	input := healthy()
	input.EBPFEnabled = true
	input.EBPFRequired = true
	input.EBPFActive = false
	snapshot := Evaluate(input)
	if snapshot.State != StateNotReady || snapshot.AcceptanceReady {
		t.Fatalf("required inactive eBPF must fail closed: %+v", snapshot)
	}
}

func TestOptionalEBPFFailureIsDegraded(t *testing.T) {
	input := healthy()
	input.EBPFEnabled = true
	input.EBPFActive = false
	snapshot := Evaluate(input)
	if snapshot.State != StateDegraded || !snapshot.AcceptanceReady {
		t.Fatalf("optional inactive eBPF should degrade only: %+v", snapshot)
	}
}

func TestDownstreamPartitionMarksRoutingCapabilityDegraded(t *testing.T) {
	input := healthy()
	input.DeliveryReady = false
	snapshot := Evaluate(input)
	for _, capability := range snapshot.Capabilities {
		if capability.Name == "global-routing" {
			if capability.State != StateDegraded {
				t.Fatalf("expected degraded routing capability, got %s", capability.State)
			}
			return
		}
	}
	t.Fatal("global-routing capability missing")
}
