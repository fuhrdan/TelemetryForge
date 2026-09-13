package autonomy

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Controller observes worker pressure and SLO signals. Its only automatic
// mutation is a temporary non-protected shaping multiplier exposed through
// CurrentMultiplier. Immutable lifecycle configuration is never rewritten.
type Controller struct {
	mu         sync.Mutex
	cfg        Config
	audit      Auditor
	multiplier atomic.Uint64

	queuePressure float64
	completed     uint64
	failed        uint64
	retries       uint64
	totalLatency  time.Duration
	lastCompleted uint64
	lastFailed    uint64
	lastRetries   uint64
	lastLatency   time.Duration

	history       []Observation
	forecast      Forecast
	active        *Action
	recent        []Action
	cooldownUntil time.Time
	sequence      uint64
}

func NewController(cfg Config, audit Auditor) (*Controller, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if audit == nil {
		audit = NopAuditor{}
	}
	c := &Controller{cfg: cfg, audit: audit, history: make([]Observation, 0, cfg.Window), recent: make([]Action, 0, 16)}
	c.multiplier.Store(math.Float64bits(1))
	return c, nil
}

func (c *Controller) CurrentMultiplier() float64 { return math.Float64frombits(c.multiplier.Load()) }
func (c *Controller) QueueDepth(current, capacity int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if capacity <= 0 {
		c.queuePressure = 0
		return
	}
	c.queuePressure = clamp(float64(current)/float64(capacity), 0, 1)
}
func (c *Controller) JobCompleted(duration time.Duration, attempts int, outcome string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completed++
	c.totalLatency += duration
	switch outcome {
	case "failed", "ack_failed", "dlq":
		c.failed++
	}
	if attempts > 1 {
		c.retries += uint64(attempts - 1)
	}
}
func (c *Controller) Retry(string) { /* retry counts are derived from attempts */ }

// Tick snapshots the current window, forecasts future pressure, and performs a
// shadow/applied/rollback transition. It is deterministic for the supplied time.
func (c *Controller) Tick(now time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now = now.UTC()
	completed := c.completed - c.lastCompleted
	failed := c.failed - c.lastFailed
	retries := c.retries - c.lastRetries
	latency := c.totalLatency - c.lastLatency
	c.lastCompleted, c.lastFailed, c.lastRetries, c.lastLatency = c.completed, c.failed, c.retries, c.totalLatency

	obs := Observation{At: now, QueuePressure: c.queuePressure, Completed: completed}
	if completed > 0 {
		obs.ErrorRate = float64(failed) / float64(completed)
		obs.RetryRate = float64(retries) / float64(completed)
		obs.MeanLatencyMS = float64(latency.Microseconds()) / 1000 / float64(completed)
	}
	c.history = append(c.history, obs)
	if len(c.history) > c.cfg.Window {
		c.history = append([]Observation(nil), c.history[len(c.history)-c.cfg.Window:]...)
	}
	c.forecast = ForecastPressure(c.history, c.cfg.Horizon, now)

	if c.active != nil {
		c.evaluateActiveLocked(obs, now)
	}
	if c.active == nil && c.cfg.Mode != ModeOff && !now.Before(c.cooldownUntil) && len(c.history) >= c.cfg.MinObservations && c.forecast.PredictedPressure >= c.cfg.TriggerPressure {
		c.startActionLocked(obs, now)
	}
	return c.snapshotLocked(obs)
}

func (c *Controller) startActionLocked(obs Observation, now time.Time) {
	c.sequence++
	multiplier := recommendedMultiplier(c.forecast.PredictedPressure, c.cfg.MinMultiplier)
	state := ActionShadow
	if c.cfg.Mode == ModeAuto {
		state = ActionApplied
	}
	action := &Action{
		ID: fmt.Sprintf("autonomy-%d-%04d", now.UnixMilli(), c.sequence), State: state,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(c.cfg.MaxActionTTL), Multiplier: multiplier,
		PredictedPressure: c.forecast.PredictedPressure, BaselineErrorRate: obs.ErrorRate,
		BaselineMeanLatencyMS: obs.MeanLatencyMS,
		Reason:                "forecast pressure crossed configured trigger; reduce only non-protected telemetry",
	}
	if err := c.audit.Record(*action); err != nil {
		// Fail safe: if an automatic action cannot be audited, do not apply it.
		action.State = ActionShadow
		action.Reason = "automatic action suppressed because audit persistence failed: " + err.Error()
	}
	if action.State == ActionApplied {
		c.multiplier.Store(math.Float64bits(multiplier))
	}
	c.active = action
}

func (c *Controller) evaluateActiveLocked(obs Observation, now time.Time) {
	action := c.active
	rollback := ""
	terminal := ActionRolledBack
	switch {
	case !now.Before(action.ExpiresAt):
		rollback = "action TTL expired"
		terminal = ActionExpired
	case obs.QueuePressure <= c.cfg.RecoverPressure:
		rollback = "queue pressure recovered below rollback threshold"
	case obs.ErrorRate > action.BaselineErrorRate+c.cfg.MaxErrorRateIncrease:
		rollback = "error rate exceeded action guardrail"
	case latencyRegression(action.BaselineMeanLatencyMS, obs.MeanLatencyMS, c.cfg.MaxLatencyIncreaseRatio):
		rollback = "mean processing latency exceeded action guardrail"
	}
	if rollback == "" {
		return
	}
	action.State = terminal
	action.UpdatedAt = now
	action.RollbackReason = rollback
	if err := c.audit.Record(*action); err != nil {
		action.RollbackReason += "; audit persistence also failed: " + err.Error()
	}
	c.multiplier.Store(math.Float64bits(1))
	c.recent = append(c.recent, *action)
	if len(c.recent) > 16 {
		c.recent = append([]Action(nil), c.recent[len(c.recent)-16:]...)
	}
	c.active = nil
	c.cooldownUntil = now.Add(c.cfg.Cooldown)
}

func latencyRegression(baseline, current, ratio float64) bool {
	if baseline <= 0 || current <= 0 {
		return false
	}
	return current > baseline*(1+ratio)
}

func recommendedMultiplier(predicted, minimum float64) float64 {
	// Three explicit bands keep the action explainable and bounded.
	candidate := 0.75
	if predicted >= 0.95 {
		candidate = 0.50
	} else if predicted >= 0.90 {
		candidate = 0.60
	}
	if candidate < minimum {
		candidate = minimum
	}
	if candidate > 1 {
		candidate = 1
	}
	return candidate
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	var last Observation
	if len(c.history) > 0 {
		last = c.history[len(c.history)-1]
	}
	return c.snapshotLocked(last)
}
func (c *Controller) snapshotLocked(last Observation) Snapshot {
	result := Snapshot{Mode: c.cfg.Mode, CurrentMultiplier: c.CurrentMultiplier(), Forecast: c.forecast, LastObservation: last, CooldownUntil: c.cooldownUntil}
	if c.active != nil {
		copy := *c.active
		result.ActiveAction = &copy
	}
	result.RecentActions = append([]Action(nil), c.recent...)
	return result
}
