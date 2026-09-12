// Package wal implements TelemetryForge's crash-recoverable edge write-ahead log.
package wal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const (
	Format  = "telemetryforge-edge-wal"
	Version = 1
)

// Record is the durable envelope written before an edge request is acknowledged.
type Record struct {
	Format         string       `json:"format"`
	FormatVersion  int          `json:"format_version"`
	EdgeID         string       `json:"edge_id"`
	Topic          string       `json:"topic"`
	EdgeSequence   uint64       `json:"edge_sequence"`
	SourceSequence uint64       `json:"source_sequence"`
	AcceptedAt     time.Time    `json:"accepted_at"`
	PayloadSHA256  string       `json:"payload_sha256"`
	Event          domain.Event `json:"event"`
}

func (record Record) Validate() error {
	if record.Format != Format || record.FormatVersion != Version {
		return errors.New("unsupported WAL record format/version")
	}
	if strings.TrimSpace(record.EdgeID) == "" || strings.TrimSpace(record.Topic) == "" {
		return errors.New("edge_id and topic are required")
	}
	if record.EdgeSequence == 0 || record.SourceSequence == 0 {
		return errors.New("edge_sequence and source_sequence must be positive")
	}
	if record.AcceptedAt.IsZero() {
		return errors.New("accepted_at is required")
	}
	if err := record.Event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}
	hash, err := EventHash(record.Event)
	if err != nil {
		return err
	}
	if record.PayloadSHA256 != hash {
		return errors.New("event payload SHA-256 mismatch")
	}
	return nil
}

// EventHash returns the SHA-256 of the canonical JSON event representation.
func EventHash(event domain.Event) (string, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("encode event for hash: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

type Stats struct {
	Directory         string `json:"directory"`
	Segments          int    `json:"segments"`
	Bytes             int64  `json:"bytes"`
	MaxBytes          int64  `json:"max_bytes"`
	PendingRecords    uint64 `json:"pending_records"`
	CommittedSequence uint64 `json:"committed_sequence"`
	LastSequence      uint64 `json:"last_sequence"`
}
