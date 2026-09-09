package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

type fakeRoutingPlanner struct {
	err   error
	calls int
}

func (planner *fakeRoutingPlanner) Process(context.Context, domain.Event) error {
	planner.calls++
	return planner.err
}

func TestRouterPlannerPassesEventUnchanged(t *testing.T) {
	planner := &fakeRoutingPlanner{}
	processor, err := NewRouterPlanner(planner)
	if err != nil {
		t.Fatal(err)
	}
	event := domain.Event{ID: "evt", TenantID: "t", Source: "s", Type: "x"}
	got, err := processor.Process(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != event.ID || planner.calls != 1 {
		t.Fatalf("got=%#v calls=%d", got, planner.calls)
	}
}

func TestRouterPlannerClassifiesOutboxFailureTransient(t *testing.T) {
	processor, _ := NewRouterPlanner(&fakeRoutingPlanner{err: errors.New("db down")})
	_, err := processor.Process(context.Background(), domain.Event{ID: "evt"})
	if !reliability.IsTransient(err) {
		t.Fatalf("expected transient routing error: %v", err)
	}
}
