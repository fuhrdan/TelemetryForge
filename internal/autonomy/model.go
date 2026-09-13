// Package autonomy implements TelemetryForge's bounded, shadow-first autonomous
// control loop. It predicts near-term worker pressure and can apply only a
// temporary non-protected sampling multiplier. The controller cannot rewrite
// immutable lifecycle artifacts or bypass shaping protection rules.
package autonomy

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Mode string

const (
	ModeOff    Mode = "off"
	ModeShadow Mode = "shadow"
	ModeAuto   Mode = "auto"
)

type ActionState string

const (
	ActionShadow     ActionState = "shadow"
	ActionApplied    ActionState = "applied"
	ActionRolledBack ActionState = "rolled_back"
	ActionExpired    ActionState = "expired"
)

// Config bounds every automatic action. Auto mode is deliberately opt-in.
type Config struct {
	Mode                    Mode          `json:"mode"`
	Window                  int           `json:"window"`
	Horizon                 time.Duration `json:"horizon"`
	TickInterval            time.Duration `json:"tick_interval"`
	TriggerPressure         float64       `json:"trigger_pressure"`
	RecoverPressure         float64       `json:"recover_pressure"`
	MinMultiplier           float64       `json:"min_multiplier"`
	MaxActionTTL            time.Duration `json:"max_action_ttl"`
	Cooldown                time.Duration `json:"cooldown"`
	MinObservations         int           `json:"min_observations"`
	MaxErrorRateIncrease    float64       `json:"max_error_rate_increase"`
	MaxLatencyIncreaseRatio float64       `json:"max_latency_increase_ratio"`
}

func DefaultConfig() Config {
	return Config{
		Mode:                    ModeShadow,
		Window:                  12,
		Horizon:                 30 * time.Second,
		TickInterval:            5 * time.Second,
		TriggerPressure:         0.82,
		RecoverPressure:         0.55,
		MinMultiplier:           0.35,
		MaxActionTTL:            2 * time.Minute,
		Cooldown:                30 * time.Second,
		MinObservations:         4,
		MaxErrorRateIncrease:    0.02,
		MaxLatencyIncreaseRatio: 0.25,
	}
}

func (c Config) Validate() error {
	switch c.Mode {
	case ModeOff, ModeShadow, ModeAuto:
	default:
		return fmt.Errorf("unsupported autonomy mode %q", c.Mode)
	}
	if c.Window < 3 || c.Window > 256 {
		return errors.New("autonomy window must be between 3 and 256")
	}
	if c.MinObservations < 3 || c.MinObservations > c.Window {
		return errors.New("autonomy min observations must be between 3 and window")
	}
	if c.Horizon <= 0 || c.Horizon > 10*time.Minute {
		return errors.New("autonomy horizon must be between 0 and 10 minutes")
	}
	if c.TickInterval <= 0 || c.TickInterval > time.Minute {
		return errors.New("autonomy tick interval must be between 0 and 1 minute")
	}
	if c.TriggerPressure <= 0 || c.TriggerPressure > 1 {
		return errors.New("autonomy trigger pressure must be in (0,1]")
	}
	if c.RecoverPressure < 0 || c.RecoverPressure >= c.TriggerPressure {
		return errors.New("autonomy recover pressure must be below trigger pressure")
	}
	if c.MinMultiplier <= 0 || c.MinMultiplier > 1 {
		return errors.New("autonomy minimum multiplier must be in (0,1]")
	}
	if c.MaxActionTTL <= 0 || c.MaxActionTTL > 30*time.Minute {
		return errors.New("autonomy action TTL must be between 0 and 30 minutes")
	}
	if c.Cooldown < 0 || c.Cooldown > 30*time.Minute {
		return errors.New("autonomy cooldown must be between 0 and 30 minutes")
	}
	if c.MaxErrorRateIncrease < 0 || c.MaxErrorRateIncrease > 1 {
		return errors.New("autonomy max error-rate increase must be between 0 and 1")
	}
	if c.MaxLatencyIncreaseRatio < 0 || c.MaxLatencyIncreaseRatio > 10 {
		return errors.New("autonomy max latency increase ratio must be between 0 and 10")
	}
	return nil
}

func ParseMode(value string) Mode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "off", "disabled":
		return ModeOff
	case "auto", "automatic":
		return ModeAuto
	default:
		return ModeShadow
	}
}

type Observation struct {
	At            time.Time `json:"at"`
	QueuePressure float64   `json:"queue_pressure"`
	ErrorRate     float64   `json:"error_rate"`
	MeanLatencyMS float64   `json:"mean_latency_ms"`
	RetryRate     float64   `json:"retry_rate"`
	Completed     uint64    `json:"completed"`
}

type Forecast struct {
	GeneratedAt       time.Time     `json:"generated_at"`
	Horizon           time.Duration `json:"horizon"`
	Samples           int           `json:"samples"`
	CurrentPressure   float64       `json:"current_pressure"`
	PredictedPressure float64       `json:"predicted_pressure"`
	SlopePerSecond    float64       `json:"slope_per_second"`
	Confidence        string        `json:"confidence"`
}

type Action struct {
	ID                    string      `json:"id"`
	State                 ActionState `json:"state"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
	ExpiresAt             time.Time   `json:"expires_at"`
	Multiplier            float64     `json:"multiplier"`
	PredictedPressure     float64     `json:"predicted_pressure"`
	BaselineErrorRate     float64     `json:"baseline_error_rate"`
	BaselineMeanLatencyMS float64     `json:"baseline_mean_latency_ms"`
	Reason                string      `json:"reason"`
	RollbackReason        string      `json:"rollback_reason,omitempty"`
}

type Snapshot struct {
	Mode              Mode        `json:"mode"`
	CurrentMultiplier float64     `json:"current_multiplier"`
	Forecast          Forecast    `json:"forecast"`
	LastObservation   Observation `json:"last_observation"`
	ActiveAction      *Action     `json:"active_action,omitempty"`
	RecentActions     []Action    `json:"recent_actions,omitempty"`
	CooldownUntil     time.Time   `json:"cooldown_until,omitempty"`
}
