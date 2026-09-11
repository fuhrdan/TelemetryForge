package proof

import (
	"math"
	"testing"
	"time"
)

func sample() Run {
	n := time.Now().UTC()
	return Run{Format: Format, FormatVersion: Version, RunID: "run-1", GitCommit: "deadbeef", Scenario: "kafka-outage", Status: "pass", StartedAt: n, CompletedAt: n.Add(time.Second), Assertions: []Assertion{{Name: "recovered", Passed: true}}, Evidence: []Evidence{{Kind: "log", Reference: "proof.log"}}, Configuration: []Fingerprint{{Name: "compose", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}
}
func TestValid(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestPassRequiresPassedAssertions(t *testing.T) {
	r := sample()
	r.Assertions[0].Passed = false
	if r.Validate() == nil {
		t.Fatal("expected failure")
	}
}
func TestNonFiniteMeasurementRejected(t *testing.T) {
	r := sample()
	r.Measurements = []Measurement{{Name: "bad", Value: math.Inf(1)}}
	if r.Validate() == nil {
		t.Fatal("expected failure")
	}
}
