package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

type failingPolicyEngine struct{}

func (failingPolicyEngine) Evaluate(context.Context, domain.Event) (domain.Event, error) {
	return domain.Event{}, errors.New("policy evidence database unavailable")
}

func TestPolicyProcessorTreatsRuntimeEvidenceFailureAsTransient(t *testing.T) {
	processor := NewPolicyProcessor(failingPolicyEngine{})
	_, err := processor.Process(context.Background(), domain.Event{ID: "evt"})
	if err == nil {
		t.Fatal("expected policy failure")
	}
	if !reliability.IsTransient(err) {
		t.Fatalf("policy runtime failure should be transient: %v", err)
	}
}
