package replication

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

	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

const maxReplicaRequestBytes int64 = 4 << 20

// Handler exposes the authenticated edge-to-edge durable receiver API.
type Handler struct {
	store *Store
	local Node
	token string
}

func NewHandler(store *Store, local Node, token string) *Handler {
	return &Handler{store: store, local: local, token: token}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !handler.authorized(request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/internal/v1/replication/health":
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"status": "ready", "node": handler.local})
	case request.Method == http.MethodPost && request.URL.Path == "/internal/v1/replicate":
		handler.replicate(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/internal/v1/replication/release":
		handler.release(writer, request)
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

func (handler *Handler) replicate(writer http.ResponseWriter, request *http.Request) {
	body := http.MaxBytesReader(writer, request.Body, maxReplicaRequestBytes)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var record wal.Record
	if err := decoder.Decode(&record); err != nil {
		http.Error(writer, "invalid replication record", http.StatusBadRequest)
		return
	}
	if err := ensureEOF(decoder); err != nil {
		http.Error(writer, "invalid replication body", http.StatusBadRequest)
		return
	}
	duplicate, err := handler.store.Put(record)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrReplicaConflict) {
			status = http.StatusConflict
		} else if errors.Is(err, ErrReplicaPressure) {
			status = http.StatusInsufficientStorage
		}
		http.Error(writer, "replication persistence failed", status)
		return
	}
	acknowledgement := Ack{Node: handler.local, EdgeID: record.EdgeID, EdgeSequence: record.EdgeSequence, PayloadSHA256: record.PayloadSHA256, Duplicate: duplicate, DurableAt: time.Now().UTC()}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(acknowledgement)
}

type releaseRequest struct {
	EdgeID          string `json:"origin_edge_id"`
	ReleasedThrough uint64 `json:"released_through"`
}

func (handler *Handler) release(writer http.ResponseWriter, request *http.Request) {
	body := http.MaxBytesReader(writer, request.Body, 64<<10)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var release releaseRequest
	if err := decoder.Decode(&release); err != nil || strings.TrimSpace(release.EdgeID) == "" || release.ReleasedThrough == 0 {
		http.Error(writer, "invalid release checkpoint", http.StatusBadRequest)
		return
	}
	if err := ensureEOF(decoder); err != nil {
		http.Error(writer, "invalid release body", http.StatusBadRequest)
		return
	}
	if err := handler.store.Release(release.EdgeID, release.ReleasedThrough); err != nil {
		http.Error(writer, "replica release failed", http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
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

func replicateToPeer(ctx context.Context, client *http.Client, peer Node, token string, record wal.Record) (Ack, error) {
	endpoint, err := joinEndpoint(peer.URL, "/internal/v1/replicate")
	if err != nil {
		return Ack{}, err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return Ack{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Ack{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return Ack{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return Ack{}, fmt.Errorf("peer %s returned %s: %s", peer.ID, response.Status, strings.TrimSpace(string(payload)))
	}
	var acknowledgement Ack
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&acknowledgement); err != nil {
		return Ack{}, err
	}
	if acknowledgement.Node.ID != peer.ID || acknowledgement.EdgeID != record.EdgeID || acknowledgement.EdgeSequence != record.EdgeSequence || acknowledgement.PayloadSHA256 != record.PayloadSHA256 {
		return Ack{}, errors.New("peer acknowledgement does not match replicated record")
	}
	return acknowledgement, nil
}

func probePeer(ctx context.Context, client *http.Client, peer Node, token string) (Ack, error) {
	endpoint, err := joinEndpoint(peer.URL, "/internal/v1/replication/health")
	if err != nil {
		return Ack{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Ack{}, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return Ack{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Ack{}, fmt.Errorf("peer %s returned %s", peer.ID, response.Status)
	}
	var payload struct {
		Status string `json:"status"`
		Node   Node   `json:"node"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload); err != nil {
		return Ack{}, err
	}
	if payload.Status != "ready" || payload.Node.ID != peer.ID {
		return Ack{}, errors.New("peer health identity mismatch")
	}
	return Ack{Node: payload.Node}, nil
}

func releasePeer(ctx context.Context, client *http.Client, peer Node, token string, edgeID string, through uint64) error {
	endpoint, err := joinEndpoint(peer.URL, "/internal/v1/replication/release")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(releaseRequest{EdgeID: edgeID, ReleasedThrough: through})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
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
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("peer %s release returned %s: %s", peer.ID, response.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}

func joinEndpoint(base, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("replication peer URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}
