package ebpf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type KernelConfig struct {
	TraceFSRoot string
	Signals     []string
}

type kernelProbe struct {
	definition Probe
	mapFD      int
	progFD     int
	perfFDs    []int
}

type KernelBackend struct {
	cfg       KernelConfig
	probes    []*kernelProbe
	probeView []Probe
	started   bool
}

func NewKernelBackend(cfg KernelConfig) *KernelBackend {
	if len(cfg.Signals) == 0 {
		cfg.Signals = DefaultSignals()
	}
	definitions := probeDefinitions(cfg.Signals)
	view := make([]Probe, len(definitions))
	copy(view, definitions)
	return &KernelBackend{cfg: cfg, probeView: view}
}

func (backend *KernelBackend) Name() string    { return "linux-ebpf-tracepoint-counter" }
func (backend *KernelBackend) Probes() []Probe { return append([]Probe(nil), backend.probeView...) }

func probeDefinitions(signals []string) []Probe {
	defs := map[string]Probe{
		SignalProcessExec:   {Signal: SignalProcessExec, Group: "sched", Event: "sched_process_exec", MetricName: MetricName(SignalProcessExec)},
		SignalSocketConnect: {Signal: SignalSocketConnect, Group: "syscalls", Event: "sys_enter_connect", MetricName: MetricName(SignalSocketConnect)},
		SignalTCPRetransmit: {Signal: SignalTCPRetransmit, Group: "tcp", Event: "tcp_retransmit_skb", MetricName: MetricName(SignalTCPRetransmit)},
	}
	out := make([]Probe, 0, len(signals))
	for _, signal := range signals {
		if def, ok := defs[signal]; ok {
			out = append(out, def)
		}
	}
	return out
}

func (backend *KernelBackend) Start() error {
	if runtime.GOOS != "linux" {
		return errors.New("eBPF collection requires Linux")
	}
	if backend.started {
		return nil
	}
	root := strings.TrimSpace(backend.cfg.TraceFSRoot)
	if root == "" {
		root = discoverTraceFS()
	}
	if root == "" {
		return errors.New("tracefs not found; expected /sys/kernel/tracing or /sys/kernel/debug/tracing")
	}
	cpus, err := onlineCPUs("/sys/devices/system/cpu/online")
	if err != nil {
		return fmt.Errorf("read online CPUs: %w", err)
	}
	defs := probeDefinitions(backend.cfg.Signals)
	if len(defs) == 0 {
		return errors.New("no supported eBPF signals configured")
	}
	backend.probes = nil
	backend.probeView = make([]Probe, 0, len(defs))
	for _, def := range defs {
		current := def
		probe, attachErr := attachCounterProbe(root, cpus, current)
		if attachErr != nil {
			current.Error = attachErr.Error()
			backend.probeView = append(backend.probeView, current)
			continue
		}
		current.Active = true
		probe.definition = current
		backend.probes = append(backend.probes, probe)
		backend.probeView = append(backend.probeView, current)
	}
	if len(backend.probes) == 0 {
		return fmt.Errorf("no eBPF probes could attach; Linux requires tracefs plus BPF/perf permissions (CAP_BPF and CAP_PERFMON on modern kernels, or equivalent privilege)")
	}
	backend.started = true
	return nil
}

func (backend *KernelBackend) Snapshot() (Snapshot, error) {
	if !backend.started {
		return Snapshot{}, errors.New("eBPF backend is not started")
	}
	totals := make(map[string]uint64, len(backend.probes))
	for _, probe := range backend.probes {
		value, err := lookupCounter(probe.mapFD)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read %s counter: %w", probe.definition.Signal, err)
		}
		totals[probe.definition.Signal] = value
	}
	return Snapshot{Totals: totals, At: time.Now().UTC()}, nil
}

func (backend *KernelBackend) Close() error {
	var first error
	for _, probe := range backend.probes {
		for _, fd := range probe.perfFDs {
			if err := closeFD(fd); err != nil && first == nil {
				first = err
			}
		}
		if probe.progFD >= 0 {
			if err := closeFD(probe.progFD); err != nil && first == nil {
				first = err
			}
		}
		if probe.mapFD >= 0 {
			if err := closeFD(probe.mapFD); err != nil && first == nil {
				first = err
			}
		}
	}
	backend.probes = nil
	backend.started = false
	return first
}

func discoverTraceFS() string {
	for _, path := range []string{"/sys/kernel/tracing", "/sys/kernel/debug/tracing"} {
		if info, err := os.Stat(filepath.Join(path, "events")); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

func tracepointID(root string, probe Probe) (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(root, "events", probe.Group, probe.Event, "id"))
	if err != nil {
		return 0, fmt.Errorf("tracepoint %s/%s unavailable: %w", probe.Group, probe.Event, err)
	}
	id, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse tracepoint id: %w", err)
	}
	return id, nil
}

func onlineCPUs(path string) ([]int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCPUList(strings.TrimSpace(string(raw)))
}

func parseCPUList(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("empty CPU list")
	}
	seen := map[int]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bounds := strings.SplitN(part, "-", 2)
		start, err := strconv.Atoi(bounds[0])
		if err != nil || start < 0 {
			return nil, fmt.Errorf("invalid CPU range %q", part)
		}
		end := start
		if len(bounds) == 2 {
			end, err = strconv.Atoi(bounds[1])
			if err != nil || end < start {
				return nil, fmt.Errorf("invalid CPU range %q", part)
			}
		}
		for cpu := start; cpu <= end; cpu++ {
			seen[cpu] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for cpu := range seen {
		out = append(out, cpu)
	}
	sort.Ints(out)
	if len(out) == 0 {
		return nil, errors.New("no online CPUs parsed")
	}
	return out, nil
}

type HostInspection struct {
	OS              string          `json:"os"`
	Architecture    string          `json:"architecture"`
	TraceFSRoot     string          `json:"tracefs_root,omitempty"`
	OnlineCPUs      int             `json:"online_cpus"`
	TracepointState map[string]bool `json:"tracepoints"`
}

// InspectHost reports whether the kernel surfaces used by v2.8 are present. It
// does not require BPF privileges and does not claim that live attachment will
// succeed under the current container/LSM/capability policy.
func InspectHost(cfg KernelConfig) HostInspection {
	info := HostInspection{OS: runtime.GOOS, Architecture: runtime.GOARCH, TracepointState: map[string]bool{}}
	root := strings.TrimSpace(cfg.TraceFSRoot)
	if root == "" {
		root = discoverTraceFS()
	}
	info.TraceFSRoot = root
	if cpus, err := onlineCPUs("/sys/devices/system/cpu/online"); err == nil {
		info.OnlineCPUs = len(cpus)
	}
	if root != "" {
		for _, probe := range probeDefinitions(cfg.Signals) {
			_, err := tracepointID(root, probe)
			info.TracepointState[probe.Signal] = err == nil
		}
	}
	return info
}
