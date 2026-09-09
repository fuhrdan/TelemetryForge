package policy

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
	"time"
)

const (
	registerCount        = 64
	exactHashWindow      = 16
	defaultMaxDimensions = 20000
)

// Finding is one cardinality observation worth surfacing operationally.
type Finding struct {
	TenantID         string    `json:"tenant_id,omitempty"`
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

// Observation is the cluster/local cardinality result returned to policy.
// Values are hashes/counts only; raw dimension values are never retained here.
type Observation struct {
	ObservedUnique  uint64    `json:"observed_unique"`
	ProjectedUnique uint64    `json:"projected_unique"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	WindowStart     time.Time `json:"window_start"`
	Samples         uint64    `json:"samples"`
	Fingerprint     string    `json:"value_fingerprint"`
}

// CardinalityTracker is the policy-facing cardinality state contract.
// Production v1.2 workers use a shared PostgreSQL/TimescaleDB implementation;
// replay/cost simulation intentionally use the local bounded implementation so
// historical analysis cannot mutate live production cardinality state.
type CardinalityTracker interface {
	ObserveCardinality(
		ctx context.Context,
		tenant, source, eventType, dimension, value string,
		now time.Time,
	) (Observation, error)
}

// DistributedState is one API/dashboard view of shared cardinality state.
type DistributedState struct {
	TenantID        string    `json:"tenant_id,omitempty"`
	Mode            string    `json:"mode"`
	Source          string    `json:"source"`
	EventType       string    `json:"event_type"`
	Dimension       string    `json:"dimension"`
	WindowStart     time.Time `json:"window_start"`
	ObservedUnique  uint64    `json:"observed_unique"`
	ProjectedUnique uint64    `json:"projected_unique"`
	GrowthPerMinute float64   `json:"growth_per_minute"`
	Samples         uint64    `json:"samples"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

// dimensionKey identifies one source/event/tag stream without retaining a raw
// dimension value.
type dimensionKey struct {
	tenant      string
	source      string
	eventType   string
	dimension   string
	windowStart time.Time
}

type cardinalityState struct {
	estimator *hyperLogLog
	firstSeen time.Time
	lastSeen  time.Time
	samples   uint64

	// exactHashes provides deterministic low-cardinality behavior before HLL
	// estimation is necessary. It stores only fixed-width SHA-256-derived
	// hashes, never raw dimension values, and cannot grow beyond 16 entries.
	exactHashes   [exactHashWindow]uint64
	exactCount    uint8
	exactOverflow bool
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

// ObserveCardinality implements CardinalityTracker using bounded in-memory
// state. It is retained for replay/cost simulation and single-process tests.
func (tracker *Tracker) ObserveCardinality(
	_ context.Context,
	tenant, source, eventType, dimension, value string,
	now time.Time,
) (Observation, error) {
	observed, projected, firstSeen, lastSeen, fingerprint := tracker.Observe(
		tenant, source, eventType, dimension, value, now,
	)
	return Observation{
		ObservedUnique: observed, ProjectedUnique: projected,
		FirstSeen: firstSeen, LastSeen: lastSeen, WindowStart: now.UTC().Truncate(time.Hour),
		Fingerprint: fingerprint,
	}, nil
}

// Observe adds one value and returns the approximate unique count, projected
// one-hour growth, first/last timestamps, and a truncated SHA-256 fingerprint.
//
// The projection is deliberately simple and explainable: observed uniques are
// scaled by elapsed time up to a one-hour horizon. It is a warning heuristic,
// not a billing forecast.
func (tracker *Tracker) Observe(
	tenant, source, eventType, dimension, value string,
	now time.Time,
) (observed, projected uint64, firstSeen, lastSeen time.Time, fingerprint string) {
	windowStart := now.UTC().Truncate(time.Hour)
	key := dimensionKey{
		tenant: tenant, source: source, eventType: eventType, dimension: dimension,
		windowStart: windowStart,
	}
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
	state.observeExact(hash)
	state.lastSeen = now
	state.samples++

	if state.exactOverflow {
		observed = state.estimator.Count()
	} else {
		observed = uint64(state.exactCount)
	}
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

func (state *cardinalityState) observeExact(hash uint64) {
	if state.exactOverflow {
		return
	}
	for index := uint8(0); index < state.exactCount; index++ {
		if state.exactHashes[index] == hash {
			return
		}
	}
	if int(state.exactCount) >= exactHashWindow {
		state.exactOverflow = true
		return
	}
	state.exactHashes[state.exactCount] = hash
	state.exactCount++
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

// HashCardinalityValue returns the fixed-width SHA-256-derived hash used by
// both local and distributed trackers.
func HashCardinalityValue(value string) uint64 { return hashValue(value) }

// HLLRegister returns the 64-register index/rank for one hashed value.
func HLLRegister(hash uint64) (int, uint8) {
	index := int(hash & (registerCount - 1))
	remaining := hash >> 6
	rank := uint8(1)
	for rank < 59 && remaining&1 == 0 {
		rank++
		remaining >>= 1
	}
	return index, rank
}

// EstimateState evaluates a persisted shared HLL/exact state.
func EstimateState(registers []byte, exactCount int, exactOverflow bool) uint64 {
	if !exactOverflow {
		return uint64(exactCount)
	}
	if len(registers) != registerCount {
		return 0
	}
	estimator := newHyperLogLog()
	copy(estimator.registers[:], registers)
	return estimator.Count()
}

// ProjectOneHour estimates unique growth through the end of a one-hour window.
// It requires at least ten samples and thirty seconds before projecting.
func ProjectOneHour(observed uint64, samples uint64, firstSeen, now time.Time) uint64 {
	projected := observed
	elapsed := now.Sub(firstSeen)
	if elapsed >= 30*time.Second && elapsed < time.Hour && samples >= 10 {
		value := float64(observed) * float64(time.Hour) / float64(elapsed)
		if value > float64(observed) {
			projected = uint64(math.Ceil(value))
		}
	}
	return projected
}

func hashValue(value string) uint64 {
	digest := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(digest[:8])
}

// FingerprintHash returns the short operational fingerprint for a hashed value.
func FingerprintHash(hash uint64) string { return shortFingerprint(hash) }

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
// It intentionally uses a very small implementation because TelemetryForge needs
// an operational policy signal, not billing-grade exact cardinality.
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
