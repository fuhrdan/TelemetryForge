package mesh

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"
)

type StateProvider func(context.Context) State

// Manager maintains a health-aware topology view and deterministic route ownership.
type Manager struct {
	config     Config
	client     *http.Client
	localState StateProvider
	mutex      sync.RWMutex
	peers      map[string]PeerStatus
	wake       chan struct{}
}

func NewManager(config Config, localState StateProvider) (*Manager, error) {
	config = normalizedConfig(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if localState == nil {
		return nil, errors.New("mesh local state provider is required")
	}
	manager := &Manager{
		config:     config,
		client:     &http.Client{Timeout: config.Timeout},
		localState: localState,
		peers:      make(map[string]PeerStatus, len(config.Peers)),
		wake:       make(chan struct{}, 1),
	}
	for _, peer := range config.Peers {
		manager.peers[peer.ID] = PeerStatus{State: State{Node: peer}}
	}
	return manager, nil
}

func (manager *Manager) Run(ctx context.Context) {
	manager.Probe(ctx)
	ticker := time.NewTicker(manager.config.ProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			manager.Probe(ctx)
		case <-manager.wake:
			manager.Probe(ctx)
		}
	}
}

// Probe refreshes all peer advertisements concurrently.
func (manager *Manager) Probe(ctx context.Context) {
	type result struct {
		node    Node
		state   State
		latency time.Duration
		err     error
	}
	results := make(chan result, len(manager.config.Peers))
	for _, peer := range manager.config.Peers {
		peer := peer
		go func() {
			probeContext, cancel := context.WithTimeout(ctx, manager.config.Timeout)
			defer cancel()
			start := time.Now()
			state, err := probePeer(probeContext, manager.client, peer, manager.config.Token)
			results <- result{node: peer, state: state, latency: time.Since(start), err: err}
		}()
	}
	for range manager.config.Peers {
		select {
		case <-ctx.Done():
			return
		case probe := <-results:
			manager.recordProbe(probe.node, probe.state, probe.latency, probe.err)
		}
	}
}

func (manager *Manager) recordProbe(node Node, state State, latency time.Duration, err error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	status := manager.peers[node.ID]
	now := time.Now().UTC()
	if err != nil {
		status.Healthy = false
		status.ConsecutiveFailures++
		status.LastFailureAt = now
		status.LastError = err.Error()
		manager.peers[node.ID] = status
		return
	}
	state.ObservedAt = now
	status.State = state
	status.Healthy = true
	status.LatencyMS = float64(latency.Microseconds()) / 1000.0
	status.ConsecutiveFailures = 0
	status.LastSuccessAt = now
	status.LastError = ""
	manager.peers[node.ID] = status
}

func (manager *Manager) ReportFailure(nodeID string, err error) {
	if nodeID == manager.config.Local.ID {
		return
	}
	manager.mutex.Lock()
	status, exists := manager.peers[nodeID]
	if exists {
		status.Healthy = false
		status.ConsecutiveFailures++
		status.LastFailureAt = time.Now().UTC()
		if err != nil {
			status.LastError = err.Error()
		}
		manager.peers[nodeID] = status
	}
	manager.mutex.Unlock()
	select {
	case manager.wake <- struct{}{}:
	default:
	}
}

func (manager *Manager) ReportSuccess(nodeID string) {
	if nodeID == manager.config.Local.ID {
		return
	}
	manager.mutex.Lock()
	status, exists := manager.peers[nodeID]
	if exists {
		status.Healthy = true
		status.ConsecutiveFailures = 0
		status.LastSuccessAt = time.Now().UTC()
		status.LastError = ""
		manager.peers[nodeID] = status
	}
	manager.mutex.Unlock()
}

// Route ranks all currently eligible nodes. Under the default locality policy,
// the best available failure-domain tier is chosen first; rendezvous hashing
// then gives stable ownership inside that tier. If every node in that tier
// becomes unavailable, the next tier becomes eligible automatically.
func (manager *Manager) Route(ctx context.Context, key string) (Route, error) {
	local := manager.localState(ctx)
	local.Node = manager.config.Local
	if local.ObservedAt.IsZero() {
		local.ObservedAt = time.Now().UTC()
	}

	manager.mutex.RLock()
	peerCopy := make([]PeerStatus, 0, len(manager.peers))
	for _, status := range manager.peers {
		peerCopy = append(peerCopy, status)
	}
	manager.mutex.RUnlock()

	candidates := make([]Candidate, 0, len(peerCopy)+1)
	if eligible(local, true, time.Now(), manager.config.StaleAfter, manager.config.MaxPressure) {
		candidates = append(candidates, candidateFor(key, manager.config.Local, local.Pressure, 0, true, manager.config.Local, manager.config.Policy))
	}
	now := time.Now()
	for _, status := range peerCopy {
		if !status.Healthy || !eligible(status.State, false, now, manager.config.StaleAfter, manager.config.MaxPressure) {
			continue
		}
		candidates = append(candidates, candidateFor(key, status.State.Node, status.State.Pressure, status.LatencyMS, false, manager.config.Local, manager.config.Policy))
	}
	if len(candidates) == 0 {
		return Route{}, ErrNoRoute
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Locality != candidates[j].Locality {
			return candidates[i].Locality < candidates[j].Locality
		}
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Node.ID < candidates[j].Node.ID
	})
	return Route{Key: key, Selected: candidates[0].Node, Candidates: candidates, Policy: manager.config.Policy, GeneratedAt: time.Now().UTC()}, nil
}

func eligible(state State, local bool, now time.Time, staleAfter time.Duration, maxPressure float64) bool {
	if !state.Ready || state.Draining || state.Pressure >= maxPressure {
		return false
	}
	if !local && (state.ObservedAt.IsZero() || now.Sub(state.ObservedAt) > staleAfter) {
		return false
	}
	return true
}

func candidateFor(key string, node Node, pressure, latency float64, local bool, localNode Node, policy string) Candidate {
	sum := sha256.Sum256([]byte(key + "|" + node.ID))
	score := binary.BigEndian.Uint64(sum[:8])
	locality := 0
	if policy == "locality" {
		locality = localityTier(localNode.Domain, node.Domain)
	}
	return Candidate{Node: node, Locality: locality, Score: score, Pressure: pressure, LatencyMS: latency, IsLocal: local}
}

func localityTier(local, remote FailureDomain) int {
	if local.Cloud == remote.Cloud && local.Region == remote.Region && local.Zone == remote.Zone {
		return 0
	}
	if local.Cloud == remote.Cloud && local.Region != "" && local.Region == remote.Region {
		return 1
	}
	if local.Cloud != "" && local.Cloud == remote.Cloud {
		return 2
	}
	return 3
}

func (manager *Manager) Snapshot(ctx context.Context) Snapshot {
	local := manager.localState(ctx)
	local.Node = manager.config.Local
	if local.ObservedAt.IsZero() {
		local.ObservedAt = time.Now().UTC()
	}
	manager.mutex.RLock()
	peers := make([]PeerStatus, 0, len(manager.peers))
	for _, status := range manager.peers {
		peers = append(peers, status)
	}
	manager.mutex.RUnlock()
	sort.Slice(peers, func(i, j int) bool { return peers[i].State.Node.ID < peers[j].State.Node.ID })
	return Snapshot{Local: local, Peers: peers, Policy: manager.config.Policy, MaxPressure: manager.config.MaxPressure, GeneratedAt: time.Now().UTC()}
}

func (manager *Manager) Config() Config { return manager.config }
