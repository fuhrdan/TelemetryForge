package incidentarchive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
)

func sampleBundle() Bundle {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	return Bundle{
		Incident: IncidentMetadata{
			ID: "INC-42", Title: "checkout regression", Status: "frozen",
			FrozenFrom: now.Add(-time.Minute), FrozenTo: now.Add(time.Minute),
			DetectedAt: now,
		},
		Events: []EventRecord{{
			CapturedAt: now,
			Event: domain.Event{
				ID: "evt-1", TenantID: "alpha", Source: "checkout",
				Type: "request.error", Timestamp: now, SchemaVersion: "1.0",
				Tags: map[string]string{"severity": "error"},
			},
		}},
		EvidenceGraph: evidence.Graph{
			TenantID: "alpha", IncidentID: "INC-42", GeneratedAt: now,
			Disclaimer: "evidence only",
		},
		Configurations: []ConfigurationSnapshot{{
			Kind: "policy", Role: "active", Name: "default", Version: "1",
			Payload: json.RawMessage(`{"name":"default","version":"1"}`),
		}},
	}
}

func TestRoundTripUnencryptedArchive(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "incident.tfincident")
	manifest, err := WriteFile(filename, sampleBundle(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.EventCount != 1 {
		t.Fatalf("event_count=%d", manifest.EventCount)
	}

	bundle, err := ReadFile(filename, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Incident.ID != "INC-42" || len(bundle.Events) != 1 {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
	if bundle.Events[0].Event.TenantID != "alpha" {
		t.Fatalf("tenant=%q", bundle.Events[0].Event.TenantID)
	}
}

func TestEncryptedArchiveRejectsWrongKeyAndTampering(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	filename := filepath.Join(t.TempDir(), "incident.tfincident.enc")
	if _, err := WriteFile(filename, sampleBundle(), key); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadFile(filename, bytes.Repeat([]byte{0x22}, 32)); err == nil {
		t.Fatal("wrong key should fail")
	}

	payload, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	payload[len(payload)-1] ^= 0xff
	if err := os.WriteFile(filename, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(filename, key); err == nil {
		t.Fatal("tampered ciphertext should fail authentication")
	}
}

func TestManifestDetectsModifiedMember(t *testing.T) {
	bundle := sampleBundle()
	filename := filepath.Join(t.TempDir(), "incident.tfincident")
	if _, err := WriteFile(filename, bundle, nil); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt bytes.Buffer
	writer := zip.NewWriter(&rebuilt)
	for _, member := range reader.File {
		handle, _ := member.Open()
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(handle)
		_ = handle.Close()
		if member.Name == "incident/metadata.json" {
			data = bytes.NewBuffer([]byte(`{"id":"evil"}`))
		}
		target, _ := writer.Create(member.Name)
		_, _ = target.Write(data.Bytes())
	}
	_ = writer.Close()

	if _, err := Read(rebuilt.Bytes()); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestUnsafeZIPMemberIsRejected(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, _ := writer.Create("../escape")
	_, _ = entry.Write([]byte("bad"))
	manifest, _ := writer.Create("manifest.json")
	_, _ = manifest.Write([]byte(`{}`))
	_ = writer.Close()

	if _, err := Read(buffer.Bytes()); err == nil {
		t.Fatal("unsafe member path should fail")
	}
}

func TestArchiveRequiresConsistentTenant(t *testing.T) {
	bundle := sampleBundle()
	bundle.Events = append(bundle.Events, EventRecord{
		CapturedAt: time.Now().UTC(),
		Event: domain.Event{
			ID: "evt-2", TenantID: "beta", Source: "checkout",
			Type: "request", Timestamp: time.Now().UTC(), SchemaVersion: "1.0",
		},
	})
	if _, err := WriteFile(filepath.Join(t.TempDir(), "bad.tfincident"), bundle, nil); err == nil {
		t.Fatal("cross-tenant bundle should be rejected")
	}
}

func TestOfflineHTMLReportEscapesTelemetryContent(t *testing.T) {
	bundle := sampleBundle()
	bundle.Incident.Title = `<script>alert("x")</script>`
	bundle.Events[0].Event.Source = `<img src=x onerror=alert(1)>`

	filename := filepath.Join(t.TempDir(), "report.html")
	if err := WriteHTMLReport(filename, bundle); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Contains(text, `<script>alert("x")</script>`) ||
		strings.Contains(text, `<img src=x onerror=alert(1)>`) {
		t.Fatal("offline report rendered unescaped telemetry-controlled HTML")
	}
	if !strings.Contains(text, "&lt;script&gt;") {
		t.Fatal("expected incident title to be HTML-escaped")
	}
}
