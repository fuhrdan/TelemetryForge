package replication

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func evaluate(config Config, acknowledgements []Ack) Result {
	config = normalizedConfig(config)
	nodes := make(map[string]struct{})
	clouds := make(map[string]struct{})
	regions := make(map[string]struct{})
	zones := make(map[string]struct{})
	sameRegionZones := make(map[string]struct{})

	for _, acknowledgement := range acknowledgements {
		node := acknowledgement.Node
		if strings.TrimSpace(node.ID) == "" {
			continue
		}
		// Regional durability is intentionally constrained to the local region;
		// an acknowledgement from another region does not inflate its quorum.
		if config.Mode == ModeRegional && node.Domain.Region != config.Local.Domain.Region {
			continue
		}
		nodes[node.ID] = struct{}{}
		if node.Domain.Cloud != "" {
			clouds[node.Domain.Cloud] = struct{}{}
		}
		if node.Domain.Region != "" {
			regions[node.Domain.Region] = struct{}{}
		}
		if node.Domain.Zone != "" {
			zones[node.Domain.Zone] = struct{}{}
		}
		if node.Domain.Region == config.Local.Domain.Region && node.Domain.Zone != "" {
			sameRegionZones[node.Domain.Zone] = struct{}{}
		}
	}

	result := Result{
		Mode:        config.Mode,
		Quorum:      config.Quorum,
		AckCount:    len(nodes),
		Nodes:       sortedKeys(nodes),
		Clouds:      sortedKeys(clouds),
		Regions:     sortedKeys(regions),
		Zones:       sortedKeys(zones),
		CompletedAt: time.Now().UTC(),
	}

	if len(nodes) < config.Quorum {
		result.FailureReason = fmt.Sprintf("need %d durable nodes, have %d", config.Quorum, len(nodes))
		return result
	}

	switch config.Mode {
	case ModeLocal:
		result.Satisfied = true
	case ModeRegional:
		if len(sameRegionZones) < 2 {
			result.FailureReason = "regional durability requires acknowledgements from at least two zones in the local region"
			return result
		}
		result.Satisfied = true
	case ModeCrossRegion:
		if len(regions) < 2 {
			result.FailureReason = "cross-region durability requires acknowledgements from at least two regions"
			return result
		}
		result.Satisfied = true
	case ModeCrossCloud:
		if len(clouds) < 2 {
			result.FailureReason = "cross-cloud durability requires acknowledgements from at least two clouds"
			return result
		}
		result.Satisfied = true
	}
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
