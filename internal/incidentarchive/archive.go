package incidentarchive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const manifestPath = "manifest.json"

// WriteFile creates one verified .tfincident archive.
//
// key is optional. When it contains 32 bytes, the complete ZIP is wrapped in
// AES-256-GCM before being written.
func WriteFile(filename string, bundle Bundle, key []byte) (Manifest, error) {
	if err := validateBundle(bundle); err != nil {
		return Manifest{}, err
	}

	entries, configurationMeta, err := buildEntries(bundle)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Format:          FormatName,
		FormatVersion:   FormatVersion,
		TelemetryForge:  ProductVersion,
		ArchiveID:       newArchiveID(),
		CreatedAt:       time.Now().UTC(),
		TenantID:        bundle.IncidentTenant(),
		IncidentID:      bundle.Incident.ID,
		EventCount:      len(bundle.Events),
		SchemaCount:     len(bundle.Schemas),
		ReplayRunCount:  len(bundle.ReplayRuns),
		CostResultCount: len(bundle.CostResults),
		Configurations:  configurationMeta,
		Entries:         make(map[string]EntryDigest, len(entries)),
	}

	names := make([]string, 0, len(entries))
	for name, payload := range entries {
		sum := sha256.Sum256(payload)
		manifest.Entries[name] = EntryDigest{
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(payload)),
		}
		names = append(names, name)
	}
	sort.Strings(names)

	manifestPayload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode archive manifest: %w", err)
	}
	manifestPayload = append(manifestPayload, '\n')

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	if err := writeZIPMember(writer, manifestPath, manifestPayload); err != nil {
		return Manifest{}, err
	}
	for _, name := range names {
		if err := writeZIPMember(writer, name, entries[name]); err != nil {
			return Manifest{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Manifest{}, fmt.Errorf("close incident archive: %w", err)
	}
	if buffer.Len() > MaxArchiveBytes {
		return Manifest{}, fmt.Errorf("archive exceeds %d-byte safety limit", MaxArchiveBytes)
	}

	payload := buffer.Bytes()
	if len(key) != 0 {
		payload, err = Encrypt(payload, key)
		if err != nil {
			return Manifest{}, err
		}
	}

	if err := os.WriteFile(filename, payload, 0600); err != nil {
		return Manifest{}, fmt.Errorf("write incident archive: %w", err)
	}
	return manifest, nil
}

// ReadFile opens, decrypts when required, verifies, and parses a .tfincident.
func ReadFile(filename string, key []byte) (Bundle, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return Bundle{}, fmt.Errorf("read incident archive: %w", err)
	}
	if len(payload) > MaxArchiveBytes+(1<<20) {
		return Bundle{}, fmt.Errorf("archive exceeds %d-byte safety limit", MaxArchiveBytes)
	}

	if IsEncrypted(payload) {
		if len(key) == 0 {
			return Bundle{}, errors.New("archive is encrypted; a decryption key is required")
		}
		payload, err = Decrypt(payload, key)
		if err != nil {
			return Bundle{}, err
		}
	}
	return Read(payload)
}

