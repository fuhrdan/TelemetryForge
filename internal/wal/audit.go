package wal

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/lineage"
)

// AuditSegment describes one signed segment attestation.
type AuditSegment struct {
	Segment         string `json:"segment"`
	FirstSequence   uint64 `json:"first_sequence"`
	LastSequence    uint64 `json:"last_sequence"`
	RecordCount     uint64 `json:"record_count"`
	MerkleRoot      string `json:"merkle_root"`
	KeyID           string `json:"key_id"`
	SignatureValid  bool   `json:"signature_valid"`
	WALPresent      bool   `json:"wal_present"`
	ContentVerified bool   `json:"content_verified"`
}

// AuditReport is emitted by telemetryctl audit verify.
type AuditReport struct {
	Directory            string         `json:"directory"`
	EdgeID               string         `json:"edge_id,omitempty"`
	TrustMode            string         `json:"trust_mode"`
	TrustedKeyID         string         `json:"trusted_key_id,omitempty"`
	SealedSegments       int            `json:"sealed_segments"`
	ContentSegments      int            `json:"content_segments"`
	SealOnlySegments     int            `json:"seal_only_segments"`
	UnsealedSegments     int            `json:"unsealed_segments"`
	LastSequence         uint64         `json:"last_sequence"`
	LastRecordHash       string         `json:"last_record_hash,omitempty"`
	LastSegmentRoot      string         `json:"last_segment_root,omitempty"`
	RecordChainVerified  bool           `json:"record_chain_verified"`
	SegmentChainVerified bool           `json:"segment_chain_verified"`
	Segments             []AuditSegment `json:"segments"`
}

// AuditDirectory verifies signed seals and any retained WAL content. Without a
// trusted public key it proves tamper evidence using each seal's embedded key;
// with a trusted key it additionally authenticates the edge signer identity.
func AuditDirectory(directory, trustedPublicKeyPath string) (AuditReport, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return AuditReport{}, errors.New("WAL directory is required")
	}
	var trusted ed25519.PublicKey
	report := AuditReport{Directory: directory, TrustMode: "embedded-key", RecordChainVerified: true, SegmentChainVerified: true}
	if strings.TrimSpace(trustedPublicKeyPath) != "" {
		key, err := lineage.LoadPublicKey(trustedPublicKeyPath)
		if err != nil {
			return report, fmt.Errorf("load trusted lineage key: %w", err)
		}
		trusted = key
		report.TrustMode = "trusted-key"
		report.TrustedKeyID = lineage.KeyID(key)
	}

	sealPaths, err := filepath.Glob(filepath.Join(directory, "*.tfseal"))
	if err != nil {
		return report, err
	}
	sort.Strings(sealPaths)
	seals := make([]lineage.Seal, 0, len(sealPaths))
	var previousRoot string
	var previousLast uint64
	for _, path := range sealPaths {
		seal, err := lineage.ReadSeal(path)
		if err != nil {
			return report, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if err := lineage.VerifySeal(seal, trusted); err != nil {
			return report, fmt.Errorf("verify %s: %w", filepath.Base(path), err)
		}
		if len(seals) > 0 {
			if seal.PreviousSegmentRoot != previousRoot || seal.FirstSequence != previousLast+1 {
				return report, fmt.Errorf("lineage seal chain discontinuity at %s", filepath.Base(path))
			}
		} else if seal.PreviousSegmentRoot != "" {
			return report, fmt.Errorf("first retained seal %s references an unavailable predecessor", filepath.Base(path))
		}
		if report.EdgeID == "" {
			report.EdgeID = seal.EdgeID
		} else if report.EdgeID != seal.EdgeID {
			return report, errors.New("lineage seals contain multiple edge identities")
		}
		previousRoot = seal.MerkleRoot
		previousLast = seal.LastSequence
		seals = append(seals, seal)
	}

	previousHash := ""
	for index, seal := range seals {
		walPath := filepath.Join(directory, seal.Segment)
		segment := AuditSegment{Segment: seal.Segment, FirstSequence: seal.FirstSequence, LastSequence: seal.LastSequence, RecordCount: seal.RecordCount, MerkleRoot: seal.MerkleRoot, KeyID: seal.KeyID, SignatureValid: true}
		records, _, _, scanErr := scanSegment(walPath, false)
		if scanErr == nil {
			segment.WALPresent = true
			finalHash, leaves, chainErr := VerifyRecordChain(records, previousHash)
			if chainErr != nil {
				return report, fmt.Errorf("verify record chain in %s: %w", seal.Segment, chainErr)
			}
			if err := verifySealAgainstRecords(seal, records, leaves); err != nil {
				return report, err
			}
			if finalHash != seal.LastRecordHash {
				return report, fmt.Errorf("last record hash mismatch in %s", seal.Segment)
			}
			segment.ContentVerified = true
			report.ContentSegments++
		} else if errors.Is(scanErr, os.ErrNotExist) {
			report.SealOnlySegments++
		} else {
			return report, scanErr
		}
		previousHash = seal.LastRecordHash
		report.Segments = append(report.Segments, segment)
		report.SealedSegments = index + 1
		report.LastSequence = seal.LastSequence
		report.LastRecordHash = seal.LastRecordHash
		report.LastSegmentRoot = seal.MerkleRoot
	}

	// A crash-recovered active segment is intentionally unsealed until rotation
	// or clean shutdown. It still has a verifiable record chain anchored to the
	// latest signed segment.
	walPaths, err := filepath.Glob(filepath.Join(directory, "*.tfwal"))
	if err != nil {
		return report, err
	}
	sort.Strings(walPaths)
	sealedNames := make(map[string]struct{}, len(seals))
	for _, seal := range seals {
		sealedNames[seal.Segment] = struct{}{}
	}
	unsealed := make([]string, 0, 1)
	for _, path := range walPaths {
		if _, ok := sealedNames[filepath.Base(path)]; !ok {
			unsealed = append(unsealed, path)
		}
	}
	if len(unsealed) > 1 {
		return report, errors.New("more than one unsealed WAL segment found")
	}
	for _, path := range unsealed {
		records, _, _, err := scanSegment(path, false)
		if err != nil {
			return report, err
		}
		if len(records) == 0 {
			continue
		}
		finalHash, _, err := VerifyRecordChain(records, previousHash)
		if err != nil {
			return report, fmt.Errorf("verify active record chain: %w", err)
		}
		if report.EdgeID == "" {
			report.EdgeID = records[0].EdgeID
		}
		report.UnsealedSegments++
		report.LastSequence = records[len(records)-1].EdgeSequence
		report.LastRecordHash = finalHash
	}
	return report, nil
}
