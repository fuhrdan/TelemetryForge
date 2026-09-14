// Package ebpf implements the optional Linux kernel collection boundary used by
// TelemetryForge edge nodes. The package deliberately emits bounded aggregate
// counters rather than packet payloads or process arguments.
package ebpf

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const SchemaVersion = "1.0.0"

const (
	SignalProcessExec   = "process_exec"
	SignalSocketConnect = "socket_connect"
	SignalTCPRetransmit = "tcp_retransmit"
)

type Probe struct {
	Signal     string `json:"signal"`
	Group      string `json:"group"`
	Event      string `json:"event"`
	MetricName string `json:"metric_name"`
	Active     bool   `json:"active"`
	Error      string `json:"error,omitempty"`
}

type Snapshot struct {
	Totals map[string]uint64 `json:"totals"`
	At     time.Time         `json:"at"`
}

type Backend interface {
	Start() error
	Snapshot() (Snapshot, error)
	Probes() []Probe
	Close() error
	Name() string
}

// Publisher reports whether an event reached the durable local WAL boundary.
// A replication error may be returned together with persisted=true; callers
// must not re-emit that counter delta because the WAL will replay it.
type Publisher interface {
	PublishKernelMetric(ctx context.Context, topic string, event domain.Event) (persisted bool, err error)
}

type Config struct {
	Enabled      bool
	Required     bool
	PollInterval time.Duration
	Topic        string
	Source       string
	TenantID     string
	EdgeID       string
	Signals      []string
}

type Status struct {
	Enabled         bool              `json:"enabled"`
	Required        bool              `json:"required"`
	Active          bool              `json:"active"`
	Backend         string            `json:"backend"`
	PollInterval    string            `json:"poll_interval"`
	Source          string            `json:"source"`
	TenantID        string            `json:"tenant_id"`
	Topic           string            `json:"topic"`
	Probes          []Probe           `json:"probes"`
	Totals          map[string]uint64 `json:"totals,omitempty"`
	PersistedTotals map[string]uint64 `json:"persisted_totals,omitempty"`
	PublishedEvents uint64            `json:"published_events"`
	PublishErrors   uint64            `json:"publish_errors"`
	SnapshotErrors  uint64            `json:"snapshot_errors"`
	LastSnapshotAt  time.Time         `json:"last_snapshot_at,omitempty"`
	LastPublishAt   time.Time         `json:"last_publish_at,omitempty"`
	LastError       string            `json:"last_error,omitempty"`
}

type Agent struct {
	cfg       Config
	backend   Backend
	publisher Publisher

	mu     sync.RWMutex
	status Status
}

func DefaultSignals() []string {
	return []string{SignalProcessExec, SignalSocketConnect, SignalTCPRetransmit}
}

func MetricName(signal string) string {
	switch signal {
	case SignalProcessExec:
		return "kernel.process.exec"
	case SignalSocketConnect:
		return "kernel.socket.connect"
	case SignalTCPRetransmit:
		return "kernel.tcp.retransmit"
	default:
		return "kernel." + strings.ReplaceAll(signal, "_", ".")
	}
}

func ParseSignals(raw string) []string {
	signals, _ := ParseSignalsStrict(raw)
	return signals
}

func ParseSignalsStrict(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultSignals(), nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		signal := strings.TrimSpace(strings.ToLower(part))
		if signal == "" {
			continue
		}
		switch signal {
		case SignalProcessExec, SignalSocketConnect, SignalTCPRetransmit:
			if _, ok := seen[signal]; !ok {
				seen[signal] = struct{}{}
				out = append(out, signal)
			}
		default:
			return nil, fmt.Errorf("unsupported eBPF signal %q", signal)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no eBPF signals configured")
	}
	return out, nil
}

