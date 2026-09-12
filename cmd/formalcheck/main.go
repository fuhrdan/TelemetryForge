// Command formalcheck runs TelemetryForge's dependency-free bounded protocol
// models. TLA+ remains the canonical readable specification; this command gives
// CI and operators an executable state-space gate without requiring a JVM.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fuhrdan/TelemetryForge/internal/formal"
)

func main() {
	output := flag.String("output", "", "optional path for JSON model-check report")
	quiet := flag.Bool("quiet", false, "suppress human-readable model summary")
	flag.Parse()

	report := formal.CheckAll()
	payload, err := formal.MarshalReport(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *output != "" {
		if err := os.WriteFile(*output, append(payload, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if !*quiet {
		for _, result := range report.Results {
			fmt.Printf("%-24s %s  states=%d transitions=%d invariants=%d\n", result.Model, result.Status, result.States, result.Transitions, len(result.Invariants))
		}
		fmt.Printf("overall: %s\n", report.Status)
	}
	if report.Status != "pass" {
		os.Exit(1)
	}
}
