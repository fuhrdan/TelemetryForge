// Package incidentarchive implements the portable TelemetryForge .tfincident
// incident archive format.
//
// The archive deliberately uses ordinary ZIP plus JSON/JSONL so an unencrypted
// archive remains inspectable with standard tools. Optional encryption wraps
// the complete ZIP in an authenticated AES-256-GCM envelope.
package incidentarchive

import (
	"encoding/json"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
)

const (
	FormatName     = "TelemetryForge Incident Archive"
	FormatVersion  = 1
	ProductVersion = "1.5.0"

	MaxEntries      = 128
	MaxArchiveBytes = 512 << 20
	MaxMemberBytes  = 128 << 20
	MaxEvents       = 100000
)

// Manifest is the integrity/compatibility root of a .tfincident archive.
//
// Every member except manifest.json is listed in Entries with a SHA-256 digest
// and byte size. Extra or missing members fail verification.
type Manifest struct {
	Format          string                 `json:"format"`
	FormatVersion   int                    `json:"format_version"`
	TelemetryForge  string                 `json:"telemetryforge_version"`
	ArchiveID       string                 `json:"archive_id"`
	CreatedAt       time.Time              `json:"created_at"`
	TenantID        string                 `json:"tenant_id"`
	IncidentID      string                 `json:"incident_id"`
	EventCount      int                    `json:"event_count"`
	SchemaCount     int                    `json:"schema_count"`
	ReplayRunCount  int                    `json:"replay_run_count"`
	CostResultCount int                    `json:"cost_result_count"`
	Configurations  []ConfigurationMeta    `json:"configurations,omitempty"`
	Entries         map[string]EntryDigest `json:"entries"`
}

// EntryDigest records one archive member's integrity metadata.
type EntryDigest struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// IncidentMetadata is the portable incident record.
type IncidentMetadata struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	TriggerReason string    `json:"trigger_reason,omitempty"`
	FrozenFrom    time.Time `json:"frozen_from"`
	FrozenTo      time.Time `json:"frozen_to"`
	DetectedAt    time.Time `json:"detected_at"`
}

// EventRecord preserves the first captured Flight Recorder timestamp together
// with the canonical event envelope.
type EventRecord struct {
	CapturedAt time.Time    `json:"captured_at"`
	Event      domain.Event `json:"event"`
}

// ConfigurationSnapshot preserves the exact policy/shaping/routing JSON that
// accompanied an exported investigation.
type ConfigurationSnapshot struct {
	Kind    string          `json:"kind"`
	Role    string          `json:"role"`
	Name    string          `json:"name,omitempty"`
	Version string          `json:"version,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// ConfigurationMeta describes one configuration file stored in the archive.
type ConfigurationMeta struct {
	Kind    string `json:"kind"`
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path"`
}

// ImportProvenance is the durable audit record for one archive import.
type ImportProvenance struct {
	ArchiveID             string    `json:"archive_id"`
	SourceTenantID        string    `json:"source_tenant_id"`
	SourceIncidentID      string    `json:"source_incident_id"`
	ImportedIncidentID    string    `json:"imported_incident_id"`
	FormatVersion         int       `json:"format_version"`
	TelemetryForgeVersion string    `json:"telemetryforge_version"`
	FileSHA256            string    `json:"file_sha256"`
	Encrypted             bool      `json:"encrypted"`
	ImportedAt            time.Time `json:"imported_at,omitempty"`
}

// Bundle is the in-memory portable incident.
type Bundle struct {
	Manifest       Manifest
	Incident       IncidentMetadata
	Events         []EventRecord
	EvidenceGraph  evidence.Graph
	ReplayRuns     []replay.Run
	CostResults    []costsim.Result
	Schemas        []schema.RegistryEntry
	SchemaDrifts   []schema.Drift
	Configurations []ConfigurationSnapshot
}
