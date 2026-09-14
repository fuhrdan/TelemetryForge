// Command fabriccheck validates the TelemetryForge v3 Global Edge Fabric
// contract against representative readiness/failure snapshots.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/fabric"
)

type assertion struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type scenario struct {
	Name       string          `json:"name"`
	Snapshot   fabric.Snapshot `json:"snapshot"`
	Assertions []assertion     `json:"assertions"`
	Passed     bool            `json:"passed"`
}

type report struct {
	Format      string     `json:"format"`
	Version     int        `json:"version"`
	Release     string     `json:"release"`
	GeneratedAt time.Time  `json:"generated_at"`
	Scenarios   []scenario `json:"scenarios"`
	Passed      bool       `json:"passed"`
}

func main() {
	output := flag.String("output", "", "optional JSON output path")
	flag.Parse()

	base := fabric.Input{
		WALReady: true, NodeID: "edge-aws-us-west-2a", WALBytes: 16 << 20, WALMaxBytes: 4 << 30, PendingRecords: 12,
		LineageKeyID: "tf-edge-demo", SealedSegments: 8,
		DurabilityMode: "cross-cloud", DurabilityQuorum: 2, ReplicationConfiguredPeers: 3, ReplicationReady: true,
		MeshPolicy: "global", MeshConfiguredPeers: 3, MeshHealthyPeers: 3, DeliveryReady: true,
		FastPathBatchSize: 64,
	}

	scenarios := []scenario{
		evaluate("healthy-cross-cloud", base, func(s fabric.Snapshot) []assertion {
			return []assertion{
				check("ready", s.State == fabric.StateReady, "healthy fabric must report ready"),
				check("acceptance-ready", s.AcceptanceReady, "durable acceptance is available"),
				check("delivery-ready", s.DeliveryReady, "a healthy mesh delivery path is available"),
				check("audit-ready", s.AuditReady, "cryptographic lineage identity is available"),
			}
		}),
		evaluate("downstream-partition", mutate(base, func(i *fabric.Input) { i.DeliveryReady = false; i.MeshHealthyPeers = 0 }), func(s fabric.Snapshot) []assertion {
			return []assertion{
				check("degraded-not-down", s.State == fabric.StateDegraded, "delivery loss degrades the fabric rather than erasing durable acceptance"),
				check("acceptance-preserved", s.AcceptanceReady, "WAL + quorum can still accept telemetry"),
				check("delivery-unavailable", !s.DeliveryReady, "downstream path is correctly reported unavailable"),
			}
		}),
		evaluate("quorum-loss", mutate(base, func(i *fabric.Input) { i.ReplicationReady = false }), func(s fabric.Snapshot) []assertion {
			return []assertion{
				check("not-ready", s.State == fabric.StateNotReady, "loss of required durability quorum fails closed"),
				check("acceptance-closed", !s.AcceptanceReady, "successful acceptance cannot be advertised without quorum"),
			}
		}),
		evaluate("wal-capacity", mutate(base, func(i *fabric.Input) { i.WALBytes = i.WALMaxBytes }), func(s fabric.Snapshot) []assertion {
			return []assertion{
				check("not-ready", s.State == fabric.StateNotReady, "capacity exhaustion fails closed"),
				check("acceptance-closed", !s.AcceptanceReady, "full WAL cannot advertise durable acceptance"),
			}
		}),
		evaluate("required-ebpf-unavailable", mutate(base, func(i *fabric.Input) { i.EBPFEnabled = true; i.EBPFRequired = true; i.EBPFActive = false }), func(s fabric.Snapshot) []assertion {
			return []assertion{
				check("not-ready", s.State == fabric.StateNotReady, "required collection dependency fails closed"),
				check("acceptance-closed", !s.AcceptanceReady, "required eBPF contract cannot be reported healthy while inactive"),
			}
		}),
	}

	passed := true
	for index := range scenarios {
		if !scenarios[index].Passed {
			passed = false
		}
	}
	r := report{Format: "telemetryforge-fabric-contract-check", Version: 1, Release: fabric.ReleaseVersion, GeneratedAt: time.Now().UTC(), Scenarios: scenarios, Passed: passed}
	payload, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	payload = append(payload, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	fmt.Printf("global edge fabric contract: pass=%t scenarios=%d release=%s\n", r.Passed, len(r.Scenarios), r.Release)
	for _, item := range r.Scenarios {
		fmt.Printf("  %-28s pass=%t state=%s acceptance=%t delivery=%t\n", item.Name, item.Passed, item.Snapshot.State, item.Snapshot.AcceptanceReady, item.Snapshot.DeliveryReady)
	}
	if !r.Passed {
		os.Exit(1)
	}
}

func evaluate(name string, input fabric.Input, assertions func(fabric.Snapshot) []assertion) scenario {
	snapshot := fabric.Evaluate(input)
	checks := assertions(snapshot)
	if err := snapshot.Validate(); err != nil {
		checks = append(checks, check("snapshot-valid", false, err.Error()))
	} else {
		checks = append(checks, check("snapshot-valid", true, "fabric snapshot is internally consistent"))
	}
	passed := true
	for _, item := range checks {
		if !item.Passed {
			passed = false
		}
	}
	return scenario{Name: name, Snapshot: snapshot, Assertions: checks, Passed: passed}
}

func mutate(input fabric.Input, change func(*fabric.Input)) fabric.Input {
	change(&input)
	return input
}

func check(name string, passed bool, detail string) assertion {
	return assertion{Name: name, Passed: passed, Detail: detail}
}
