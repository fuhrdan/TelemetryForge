package autonomy

import (
	"errors"
	"math"
	"testing"
	"time"
)

type memoryAudit struct {
	actions []Action
	fail    bool
}

func (a *memoryAudit) Record(v Action) error {
	if a.fail {
		return errors.New("audit unavailable")
	}
	a.actions = append(a.actions, v)
	return nil
}

func testConfig(mode Mode) Config {
	c := DefaultConfig()
	c.Mode = mode
	c.MinObservations = 4
	c.Window = 8
	c.Horizon = 20 * time.Second
	c.MaxActionTTL = time.Minute
	c.Cooldown = 10 * time.Second
	return c
}
func feed(c *Controller, start time.Time, values []float64) Snapshot {
	var s Snapshot
	for i, v := range values {
		c.QueueDepth(int(v*1000), 1000)
		c.JobCompleted(100*time.Millisecond, 1, "success")
		s = c.Tick(start.Add(time.Duration(i) * 5 * time.Second))
	}
	return s
}
func TestShadowPredictsWithoutMutation(t *testing.T) {
	audit := &memoryAudit{}
	c, _ := NewController(testConfig(ModeShadow), audit)
	s := feed(c, time.Unix(0, 0), []float64{.45, .58, .70, .81})
	if s.ActiveAction == nil || s.ActiveAction.State != ActionShadow {
		t.Fatalf("expected shadow action: %+v", s)
	}
	if c.CurrentMultiplier() != 1 {
		t.Fatalf("shadow mode changed multiplier: %v", c.CurrentMultiplier())
	}
}
func TestAutoAppliesAndRollsBackOnRecovery(t *testing.T) {
	audit := &memoryAudit{}
	c, _ := NewController(testConfig(ModeAuto), audit)
	start := time.Unix(0, 0)
	s := feed(c, start, []float64{.45, .58, .70, .81})
	if s.ActiveAction == nil || s.ActiveAction.State != ActionApplied {
		t.Fatalf("expected applied action: %+v", s)
	}
	if !(c.CurrentMultiplier() < 1 && c.CurrentMultiplier() >= testConfig(ModeAuto).MinMultiplier) {
		t.Fatalf("unexpected multiplier %v", c.CurrentMultiplier())
	}
	c.QueueDepth(20, 100)
	c.JobCompleted(100*time.Millisecond, 1, "success")
	s = c.Tick(start.Add(25 * time.Second))
	if s.ActiveAction != nil || math.Abs(c.CurrentMultiplier()-1) > 1e-9 {
		t.Fatalf("expected rollback: %+v", s)
	}
	if len(s.RecentActions) != 1 || s.RecentActions[0].RollbackReason == "" {
		t.Fatalf("missing rollback evidence: %+v", s.RecentActions)
	}
}
func TestAutoRollsBackOnErrorRegression(t *testing.T) {
	audit := &memoryAudit{}
	cfg := testConfig(ModeAuto)
	cfg.RecoverPressure = .10
	c, _ := NewController(cfg, audit)
	start := time.Unix(0, 0)
	feed(c, start, []float64{.45, .58, .70, .81})
	c.QueueDepth(95, 100)
	for i := 0; i < 10; i++ {
		outcome := "success"
		if i < 2 {
			outcome = "failed"
		}
		c.JobCompleted(100*time.Millisecond, 1, outcome)
	}
	s := c.Tick(start.Add(25 * time.Second))
	if s.ActiveAction != nil || c.CurrentMultiplier() != 1 {
		t.Fatalf("guardrail did not roll back: %+v", s)
	}
}
func TestAuditFailureSuppressesAutoMutation(t *testing.T) {
	audit := &memoryAudit{fail: true}
	c, _ := NewController(testConfig(ModeAuto), audit)
	s := feed(c, time.Unix(0, 0), []float64{.45, .58, .70, .81})
	if s.ActiveAction == nil || s.ActiveAction.State != ActionShadow {
		t.Fatalf("audit failure must suppress auto mutation: %+v", s)
	}
	if c.CurrentMultiplier() != 1 {
		t.Fatal("unaudited action changed multiplier")
	}
}
func TestForecastIncreasingPressure(t *testing.T) {
	start := time.Unix(0, 0)
	history := []Observation{{At: start, QueuePressure: .2}, {At: start.Add(5 * time.Second), QueuePressure: .3}, {At: start.Add(10 * time.Second), QueuePressure: .4}, {At: start.Add(15 * time.Second), QueuePressure: .5}}
	f := ForecastPressure(history, 10*time.Second, start.Add(15*time.Second))
	if f.PredictedPressure <= .5 || f.SlopePerSecond <= 0 {
		t.Fatalf("unexpected forecast %+v", f)
	}
}
func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinMultiplier = 0
	if cfg.Validate() == nil {
		t.Fatal("expected invalid multiplier")
	}
}
