package multicloudproof

import "testing"

func TestRunAllScenariosPreserveAccounting(t *testing.T) {
	report, err := Run(1000)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("report failed: %+v", report)
	}
	if report.TotalLost != 0 || report.TotalCorrupted != 0 || report.TotalUnaccounted != 0 {
		t.Fatalf("unexpected accounting loss: lost=%d corrupted=%d unaccounted=%d", report.TotalLost, report.TotalCorrupted, report.TotalUnaccounted)
	}
	if len(report.Scenarios) != 6 {
		t.Fatalf("got %d scenarios, want 6", len(report.Scenarios))
	}
}

func TestDiskPressureRejectsBeforeAcceptance(t *testing.T) {
	report, err := Run(100)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range report.Scenarios {
		if s.Name != "disk-pressure-backpressure" {
			continue
		}
		found = true
		if s.RejectedUnderLoad == 0 {
			t.Fatal("expected backpressure rejection")
		}
		if s.AfterRecovery.Accepted+s.AfterRecovery.Rejected != s.AfterRecovery.Offered {
			t.Fatalf("offered events are not fully classified: %+v", s.AfterRecovery)
		}
	}
	if !found {
		t.Fatal("disk-pressure scenario missing")
	}
}

func TestAmbiguousNetworkAttemptsAreCounted(t *testing.T) {
	report, err := Run(100)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range report.Scenarios {
		if s.Name == "packet-loss-latency" {
			if s.AfterRecovery.DuplicateAttempts == 0 {
				t.Fatal("expected duplicate attempts")
			}
			if s.AfterRecovery.DeliveredUnique != s.AfterRecovery.Accepted {
				t.Fatalf("duplicate retry changed logical delivery count: %+v", s.AfterRecovery)
			}
			return
		}
	}
	t.Fatal("packet-loss-latency scenario missing")
}

func TestNoQuorumMeansNoAcceptance(t *testing.T) {
	f, err := NewFabric(DefaultTopology(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	f.SetCloud("aws", true, false, false)
	f.SetCloud("gcp", true, false, false)
	f.SetCloud("azure", true, false, false)
	f.Offer(10)
	a := f.Accounting()
	if a.Accepted != 0 || a.Rejected != 10 {
		t.Fatalf("got accepted=%d rejected=%d", a.Accepted, a.Rejected)
	}
}

func TestGlobalPartitionRetainsAcceptedUntilRecovery(t *testing.T) {
	report, err := Run(100)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range report.Scenarios {
		if s.Name == "global-delivery-partition" {
			if s.DuringFault.Retained != s.DuringFault.Accepted {
				t.Fatalf("accepted telemetry not fully retained during partition: %+v", s.DuringFault)
			}
			if s.AfterRecovery.DeliveredUnique != s.AfterRecovery.Accepted || s.AfterRecovery.Retained != 0 {
				t.Fatalf("retained telemetry did not drain after recovery: %+v", s.AfterRecovery)
			}
			return
		}
	}
	t.Fatal("global-delivery-partition scenario missing")
}
