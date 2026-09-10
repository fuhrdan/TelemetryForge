// Command telemetryctl provides small operational tools for TelemetryForge.
//
// Commands are intentionally narrow and auditable: incident freezing, DLQ
// replay, incident analysis replay, cost simulation, deduplication maintenance,
// and policy validation.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	incidentarchive "github.com/fuhrdan/TelemetryForge/internal/incidentarchive"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
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
		case "export":
			if err := incidentExport(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "import":
			if err := incidentImport(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "inspect":
			if err := incidentInspect(os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "verify":
			if err := incidentVerify(os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "report":
			if err := incidentReport(os.Args[3:]); err != nil {
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
	case "cardinality":
		switch os.Args[2] {
		case "top":
			if err := cardinalityTop(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "budgets":
			if err := cardinalityBudgets(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		default:
			usage()
			os.Exit(2)
		}
	case "shaping":
		switch os.Args[2] {
		case "validate":
			if err := shapingValidate(os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "preview":
			if err := shapingPreview(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "prune":
			if err := shapingPrune(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		default:
			usage()
			os.Exit(2)
		}
	case "routing":
		switch os.Args[2] {
		case "validate":
			if err := routingValidate(os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "destinations":
			if err := routingDestinations(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "deliveries":
			if err := routingDeliveries(ctx, os.Args[3:]); err != nil {
				exitErr(err)
			}
		case "dlq":
			if len(os.Args) < 4 {
				usage()
				os.Exit(2)
			}
			switch os.Args[3] {
			case "list":
				if err := routingDLQList(ctx, os.Args[4:]); err != nil {
					exitErr(err)
				}
			case "requeue":
				if err := routingDLQRequeue(ctx, os.Args[4:]); err != nil {
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

func incidentExport(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("incident export", flag.ContinueOnError)
	incidentID := set.String("id", "", "frozen incident identifier")
	output := set.String("out", "", "output .tfincident or .tfincident.enc path")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	maxEvents := set.Int("max-events", 10000, "maximum frozen events to export")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	keyFile := set.String("encrypt-key-file", env("TELEMETRYFORGE_ARCHIVE_KEY_FILE", ""), "optional file containing a 64-hex-character AES-256 key")
	force := set.Bool("force", false, "overwrite an existing output file")
	activePolicy := set.String("active-policy", "policies/active.json", "active cardinality policy snapshot; use disabled to omit")
	shadowPolicy := set.String("shadow-policy", "policies/shadow.json", "shadow cardinality policy snapshot; use disabled to omit")
	activeShaping := set.String("active-shaping", "shaping/active.json", "active shaping config snapshot; use disabled to omit")
	shadowShaping := set.String("shadow-shaping", "shaping/shadow.json", "shadow shaping config snapshot; use disabled to omit")
	activeRouting := set.String("active-routing", "routing/active.json", "active routing config snapshot; use disabled to omit")
	shadowRouting := set.String("shadow-routing", "routing/shadow.json", "shadow routing config snapshot; use disabled to omit")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*incidentID) == "" {
		return errors.New("--id is required")
	}
	if strings.TrimSpace(*output) == "" {
		*output = strings.TrimSpace(*incidentID) + ".tfincident"
	}
	if *maxEvents < 1 || *maxEvents > incidentarchive.MaxEvents {
		return fmt.Errorf("--max-events must be between 1 and %d", incidentarchive.MaxEvents)
	}
	if !*force {
		if _, err := os.Stat(*output); err == nil {
			return fmt.Errorf("output file %q already exists; use --force to overwrite", *output)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	configurations, err := loadArchiveConfigurations(map[string]string{
		"policy:active":  *activePolicy,
		"policy:shadow":  *shadowPolicy,
		"shaping:active": *activeShaping,
		"shaping:shadow": *shadowShaping,
		"routing:active": *activeRouting,
		"routing:shadow": *shadowRouting,
	})
	if err != nil {
		return err
	}

	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	bundle, err := incidentarchive.BuildFromSource(
		ctx, store, strings.TrimSpace(*incidentID), *maxEvents, configurations,
	)
	if err != nil {
		return err
	}

	var key []byte
	if strings.TrimSpace(*keyFile) != "" {
		key, err = incidentarchive.LoadKeyFile(*keyFile)
		if err != nil {
			return err
		}
	}
	manifest, err := incidentarchive.WriteFile(*output, bundle, key)
	if err != nil {
		return err
	}

	fileHash, err := fileSHA256(*output)
	if err != nil {
		return err
	}
	fmt.Printf(
		"exported incident %s to %s (%d events, archive %s, sha256 %s, encrypted=%t)\n",
		manifest.IncidentID, *output, manifest.EventCount, manifest.ArchiveID,
		fileHash, len(key) != 0,
	)
	return nil
}

func incidentImport(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("incident import", flag.ContinueOnError)
	filename := set.String("file", "", "input .tfincident or .tfincident.enc path")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", ""), "target tenant; defaults to archive tenant")
	newID := set.String("new-id", "", "optional imported incident identifier")
	allowTenantRemap := set.Bool("allow-tenant-remap", false, "explicitly allow import into a different tenant")
	keyFile := set.String("decrypt-key-file", env("TELEMETRYFORGE_ARCHIVE_KEY_FILE", ""), "optional file containing a 64-hex-character AES-256 key")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*filename) == "" {
		return errors.New("--file is required")
	}

	raw, err := os.ReadFile(*filename)
	if err != nil {
		return err
	}
	encrypted := incidentarchive.IsEncrypted(raw)

	var key []byte
	if encrypted {
		if strings.TrimSpace(*keyFile) == "" {
			return errors.New("archive is encrypted; --decrypt-key-file or TELEMETRYFORGE_ARCHIVE_KEY_FILE is required")
		}
		key, err = incidentarchive.LoadKeyFile(*keyFile)
		if err != nil {
			return err
		}
	}

	bundle, err := incidentarchive.ReadFile(*filename, key)
	if err != nil {
		return err
	}

	sourceTenant := bundle.Manifest.TenantID
	targetTenant := strings.TrimSpace(*tenant)
	if targetTenant == "" {
		targetTenant = sourceTenant
	}
	if targetTenant != sourceTenant && !*allowTenantRemap {
		return fmt.Errorf(
			"archive belongs to tenant %q; importing into %q requires --allow-tenant-remap",
			sourceTenant, targetTenant,
		)
	}

	incident := bundle.Incident
	if strings.TrimSpace(*newID) != "" {
		incident.ID = strings.TrimSpace(*newID)
	}
	records := storage.NormalizeImportedEvents(bundle.Events, targetTenant)

	fileHash, err := fileSHA256(*filename)
	if err != nil {
		return err
	}
	provenance := incidentarchive.ImportProvenance{
		ArchiveID:             bundle.Manifest.ArchiveID,
		SourceTenantID:        sourceTenant,
		SourceIncidentID:      bundle.Manifest.IncidentID,
		ImportedIncidentID:    incident.ID,
		FormatVersion:         bundle.Manifest.FormatVersion,
		TelemetryForgeVersion: bundle.Manifest.TelemetryForge,
		FileSHA256:            fileHash,
		Encrypted:             encrypted,
	}

	ctx = security.WithTenant(ctx, targetTenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	inserted, err := store.ImportIncidentArchive(ctx, incident, records, provenance)
	if err != nil {
		return err
	}

	// Rebuild the graph for the imported tenant/incident rather than blindly
	// trusting embedded tenant/root IDs from another environment.
	events := make([]domain.Event, 0, len(records))
	for _, record := range records {
		events = append(events, record.Event)
	}
	// Replay/cost rows are intentionally not inserted into live operational
	// history during import, so the imported live Evidence Graph is rebuilt
	// from frozen events only. The original graph/replay/cost artifacts remain
	// intact inside the archive for offline inspection.
	graph := evidence.Build(targetTenant, incident.ID, events, nil, nil)
	if err := store.SaveEvidenceGraph(ctx, graph); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: incident import committed but Evidence Graph snapshot refresh failed: %v\n",
			err,
		)
	}

	fmt.Printf(
		"imported archive %s as incident %s into tenant %s (%d events, source tenant %s)\n",
		bundle.Manifest.ArchiveID, incident.ID, targetTenant, inserted, sourceTenant,
	)
	return nil
}

func incidentInspect(args []string) error {
	set := flag.NewFlagSet("incident inspect", flag.ContinueOnError)
	filename := set.String("file", "", "input .tfincident or .tfincident.enc path")
	keyFile := set.String("decrypt-key-file", env("TELEMETRYFORGE_ARCHIVE_KEY_FILE", ""), "optional AES-256 key file")
	if err := set.Parse(args); err != nil {
		return err
	}
	bundle, encrypted, fileHash, err := readArchiveForCLI(*filename, *keyFile)
	if err != nil {
		return err
	}

	summary := map[string]any{
		"archive_id":              bundle.Manifest.ArchiveID,
		"format_version":          bundle.Manifest.FormatVersion,
		"telemetryforge_version":  bundle.Manifest.TelemetryForge,
		"tenant_id":               bundle.Manifest.TenantID,
		"incident":                bundle.Incident,
		"event_count":             len(bundle.Events),
		"schema_count":            len(bundle.Schemas),
		"schema_drift_count":      len(bundle.SchemaDrifts),
		"replay_run_count":        len(bundle.ReplayRuns),
		"cost_simulation_count":   len(bundle.CostResults),
		"configuration_snapshots": bundle.Manifest.Configurations,
		"evidence_summary":        bundle.EvidenceGraph.Summary,
		"encrypted":               encrypted,
		"file_sha256":             fileHash,
	}
	payload, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func incidentVerify(args []string) error {
	set := flag.NewFlagSet("incident verify", flag.ContinueOnError)
	filename := set.String("file", "", "input .tfincident or .tfincident.enc path")
	keyFile := set.String("decrypt-key-file", env("TELEMETRYFORGE_ARCHIVE_KEY_FILE", ""), "optional AES-256 key file")
	if err := set.Parse(args); err != nil {
		return err
	}
	bundle, encrypted, fileHash, err := readArchiveForCLI(*filename, *keyFile)
	if err != nil {
		return err
	}
	fmt.Printf(
		"verified archive %s: incident=%s tenant=%s events=%d entries=%d encrypted=%t sha256=%s\n",
		bundle.Manifest.ArchiveID, bundle.Manifest.IncidentID,
		bundle.Manifest.TenantID, bundle.Manifest.EventCount,
		len(bundle.Manifest.Entries), encrypted, fileHash,
	)
	return nil
}

func incidentReport(args []string) error {
	set := flag.NewFlagSet("incident report", flag.ContinueOnError)
	filename := set.String("file", "", "input .tfincident or .tfincident.enc path")
	output := set.String("out", "", "standalone HTML report path")
	keyFile := set.String("decrypt-key-file", env("TELEMETRYFORGE_ARCHIVE_KEY_FILE", ""), "optional AES-256 key file")
	force := set.Bool("force", false, "overwrite an existing report")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("--out is required")
	}
	if !*force {
		if _, err := os.Stat(*output); err == nil {
			return fmt.Errorf("report file %q already exists; use --force to overwrite", *output)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	bundle, _, _, err := readArchiveForCLI(*filename, *keyFile)
	if err != nil {
		return err
	}
	if err := incidentarchive.WriteHTMLReport(*output, bundle); err != nil {
		return err
	}
	fmt.Printf("wrote standalone incident report %s\n", *output)
	return nil
}

func readArchiveForCLI(filename, keyFile string) (incidentarchive.Bundle, bool, string, error) {
	if strings.TrimSpace(filename) == "" {
		return incidentarchive.Bundle{}, false, "", errors.New("--file is required")
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		return incidentarchive.Bundle{}, false, "", err
	}
	encrypted := incidentarchive.IsEncrypted(raw)

	var key []byte
	if encrypted {
		if strings.TrimSpace(keyFile) == "" {
			return incidentarchive.Bundle{}, false, "", errors.New("archive is encrypted; a decryption key file is required")
		}
		key, err = incidentarchive.LoadKeyFile(keyFile)
		if err != nil {
			return incidentarchive.Bundle{}, false, "", err
		}
	}
	bundle, err := incidentarchive.ReadFile(filename, key)
	if err != nil {
		return incidentarchive.Bundle{}, false, "", err
	}
	fileHash, err := fileSHA256(filename)
	return bundle, encrypted, fileHash, err
}

func loadArchiveConfigurations(files map[string]string) ([]incidentarchive.ConfigurationSnapshot, error) {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]incidentarchive.ConfigurationSnapshot, 0, len(keys))
	for _, key := range keys {
		filename := strings.TrimSpace(files[key])
		if filename == "" || strings.EqualFold(filename, "disabled") {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid archive configuration key %q", key)
		}

		// Validate with the real configuration parser before preserving exact
		// bytes in the archive.
		switch parts[0] {
		case "policy":
			if _, err := policy.Load(filename); err != nil {
				return nil, err
			}
		case "shaping":
			if _, err := shaping.Load(filename); err != nil {
				return nil, err
			}
		case "routing":
			if _, err := router.Load(filename); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported configuration kind %q", parts[0])
		}

		snapshot, err := incidentarchive.LoadConfigurationSnapshot(parts[0], parts[1], filename)
		if err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	return result, nil
}

func fileSHA256(filename string) (string, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
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

func cardinalityTop(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("cardinality top", flag.ContinueOnError)
	mode := set.String("mode", "active", "active or shadow policy state")
	limit := set.Int("limit", 25, "maximum dimensions to return (1..500)")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *mode != "active" && *mode != "shadow" {
		return errors.New("--mode must be active or shadow")
	}
	if *limit < 1 || *limit > 500 {
		return errors.New("--limit must be between 1 and 500")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	states, err := store.ListDistributedCardinalityStates(ctx, *mode, *limit)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"tenant": *tenant,
		"mode":   *mode,
		"window": "1h",
		"states": states,
	}, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func cardinalityBudgets(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("cardinality budgets", flag.ContinueOnError)
	limit := set.Int("limit", 100, "maximum budget rows to return (1..500)")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *limit < 1 || *limit > 500 {
		return errors.New("--limit must be between 1 and 500")
	}
	ctx = security.WithTenant(ctx, *tenant)

	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	budgets, err := store.ListCardinalityBudgetStatus(ctx, *limit)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"tenant":  *tenant,
		"budgets": budgets,
	}, "", "  ")
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

func shapingValidate(args []string) error {
	set := flag.NewFlagSet("shaping validate", flag.ContinueOnError)
	file := set.String("file", "shaping/active.json", "shaping policy JSON")
	if err := set.Parse(args); err != nil {
		return err
	}
	config, err := shaping.Load(*file)
	if err != nil {
		return err
	}
	fmt.Printf("valid shaping config %s@%s (%d rules)\n", config.Name, config.Version, len(config.Rules))
	return nil
}

func shapingPreview(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("shaping preview", flag.ContinueOnError)
	incidentID := set.String("incident", "", "frozen incident identifier")
	file := set.String("file", "shaping/active.json", "shaping policy JSON")
	candidateFile := set.String("candidate", "", "optional candidate shaping policy JSON")
	policyFile := set.String("policy", "policies/active.json", "active Cardinality Firewall policy replayed before shaping")
	pressure := set.Float64("pressure", 0, "simulated worker queue pressure from 0 to 1")
	maxEvents := set.Int("max-events", 10000, "maximum frozen events to preview")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*incidentID) == "" {
		return errors.New("--incident is required")
	}
	if *pressure < 0 || *pressure > 1 {
		return errors.New("--pressure must be between 0 and 1")
	}
	config, err := shaping.Load(*file)
	if err != nil {
		return err
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
		return fmt.Errorf("incident %q not found or contains no events", *incidentID)
	}
	activePolicy, err := policy.Load(*policyFile)
	if err != nil {
		return err
	}
	policyEngine, err := policy.NewEngine(activePolicy, nil, nil, 20000)
	if err != nil {
		return err
	}
	postPolicy := make([]domain.Event, 0, len(events))
	for _, event := range events {
		processed, evalErr := policyEngine.EvaluateAt(ctx, event, event.Timestamp)
		if evalErr != nil {
			return evalErr
		}
		postPolicy = append(postPolicy, processed)
	}
	active, err := shaping.PreviewEvents(config, postPolicy, *pressure)
	if err != nil {
		return err
	}
	result := map[string]any{"incident_id": *incidentID, "active": active}
	if strings.TrimSpace(*candidateFile) != "" {
		candidate, loadErr := shaping.Load(*candidateFile)
		if loadErr != nil {
			return loadErr
		}
		preview, previewErr := shaping.PreviewEvents(candidate, postPolicy, *pressure)
		if previewErr != nil {
			return previewErr
		}
		result["candidate"] = preview
	}
	payload, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func shapingPrune(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("shaping prune", flag.ContinueOnError)
	olderThan := set.Duration("older-than", 35*24*time.Hour, "minimum shaping evidence age")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *olderThan < 35*24*time.Hour {
		return errors.New("--older-than must be at least 35 days so retry decisions outlive the normal dedup/replay investigation horizon")
	}
	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	cutoff := time.Now().UTC().Add(-*olderThan)
	result, err := store.PruneShapingBefore(ctx, cutoff)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"tenant": *tenant, "before": cutoff, "deleted": result,
	}, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func routingValidate(args []string) error {
	set := flag.NewFlagSet("routing validate", flag.ContinueOnError)
	file := set.String("file", "routing/active.json", "routing JSON file")
	if err := set.Parse(args); err != nil {
		return err
	}
	config, err := router.Load(*file)
	if err != nil {
		return err
	}
	fmt.Printf("routing config %s@%s is valid (%d destinations, %d rules, fallback=%q)\n",
		config.Name, config.Version, len(config.EnabledDestinations()), len(config.Rules), config.FallbackDestination)
	return nil
}

func routingDestinations(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("routing destinations", flag.ContinueOnError)
	limit := set.Int("limit", 100, "maximum destinations to return")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	rows, err := store.ListRoutingDestinationHealth(ctx, *limit)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{"tenant": *tenant, "destinations": rows}, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func routingDeliveries(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("routing deliveries", flag.ContinueOnError)
	status := set.String("status", "", "optional pending|sending|retry|delivered|dead_letter")
	limit := set.Int("limit", 100, "maximum rows to return")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	rows, err := store.ListRoutingDeliveries(ctx, *status, *limit)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{"tenant": *tenant, "status": *status, "deliveries": rows}, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func routingDLQList(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("routing dlq list", flag.ContinueOnError)
	destination := set.String("destination", "", "optional destination filter")
	limit := set.Int("limit", 100, "maximum rows to return")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	rows, err := store.ListRoutingDeadLetters(ctx, *destination, *limit)
	if err != nil {
		return err
	}
	payload, _ := json.MarshalIndent(map[string]any{"tenant": *tenant, "destination": *destination, "dead_letters": rows}, "", "  ")
	fmt.Println(string(payload))
	return nil
}

func routingDLQRequeue(ctx context.Context, args []string) error {
	set := flag.NewFlagSet("routing dlq requeue", flag.ContinueOnError)
	eventID := set.String("event", "", "event identifier")
	destination := set.String("destination", "", "destination name")
	tenant := set.String("tenant", env("TELEMETRYFORGE_TENANT_ID", "default"), "tenant identifier")
	databaseURL := set.String("database-url", env("TELEMETRYFORGE_DATABASE_URL", ""), "PostgreSQL URL")
	if err := set.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*eventID) == "" || strings.TrimSpace(*destination) == "" {
		return errors.New("--event and --destination are required")
	}
	ctx = security.WithTenant(ctx, *tenant)
	store, err := storage.NewPostgresStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.RequeueRoutingDeadLetter(ctx, *eventID, *destination); err != nil {
		return err
	}
	fmt.Printf("requeued routing dead letter event %s for destination %s in tenant %s\n", *eventID, *destination, *tenant)
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
		"policy %s@%s is valid (%d rules, %d budgets, default threshold %d, default action %s)\n",
		loaded.Name,
		loaded.Version,
		len(loaded.Rules),
		len(loaded.Budgets),
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
  telemetryctl incident export --id INC-42 [--out INC-42.tfincident] [--encrypt-key-file archive.key]
  telemetryctl incident import --file INC-42.tfincident [--tenant default] [--allow-tenant-remap] [--new-id INC-IMPORTED]
  telemetryctl incident inspect --file INC-42.tfincident [--decrypt-key-file archive.key]
  telemetryctl incident verify --file INC-42.tfincident [--decrypt-key-file archive.key]
  telemetryctl incident report --file INC-42.tfincident --out INC-42.html
  telemetryctl cost simulate --incident INC-42 [--tenant default] [--pricing pricing/vendor.json]
  telemetryctl dlq replay --file dead-letter.json [--topic telemetry.raw]
  telemetryctl dedup prune [--older-than 840h]
  telemetryctl policy validate --file policies/active.json
  telemetryctl cardinality top [--mode active] [--limit 25] [--tenant default]
  telemetryctl cardinality budgets [--limit 100] [--tenant default]
  telemetryctl shaping validate [--file shaping/active.json]
  telemetryctl shaping preview --incident INC-42 [--candidate shaping/shadow.json] [--pressure 0.9] [--tenant default]
  telemetryctl shaping prune [--older-than 840h] [--tenant default]
  telemetryctl schema inspect --source checkout-api --type request.duration [--tenant default]
  telemetryctl schema diff --source checkout-api --type request.duration --from 1.0 --to 2.0 [--tenant default]
  telemetryctl schema prune [--older-than 840h] [--tenant default]
  telemetryctl routing validate [--file routing/active.json]
  telemetryctl routing destinations [--tenant default]
  telemetryctl routing deliveries [--status retry] [--tenant default]
  telemetryctl routing dlq list [--destination primary] [--tenant default]
  telemetryctl routing dlq requeue --event EVT --destination primary [--tenant default]`)
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
