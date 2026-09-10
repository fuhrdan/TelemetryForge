package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func TestChangeIntelligenceBeforeAfterAndRollback(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	tenant := "change-intel-" + unique
	tenantCtx := security.WithTenant(ctx, tenant)
	source := "checkout-" + unique
	peer := "payments-" + now.Format("150405")
	changeID := "DEP-" + now.Format("150405")

	writeMetric := func(id, src string, at time.Time, value float64, isError bool) {
		eventType := "request.duration"
		tags := map[string]string{}
		var metric *float64
		unit := ""
		if isError {
			eventType = "request.error"
			tags["severity"] = "error"
		} else {
			metric = &value
			unit = "ms"
		}
		event := domain.Event{ID: id, TenantID: tenant, Source: src, Type: eventType, Timestamp: at, SchemaVersion: "1.0", Tags: tags, Value: metric, Unit: unit}
		if err := store.WriteEvent(tenantCtx, event); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 12; i++ {
		writeMetric(fmt.Sprintf("b-%s-%02d", source, i), source, now.Add(time.Duration(-12+i)*time.Minute), 180, false)
		writeMetric(fmt.Sprintf("bp-%s-%02d", peer, i), peer, now.Add(time.Duration(-12+i)*time.Minute), 140, false)
	}
	marker := changeintel.Marker{TenantID: tenant, ChangeID: changeID, EventID: "event-" + changeID, Source: source, Kind: changeintel.KindDeployment, Status: "completed", Version: "2.0", PreviousVersion: "1.9", GitSHA: "abc123", ChangedAt: now}
	if err := store.RecordChangeMarker(tenantCtx, marker); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 12; i++ {
		at := now.Add(time.Duration(i+1) * time.Minute)
		writeMetric(fmt.Sprintf("a-%s-%02d", source, i), source, at, 1500, i%3 == 0)
		writeMetric(fmt.Sprintf("ap-%s-%02d", peer, i), peer, at, 900, i%4 == 0)
	}
	analysis, err := store.AnalyzeChange(tenantCtx, changeID, 15*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Assessment != "regression-associated" {
		t.Fatalf("assessment=%s", analysis.Assessment)
	}
	if analysis.RegressedSources < 1 || analysis.BlastRadiusPercent <= 0 {
		t.Fatalf("analysis=%#v", analysis)
	}

	rollbackAt := now.Add(16 * time.Minute)
	rollback := changeintel.Marker{TenantID: tenant, ChangeID: "RB-" + unique, EventID: "event-rb-" + unique, Source: source, Kind: changeintel.KindRollback, Status: "completed", Version: "1.9", PreviousVersion: "2.0", RollbackOf: changeID, ChangedAt: rollbackAt}
	if err := store.RecordChangeMarker(tenantCtx, rollback); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		writeMetric(fmt.Sprintf("r-%s-%02d", source, i), source, rollbackAt.Add(time.Duration(i+1)*time.Minute), 180, false)
	}
	analysis, err = store.AnalyzeChange(tenantCtx, changeID, 15*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Recovery.Rollback == nil || analysis.Recovery.Rollback.ChangeID != rollback.ChangeID {
		t.Fatalf("recovery=%#v", analysis.Recovery)
	}
}