func NewAgent(cfg Config, backend Backend, publisher Publisher) (*Agent, error) {
	if backend == nil {
		return nil, errors.New("ebpf backend is required")
	}
	if publisher == nil {
		return nil, errors.New("ebpf publisher is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.PollInterval < 250*time.Millisecond {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if strings.TrimSpace(cfg.Topic) == "" {
		return nil, errors.New("ebpf metric topic is required")
	}
	if strings.TrimSpace(cfg.Source) == "" {
		cfg.Source = "telemetryforge-ebpf"
	}
	if len(cfg.Signals) == 0 {
		cfg.Signals = DefaultSignals()
	}
	status := Status{
		Enabled: cfg.Enabled, Required: cfg.Required, Backend: backend.Name(),
		PollInterval: cfg.PollInterval.String(), Source: cfg.Source, TenantID: nonempty(cfg.TenantID, "default"), Topic: cfg.Topic,
		Totals: map[string]uint64{}, PersistedTotals: map[string]uint64{}, Probes: backend.Probes(),
	}
	return &Agent{cfg: cfg, backend: backend, publisher: publisher, status: status}, nil
}

// Start attaches the configured kernel probes. It is separate from Run so an
// edge process configured with Required=true can fail startup synchronously.
func (agent *Agent) Start() error {
	if !agent.cfg.Enabled {
		return nil
	}
	agent.mu.RLock()
	active := agent.status.Active
	agent.mu.RUnlock()
	if active {
		return nil
	}
	if err := agent.backend.Start(); err != nil {
		agent.setError(err)
		return err
	}
	agent.mu.Lock()
	agent.status.Active = true
	agent.status.Probes = agent.backend.Probes()
	agent.status.LastError = ""
	agent.mu.Unlock()
	return nil
}

func (agent *Agent) Run(ctx context.Context) error {
	if !agent.cfg.Enabled {
		return nil
	}
	if err := agent.Start(); err != nil {
		if agent.cfg.Required {
			return err
		}
		return nil
	}
	defer agent.backend.Close()
	defer func() {
		agent.mu.Lock()
		agent.status.Active = false
		for index := range agent.status.Probes {
			agent.status.Probes[index].Active = false
		}
		agent.mu.Unlock()
	}()

	ticker := time.NewTicker(agent.cfg.PollInterval)
	defer ticker.Stop()
	// Take one snapshot immediately so short-lived edge processes still expose
	// collector state without waiting for the first interval.
	agent.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			agent.poll(ctx)
		}
	}
}

func (agent *Agent) poll(ctx context.Context) {
	snapshot, err := agent.backend.Snapshot()
	if err != nil {
		agent.mu.Lock()
		agent.status.SnapshotErrors++
		agent.status.LastError = err.Error()
		agent.mu.Unlock()
		return
	}
	if snapshot.At.IsZero() {
		snapshot.At = time.Now().UTC()
	}
	agent.mu.Lock()
	agent.status.LastSnapshotAt = snapshot.At
	agent.status.Totals = copyTotals(snapshot.Totals)
	persisted := copyTotals(agent.status.PersistedTotals)
	agent.mu.Unlock()

	signals := make([]string, 0, len(snapshot.Totals))
	for signal := range snapshot.Totals {
		signals = append(signals, signal)
	}
	sort.Strings(signals)
	for _, signal := range signals {
		total := snapshot.Totals[signal]
		base := persisted[signal]
		if total < base { // counter/map reset after probe reattach
			base = 0
		}
		if total == base {
			continue
		}
		delta := total - base
		value := float64(delta)
		event := domain.Event{
			ID:        fmt.Sprintf("ebpf:%s:%s:%d", nonempty(agent.cfg.EdgeID, "edge"), signal, total),
			TenantID:  nonempty(agent.cfg.TenantID, "default"),
			Source:    agent.cfg.Source,
			Type:      MetricName(signal),
			Timestamp: snapshot.At,
			Tags: map[string]string{
				"collector": "ebpf",
				"signal":    signal,
				"edge_id":   nonempty(agent.cfg.EdgeID, "unknown"),
			},
			Value:         &value,
			Unit:          "1",
			SchemaVersion: SchemaVersion,
		}
		durable, publishErr := agent.publisher.PublishKernelMetric(ctx, agent.cfg.Topic, event)
		agent.mu.Lock()
		if durable {
			agent.status.PersistedTotals[signal] = total
			persisted[signal] = total
			agent.status.PublishedEvents++
			agent.status.LastPublishAt = time.Now().UTC()
		}
		if publishErr != nil {
			agent.status.PublishErrors++
			agent.status.LastError = publishErr.Error()
		} else if durable {
			agent.status.LastError = ""
		}
		agent.mu.Unlock()
	}
}

func (agent *Agent) Status() Status {
	agent.mu.RLock()
	defer agent.mu.RUnlock()
	out := agent.status
	out.Totals = copyTotals(agent.status.Totals)
	out.PersistedTotals = copyTotals(agent.status.PersistedTotals)
	out.Probes = append([]Probe(nil), agent.status.Probes...)
	return out
}

func (agent *Agent) setError(err error) {
	agent.mu.Lock()
	defer agent.mu.Unlock()
	agent.status.Active = false
	agent.status.Probes = agent.backend.Probes()
	agent.status.LastError = err.Error()
}

func copyTotals(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func nonempty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
