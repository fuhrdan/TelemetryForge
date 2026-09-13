package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/autonomy"
)

type assertion struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}
type report struct {
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	GeneratedAt time.Time         `json:"generated_at"`
	Assertions  []assertion       `json:"assertions"`
	Shadow      autonomy.Snapshot `json:"shadow"`
	AutoApplied autonomy.Snapshot `json:"auto_applied"`
	RolledBack  autonomy.Snapshot `json:"rolled_back"`
}

type memoryAudit struct{ actions []autonomy.Action }

func (a *memoryAudit) Record(v autonomy.Action) error { a.actions = append(a.actions, v); return nil }

func main() {
	output := flag.String("output", "", "optional report path")
	flag.Parse()
	start := time.Unix(1_700_000_000, 0).UTC()
	rising := []float64{.45, .58, .70, .81}

	shadowAudit := &memoryAudit{}
	shadowCfg := autonomy.DefaultConfig()
	shadowCfg.Mode = autonomy.ModeShadow
	shadowCfg.Window = 8
	shadowCfg.MinObservations = 4
	shadowCfg.Horizon = 20 * time.Second
	shadow, _ := autonomy.NewController(shadowCfg, shadowAudit)
	shadowSnap := feed(shadow, start, rising)

	autoAudit := &memoryAudit{}
	autoCfg := shadowCfg
	autoCfg.Mode = autonomy.ModeAuto
	autoCfg.RecoverPressure = .10
	auto, _ := autonomy.NewController(autoCfg, autoAudit)
	autoSnap := feed(auto, start, rising)
	auto.QueueDepth(95, 100)
	for i := 0; i < 10; i++ {
		outcome := "success"
		if i < 2 {
			outcome = "failed"
		}
		auto.JobCompleted(100*time.Millisecond, 1, outcome)
	}
	rollbackSnap := auto.Tick(start.Add(25 * time.Second))

	checks := []assertion{
		{"shadow-no-mutation", shadowSnap.ActiveAction != nil && shadowSnap.ActiveAction.State == autonomy.ActionShadow && shadowSnap.CurrentMultiplier == 1, "shadow mode predicts but does not change the active multiplier"},
		{"auto-bounded-action", autoSnap.ActiveAction != nil && autoSnap.ActiveAction.State == autonomy.ActionApplied && autoSnap.CurrentMultiplier < 1 && autoSnap.CurrentMultiplier >= autoCfg.MinMultiplier, "auto mode applies only the configured bounded temporary multiplier"},
		{"slo-rollback", rollbackSnap.ActiveAction == nil && rollbackSnap.CurrentMultiplier == 1 && len(rollbackSnap.RecentActions) > 0, "error-rate guardrail restores multiplier to 1.0"},
		{"audited-transitions", len(autoAudit.actions) >= 2, "apply and rollback state transitions were both written to the audit sink"},
	}
	status := "pass"
	for _, c := range checks {
		if !c.Passed {
			status = "fail"
		}
	}
	r := report{Format: "telemetryforge-autonomy-check", Version: 1, Status: status, GeneratedAt: time.Now().UTC(), Assertions: checks, Shadow: shadowSnap, AutoApplied: autoSnap, RolledBack: rollbackSnap}
	raw, _ := json.MarshalIndent(r, "", "  ")
	raw = append(raw, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, raw, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Stdout.Write(raw)
	if status != "pass" {
		os.Exit(1)
	}
}
func feed(c *autonomy.Controller, start time.Time, values []float64) autonomy.Snapshot {
	var s autonomy.Snapshot
	for i, v := range values {
		c.QueueDepth(int(v*1000), 1000)
		c.JobCompleted(100*time.Millisecond, 1, "success")
		s = c.Tick(start.Add(time.Duration(i) * 5 * time.Second))
	}
	return s
}
