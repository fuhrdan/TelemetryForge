package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type testPublisher struct {
	mutex  sync.Mutex
	events []domain.Event
	err    error
}

func (publisher *testPublisher) Publish(_ context.Context, _ string, event domain.Event) error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	if publisher.err != nil {
		return publisher.err
	}
	publisher.events = append(publisher.events, event)
	return nil
}
func (publisher *testPublisher) Ready(context.Context) error { return publisher.err }
func (publisher *testPublisher) Close()                      {}

func readyState(node Node) State {
	return State{Node: node, Ready: true, WALMaxBytes: 100, WALBytes: 10, Pressure: 0.1, ObservedAt: time.Now().UTC()}
}

func TestRoutePrefersBestLocalityTier(t *testing.T) {
	local := Node{ID: "a", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}}
	sameRegion := Node{ID: "b", URL: "http://b", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}
	remote := Node{ID: "c", URL: "http://c", Domain: FailureDomain{Cloud: "gcp", Region: "us-central1", Zone: "a"}}
	manager, err := NewManager(Config{Local: local, Peers: []Node{sameRegion, remote}, Policy: "locality", Token: "token"}, func(context.Context) State { return readyState(local) })
	if err != nil {
		t.Fatal(err)
	}
	manager.mutex.Lock()
	manager.peers[sameRegion.ID] = PeerStatus{State: readyState(sameRegion), Healthy: true}
	manager.peers[remote.ID] = PeerStatus{State: readyState(remote), Healthy: true}
	manager.mutex.Unlock()

	route, err := manager.Route(context.Background(), "tenant|source")
	if err != nil {
		t.Fatal(err)
	}
	if route.Selected.ID != local.ID {
		t.Fatalf("expected exact-locality node %q, got %q", local.ID, route.Selected.ID)
	}
	for _, candidate := range route.Candidates {
		if candidate.Node.ID == remote.ID && candidate.Locality != 3 {
			t.Fatalf("expected cross-cloud locality tier 3, got %d", candidate.Locality)
		}
	}
}

func TestGlobalRouteIsDeterministic(t *testing.T) {
	local := Node{ID: "a", Domain: FailureDomain{Cloud: "aws", Region: "west", Zone: "a"}}
	peerB := Node{ID: "b", URL: "http://b", Domain: FailureDomain{Cloud: "gcp", Region: "central", Zone: "a"}}
	peerC := Node{ID: "c", URL: "http://c", Domain: FailureDomain{Cloud: "azure", Region: "west", Zone: "b"}}
	manager, err := NewManager(Config{Local: local, Peers: []Node{peerB, peerC}, Policy: "global", Token: "token"}, func(context.Context) State { return readyState(local) })
	if err != nil {
		t.Fatal(err)
	}
	manager.mutex.Lock()
	manager.peers[peerB.ID] = PeerStatus{State: readyState(peerB), Healthy: true}
	manager.peers[peerC.ID] = PeerStatus{State: readyState(peerC), Healthy: true}
	manager.mutex.Unlock()

	first, err := manager.Route(context.Background(), "tenant|checkout")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Route(context.Background(), "tenant|checkout")
	if err != nil {
		t.Fatal(err)
	}
	if first.Selected.ID != second.Selected.ID {
		t.Fatalf("route owner changed for identical topology/key: %q then %q", first.Selected.ID, second.Selected.ID)
	}
	if len(first.Candidates) != 3 {
		t.Fatalf("expected 3 eligible global candidates, got %d", len(first.Candidates))
	}
}

