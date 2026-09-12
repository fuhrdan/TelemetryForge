// Package replication implements TelemetryForge edge-to-edge durable replication.
package replication

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Mode string

const (
	ModeLocal       Mode = "local"
	ModeRegional    Mode = "regional"
	ModeCrossRegion Mode = "cross-region"
	ModeCrossCloud  Mode = "cross-cloud"
)

var ErrQuorumUnavailable = errors.New("replication quorum unavailable")

// FailureDomain describes the placement of an edge node.
type FailureDomain struct {
	Cloud  string `json:"cloud"`
	Region string `json:"region"`
	Zone   string `json:"zone"`
}

// Node describes one durable edge replication participant.
type Node struct {
	ID     string        `json:"id"`
	URL    string        `json:"url,omitempty"`
	Domain FailureDomain `json:"domain"`
}

// Config controls synchronous replication and quorum evaluation.
type Config struct {
	Local   Node
	Peers   []Node
	Mode    Mode
	Quorum  int
	Timeout time.Duration
	Token   string
}

// Ack is returned only after a receiving peer has durably fsynced a replica.
type Ack struct {
	Node          Node      `json:"node"`
	EdgeID        string    `json:"origin_edge_id"`
	EdgeSequence  uint64    `json:"edge_sequence"`
	PayloadSHA256 string    `json:"payload_sha256"`
	Duplicate     bool      `json:"duplicate"`
	DurableAt     time.Time `json:"durable_at"`
}

// Result summarizes a single quorum decision.
type Result struct {
	Mode          Mode      `json:"mode"`
	Quorum        int       `json:"quorum"`
	AckCount      int       `json:"ack_count"`
	Satisfied     bool      `json:"satisfied"`
	Nodes         []string  `json:"nodes,omitempty"`
	Clouds        []string  `json:"clouds,omitempty"`
	Regions       []string  `json:"regions,omitempty"`
	Zones         []string  `json:"zones,omitempty"`
	CompletedAt   time.Time `json:"completed_at"`
	FailureReason string    `json:"failure_reason,omitempty"`
}

// Status is exposed by /edge/status.
type Status struct {
	Mode            Mode      `json:"mode"`
	Quorum          int       `json:"quorum"`
	ConfiguredPeers int       `json:"configured_peers"`
	LastAckCount    int       `json:"last_ack_count"`
	LastSatisfied   bool      `json:"last_satisfied"`
	LastSuccessAt   time.Time `json:"last_success_at,omitempty"`
	LastFailureAt   time.Time `json:"last_failure_at,omitempty"`
	LastFailure     string    `json:"last_failure,omitempty"`
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.Local.ID) == "" {
		return errors.New("local replication node ID is required")
	}
	switch config.Mode {
	case "", ModeLocal:
	case ModeRegional:
		if strings.TrimSpace(config.Local.Domain.Region) == "" || strings.TrimSpace(config.Local.Domain.Zone) == "" {
			return errors.New("regional durability requires local region and zone")
		}
	case ModeCrossRegion:
		if strings.TrimSpace(config.Local.Domain.Region) == "" {
			return errors.New("cross-region durability requires local region")
		}
	case ModeCrossCloud:
		if strings.TrimSpace(config.Local.Domain.Cloud) == "" {
			return errors.New("cross-cloud durability requires local cloud")
		}
	default:
		return fmt.Errorf("unsupported durability mode %q", config.Mode)
	}
	quorum := config.Quorum
	if quorum == 0 {
		if config.Mode == "" || config.Mode == ModeLocal {
			quorum = 1
		} else {
			quorum = 2
		}
	}
	if quorum < 1 {
		return errors.New("replication quorum must be positive")
	}
	if quorum > len(config.Peers)+1 {
		return fmt.Errorf("replication quorum %d cannot be met by %d configured nodes", quorum, len(config.Peers)+1)
	}
	if config.Mode != ModeLocal && strings.TrimSpace(config.Token) == "" {
		return errors.New("non-local durability requires TELEMETRYFORGE_EDGE_REPLICATION_TOKEN")
	}
	ids := map[string]struct{}{config.Local.ID: {}}
	for _, peer := range config.Peers {
		if strings.TrimSpace(peer.ID) == "" || strings.TrimSpace(peer.URL) == "" {
			return errors.New("replication peers require ID and URL")
		}
		if _, exists := ids[peer.ID]; exists {
			return fmt.Errorf("duplicate replication node ID %q", peer.ID)
		}
		ids[peer.ID] = struct{}{}
	}
	return nil
}

func normalizedConfig(config Config) Config {
	if config.Mode == "" {
		config.Mode = ModeLocal
	}
	if config.Quorum == 0 {
		if config.Mode == ModeLocal {
			config.Quorum = 1
		} else {
			config.Quorum = 2
		}
	}
	if config.Timeout <= 0 {
		config.Timeout = 3 * time.Second
	}
	return config
}
