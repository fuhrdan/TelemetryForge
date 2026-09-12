package formal

import (
	"encoding/json"
	"testing"
)

func TestCheckAll(t *testing.T) {
	report := CheckAll()
	if err := report.Validate(); err != nil {
		t.Fatal(err)
	}
	if report.Status != "pass" {
		t.Fatalf("formal check failed: %+v", report.Results)
	}
	if len(report.Results) != 4 {
		t.Fatalf("got %d results, want 4", len(report.Results))
	}
	for _, result := range report.Results {
		if result.States < 2 || result.Transitions < 1 {
			t.Fatalf("model %s explored too little state: %+v", result.Model, result)
		}
	}
}

func TestMarshalReportRoundTrip(t *testing.T) {
	payload, err := MarshalReport(CheckAll())
	if err != nil {
		t.Fatal(err)
	}
	var got Report
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsStatusMismatch(t *testing.T) {
	report := CheckAll()
	report.Status = "fail"
	if err := report.Validate(); err == nil {
		t.Fatal("expected status mismatch to fail validation")
	}
}

func TestExplorerDetectsInvariantViolation(t *testing.T) {
	broken := model[durableState]{
		Name:    "broken-durable-ingest",
		Initial: durableState{ClientAck: true},
		Next:    func(durableState) []transition[durableState] { return nil },
		Invariants: []namedInvariant[durableState]{
			{Name: "ack-requires-evidence", Check: func(s durableState) bool {
				return !s.ClientAck || s.LocalDurable || s.Downstream
			}},
		},
	}
	result := explore(broken)
	if result.Status != "fail" || result.Error == "" {
		t.Fatalf("checker failed to discriminate broken model: %+v", result)
	}
}
