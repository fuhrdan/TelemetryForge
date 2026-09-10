// Package lifecycle implements immutable configuration change control for
// TelemetryForge policy, shaping, and routing documents.
package lifecycle

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "strings"
    "time"

    "github.com/fuhrdan/TelemetryForge/internal/policy"
    "github.com/fuhrdan/TelemetryForge/internal/router"
    "github.com/fuhrdan/TelemetryForge/internal/shaping"
)

type Kind string
type State string

const (
    KindPolicy  Kind = "policy"
    KindShaping Kind = "shaping"
    KindRouting Kind = "routing"

    StateDraft     State = "draft"
    StateShadow    State = "shadow"
    StateApproved  State = "approved"
    StateScheduled State = "scheduled"
    StateActive    State = "active"
    StateRetired   State = "retired"
)

type Artifact struct {
    ID           string          `json:"artifact_id"`
    Kind         Kind            `json:"kind"`
    Name         string          `json:"name"`
    Version      string          `json:"version"`
    SHA256       string          `json:"sha256"`
    Payload      json.RawMessage `json:"payload,omitempty"`
    State        State           `json:"state"`
    CreatedBy    string          `json:"created_by"`
    CreatedAt    time.Time       `json:"created_at"`
    ScheduledFor *time.Time      `json:"scheduled_for,omitempty"`
    ActivatedAt  *time.Time      `json:"activated_at,omitempty"`
    RetiredAt    *time.Time      `json:"retired_at,omitempty"`
}

type Approval struct {
    ArtifactID string    `json:"artifact_id"`
    Actor      string    `json:"actor"`
    Comment    string    `json:"comment,omitempty"`
    ApprovedAt time.Time `json:"approved_at"`
}

type Evidence struct {
    EvidenceID string    `json:"evidence_id"`
    ArtifactID string    `json:"artifact_id"`
    Type       string    `json:"type"`
    Reference  string    `json:"reference"`
    Result     string    `json:"result"`
    Reason     string    `json:"reason,omitempty"`
    TenantID   string    `json:"tenant_id,omitempty"`
    RecordedBy string    `json:"recorded_by"`
    RecordedAt time.Time `json:"recorded_at"`
}

type AuditEntry struct {
    AuditID    int64     `json:"audit_id"`
    ArtifactID string    `json:"artifact_id,omitempty"`
    Kind       Kind      `json:"kind,omitempty"`
    Action     string    `json:"action"`
    Actor      string    `json:"actor"`
    Detail     string    `json:"detail,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}

type RuntimeLoad struct {
    InstanceID string    `json:"instance_id"`
    Component  string    `json:"component"`
    Kind       Kind      `json:"kind"`
    Role       string    `json:"role"`
    ArtifactID string    `json:"artifact_id"`
    SHA256     string    `json:"sha256"`
    LoadedAt   time.Time `json:"loaded_at"`
}

type Convergence struct {
    Kind          Kind   `json:"kind"`
    Role          string `json:"role"`
    DesiredID     string `json:"desired_artifact_id"`
    DesiredSHA256 string `json:"desired_sha256"`
    LiveInstances int    `json:"live_instances"`
    Matching      int    `json:"matching_instances"`
    Converged     bool   `json:"converged"`
}

func ValidateKind(kind Kind) error {
    switch kind {
    case KindPolicy, KindShaping, KindRouting:
        return nil
    default:
        return fmt.Errorf("unsupported configuration kind %q", kind)
    }
}

func ValidatePayload(kind Kind, payload []byte) (name, version, digest string, err error) {
    if err = ValidateKind(kind); err != nil { return }
    if !json.Valid(payload) { err = errors.New("configuration payload is not valid JSON"); return }

    switch kind {
    case KindPolicy:
        var value policy.Policy
        if err = decodeStrict(payload, &value); err != nil { return }
        if err = value.Validate(); err != nil { return }
        name, version = value.Name, value.Version
    case KindShaping:
        var value shaping.Config
        if err = decodeStrict(payload, &value); err != nil { return }
        if err = value.Validate(); err != nil { return }
        name, version = value.Name, value.Version
    case KindRouting:
        var value router.Config
        if err = decodeStrict(payload, &value); err != nil { return }
        if err = value.Validate(); err != nil { return }
        name, version = value.Name, value.Version
    }

    sum := sha256.Sum256(payload)
    digest = hex.EncodeToString(sum[:])
    return
}

func CanTransition(from, to State) bool {
    allowed := map[State][]State{
        StateDraft:     {StateShadow, StateRetired},
        StateShadow:    {StateApproved, StateRetired},
        StateApproved:  {StateScheduled, StateActive, StateRetired},
        StateScheduled: {StateActive, StateRetired},
        StateActive:    {StateRetired},
        StateRetired:   {StateActive}, // explicit rollback only
    }
    for _, candidate := range allowed[from] {
        if candidate == to { return true }
    }
    return false
}

func ValidateEvidence(result, reason string) error {
    switch strings.ToLower(strings.TrimSpace(result)) {
    case "pass":
        return nil
    case "waived":
        if strings.TrimSpace(reason) == "" { return errors.New("waived evidence requires a reason") }
        return nil
    case "fail":
        return nil
    default:
        return fmt.Errorf("unsupported evidence result %q", result)
    }
}

func decodeStrict(payload []byte, target any) error {
    decoder := json.NewDecoder(bytes.NewReader(payload))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(target); err != nil { return err }
    var extra any
    if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
        if err == nil { return errors.New("unexpected trailing JSON") }
        return err
    }
    return nil
}
