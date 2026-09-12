package replication

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// ParsePeers parses comma-separated peer specifications:
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
			return nil, errors.New("replication peer must be id|url|cloud|region|zone")
		}
		peers = append(peers, Node{ID: strings.TrimSpace(fields[0]), URL: strings.TrimSpace(fields[1]), Domain: FailureDomain{Cloud: strings.TrimSpace(fields[2]), Region: strings.TrimSpace(fields[3]), Zone: strings.TrimSpace(fields[4])}})
	}
	return peers, nil
}

func ParseMode(raw string) Mode {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ModeLocal
	}
	return Mode(value)
}

func ParseQuorum(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func ParseTimeout(raw string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
