package replication

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

// Manager synchronously replicates accepted WAL records and evaluates the
// configured quorum/failure-domain policy.
type Manager struct {
	config Config
	client *http.Client
	mutex  sync.Mutex
	status Status
}

func NewManager(config Config) (*Manager, error) {
	config = normalizedConfig(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	manager := &Manager{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		status: Status{Mode: config.Mode, Quorum: config.Quorum, ConfiguredPeers: len(config.Peers)},
	}
	// Catch impossible failure-domain layouts at startup rather than accepting
	// traffic that could never satisfy the requested durability class.
	acknowledgements := []Ack{{Node: config.Local}}
	for _, peer := range config.Peers {
		acknowledgements = append(acknowledgements, Ack{Node: peer})
	}
	possible := evaluate(config, acknowledgements)
	if !possible.Satisfied {
		return nil, fmt.Errorf("configured replication topology cannot satisfy %s durability: %s", config.Mode, possible.FailureReason)
	}
	return manager, nil
}

func (manager *Manager) Replicate(ctx context.Context, record wal.Record) (Result, error) {
	acknowledgements := []Ack{{Node: manager.config.Local, EdgeID: record.EdgeID, EdgeSequence: record.EdgeSequence, PayloadSHA256: record.PayloadSHA256, DurableAt: record.AcceptedAt}}
	if manager.config.Mode == ModeLocal {
		result := evaluate(manager.config, acknowledgements)
		manager.record(result, nil)
		return result, nil
	}

	replicationContext, cancel := context.WithTimeout(ctx, manager.config.Timeout)
	defer cancel()
	type peerResult struct {
		acknowledgement Ack
		err             error
	}
	results := make(chan peerResult, len(manager.config.Peers))
	for _, peer := range manager.config.Peers {
		peer := peer
		go func() {
			acknowledgement, err := replicateToPeer(replicationContext, manager.client, peer, manager.config.Token, record)
			results <- peerResult{acknowledgement: acknowledgement, err: err}
		}()
	}

	failures := 0
	for range manager.config.Peers {
		select {
		case <-replicationContext.Done():
			result := evaluate(manager.config, acknowledgements)
			result.FailureReason = "replication timeout before durability policy was satisfied"
			manager.record(result, replicationContext.Err())
			return result, fmt.Errorf("%w: %s", ErrQuorumUnavailable, result.FailureReason)
		case peerResult := <-results:
			if peerResult.err != nil {
				failures++
			} else {
				acknowledgements = append(acknowledgements, peerResult.acknowledgement)
				result := evaluate(manager.config, acknowledgements)
				if result.Satisfied {
					cancel()
					manager.record(result, nil)
					return result, nil
				}
			}
		}
	}
	result := evaluate(manager.config, acknowledgements)
	if result.FailureReason == "" {
		result.FailureReason = fmt.Sprintf("durability policy not satisfied after %d peer failures", failures)
	}
	manager.record(result, ErrQuorumUnavailable)
	return result, fmt.Errorf("%w: %s", ErrQuorumUnavailable, result.FailureReason)
}

func (manager *Manager) Ready(ctx context.Context) error {
	if manager.config.Mode == ModeLocal {
		return nil
	}
	probeContext, cancel := context.WithTimeout(ctx, manager.config.Timeout)
	defer cancel()
	acknowledgements := []Ack{{Node: manager.config.Local}}
	type peerResult struct {
		acknowledgement Ack
	}
	results := make(chan peerResult, len(manager.config.Peers))
	for _, peer := range manager.config.Peers {
		peer := peer
		go func() {
			acknowledgement, _ := probePeer(probeContext, manager.client, peer, manager.config.Token)
			results <- peerResult{acknowledgement: acknowledgement}
		}()
	}
	for range manager.config.Peers {
		select {
		case <-probeContext.Done():
			return ErrQuorumUnavailable
		case result := <-results:
			if result.acknowledgement.Node.ID != "" {
				acknowledgements = append(acknowledgements, result.acknowledgement)
				if evaluate(manager.config, acknowledgements).Satisfied {
					return nil
				}
			}
		}
	}
	return ErrQuorumUnavailable
}

// Release informs configured peers that the origin has durably checkpointed
// downstream delivery through the supplied edge sequence. Release is best-effort
// from the data-plane perspective; failures retain extra replica data rather than
// weakening durability.
func (manager *Manager) Release(ctx context.Context, through uint64) error {
	if manager.config.Mode == ModeLocal || through == 0 {
		return nil
	}
	releaseContext, cancel := context.WithTimeout(ctx, manager.config.Timeout)
	defer cancel()
	errorsChannel := make(chan error, len(manager.config.Peers))
	for _, peer := range manager.config.Peers {
		peer := peer
		go func() {
			errorsChannel <- releasePeer(releaseContext, manager.client, peer, manager.config.Token, manager.config.Local.ID, through)
		}()
	}
	var first error
	for range manager.config.Peers {
		select {
		case <-releaseContext.Done():
			if first == nil {
				first = releaseContext.Err()
			}
			return first
		case err := <-errorsChannel:
			if err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func (manager *Manager) Status() Status {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	return manager.status
}

func (manager *Manager) Config() Config { return manager.config }

func (manager *Manager) record(result Result, err error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.status.LastAckCount = result.AckCount
	manager.status.LastSatisfied = result.Satisfied
	now := time.Now().UTC()
	if err == nil && result.Satisfied {
		manager.status.LastSuccessAt = now
		manager.status.LastFailure = ""
	} else {
		manager.status.LastFailureAt = now
		manager.status.LastFailure = result.FailureReason
		if manager.status.LastFailure == "" && err != nil {
			manager.status.LastFailure = err.Error()
		}
	}
}