// Read verifies and parses unencrypted ZIP bytes.
func Read(payload []byte) (Bundle, error) {
	if len(payload) > MaxArchiveBytes {
		return Bundle{}, fmt.Errorf("archive exceeds %d-byte safety limit", MaxArchiveBytes)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return Bundle{}, fmt.Errorf("open ZIP incident archive: %w", err)
	}
	if len(reader.File) < 2 || len(reader.File) > MaxEntries {
		return Bundle{}, fmt.Errorf("archive contains %d entries; expected 2..%d", len(reader.File), MaxEntries)
	}

	members := make(map[string][]byte, len(reader.File))
	var total uint64
	for _, file := range reader.File {
		if !safeMemberName(file.Name) {
			return Bundle{}, fmt.Errorf("unsafe archive member path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			return Bundle{}, fmt.Errorf("archive directories are not permitted: %q", file.Name)
		}
		if file.UncompressedSize64 > MaxMemberBytes {
			return Bundle{}, fmt.Errorf("archive member %q exceeds %d-byte safety limit", file.Name, MaxMemberBytes)
		}
		total += file.UncompressedSize64
		if total > MaxArchiveBytes {
			return Bundle{}, fmt.Errorf("archive uncompressed content exceeds %d-byte safety limit", MaxArchiveBytes)
		}
		if _, exists := members[file.Name]; exists {
			return Bundle{}, fmt.Errorf("duplicate archive member %q", file.Name)
		}

		handle, err := file.Open()
		if err != nil {
			return Bundle{}, fmt.Errorf("open archive member %q: %w", file.Name, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(handle, MaxMemberBytes+1))
		closeErr := handle.Close()
		if readErr != nil {
			return Bundle{}, fmt.Errorf("read archive member %q: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return Bundle{}, fmt.Errorf("close archive member %q: %w", file.Name, closeErr)
		}
		if len(data) > MaxMemberBytes {
			return Bundle{}, fmt.Errorf("archive member %q exceeds safety limit", file.Name)
		}
		members[file.Name] = data
	}

	manifestPayload, exists := members[manifestPath]
	if !exists {
		return Bundle{}, errors.New("manifest.json is missing")
	}

	var manifest Manifest
	if err := decodeStrict(manifestPayload, &manifest); err != nil {
		return Bundle{}, fmt.Errorf("decode archive manifest: %w", err)
	}
	if err := validateManifest(manifest, members); err != nil {
		return Bundle{}, err
	}

	bundle := Bundle{Manifest: manifest}
	if err := decodeStrict(members["incident/metadata.json"], &bundle.Incident); err != nil {
		return Bundle{}, fmt.Errorf("decode incident metadata: %w", err)
	}

	events, err := decodeEvents(members["incident/events.jsonl"])
	if err != nil {
		return Bundle{}, err
	}
	bundle.Events = events

	if data := members["evidence/graph.json"]; len(data) > 0 {
		if err := decodeStrict(data, &bundle.EvidenceGraph); err != nil {
			return Bundle{}, fmt.Errorf("decode Evidence Graph: %w", err)
		}
	}
	if data := members["analysis/replay-runs.json"]; len(data) > 0 {
		if err := decodeStrict(data, &bundle.ReplayRuns); err != nil {
			return Bundle{}, fmt.Errorf("decode replay history: %w", err)
		}
	}
	if data := members["analysis/cost-simulations.json"]; len(data) > 0 {
		if err := decodeStrict(data, &bundle.CostResults); err != nil {
			return Bundle{}, fmt.Errorf("decode cost simulations: %w", err)
		}
	}
	if data := members["schema/registry.json"]; len(data) > 0 {
		if err := decodeStrict(data, &bundle.Schemas); err != nil {
			return Bundle{}, fmt.Errorf("decode schema registry snapshot: %w", err)
		}
	}
	if data := members["schema/drift.json"]; len(data) > 0 {
		if err := decodeStrict(data, &bundle.SchemaDrifts); err != nil {
			return Bundle{}, fmt.Errorf("decode schema drift snapshot: %w", err)
		}
	}

	if manifest.IncidentID != bundle.Incident.ID {
		return Bundle{}, errors.New("manifest incident ID does not match incident metadata")
	}
	if manifest.EventCount != len(bundle.Events) {
		return Bundle{}, errors.New("manifest event count does not match incident events")
	}
	if manifest.SchemaCount != len(bundle.Schemas) {
		return Bundle{}, errors.New("manifest schema count does not match schema snapshot")
	}
	if manifest.ReplayRunCount != len(bundle.ReplayRuns) {
		return Bundle{}, errors.New("manifest replay count does not match replay history")
	}
	if manifest.CostResultCount != len(bundle.CostResults) {
		return Bundle{}, errors.New("manifest cost count does not match cost history")
	}

	for _, meta := range manifest.Configurations {
		data, ok := members[meta.Path]
		if !ok {
			return Bundle{}, fmt.Errorf("configuration member %q is missing", meta.Path)
		}
		if !json.Valid(data) {
			return Bundle{}, fmt.Errorf("configuration member %q is not valid JSON", meta.Path)
		}
		bundle.Configurations = append(bundle.Configurations, ConfigurationSnapshot{
			Kind: meta.Kind, Role: meta.Role, Name: meta.Name,
			Version: meta.Version, Payload: append(json.RawMessage(nil), data...),
		})
	}

	if err := validateBundle(bundle); err != nil {
		return Bundle{}, fmt.Errorf("archive content validation failed: %w", err)
	}
	return bundle, nil
}

// IncidentTenant returns the canonical tenant represented by the archive.
func (bundle Bundle) IncidentTenant() string {
	if bundle.Manifest.TenantID != "" {
		return bundle.Manifest.TenantID
	}
	for _, record := range bundle.Events {
		if record.Event.TenantID != "" {
			return record.Event.TenantID
		}
	}
	if bundle.EvidenceGraph.TenantID != "" {
		return bundle.EvidenceGraph.TenantID
	}
	return "default"
}

func buildEntries(bundle Bundle) (map[string][]byte, []ConfigurationMeta, error) {
	entries := make(map[string][]byte)

	readme := `TelemetryForge Incident Archive (.tfincident)
Format version: 1

This archive is a portable incident evidence package. The source of truth is
manifest.json plus SHA-256 checksums for every other member.

Important:
- supporting evidence is not automatic proof of causality;
- payloads can contain sensitive production telemetry;
- verify the archive before importing or sharing it;
- encrypted archives are an outer AES-256-GCM envelope around this ZIP.
`
	entries["README.txt"] = []byte(readme)

	metadata, err := marshalPretty(bundle.Incident)
	if err != nil {
		return nil, nil, err
	}
	entries["incident/metadata.json"] = metadata

	eventPayload, err := encodeEvents(bundle.Events)
	if err != nil {
		return nil, nil, err
	}
	entries["incident/events.jsonl"] = eventPayload

	if bundle.EvidenceGraph.IncidentID != "" {
		payload, err := marshalPretty(bundle.EvidenceGraph)
		if err != nil {
			return nil, nil, err
		}
		entries["evidence/graph.json"] = payload
	}
	if len(bundle.ReplayRuns) > 0 {
		payload, err := marshalPretty(bundle.ReplayRuns)
		if err != nil {
			return nil, nil, err
		}
		entries["analysis/replay-runs.json"] = payload
	}
	if len(bundle.CostResults) > 0 {
		payload, err := marshalPretty(bundle.CostResults)
		if err != nil {
			return nil, nil, err
		}
		entries["analysis/cost-simulations.json"] = payload
	}
	if len(bundle.Schemas) > 0 {
		payload, err := marshalPretty(bundle.Schemas)
		if err != nil {
			return nil, nil, err
		}
		entries["schema/registry.json"] = payload
	}
	if len(bundle.SchemaDrifts) > 0 {
		payload, err := marshalPretty(bundle.SchemaDrifts)
		if err != nil {
			return nil, nil, err
		}
		entries["schema/drift.json"] = payload
	}

	meta := make([]ConfigurationMeta, 0, len(bundle.Configurations))
	seen := make(map[string]struct{})
	for _, snapshot := range bundle.Configurations {
		if !validConfigurationKind(snapshot.Kind) || !validConfigurationRole(snapshot.Role) {
			return nil, nil, fmt.Errorf("unsupported configuration snapshot %q/%q", snapshot.Kind, snapshot.Role)
		}
		if !json.Valid(snapshot.Payload) {
			return nil, nil, fmt.Errorf("configuration snapshot %s/%s is not valid JSON", snapshot.Kind, snapshot.Role)
		}
		member := fmt.Sprintf("configuration/%s-%s.json", snapshot.Kind, snapshot.Role)
		if _, exists := seen[member]; exists {
			return nil, nil, fmt.Errorf("duplicate configuration snapshot %s/%s", snapshot.Kind, snapshot.Role)
		}
		seen[member] = struct{}{}
		entries[member] = append([]byte(nil), snapshot.Payload...)
		meta = append(meta, ConfigurationMeta{
			Kind: snapshot.Kind, Role: snapshot.Role, Name: snapshot.Name,
			Version: snapshot.Version, Path: member,
		})
	}
	sort.Slice(meta, func(i, j int) bool { return meta[i].Path < meta[j].Path })
	return entries, meta, nil
}

func validateBundle(bundle Bundle) error {
	if strings.TrimSpace(bundle.Incident.ID) == "" {
		return errors.New("incident ID is required")
	}
	if bundle.Incident.FrozenFrom.IsZero() || bundle.Incident.FrozenTo.IsZero() ||
		bundle.Incident.FrozenTo.Before(bundle.Incident.FrozenFrom) {
		return errors.New("incident frozen window is invalid")
	}
	if len(bundle.Events) == 0 {
		return errors.New("incident archive must contain at least one event")
	}
	if len(bundle.Events) > MaxEvents {
		return fmt.Errorf("incident contains %d events; maximum is %d", len(bundle.Events), MaxEvents)
	}

	tenant := bundle.IncidentTenant()
	for _, record := range bundle.Events {
		if record.Event.ID == "" {
			return errors.New("incident event ID is required")
		}
		if record.Event.TenantID != "" && record.Event.TenantID != tenant {
			return fmt.Errorf("event %q belongs to tenant %q, archive tenant is %q",
				record.Event.ID, record.Event.TenantID, tenant)
		}
	}
	if bundle.EvidenceGraph.IncidentID != "" &&
		bundle.EvidenceGraph.IncidentID != bundle.Incident.ID {
		return errors.New("Evidence Graph incident ID does not match archive incident")
	}
	return nil
}

func validateManifest(manifest Manifest, members map[string][]byte) error {
	if manifest.Format != FormatName {
		return fmt.Errorf("unsupported archive format %q", manifest.Format)
	}
	if manifest.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported archive format version %d", manifest.FormatVersion)
	}
	if manifest.ArchiveID == "" || manifest.IncidentID == "" || manifest.TenantID == "" {
		return errors.New("archive manifest is missing identity fields")
	}
	if len(manifest.Entries) != len(members)-1 {
		return errors.New("archive member count does not match manifest")
	}

	for name, data := range members {
		if name == manifestPath {
			continue
		}
		expected, exists := manifest.Entries[name]
		if !exists {
			return fmt.Errorf("archive contains unlisted member %q", name)
		}
		if expected.Bytes != int64(len(data)) {
			return fmt.Errorf("archive member %q byte length mismatch", name)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(expected.SHA256, hex.EncodeToString(sum[:])) {
			return fmt.Errorf("archive member %q SHA-256 mismatch", name)
		}
	}
	for name := range manifest.Entries {
		if _, exists := members[name]; !exists {
			return fmt.Errorf("manifest lists missing member %q", name)
		}
	}
	return nil
}

func encodeEvents(records []EventRecord) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return nil, fmt.Errorf("encode incident event %q: %w", record.Event.ID, err)
		}
	}
	return buffer.Bytes(), nil
}

func decodeEvents(payload []byte) ([]EventRecord, error) {
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	records := make([]EventRecord, 0, 256)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		if len(records) >= MaxEvents {
			return nil, fmt.Errorf("incident events exceed %d-record safety limit", MaxEvents)
		}
		var record EventRecord
		if err := decodeStrict(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode incident event line %d: %w", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan incident events: %w", err)
	}
	return records, nil
}

func writeZIPMember(writer *zip.Writer, name string, payload []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetModTime(time.Unix(0, 0).UTC())
	header.SetMode(0600)
	member, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create archive member %q: %w", name, err)
	}
	if _, err := member.Write(payload); err != nil {
		return fmt.Errorf("write archive member %q: %w", name, err)
	}
	return nil
}

func safeMemberName(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return false
	}
	clean := path.Clean(name)
	return clean == name && clean != "." && !strings.HasPrefix(clean, "../")
}

func marshalPretty(value any) ([]byte, error) {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func decodeStrict(payload []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}

func newArchiveID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func validConfigurationKind(value string) bool {
	switch value {
	case "policy", "shaping", "routing":
		return true
	default:
		return false
	}
}

func validConfigurationRole(value string) bool {
	return value == "active" || value == "shadow"
}