func TestRouteFailsOverWhenLocalDraining(t *testing.T) {
	local := Node{ID: "a", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}}
	peer := Node{ID: "b", URL: "http://b", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}
	manager, err := NewManager(Config{Local: local, Peers: []Node{peer}, Policy: "locality", Token: "token"}, func(context.Context) State {
		state := readyState(local)
		state.Draining = true
		return state
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.mutex.Lock()
	manager.peers[peer.ID] = PeerStatus{State: readyState(peer), Healthy: true}
	manager.mutex.Unlock()

	route, err := manager.Route(context.Background(), "tenant|source")
	if err != nil {
		t.Fatal(err)
	}
	if route.Selected.ID != peer.ID {
		t.Fatalf("expected peer failover, got %q", route.Selected.ID)
	}
}

func TestRouteRejectsStaleAndPressuredPeers(t *testing.T) {
	local := Node{ID: "a", Domain: FailureDomain{Cloud: "aws", Region: "x", Zone: "a"}}
	peer := Node{ID: "b", URL: "http://b", Domain: FailureDomain{Cloud: "gcp", Region: "y", Zone: "b"}}
	manager, err := NewManager(Config{Local: local, Peers: []Node{peer}, Token: "token", MaxPressure: 0.8, StaleAfter: time.Second}, func(context.Context) State {
		state := readyState(local)
		state.Pressure = 0.95
		return state
	})
	if err != nil {
		t.Fatal(err)
	}
	state := readyState(peer)
	state.ObservedAt = time.Now().Add(-2 * time.Second)
	manager.mutex.Lock()
	manager.peers[peer.ID] = PeerStatus{State: state, Healthy: true}
	manager.mutex.Unlock()
	if _, err := manager.Route(context.Background(), "key"); !errors.Is(err, ErrNoRoute) {
		t.Fatalf("expected ErrNoRoute, got %v", err)
	}
}

func TestProbeAndForwardEndpoints(t *testing.T) {
	node := Node{ID: "peer-a", Domain: FailureDomain{Cloud: "aws", Region: "west", Zone: "a"}}
	localPublisher := &testPublisher{}
	handler := NewHandler(node, "secret", func(context.Context) State { return readyState(node) }, localPublisher)
	server := httptest.NewServer(handler)
	defer server.Close()

	configured := node
	configured.URL = server.URL
	state, err := probePeer(context.Background(), server.Client(), configured, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if state.Node.ID != node.ID || !state.Ready {
		t.Fatalf("unexpected state: %+v", state)
	}

	event := domain.Event{ID: "evt-1", TenantID: "t", Source: "svc", Type: "metric", Timestamp: time.Now().UTC(), SchemaVersion: "1"}
	if err := forwardToPeer(context.Background(), server.Client(), configured, "secret", ForwardRequest{TargetNodeID: node.ID, RouteKey: "t|svc", Topic: "telemetry.raw", Event: event}); err != nil {
		t.Fatal(err)
	}
	if len(localPublisher.events) != 1 || localPublisher.events[0].ID != event.ID {
		t.Fatalf("forwarded event not delivered: %+v", localPublisher.events)
	}
}

func TestPublisherFallsBackAfterRemoteFailure(t *testing.T) {
	local := Node{ID: "local", Domain: FailureDomain{Cloud: "aws", Region: "west", Zone: "a"}}
	peer := Node{ID: "peer", Domain: FailureDomain{Cloud: "gcp", Region: "central", Zone: "a"}}
	peerPublisher := &testPublisher{}
	peerHandler := NewHandler(peer, "secret", func(context.Context) State { return readyState(peer) }, peerPublisher)
	server := httptest.NewServer(peerHandler)
	peer.URL = server.URL

	localPublisher := &testPublisher{}
	manager, err := NewManager(Config{Local: local, Peers: []Node{peer}, Policy: "global", Token: "secret", Timeout: 200 * time.Millisecond}, func(context.Context) State { return readyState(local) })
	if err != nil {
		t.Fatal(err)
	}
	manager.mutex.Lock()
	manager.peers[peer.ID] = PeerStatus{State: readyState(peer), Healthy: true}
	manager.mutex.Unlock()

	key := ""
	for index := 0; index < 1000; index++ {
		candidateKey := fmt.Sprintf("tenant|source-%d", index)
		route, routeErr := manager.Route(context.Background(), candidateKey)
		if routeErr == nil && route.Selected.ID == peer.ID {
			key = candidateKey
			break
		}
	}
	if key == "" {
		t.Fatal("could not find rendezvous key owned by peer")
	}

	server.Close()
	publisher := NewPublisher(manager, localPublisher)
	event := domain.Event{ID: "evt-failover", TenantID: "tenant", Source: strings.TrimPrefix(key, "tenant|"), Type: "metric", Timestamp: time.Now().UTC(), SchemaVersion: "1"}
	if err := publisher.Publish(context.Background(), "telemetry.raw", event); err != nil {
		t.Fatal(err)
	}
	if len(localPublisher.events) != 1 {
		t.Fatalf("expected local fallback delivery, got %d events", len(localPublisher.events))
	}
	manager.mutex.RLock()
	status := manager.peers[peer.ID]
	manager.mutex.RUnlock()
	if status.Healthy {
		t.Fatal("failed remote peer should be marked unhealthy")
	}
}

func TestHandlerRejectsWrongTarget(t *testing.T) {
	node := Node{ID: "peer-a"}
	handler := NewHandler(node, "secret", func(context.Context) State { return readyState(node) }, &testPublisher{})
	event := domain.Event{Source: "svc", Type: "metric", Timestamp: time.Now().UTC(), SchemaVersion: "1"}
	payload, _ := json.Marshal(ForwardRequest{TargetNodeID: "other", RouteKey: "svc", Topic: "telemetry.raw", Event: event})
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/mesh/forward", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, recorder.Code)
	}
}
