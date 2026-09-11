// Package proof defines reproducible operational evidence artifacts for
// TelemetryForge scale, HA, recovery, and benchmark runs.
package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

const (
	Format  = "telemetryforge-operational-proof"
	Version = 1
)

type Assertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected,omitempty"`
	Observed string `json:"observed,omitempty"`
	Detail   string `json:"detail,omitempty"`
}
type Measurement struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}
type Evidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	SHA256    string `json:"sha256,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Detail    string `json:"detail,omitempty"`
}
type Fingerprint struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type Run struct {
	Format        string            `json:"format"`
	FormatVersion int               `json:"format_version"`
	RunID         string            `json:"run_id"`
	GitCommit     string            `json:"git_commit"`
	Scenario      string            `json:"scenario"`
	Status        string            `json:"status"`
	StartedAt     time.Time         `json:"started_at"`
	CompletedAt   time.Time         `json:"completed_at"`
	Environment   map[string]string `json:"environment,omitempty"`
	Assertions    []Assertion       `json:"assertions"`
	Measurements  []Measurement     `json:"measurements,omitempty"`
	Evidence      []Evidence        `json:"evidence"`
	Configuration []Fingerprint     `json:"configuration_fingerprints"`
	Notes         []string          `json:"notes,omitempty"`
}
type Stored struct {
	Run            Run       `json:"run"`
	ArtifactSHA256 string    `json:"artifact_sha256"`
	ArtifactBytes  int64     `json:"artifact_bytes"`
	RecordedAt     time.Time `json:"recorded_at"`
}

func (r Run) Validate() error {
	if r.Format != Format || r.FormatVersion != Version {
		return fmt.Errorf("unsupported proof format/version")
	}
	if strings.TrimSpace(r.RunID) == "" || strings.TrimSpace(r.Scenario) == "" || strings.TrimSpace(r.GitCommit) == "" {
		return errors.New("run_id, scenario, and git_commit are required")
	}
	if r.Status != "pass" && r.Status != "fail" && r.Status != "error" {
		return errors.New("status must be pass, fail, or error")
	}
	if r.StartedAt.IsZero() || r.CompletedAt.IsZero() || r.CompletedAt.Before(r.StartedAt) {
		return errors.New("invalid proof timestamps")
	}
	if len(r.Assertions) == 0 {
		return errors.New("proof requires at least one assertion")
	}
	if len(r.Evidence) == 0 {
		return errors.New("proof requires at least one evidence item")
	}
	if len(r.Configuration) == 0 {
		return errors.New("proof requires configuration fingerprints")
	}
	allPassed := true
	for _, a := range r.Assertions {
		if strings.TrimSpace(a.Name) == "" {
			return errors.New("assertion name is required")
		}
		if !a.Passed {
			allPassed = false
		}
	}
	if r.Status == "pass" && !allPassed {
		return errors.New("pass proof contains failed assertion")
	}
	for _, m := range r.Measurements {
		if strings.TrimSpace(m.Name) == "" || math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			return errors.New("measurement must have a name and finite value")
		}
	}
	for _, e := range r.Evidence {
		if strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.Reference) == "" {
			return errors.New("evidence kind/reference are required")
		}
		if e.SHA256 != "" && !validSHA(e.SHA256) {
			return errors.New("invalid evidence sha256")
		}
		if e.Bytes < 0 {
			return errors.New("invalid evidence byte count")
		}
	}
	for _, f := range r.Configuration {
		if strings.TrimSpace(f.Name) == "" || !validSHA(f.SHA256) {
			return errors.New("configuration fingerprint requires name and sha256")
		}
	}
	return nil
}
func validSHA(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
func ReadFile(path string) (Run, string, int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Run{}, "", 0, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var r Run
	if err := dec.Decode(&r); err != nil {
		return Run{}, "", 0, fmt.Errorf("decode proof: %w", err)
	}
	if err := r.Validate(); err != nil {
		return Run{}, "", 0, err
	}
	sum := sha256.Sum256(b)
	return r, hex.EncodeToString(sum[:]), int64(len(b)), nil
}
