package policy

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
	"time"
)

const (
	registerCount        = 64
	defaultMaxDimensions = 20000
)

// Finding is one cardinality observation worth surfacing operationally.
type Finding struct {
	PolicyName       string    `json:"policy_name"`
	PolicyVersion    string    `json:"policy_version"`
	Mode             string    `json:"mode"`
	Source           string    `json:"source"`
	EventType        string    `json:"event_type"`
	Dimension        string    `json:"dimension"`
	ObservedUnique   uint64    `json:"observed_unique"`
	ProjectedUnique  uint64    `json:"projected_unique"`
	Action           Action    `json:"action"`
	Reason           string    `json:"reason"`
	ValueFingerprint string    `json:"value_fingerprint"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
}

// dimensionKey identifies one source/event/tag stream without retaining a raw
// dimension value.
type dimensionKey struct {
	source    string
	eventType string
	dimension string
}

type cardinalityState struct {
	estimator *hyperLogLog
	firstSeen time.Time
	lastSeen  time.Time
	samples   uint64
}

// Tracker maintains bounded approximate-cardinality state.
//
// HyperLogLog uses fixed memory per tracked dimension. MaxDimensions places a
// second bound on the number of distinct source/type/tag combinations an
// untrusted telemetry stream can force into memory.
type Tracker struct {
	mu            sync.Mutex
	states        map[dimensionKey]*cardinalityState
	maxDimensions int
}

// NewTracker creates a bounded cardinality tracker.
func NewTracker(maxDimensions int) *Tracker {
	if maxDimensions < 1 {
		maxDimensions = defaultMaxDimensions
	}
	return &Tracker{
		states:        make(map[dimensionKey]*cardinalityState),
		maxDimensions: maxDimensions,
	}
}

// Observe adds one value and returns the approximate unique count, projected
// one-hour growth, first/last timestamps, and a truncated SHA-256 fingerprint.
//
// The projection is deliberately simple and explainable: observed uniques are
// scaled by elapsed time up to a one-hour horizon. It is a warning heuristic,
// not a billing forecast.
func (tracker *Tracker) Observe(
	source, eventType, dimension, value string,
	now time.Time,
) (observed, projected uint64, firstSeen, lastSeen time.Time, fingerprint string) {
	key := dimensionKey{source: source, eventType: eventType, dimension: dimension}
	hash := hashValue(value)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	state := tracker.states[key]
	if state == nil {
		if len(tracker.states) >= tracker.maxDimensions {
			tracker.evictOldestLocked()
		}
		state = &cardinalityState{
			estimator: newHyperLogLog(),
			firstSeen: now,
		}
		tracker.states[key] = state
	}

	state.estimator.AddHash(hash)
	state.lastSeen = now
	state.samples++

	observed = state.estimator.Count()
	elapsed := now.Sub(state.firstSeen)
	projected = observed

	// Avoid projecting a one-hour growth rate from the first few milliseconds
	// of a process start or traffic burst. Identifier-shaped dimensions have a
	// separate early-warning rule in Engine.evaluate.
	if elapsed >= 30*time.Second && elapsed < time.Hour && state.samples >= 10 {
		projectedFloat := float64(observed) * float64(time.Hour) / float64(elapsed)
		if projectedFloat > float64(observed) {
			projected = uint64(math.Ceil(projectedFloat))
		}
	}

	return observed, projected, state.firstSeen, state.lastSeen, shortFingerprint(hash)
}

func (tracker *Tracker) evictOldestLocked() {
	var oldestKey dimensionKey
	var oldest time.Time
	found := false

	for key, state := range tracker.states {
		if !found || state.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = state.lastSeen
			found = true
		}
	}
	if found {
		delete(tracker.states, oldestKey)
	}
}

func hashValue(value string) uint64 {
	digest := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(digest[:8])
}

func shortFingerprint(hash uint64) string {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], hash)
	const hex = "0123456789abcdef"
	out := make([]byte, 12)
	for index := 0; index < 6; index++ {
		out[index*2] = hex[raw[index]>>4]
		out[index*2+1] = hex[raw[index]&0x0f]
	}
	return string(out)
}

// hyperLogLog is a compact 64-register estimator.
//
// It intentionally uses a very small implementation because TelemetryForge only
// needs an operational warning signal in v0.8.0, not billing-grade cardinality.
type hyperLogLog struct {
	registers [registerCount]uint8
}

func newHyperLogLog() *hyperLogLog {
	return &hyperLogLog{}
}

func (estimator *hyperLogLog) AddHash(hash uint64) {
	index := hash & (registerCount - 1)
	remaining := hash >> 6
	rank := uint8(1)
	for rank < 59 && remaining&1 == 0 {
		rank++
		remaining >>= 1
	}
	if rank > estimator.registers[index] {
		estimator.registers[index] = rank
	}
}

func (estimator *hyperLogLog) Count() uint64 {
	const alpha = 0.709 // correction for m=64
	sum := 0.0
	zeros := 0

	for _, register := range estimator.registers {
		sum += math.Pow(2, -float64(register))
		if register == 0 {
			zeros++
		}
	}

	raw := alpha * registerCount * registerCount / sum

	// Linear counting is much more accurate for the small-cardinality range
	// where operational thresholds are often first crossed.
	if raw <= 2.5*registerCount && zeros > 0 {
		raw = registerCount * math.Log(registerCount/float64(zeros))
	}

	if raw < 1 {
		return 1
	}
	return uint64(math.Round(raw))
}
