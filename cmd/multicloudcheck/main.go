// Command multicloudcheck runs TelemetryForge's deterministic multi-cloud
// failure-domain/accounting model and emits machine-readable evidence.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/fuhrdan/TelemetryForge/internal/multicloudproof"
)

func main() {
	events := flag.Int("events", 10000, "offered events per fault scenario")
	output := flag.String("output", "", "optional JSON output path")
	flag.Parse()

	report, err := multicloudproof.Run(*events)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	data = append(data, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	fmt.Printf("multi-cloud proof: pass=%t scenarios=%d offered=%d accepted=%d rejected=%d duplicates=%d lost=%d corrupted=%d unaccounted=%d\n", report.Passed, len(report.Scenarios), report.TotalOffered, report.TotalAccepted, report.TotalRejected, report.TotalDuplicates, report.TotalLost, report.TotalCorrupted, report.TotalUnaccounted)
	for _, scenario := range report.Scenarios {
		fmt.Printf("  %-28s pass=%t accepted=%d rejected=%d delivered=%d duplicates=%d\n", scenario.Name, scenario.Passed, scenario.AfterRecovery.Accepted, scenario.AfterRecovery.Rejected, scenario.AfterRecovery.DeliveredUnique, scenario.AfterRecovery.DuplicateAttempts)
	}
	if !report.Passed {
		os.Exit(1)
	}
}
