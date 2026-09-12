// Package mesh implements TelemetryForge global edge routing and failover.
package mesh

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var ErrNoRoute = errors.New("no healthy mesh route available")

// FailureDomain describes the physical or logical placement of a mesh node.
type FailureDomain struct {
	Cloud  string `json:"cloud"`
	Region string `json:"region"`
	Zone   string `json:"zone"`
}

// Node is one routable TelemetryForge edge or relay.
type Node struct {
	ID     string        `json:"id"`
	URL    string        `json:"url,omitempty"`
	Domain FailureDomain `json:"domain"`
}

// State is the authenticated operational advertisement exchanged by peers.
// Pressure is normalized from 0.0 to 1.0 and is used as an eligibility guard,
// not as a promise of exact capacity.
type State struct {
	Node           Node      `json:"node"`
	Ready          bool      `json:"ready"`
	Draining       bool      `json:"draining"`
	PendingRecords uint64    `json:"pending_records"`
	WALBytes       int64     `json:"wal_bytes"`
	WALMaxBytes    int64     `json:"wal_max_bytes"`
	Pressure       float64   `json:"pressure"`
	ObservedAt     time.Time `json:"observed_at"`
}

// PeerStatus is the local view of a peer after active probing.
type PeerStatus struct {
	State               State     `json:"state"`
	Healthy             bool      `json:"healthy"`
	LatencyMS           float64   `json:"latency_ms,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastSuccessAt       time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
}

// Candidate explains one node considered for deterministic ownership.
type Candidate struct {
	Node      Node    `json:"node"`
	Locality  int     `json:"locality_tier"`
	Score     uint64  `json:"rendezvous_score"`
	Pressure  float64 `json:"pressure"`
	LatencyMS float64 `json:"latency_ms,omitempty"`
	IsLocal   bool    `json:"is_local"`
}

// Route is the result of one mesh ownership/failover decision.
type Route struct {
	Key         string      `json:"key"`
	Selected    Node        `json:"selected"`
	Candidates  []Candidate `json:"candidates"`
	Policy      string      `json:"policy"`
	GeneratedAt time.Time   `json:"generated_at"`
}

// Snapshot exposes the current topology and probe state.
type Snapshot struct {
	Local       State        `json:"local"`
	Peers       []PeerStatus `json:"peers"`
	Policy      string       `json:"policy"`
	MaxPressure float64      `json:"max_pressure"`
	GeneratedAt time.Time    `json:"generated_at"`
}

// Config controls peer discovery, probing, and route eligibility.
type Config struct {
	Local         Node
	Peers         []Node
	Token         string
	Policy        string
	ProbeInterval time.Duration
	Timeout       time.Duration
	StaleAfter    time.Duration
	MaxPressure   float64
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.Local.ID) == "" {
		return errors.New("local mesh node ID is required")
	}
	switch config.Policy {
	case "", "locality", "global":
	default:
		return fmt.Errorf("unsupported mesh routing policy %q", config.Policy)
	}
	if config.MaxPressure < 0 || config.MaxPressure > 1 {
		return errors.New("mesh max pressure must be between 0 and 1")
	}
	ids := map[string]struct{}{config.Local.ID: {}}
	for _, peer := range config.Peers {
		if strings.TrimSpace(peer.ID) == "" || strings.TrimSpace(peer.URL) == "" {
			return errors.New("mesh peers require ID and URL")
		}
		parsed, err := url.Parse(strings.TrimSpace(peer.URL))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Host) == "" {
			return fmt.Errorf("mesh peer %q URL must use http or https with a host", peer.ID)
		}
		if _, exists := ids[peer.ID]; exists {
			return fmt.Errorf("duplicate mesh node ID %q", peer.ID)
		}
		ids[peer.ID] = struct{}{}
	}
	if len(config.Peers) > 0 && strings.TrimSpace(config.Token) == "" {
		return errors.New("configured mesh peers require TELEMETRYFORGE_MESH_TOKEN")
	}
	return nil
}

func normalizedConfig(config Config) Config {
	config.Policy = strings.ToLower(strings.TrimSpace(config.Policy))
	if config.Policy == "" {
		config.Policy = "locality"
	}
	if config.ProbeInterval <= 0 {
		config.ProbeInterval = 5 * time.Second
	}
	if config.Timeout <= 0 {
		config.Timeout = 2 * time.Second
	}
	if config.StaleAfter <= 0 {
		config.StaleAfter = 3 * config.ProbeInterval
	}
	if config.MaxPressure == 0 {
		config.MaxPressure = 0.90
	}
	return config
}
