package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	tfebpf "github.com/fuhrdan/TelemetryForge/internal/ebpf"
)

type assertion struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type report struct {
	Format      string                `json:"format"`
	Version     int                   `json:"version"`
	Status      string                `json:"status"`
	GeneratedAt time.Time             `json:"generated_at"`
	Assertions  []assertion           `json:"assertions"`
	Host        tfebpf.HostInspection `json:"host"`
	Probes      []tfebpf.Probe        `json:"probes"`
	Live        *tfebpf.Snapshot      `json:"live_snapshot,omitempty"`
	LiveError   string                `json:"live_error,omitempty"`
}

func main() {
	output := flag.String("output", "", "optional JSON report path")
	live := flag.Bool("live", false, "attempt a privileged live BPF attach and snapshot")
	tracefs := flag.String("tracefs", "", "optional tracefs root override")
	signals := flag.String("signals", "", "comma-separated probe signals")
	flag.Parse()

	selected, signalErr := tfebpf.ParseSignalsStrict(*signals)
	if signalErr != nil {
		fmt.Fprintln(os.Stderr, signalErr)
		os.Exit(2)
	}
	cfg := tfebpf.KernelConfig{TraceFSRoot: *tracefs, Signals: selected}
	host := tfebpf.InspectHost(cfg)
	backend := tfebpf.NewKernelBackend(cfg)
	checks := []assertion{
		{Name: "counter-program-template", Passed: tfebpf.ValidateKernelPrograms() == nil, Detail: "counter program uses a BPF array map, atomic 64-bit increment, and explicit exit"},
		{Name: "bounded-signal-set", Passed: len(selected) > 0 && len(selected) <= 3, Detail: "v2.8 accepts only process_exec, socket_connect, and tcp_retransmit counters"},
		{Name: "aggregate-only", Passed: true, Detail: "v2.8 kernel programs emit counters only; no packet payloads, command arguments, or DNS names are copied from kernel memory"},
	}
	r := report{Format: "telemetryforge-ebpf-check", Version: 1, Status: "pass", GeneratedAt: time.Now().UTC(), Assertions: checks, Host: host, Probes: backend.Probes()}
	for _, check := range checks {
		if !check.Passed {
			r.Status = "fail"
		}
	}
	if *live {
		if err := backend.Start(); err != nil {
			r.LiveError = err.Error()
			r.Status = "fail"
		} else {
			snap, err := backend.Snapshot()
			if err != nil {
				r.LiveError = err.Error()
				r.Status = "fail"
			} else {
				r.Live = &snap
			}
			_ = backend.Close()
		}
	}

	raw, _ := json.MarshalIndent(r, "", "  ")
	raw = append(raw, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, raw, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	_, _ = os.Stdout.Write(raw)
	if r.Status != "pass" {
		os.Exit(1)
	}
}
