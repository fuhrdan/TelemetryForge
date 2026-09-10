package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
)

type shapingMemoryRecorder struct {
	decisions []shaping.Decision
	diffs     []shaping.ShadowDiff
	err       error
}

func (r *shapingMemoryRecorder) ResolveShapingDecision(_ context.Context, d shaping.Decision) (shaping.Decision, bool, error) {
	if r.err != nil {
		return shaping.Decision{}, false, r.err
	}
	for _, existing := range r.decisions {
		if existing.EventID == d.EventID && existing.ConfigName == d.ConfigName && existing.ConfigVersion == d.ConfigVersion {
			return existing, true, nil
		}
	}
	r.decisions = append(r.decisions, d)
	return d, false, nil
}
func (r *shapingMemoryRecorder) RecordShapingShadowDiff(_ context.Context, d shaping.ShadowDiff) error {
	r.diffs = append(r.diffs, d)
	return nil
}

type fixedPressure float64

func (p fixedPressure) Current() float64 { return float64(p) }

type countProcessor struct{ count int }

func (p *countProcessor) Process(_ context.Context, event domain.Event) (domain.Event, error) {
	p.count++
	return event, nil
}

func samplingConfig(rate float64) shaping.Config {
	return shaping.Config{Name: "test", Version: "1", DefaultSampleRate: rate, Protection: shaping.Protection{Errors: true}, Pressure: shaping.PressureConfig{}}
}

func TestAdaptiveShaperStopsDownstreamAfterAuditedSampleOut(t *testing.T) {
	engine, _ := shaping.NewEngine(samplingConfig(0), nil)
	recorder := &shapingMemoryRecorder{}
	shaper, _ := NewAdaptiveShaper(engine, recorder, fixedPressure(0), slog.New(slog.NewTextHandler(io.Discard, nil)))
	downstream := &countProcessor{}
	chain, _ := NewChain(shaper, downstream)
	event := domain.Event{ID: "evt", TenantID: "t", Source: "api", Type: "request", Timestamp: time.Now(), SchemaVersion: "1"}
	processed, err := chain.Process(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if downstream.count != 0 {
		t.Fatalf("downstream count=%d", downstream.count)
	}
	if processed.ID != "evt" || len(recorder.decisions) != 1 {
		t.Fatalf("processed=%#v decisions=%d", processed, len(recorder.decisions))
	}
}
func TestAdaptiveShaperAuditFailureFailsOpen(t *testing.T) {
	engine, _ := shaping.NewEngine(samplingConfig(0), nil)
	recorder := &shapingMemoryRecorder{err: errors.New("db down")}
	shaper, _ := NewAdaptiveShaper(engine, recorder, fixedPressure(0), nil)
	downstream := &countProcessor{}
	chain, _ := NewChain(shaper, downstream)
	event := domain.Event{ID: "evt", TenantID: "t", Source: "api", Type: "request", Timestamp: time.Now(), SchemaVersion: "1"}
	_, err := chain.Process(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if downstream.count != 1 {
		t.Fatalf("downstream count=%d want 1", downstream.count)
	}
}
func TestCompositeObserverUpdatesPressure(t *testing.T) {
	controller := shaping.NewPressureController()
	observer := NewCompositeObserver(controller)
	observer.QueueDepth(75, 100)
	if controller.Current() < .749 || controller.Current() > .751 {
		t.Fatalf("pressure=%v", controller.Current())
	}
}

type changingPressure struct {
	values []float64
	index  int
}

func (pressure *changingPressure) Current() float64 {
	if len(pressure.values) == 0 {
		return 0
	}
	index := pressure.index
	if index >= len(pressure.values) {
		index = len(pressure.values) - 1
	}
	pressure.index++
	return pressure.values[index]
}

func TestAdaptiveShaperReusesFirstPressureDecisionOnRetry(t *testing.T) {
	config := samplingConfig(1)
	config.Pressure = shaping.PressureConfig{
		Enabled: true, HighWatermark: .5, CriticalWatermark: .9,
		HighFactor: .4, CriticalFactor: .1,
	}
	config.Rules = []shaping.Rule{{
		Name: "request", Type: "request", SampleRate: shapingRate(1), MinSampleRate: 0,
	}}
	engine, err := shaping.NewEngine(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &shapingMemoryRecorder{}
	pressure := &changingPressure{values: []float64{0, 1}}
	shaper, err := NewAdaptiveShaper(engine, recorder, pressure, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	event := domain.Event{TenantID: "t", Source: "api", Type: "request", Timestamp: time.Now(), SchemaVersion: "1"}
	for index := 0; index < 1000; index++ {
		event.ID = fmt.Sprintf("stable-retry-%d", index)
		_, highPressureDecision, _ := engine.Evaluate(event, 1, time.Now())
		if !highPressureDecision.Keep {
			break
		}
	}
	if event.ID == "" {
		t.Fatal("could not find deterministic event excluded under critical pressure")
	}

	first, firstErr := shaper.Process(context.Background(), event)
	if firstErr != nil || first.ID == "" {
		t.Fatalf("first=%#v err=%v", first, firstErr)
	}
	second, secondErr := shaper.Process(context.Background(), event)
	if secondErr != nil || second.ID == "" {
		t.Fatalf("second=%#v err=%v", second, secondErr)
	}
	if len(recorder.decisions) != 1 {
		t.Fatalf("decisions=%d, want exactly one durable decision", len(recorder.decisions))
	}
	if recorder.decisions[0].QueuePressure != 0 || !recorder.decisions[0].Keep {
		t.Fatalf("stored decision=%#v", recorder.decisions[0])
	}
}

func shapingRate(value float64) *float64 { return &value }
