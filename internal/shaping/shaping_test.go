package shaping

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func rate(value float64) *float64 { return &value }

func testConfig() Config {
	return Config{Name: "test", Version: "1", DefaultSampleRate: 1, Protection: Protection{Errors: true, Severities: []string{"critical"}, LatencyThresholdMS: 1000, Types: []string{"audit.*"}, Tags: map[string]string{"incident_id": "*"}}, Pressure: PressureConfig{Enabled: true, HighWatermark: .7, CriticalWatermark: .9, HighFactor: .5, CriticalFactor: .2}, Rules: []Rule{{Name: "healthy", Type: "request.duration", SampleRate: rate(.5), MinSampleRate: .1, DropTags: []string{"debug_id"}, RenameTags: map[string]string{"http.method": "http.request.method"}, PayloadMaxBytes: 16, PayloadAction: PayloadDrop}}}
}

func event(id string) domain.Event {
	v := 100.0
	return domain.Event{ID: id, TenantID: "a", Source: "api", Type: "request.duration", Timestamp: time.Now().UTC(), SchemaVersion: "1", Value: &v, Unit: "ms", Tags: map[string]string{"debug_id": "x", "http.method": "GET"}, Payload: json.RawMessage(`{"long":"abcdefghijklmnopqrstuvwxyz"}`)}
}

func TestProtectedErrorAlwaysKept(t *testing.T) {
	config := testConfig()
	zero := 0.0
	config.Rules[0].SampleRate = &zero
	config.Rules[0].MinSampleRate = 0
	engine, err := NewEngine(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := event("err")
	input.Type = "request.error"
	input.Tags["severity"] = "error"
	_, decision, _ := engine.Evaluate(input, 1, time.Now())
	if !decision.Keep || !decision.Protected {
		t.Fatalf("decision=%#v", decision)
	}
}
func TestHighLatencyAlwaysKept(t *testing.T) {
	engine, _ := NewEngine(testConfig(), nil)
	input := event("slow")
	value := 1500.0
	input.Value = &value
	output, decision, _ := engine.Evaluate(input, 1, time.Now())
	if !decision.Keep || !decision.Protected {
		t.Fatalf("decision=%#v", decision)
	}
	if output.Tags["debug_id"] != "x" || len(output.Payload) == 0 {
		t.Fatalf("protected telemetry was destructively shaped: %#v", output)
	}
}
func TestPressureReducesRateButHonorsFloor(t *testing.T) {
	engine, _ := NewEngine(testConfig(), nil)
	_, decision, _ := engine.Evaluate(event("a"), .95, time.Now())
	if decision.EffectiveRate != .1 {
		t.Fatalf("rate=%v want .1", decision.EffectiveRate)
	}
}
func TestTransformsAreDeterministicAndPayloadRemainsValid(t *testing.T) {
	engine, _ := NewEngine(testConfig(), nil)
	output, decision, _ := engine.Evaluate(event("shape"), 0, time.Now())
	if _, ok := output.Tags["debug_id"]; ok {
		t.Fatal("debug_id should be dropped")
	}
	if output.Tags["http.request.method"] != "GET" {
		t.Fatalf("tags=%v", output.Tags)
	}
	if !decision.PayloadDropped || len(output.Payload) != 0 {
		t.Fatalf("payload=%s decision=%#v", output.Payload, decision)
	}
	_, again, _ := engine.Evaluate(event("shape"), 0, time.Now())
	if decision.Keep != again.Keep {
		t.Fatal("same event/config must sample deterministically")
	}
}
func TestShadowNeverChangesActiveOutput(t *testing.T) {
	active := testConfig()
	active.DefaultSampleRate = 1
	active.Rules = nil
	shadow := active
	shadow.Name = "shadow"
	zero := 0.0
	shadow.Rules = []Rule{{Name: "drop", Type: "*", SampleRate: &zero}}
	engine, _ := NewEngine(active, &shadow)
	output, decision, diff := engine.Evaluate(event("shadow-event"), 0, time.Now())
	if !decision.Keep || output.ID == "" {
		t.Fatal("active path changed")
	}
	if diff == nil || diff.ShadowKeep {
		t.Fatalf("diff=%#v", diff)
	}
}
func TestPreviewReportsRetentionWithoutOpaqueScore(t *testing.T) {
	config := testConfig()
	config.DefaultSampleRate = 0
	config.Rules = nil
	events := []domain.Event{event("a"), event("b")}
	preview, err := PreviewEvents(config, events, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Observed != 2 || preview.Kept != 0 || preview.EventRetentionPercent != 0 {
		t.Fatalf("preview=%#v", preview)
	}
}
func TestValidateRejectsBadRateAndRenameCollision(t *testing.T) {
	config := testConfig()
	bad := 1.2
	config.Rules[0].SampleRate = &bad
	if config.Validate() == nil {
		t.Fatal("expected invalid rate")
	}
	config = testConfig()
	config.Rules[0].RenameTags = map[string]string{"a": "x", "b": "x"}
	if config.Validate() == nil {
		t.Fatal("expected rename target collision")
	}
}

func TestValidateBoundsRuleCountAndNames(t *testing.T) {
	config := testConfig()
	config.Rules = make([]Rule, maxRules+1)
	for index := range config.Rules {
		config.Rules[index].Name = fmt.Sprintf("rule-%d", index)
	}
	if config.Validate() == nil {
		t.Fatal("expected too many shaping rules to fail validation")
	}

	config = testConfig()
	config.Rules[0].Name = strings.Repeat("x", maxRuleNameLength+1)
	if config.Validate() == nil {
		t.Fatal("expected oversized shaping rule name to fail validation")
	}
}

func TestProtectedTelemetryCanBeExplicitlyShaped(t *testing.T) {
	config := testConfig()
	config.Rules[0].ShapeProtected = true
	engine, err := NewEngine(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := event("protected-explicit")
	value := 1500.0
	input.Value = &value
	output, decision, _ := engine.Evaluate(input, 0, time.Now())
	if !decision.Protected || output.Tags["debug_id"] != "" || !decision.PayloadDropped {
		t.Fatalf("explicit protected shaping was not applied: output=%#v decision=%#v", output, decision)
	}
}
