package mesh

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const maxForwardRequestBytes int64 = 4 << 20

type ForwardRequest struct {
	TargetNodeID string       `json:"target_node_id"`
	RouteKey     string       `json:"route_key"`
	Topic        string       `json:"topic"`
	Event        domain.Event `json:"event"`
}

// Handler exposes authenticated mesh state and terminal forwarding endpoints.
// Forwarded events terminate at the receiving node's local Kafka publisher and
// are never recursively re-routed through the mesh.
type Handler struct {
	local        Node
	token        string
	state        StateProvider
	localPublish Downstream
}

func NewHandler(local Node, token string, state StateProvider, localPublisher Downstream) *Handler {
	return &Handler{local: local, token: token, state: state, localPublish: localPublisher}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !handler.authorized(request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/internal/v1/mesh/state":
		state := handler.state(request.Context())
		state.Node = handler.local
		state.ObservedAt = time.Now().UTC()
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(state)
	case request.Method == http.MethodPost && request.URL.Path == "/internal/v1/mesh/forward":
		handler.forward(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (handler *Handler) authorized(request *http.Request) bool {
	if handler.token == "" {
		return false
	}
	expected := "Bearer " + handler.token
	provided := request.Header.Get("Authorization")
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func (handler *Handler) forward(writer http.ResponseWriter, request *http.Request) {
	body := http.MaxBytesReader(writer, request.Body, maxForwardRequestBytes)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var forward ForwardRequest
	if err := decoder.Decode(&forward); err != nil {
		http.Error(writer, "invalid mesh forward body", http.StatusBadRequest)
		return
	}
	if err := ensureEOF(decoder); err != nil || strings.TrimSpace(forward.Topic) == "" || strings.TrimSpace(forward.TargetNodeID) == "" || strings.TrimSpace(forward.RouteKey) == "" {
		http.Error(writer, "invalid mesh forward request", http.StatusBadRequest)
		return
	}
	if forward.TargetNodeID != handler.local.ID {
		http.Error(writer, "mesh target identity mismatch", http.StatusConflict)
		return
	}
	if err := forward.Event.Validate(); err != nil {
		http.Error(writer, "invalid forwarded event", http.StatusBadRequest)
		return
	}
	if err := handler.localPublish.Publish(request.Context(), forward.Topic, forward.Event); err != nil {
		http.Error(writer, "mesh terminal delivery unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func probePeer(ctx context.Context, client *http.Client, peer Node, token string) (State, error) {
	endpoint, err := joinEndpoint(peer.URL, "/internal/v1/mesh/state")
	if err != nil {
		return State{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return State{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return State{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return State{}, fmt.Errorf("mesh peer %s returned %s", peer.ID, response.Status)
	}
	var state State
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&state); err != nil {
		return State{}, err
	}
	if state.Node.ID != peer.ID {
		return State{}, errors.New("mesh peer identity mismatch")
	}
	// Configuration is authoritative for the URL, while the peer's advertised
	// failure domain must match to prevent accidental topology drift.
	if state.Node.Domain != peer.Domain {
		return State{}, errors.New("mesh peer failure-domain mismatch")
	}
	state.Node.URL = peer.URL
	return state, nil
}

func forwardToPeer(ctx context.Context, client *http.Client, peer Node, token string, payload ForwardRequest) error {
	endpoint, err := joinEndpoint(peer.URL, "/internal/v1/mesh/forward")
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("mesh peer %s forward returned %s: %s", peer.ID, response.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func joinEndpoint(base, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("mesh peer URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("unexpected trailing JSON")
}
