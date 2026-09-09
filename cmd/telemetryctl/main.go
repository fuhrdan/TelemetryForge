// Command telemetryctl provides small operational tools for TelemetryForge.
//
// Commands are intentionally narrow and auditable: incident freezing, DLQ
// replay, incident analysis replay, cost simulation, deduplication maintenance,
// and policy validation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "incident":
		switch os.Args[2] {
		case "freeze":
			if err := incidentFreeze(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "replay":
			if err := incidentReplay(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "graph":
			if err := incidentGraph(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		default:
			usage()
			os.Exit(2)
		}
	case "dlq":
		if os.Args[2] != "replay" {
			usage()
			os.Exit(2)
		}
		if err := dlqReplay(ctx, os.Args[3:]); err != nil {
			exitErr(err)
		}
	case "dedup":
		if os.Args[2] != "prune" {
			usage()
			os.Exit(2)
		}
		if err := dedupPrune(ctx, os.Args[3:]); err != nil {
			exitErr(err)
		}
	case "cost":
		if os.Args[2] != "simulate" {
			usage()
			os.Exit(2)
		}
		if err := costSimulate(ctx, os.Args[3:]); err != nil {
			exitErr(err)
		}
	case "policy":
		if os.Args[2] != "validate" {
			usage()
			os.Exit(2)
		}
		if err := policyValidate(os.Args[3:]); err != nil {
			exitErr(err)
		}
	case "schema":
		switch os.Args[2] {
		case "inspect":
			if err := schemaInspect(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "diff":
			if err := schemaDiff(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "prune":
			if err := schemaPrune(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		default:
			usage()
			os.Exit(2)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func incidentFreeze(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("incident freeze", flag.ContinueOnError)
	id := set.String("id", "", "incident identifier")
	title := set.String("title", "", "human-readable incident title")
	fromRaw := set.String("from", "", "RFC3339 capture-window start")
	toRaw := set.String("to", "", "RFC3339 capture-window end")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)

	from, err := time.Parse(time.RFC3339, *fromRaw)
	if err != nil {
		return errors.New("--from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, *toRaw)
	if err != nil {
		return errors.New("--to must be RFC3339")
	}

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	count, err := store.FreezeIncident(ctx, *id, *title, from, to)
	if err != nil {
		return err
	}
	fmt.Printf("froze %d flight-recorder events into incident %s\n", count, *id)
	return nil
}

func dlqReplay(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("dlq replay", flag.ContinueOnError)
	file := set.String("file", "", "dead-letter JSON file")
	topic := set.String("topic", "", "optional destination topic override")
	brokersRaw := set.String("brokers", env("TELEMETRYFORGE_KAFKA_BROKERS", "localhost:9092"), "comma-separated Kafka brokers")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return errors.New("--file is required")
	}

	payload, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read dead-letter file: %w", err)
	}

	var dead domain.DeadLetter
	if err := json.Unmarshal(payload, &dead); err != nil {
		return fmt.Errorf("decode dead-letter file: %w", err)
	}
	if dead.Event == nil {
		return errors.New("dead letter contains raw malformed payload and cannot be safely replayed as an event")
	}

	destination := dead.OriginalTopic
	if strings.TrimSpace(*topic) != "" {
		destination = strings.TrimSpace(*topic)
	}
	if destination == "" {
		return errors.New("replay destination topic is empty")
	}

	publisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{
		Brokers:  splitCSV(*brokersRaw),
		ClientID: "telemetryctl-replay",
		Security: stream.KafkaSecurityFromEnv(),
	}, logging.New())
	if err != nil {
		return err
	}
	defer publisher.Close()

	if err := publisher.Publish(ctx, destination, *dead.Event); err != nil {
		return err
	}
	fmt.Printf("replayed event %s to %s\n", dead.Event.ID, destination)
	return nil
}

func dedupPrune(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("dedup prune", flag.ContinueOnError)
	olderThan := set.Duration("older-than", 35*24*time.Hour, "minimum age to remove")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	if *olderThan < 30*24*time.Hour {
		return errors.New("--older-than must be at least 30 days")
	}

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	count, err := store.PruneDedupBefore(ctx, time.Now().UTC().Add(-*olderThan))
	if err != nil {
		return err
	}
	fmt.Printf("pruned %d deduplication rows older than %s\n", count, olderThan.String())
	return nil
}

func incidentReplay(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("incident replay", flag.ContinueOnError)
	incidentID := set.String("id", "", "frozen incident identifier")
	activeFile := set.String("policy", "policies/active.json", "active policy JSON")
	shadowFile := set.String("shadow-policy", "policies/shadow.json", "optional shadow policy JSON; use disabled to turn off")
	publishTopic := set.String("publish-topic", "", "optional isolated topic; must begin telemetry.replay")
	maxEvents := set.Int("max-events", 10000, "maximum frozen events to replay")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	brokersRaw := set.String("brokers", env("TELEMETRYFORGE_KAFKA_BROKERS", "localhost:9092"), "Kafka brokers used only with --publish-topic")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	if strings.TrimSpace(*incidentID) == "" {
		return errors.New("--id is required")
	}

	active, err := policy.Load(*activeFile)
	if err != nil {
		return err
	}
	var shadow *policy.Policy
	if !strings.EqualFold(strings.TrimSpace(*shadowFile), "disabled") && strings.TrimSpace(*shadowFile) != "" {
		loaded, err := policy.Load(*shadowFile)
		if err != nil {
			return err
		}
		shadow = &loaded
	}

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	var publisher *stream.KafkaPublisher
	if strings.TrimSpace(*publishTopic) != "" {
		replayTopic := strings.TrimSpace(*publishTopic)
		if replayTopic != "telemetry.replay" && !strings.HasPrefix(replayTopic, "telemetry.replay.") {
			return errors.New("--publish-topic must be telemetry.replay or begin telemetry.replay.; production ingest topics are intentionally rejected")
		}
		publisher, err = stream.NewKafkaPublisher(stream.KafkaConfig{
			Brokers: splitCSV(*brokersRaw), ClientID: "telemetryctl-incident-replay",
			Security: stream.KafkaSecurityFromEnv(),
		}, logging.New())
		if err != nil {
			return err
		}
		defer publisher.Close()
	}

	runner := replay.NewRunner(store)
	run, err := runner.Run(ctx, replay.Options{
		IncidentID: *incidentID, ActivePolicy: active, ShadowPolicy: shadow,
		MaxEvents: *maxEvents, MaxDimensions: 20000,
		PublishTopic: strings.TrimSpace(*publishTopic), Publisher: publisher,
	})
	if err != nil {
		return err
	}

	payload, _ := json.MarshalIndent(run, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func costSimulate(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("cost simulate", flag.ContinueOnError)
	incidentID := set.String("incident", "", "frozen incident identifier")
	activeFile := set.String("policy", "policies/active.json", "active policy JSON")
	shadowFile := set.String("shadow-policy", "policies/shadow.json", "optional candidate policy JSON; use disabled to turn off")
	pricingFile := set.String("pricing", "", "optional pricing JSON; omit for volume/series-only simulation")
	maxEvents := set.Int("max-events", 10000, "maximum frozen events to simulate")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	if strings.TrimSpace(*incidentID) == "" {
		return errors.New("--incident is required")
	}

	active, err := policy.Load(*activeFile)
	if err != nil {
		return err
	}
	var shadow *policy.Policy
	if !strings.EqualFold(strings.TrimSpace(*shadowFile), "disabled") && strings.TrimSpace(*shadowFile) != "" {
		loaded, err := policy.Load(*shadowFile)
		if err != nil {
			return err
		}
		shadow = &loaded
	}

	var pricing *costsim.Pricing
	if strings.TrimSpace(*pricingFile) != "" {
		loaded, err := costsim.LoadPricing(*pricingFile)
		if err != nil {
			return err
		}
		pricing = &loaded
	}

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	result, err := costsim.New(store).Simulate(ctx, *incidentID, active, shadow, pricing, *maxEvents)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func incidentGraph(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("incident graph", flag.ContinueOnError)
	incidentID := set.String("id", "", "frozen incident identifier")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	maxEvents := set.Int("max-events", 1000, "maximum frozen events to include")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*incidentID) == "" {
		return errors.New("--id is required")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	events, err := store.IncidentEvents(ctx, *incidentID, *maxEvents)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("incident %q not found or contains no events for tenant %q", *incidentID, *tenant)
	}

	runs, err := store.ListReplayRuns(ctx, 200)
	if err != nil {
		return err
	}
	simulations, err := store.ListCostSimulations(ctx, 200)
	if err != nil {
		return err
	}

	graph := evidence.Build(*tenant, *incidentID, events, runs, simulations)
	if err := store.SaveEvidenceGraph(ctx, graph); err != nil {
		return err
	}

	payload, _ := json.MarshalIndent(graph, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func schemaInspect(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("schema inspect", flag.ContinueOnError)
	source := set.String("source", "", "telemetry source")
	eventType := set.String("type", "", "telemetry event type")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*eventType) == "" {
		return errors.New("--source and --type are required")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	history, err := store.SchemaHistory(ctx, *source, *eventType)
	if err != nil {
		return err
	}
	if len(history) == 0 {
		return fmt.Errorf("no schema history for %s / %s in tenant %s", *source, *eventType, *tenant)
	}
	payload, _ := json.MarshalIndent(history, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func schemaDiff(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("schema diff", flag.ContinueOnError)
	source := set.String("source", "", "telemetry source")
	eventType := set.String("type", "", "telemetry event type")
	fromVersion := set.String("from", "", "older declared schema version")
	toVersion := set.String("to", "", "newer declared schema version")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*eventType) == "" ||
		strings.TrimSpace(*fromVersion) == "" || strings.TrimSpace(*toVersion) == "" {
		return errors.New("--source, --type, --from, and --to are required")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	diff, err := store.SchemaDiff(ctx, *source, *eventType, *fromVersion, *toVersion)
	if err != nil {
		return err
	}
	printSchemaDiff(diff)
	return nil
}

func printSchemaDiff(diff schema.VersionDiff) {
	payload, _ := json.MarshalIndent(diff, "", "  ")
	fmt.Println(string(payload))
}

func schemaPrune(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("schema prune", flag.ContinueOnError)
	olderThan := set.Duration("older-than", 35*24*time.Hour, "minimum age of schema observation ledger rows to remove")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *olderThan < 30*24*time.Hour {
		return errors.New("--older-than must be at least 30 days to preserve the normal telemetry/replay investigation horizon")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	cutoff := time.Now().UTC().Add(-*olderThan)
	deleted, err := store.PruneSchemaObservations(ctx, cutoff)
	if err != nil {
		return err
	}
	fmt.Printf("pruned %d schema observation ledger rows for tenant %s before %s\n", deleted, *tenant, cutoff.Format(time.RFC3339))
	return nil
}

func policyValidate(args []string) error {
	set := flag.NewFlagSet("policy validate", flag.ContinueOnError)
	file := set.String("file", "", "policy JSON file")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return errors.New("--file is required")
	}

	loaded, err := policy.Load(*file)
	if err != nil {
		return err
	}

	fmt.Printf(
		"policy %s@%s is valid (%d rules, default threshold %d, default action %s)\n",
		loaded.Name,
		loaded.Version,
		len(loaded.Rules),
		loaded.DefaultUniqueThreshold,
		loaded.DefaultAction,
	)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  telemetryctl incident freeze --id INC-42 --title "Checkout latency" --from <RFC3339> --to <RFC3339>
  telemetryctl incident replay --id INC-42 [--tenant default] [--policy policies/active.json] [--shadow-policy policies/shadow.json]
  telemetryctl incident graph --id INC-42 [--tenant default]
  telemetryctl cost simulate --incident INC-42 [--tenant default] [--pricing pricing/vendor.json]
  telemetryctl dlq replay --file dead-letter.json [--topic telemetry.raw]
  telemetryctl dedup prune [--older-than 840h]
  telemetryctl policy validate --file policies/active.json
  telemetryctl schema inspect --source checkout-api --type request.duration [--tenant default]
  telemetryctl schema diff --source checkout-api --type request.duration --from 1.0 --to 2.0 [--tenant default]
  telemetryctl schema prune [--older-than 840h] [--tenant default]`)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
