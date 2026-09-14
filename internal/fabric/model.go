// Package fabric evaluates the TelemetryForge v3 Global Edge Fabric runtime
// contract. It intentionally separates durable acceptance readiness from
// downstream delivery readiness so an observable backend outage does not erase
// the value of a healthy WAL + replication quorum.
package fabric

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	Format          = "telemetryforge-global-edge-fabric"
	ContractVersion = 1
	ReleaseVersion  = "3.0.0"
)

type State string

const (
	StateReady    State = "ready"
	StateDegraded State = "degraded"
	StateNotReady State = "not_ready"
)

type Input struct {
	WALReady                   bool
	NodeID                     string
	WALBytes                   int64
	WALMaxBytes                int64
	PendingRecords             uint64
	LineageKeyID               string
	SealedSegments             int
	DurabilityMode             string
	DurabilityQuorum           int
	ReplicationConfiguredPeers int
	ReplicationReady           bool
	MeshPolicy                 string
	MeshConfiguredPeers        int
	MeshHealthyPeers           int
	DeliveryReady              bool
	FastPathBatchSize          int
	EBPFEnabled                bool
	EBPFRequired               bool
	EBPFActive                 bool
}

type Capability struct {
	Name     string `json:"name"`
	State    State  `json:"state"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

type Snapshot struct {
	Format          string       `json:"format"`
	ContractVersion int          `json:"contract_version"`
	Release         string       `json:"release"`
	NodeID          string       `json:"node_id"`
	State           State        `json:"state"`
	AcceptanceReady bool         `json:"acceptance_ready"`
	DeliveryReady   bool         `json:"delivery_ready"`
	AuditReady      bool         `json:"audit_ready"`
	WALPressure     float64      `json:"wal_pressure"`
	PendingRecords  uint64       `json:"pending_records"`
	Capabilities    []Capability `json:"capabilities"`
	Reasons         []string     `json:"reasons,omitempty"`
	GeneratedAt     time.Time    `json:"generated_at"`
}

func Evaluate(input Input) Snapshot {
	nodeID := strings.TrimSpace(input.NodeID)
	pressure := 1.0
	walReady := input.WALReady && input.WALMaxBytes > 0 && input.WALBytes >= 0 && input.WALBytes < input.WALMaxBytes
	if input.WALMaxBytes > 0 && input.WALBytes >= 0 {
		pressure = float64(input.WALBytes) / float64(input.WALMaxBytes)
		if pressure < 0 {
			pressure = 0
		}
	}

	mode := strings.TrimSpace(strings.ToLower(input.DurabilityMode))
	if mode == "" {
		mode = "local"
	}
	replicationRequired := mode != "local"
	replicationReady := !replicationRequired || input.ReplicationReady
	lineageReady := strings.TrimSpace(input.LineageKeyID) != ""
	fastPathReady := input.FastPathBatchSize > 0 && input.FastPathBatchSize <= 1024
	ebpfReady := !input.EBPFRequired || (input.EBPFEnabled && input.EBPFActive)

	acceptanceReady := walReady && replicationReady && lineageReady && ebpfReady
	state := StateReady
	reasons := make([]string, 0, 5)
	if !walReady {
		state = StateNotReady
		reasons = append(reasons, "durable WAL cannot safely accept additional telemetry")
	}
	if !replicationReady {
		state = StateNotReady
		reasons = append(reasons, "configured durability quorum is unavailable")
	}
	if !lineageReady {
		state = StateNotReady
		reasons = append(reasons, "cryptographic lineage identity is unavailable")
	}
	if !ebpfReady {
		state = StateNotReady
		reasons = append(reasons, "required eBPF collection is inactive")
	}
	if state != StateNotReady && !input.DeliveryReady {
		state = StateDegraded
		reasons = append(reasons, "no healthy downstream mesh delivery path is currently available; accepted telemetry remains WAL-backed")
	}
	if state == StateReady && input.EBPFEnabled && !input.EBPFActive {
		state = StateDegraded
		reasons = append(reasons, "optional eBPF collection is configured but inactive")
	}
	if state == StateReady && !fastPathReady {
		state = StateDegraded
		reasons = append(reasons, "fast-path replay configuration is outside the supported bounded range")
	}

	capabilities := []Capability{
		{
			Name: "durable-acceptance", State: boolState(walReady), Required: true,
			Detail: fmt.Sprintf("wal=%d/%d bytes pending=%d", input.WALBytes, input.WALMaxBytes, input.PendingRecords),
		},
		{
			Name: "replicated-durability", State: boolState(replicationReady), Required: replicationRequired,
			Detail: fmt.Sprintf("mode=%s quorum=%d peers=%d", mode, input.DurabilityQuorum, input.ReplicationConfiguredPeers),
		},
		{
			Name: "global-routing", State: degradedState(input.DeliveryReady), Required: true,
			Detail: fmt.Sprintf("policy=%s configured_peers=%d healthy_peers=%d", nonempty(input.MeshPolicy, "locality"), input.MeshConfiguredPeers, input.MeshHealthyPeers),
		},
		{
			Name: "cryptographic-lineage", State: boolState(lineageReady), Required: true,
			Detail: fmt.Sprintf("key_id=%s sealed_segments=%d", nonempty(input.LineageKeyID, "unavailable"), input.SealedSegments),
		},
		{
			Name: "fast-path", State: degradedState(fastPathReady), Required: false,
			Detail: fmt.Sprintf("replay_batch_size=%d", input.FastPathBatchSize),
		},
		{
			Name: "ebpf-collection", State: optionalState(!input.EBPFEnabled, input.EBPFActive), Required: input.EBPFRequired,
			Detail: fmt.Sprintf("enabled=%t required=%t active=%t", input.EBPFEnabled, input.EBPFRequired, input.EBPFActive),
		},
	}

	return Snapshot{
		Format: Format, ContractVersion: ContractVersion, Release: ReleaseVersion,
		NodeID: nodeID, State: state, AcceptanceReady: acceptanceReady,
		DeliveryReady: input.DeliveryReady, AuditReady: lineageReady,
		WALPressure: pressure, PendingRecords: input.PendingRecords,
		Capabilities: capabilities, Reasons: reasons, GeneratedAt: time.Now().UTC(),
	}
}

func (snapshot Snapshot) Validate() error {
	if snapshot.Format != Format || snapshot.ContractVersion != ContractVersion || snapshot.Release != ReleaseVersion {
		return errors.New("unsupported fabric status format/version")
	}
	if strings.TrimSpace(snapshot.NodeID) == "" {
		return errors.New("fabric node_id is required")
	}
	if snapshot.WALPressure < 0 {
		return errors.New("wal pressure cannot be negative")
	}
	if snapshot.State == StateReady && (!snapshot.AcceptanceReady || !snapshot.DeliveryReady || !snapshot.AuditReady) {
		return errors.New("ready fabric snapshot contradicts readiness dimensions")
	}
	if snapshot.State == StateNotReady && snapshot.AcceptanceReady {
		return errors.New("not_ready fabric snapshot cannot advertise acceptance_ready")
	}
	if len(snapshot.Capabilities) < 6 {
		return errors.New("fabric snapshot is missing required capability records")
	}
	return nil
}

func boolState(ok bool) State {
	if ok {
		return StateReady
	}
	return StateNotReady
}

func degradedState(ok bool) State {
	if ok {
		return StateReady
	}
	return StateDegraded
}

func optionalState(disabled, active bool) State {
	if disabled || active {
		return StateReady
	}
	return StateDegraded
}

func nonempty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
