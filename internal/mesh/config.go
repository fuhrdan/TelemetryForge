package mesh

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// ParsePeers parses comma-separated mesh peer specifications:
// id|url|cloud|region|zone
func ParsePeers(raw string) ([]Node, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	entries := strings.Split(raw, ",")
	peers := make([]Node, 0, len(entries))
	for _, entry := range entries {
		fields := strings.Split(strings.TrimSpace(entry), "|")
		if len(fields) != 5 {
			return nil, errors.New("mesh peer must be id|url|cloud|region|zone")
		}
		peers = append(peers, Node{
			ID:  strings.TrimSpace(fields[0]),
			URL: strings.TrimSpace(fields[1]),
			Domain: FailureDomain{
				Cloud: strings.TrimSpace(fields[2]), Region: strings.TrimSpace(fields[3]), Zone: strings.TrimSpace(fields[4]),
			},
		})
	}
	return peers, nil
}

func ParseDuration(raw string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func ParsePressure(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value <= 0 || value > 1 {
		return fallback
	}
	return value
}
