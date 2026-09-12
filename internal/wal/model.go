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
	Format         = "telemetryforge-edge-wal"
	LegacyVersion  = 1
	Version        = 2
	LineageVersion = 1
)

// Record is the durable envelope written before an edge request is acknowledged.
// Version 2 adds an ordered cryptographic hash chain without changing the event payload.
type Record struct {
	Format             string       `json:"format"`
	FormatVersion      int          `json:"format_version"`
	EdgeID             string       `json:"edge_id"`
	Topic              string       `json:"topic"`
	EdgeSequence       uint64       `json:"edge_sequence"`
	SourceSequence     uint64       `json:"source_sequence"`
	AcceptedAt         time.Time    `json:"accepted_at"`
	PayloadSHA256      string       `json:"payload_sha256"`
	LineageVersion     int          `json:"lineage_version,omitempty"`
	PreviousRecordHash string       `json:"previous_record_hash,omitempty"`
	RecordHash         string       `json:"record_hash,omitempty"`
	Event              domain.Event `json:"event"`
}

type recordHashMaterial struct {
	Format             string `json:"format"`
	FormatVersion      int    `json:"format_version"`
	EdgeID             string `json:"edge_id"`
	Topic              string `json:"topic"`
	EdgeSequence       uint64 `json:"edge_sequence"`
	SourceSequence     uint64 `json:"source_sequence"`
	AcceptedAt         string `json:"accepted_at"`
	PayloadSHA256      string `json:"payload_sha256"`
	LineageVersion     int    `json:"lineage_version"`
	PreviousRecordHash string `json:"previous_record_hash,omitempty"`
}

func (record Record) Validate() error {
	if record.Format != Format || (record.FormatVersion != LegacyVersion && record.FormatVersion != Version) {
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
	if record.FormatVersion == LegacyVersion {
		if record.LineageVersion != 0 || record.PreviousRecordHash != "" || record.RecordHash != "" {
			return errors.New("legacy WAL record contains unsupported lineage fields")
		}
		return nil
	}
	if record.LineageVersion != LineageVersion {
		return errors.New("unsupported WAL lineage version")
	}
	if record.PreviousRecordHash != "" && !validSHA256(record.PreviousRecordHash) {
		return errors.New("invalid previous record hash")
	}
	if !validSHA256(record.RecordHash) {
		return errors.New("invalid record hash")
	}
	expected, err := ComputeRecordHash(record)
	if err != nil {
		return err
	}
	if record.RecordHash != expected {
		return errors.New("WAL record lineage hash mismatch")
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

// ComputeRecordHash hashes the immutable routing and sequencing metadata plus
// the event payload digest and previous record hash. The event bytes are covered
// transitively by PayloadSHA256.
func ComputeRecordHash(record Record) (string, error) {
	material := recordHashMaterial{
		Format: record.Format, FormatVersion: record.FormatVersion, EdgeID: record.EdgeID,
		Topic: record.Topic, EdgeSequence: record.EdgeSequence, SourceSequence: record.SourceSequence,
		AcceptedAt: record.AcceptedAt.UTC().Format(time.RFC3339Nano), PayloadSHA256: record.PayloadSHA256,
		LineageVersion: record.LineageVersion, PreviousRecordHash: record.PreviousRecordHash,
	}
	payload, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// RecordDigest returns the hash used as a Merkle leaf. Legacy v1 records are
// deterministically anchored by hashing their canonical JSON representation;
// v2 records use their explicit RecordHash.
func RecordDigest(record Record) (string, error) {
	if record.FormatVersion == Version {
		if err := record.Validate(); err != nil {
			return "", err
		}
		return record.RecordHash, nil
	}
	if err := record.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyRecordChain validates ordered record lineage starting from previousHash
// and returns the final digest plus every Merkle leaf.
func VerifyRecordChain(records []Record, previousHash string) (string, []string, error) {
	if previousHash != "" && !validSHA256(previousHash) {
		return "", nil, errors.New("invalid record-chain anchor")
	}
	current := previousHash
	leaves := make([]string, 0, len(records))
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return "", nil, err
		}
		if record.FormatVersion == Version && record.PreviousRecordHash != current {
			return "", nil, fmt.Errorf("WAL lineage discontinuity at edge sequence %d", record.EdgeSequence)
		}
		digest, err := RecordDigest(record)
		if err != nil {
			return "", nil, err
		}
		current = digest
		leaves = append(leaves, digest)
	}
	return current, leaves, nil
}

type Stats struct {
	Directory         string `json:"directory"`
	Segments          int    `json:"segments"`
	Bytes             int64  `json:"bytes"`
	MaxBytes          int64  `json:"max_bytes"`
	PendingRecords    uint64 `json:"pending_records"`
	CommittedSequence uint64 `json:"committed_sequence"`
	LastSequence      uint64 `json:"last_sequence"`
	LineageKeyID      string `json:"lineage_key_id"`
	SealedSegments    int    `json:"sealed_segments"`
	LastRecordHash    string `json:"last_record_hash,omitempty"`
	LastSegmentRoot   string `json:"last_segment_root,omitempty"`
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
