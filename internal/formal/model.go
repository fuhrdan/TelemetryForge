// Package formal contains small, executable abstractions of TelemetryForge's
// critical durability protocols. The models deliberately omit implementation
// detail and keep only state that matters to the documented safety invariants.
package formal

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Result is one exhaustive bounded model-check result.
type Result struct {
	Model       string   `json:"model"`
	States      int      `json:"states"`
	Transitions int      `json:"transitions"`
	Invariants  []string `json:"invariants"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
}

// Report is emitted by the dependency-free model checker and embedded into
// operational proof evidence.
type Report struct {
	Format  string   `json:"format"`
	Version int      `json:"version"`
	Results []Result `json:"results"`
	Status  string   `json:"status"`
}

const (
	ReportFormat  = "telemetryforge-formal-check"
	ReportVersion = 1
)

type transition[S comparable] struct {
	Name string
	Next S
}

type model[S comparable] struct {
	Name       string
	Initial    S
	Next       func(S) []transition[S]
	Invariants []namedInvariant[S]
}

type namedInvariant[S comparable] struct {
	Name  string
	Check func(S) bool
}

func explore[S comparable](m model[S]) Result {
	result := Result{Model: m.Name, Status: "pass"}
	for _, invariant := range m.Invariants {
		result.Invariants = append(result.Invariants, invariant.Name)
	}
	sort.Strings(result.Invariants)

	seen := map[S]bool{m.Initial: true}
	queue := []S{m.Initial}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result.States++

		for _, invariant := range m.Invariants {
			if !invariant.Check(current) {
				result.Status = "fail"
				result.Error = fmt.Sprintf("invariant %q failed in state %+v", invariant.Name, current)
				return result
			}
		}

		for _, step := range m.Next(current) {
			_ = step.Name
			result.Transitions++
			if !seen[step.Next] {
				seen[step.Next] = true
				queue = append(queue, step.Next)
			}
		}
	}
	return result
}

// CheckAll exhaustively checks every bounded protocol model shipped with the
// release. These are safety proofs over the abstraction and are intentionally
// not presented as liveness proofs of an unbounded production deployment.
func CheckAll() Report {
	results := []Result{
		checkDurableIngest(),
		checkReplicatedDurability(),
		checkMeshFailover(),
		checkCryptographicLineage(),
	}
	report := Report{Format: ReportFormat, Version: ReportVersion, Results: results, Status: "pass"}
	for _, result := range results {
		if result.Status != "pass" {
			report.Status = "fail"
			break
		}
	}
	return report
}

func (report Report) Validate() error {
	if report.Format != ReportFormat || report.Version != ReportVersion {
		return fmt.Errorf("unsupported formal-check report")
	}
	if len(report.Results) == 0 {
		return fmt.Errorf("formal-check report has no results")
	}
	expected := "pass"
	for _, result := range report.Results {
		if result.Model == "" || result.States == 0 || len(result.Invariants) == 0 {
			return fmt.Errorf("invalid result for model %q", result.Model)
		}
		if result.Status != "pass" && result.Status != "fail" {
			return fmt.Errorf("invalid status for model %q", result.Model)
		}
		if result.Status == "fail" {
			expected = "fail"
		}
	}
	if report.Status != expected {
		return fmt.Errorf("report status %q does not match result status %q", report.Status, expected)
	}
	return nil
}

func MarshalReport(report Report) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(report, "", "  ")
}
