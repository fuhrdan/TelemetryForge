// Package schema derives bounded, tenant-aware telemetry schemas from the
// canonical TelemetryForge event envelope.
//
// Schema Intelligence intentionally observes telemetry after normalization but
// before policy mutation. That lets operators see what applications actually
// emitted even when a later Cardinality Firewall rule drops a dangerous tag.
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const (
	// MaxPayloadDepth bounds recursive JSON object discovery per event.
	MaxPayloadDepth = 5
	// MaxEventFields bounds the number of paths derived from one event.
	MaxEventFields = 512
	// MaxRegistryFields bounds accumulated unique paths for one declared schema.
	MaxRegistryFields = 2048
)

// ValueType is the normalized JSON/schema type tracked by Schema Intelligence.
type ValueType string

const (
	TypeString ValueType = "string"
	TypeNumber ValueType = "number"
	TypeBool   ValueType = "boolean"
	TypeObject ValueType = "object"
	TypeArray  ValueType = "array"
	TypeNull   ValueType = "null"
)

// Field describes one observed path in an event schema.
type Field struct {
	Path                 string    `json:"path"`
	Type                 ValueType `json:"type"`
	SemanticKey          string    `json:"semantic_key,omitempty"`
	SemanticStability    string    `json:"semantic_stability,omitempty"`
	SemanticRequirement  string    `json:"semantic_requirement,omitempty"`
	SemanticReplacement  string    `json:"semantic_replacement,omitempty"`
	SemanticExpectedType ValueType `json:"semantic_expected_type,omitempty"`
}

// SemanticFinding records an OpenTelemetry semantic-convention observation.
type SemanticFinding struct {
	Key         string `json:"key"`
	Path        string `json:"path"`
	Severity    string `json:"severity"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
	Replacement string `json:"replacement,omitempty"`
}

// Description is the deterministic schema derived from one canonical event.
type Description struct {
	TenantID        string            `json:"tenant_id"`
	Source          string            `json:"source"`
	EventType       string            `json:"event_type"`
	DeclaredVersion string            `json:"declared_version"`
	SchemaURL       string            `json:"schema_url,omitempty"`
	Fingerprint     string            `json:"fingerprint"`
	Fields          []Field           `json:"fields"`
	Semantic        []SemanticFinding `json:"semantic_findings,omitempty"`
	Truncated       bool              `json:"truncated"`
}

// FieldState is the accumulated registry state for one path.
type FieldState struct {
	Field
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	SeenCount     int64     `json:"seen_count"`
	ConflictCount int64     `json:"conflict_count"`
	Required      bool      `json:"required"`
}

// RegistryEntry is one source/type/declared-version schema history record.
type RegistryEntry struct {
	TenantID           string            `json:"tenant_id"`
	Source             string            `json:"source"`
	EventType          string            `json:"event_type"`
	DeclaredVersion    string            `json:"declared_version"`
	SchemaURL          string            `json:"schema_url,omitempty"`
	Fingerprint        string            `json:"fingerprint"`
	Health             string            `json:"health"`
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
	ObservationCount   int64             `json:"observation_count"`
	FieldCount         int               `json:"field_count"`
	RequiredFieldCount int               `json:"required_field_count"`
	Fields             []FieldState      `json:"fields,omitempty"`
	Semantic           []SemanticFinding `json:"semantic_findings,omitempty"`
}

// Drift is a deduplicated schema change/warning tracked over time.
type Drift struct {
	TenantID        string    `json:"tenant_id"`
	Source          string    `json:"source"`
	EventType       string    `json:"event_type"`
	DeclaredVersion string    `json:"declared_version"`
	Signature       string    `json:"signature"`
	Severity        string    `json:"severity"`
	Kind            string    `json:"kind"`
	Path            string    `json:"path,omitempty"`
	PreviousType    ValueType `json:"previous_type,omitempty"`
	CurrentType     ValueType `json:"current_type,omitempty"`
	Message         string    `json:"message"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Occurrences     int64     `json:"occurrences"`
}

// VersionDiff compares two explicitly stored schema versions.
type VersionDiff struct {
	Source        string   `json:"source"`
	EventType     string   `json:"event_type"`
	FromVersion   string   `json:"from_version"`
	ToVersion     string   `json:"to_version"`
	Compatibility string   `json:"compatibility"`
	Changes       []Change `json:"changes"`
}

// Change is one field-level difference between schema versions.
type Change struct {
	Kind         string    `json:"kind"`
	Severity     string    `json:"severity"`
	Path         string    `json:"path"`
	PreviousType ValueType `json:"previous_type,omitempty"`
	CurrentType  ValueType `json:"current_type,omitempty"`
	Message      string    `json:"message"`
}

// Describe derives a deterministic bounded schema from one event.
func Describe(event domain.Event) Description {
	fields := make([]Field, 0, 32)
	semantic := make([]SemanticFinding, 0, 4)
	truncated := false

	appendField := func(field Field) bool {
		if len(fields) >= MaxEventFields {
			truncated = true
			return false
		}
		fields = append(fields, field)
		return true
	}

	if event.Value != nil {
		appendField(Field{Path: "metric.value", Type: TypeNumber})
	}
	if strings.TrimSpace(event.Unit) != "" {
		appendField(Field{Path: "metric.unit", Type: TypeString})
	}

	tagKeys := make([]string, 0, len(event.Tags))
	for key := range event.Tags {
		tagKeys = append(tagKeys, key)
	}
	sort.Strings(tagKeys)
	for _, key := range tagKeys {
		field, finding := semanticField("tags."+key, key, TypeString)
		if !appendField(field) {
			break
		}
		if finding != nil {
			semantic = append(semantic, *finding)
		}
	}

	if len(event.Payload) > 0 && len(fields) < MaxEventFields {
		var payload any
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			walkPayload("payload", payload, 0, &fields, &semantic, &truncated)
		} else {
			appendField(Field{Path: "payload", Type: TypeString})
		}
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	sort.Slice(semantic, func(i, j int) bool {
		if semantic[i].Path == semantic[j].Path {
			return semantic[i].Kind < semantic[j].Kind
		}
		return semantic[i].Path < semantic[j].Path
	})

	description := Description{
		TenantID:        event.TenantID,
		Source:          strings.TrimSpace(event.Source),
		EventType:       strings.TrimSpace(event.Type),
		DeclaredVersion: strings.TrimSpace(event.SchemaVersion),
		SchemaURL:       strings.TrimSpace(event.SchemaURL),
		Fields:          fields,
		Semantic:        semantic,
		Truncated:       truncated,
	}
	description.Fingerprint = Fingerprint(fields)
	return description
}

func walkPayload(
	path string,
	value any,
	depth int,
	fields *[]Field,
	semantic *[]SemanticFinding,
	truncated *bool,
) {
	if len(*fields) >= MaxEventFields {
		*truncated = true
		return
	}
	if depth > MaxPayloadDepth {
		*truncated = true
		return
	}

	typeOf := jsonType(value)
	field := Field{Path: path, Type: typeOf}
	if key := semanticCandidate(path); key != "" {
		field, finding := semanticField(path, key, typeOf)
		if finding != nil {
			*semantic = append(*semantic, *finding)
		}
		*fields = append(*fields, field)
	} else {
		*fields = append(*fields, field)
	}

	if len(*fields) >= MaxEventFields {
		*truncated = true
		return
	}

	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		walkPayload(path+"."+key, object[key], depth+1, fields, semantic, truncated)
		if *truncated && len(*fields) >= MaxEventFields {
			return
		}
	}
}

func jsonType(value any) ValueType {
	switch value.(type) {
	case nil:
		return TypeNull
	case string:
		return TypeString
	case float64:
		return TypeNumber
	case bool:
		return TypeBool
	case []any:
		return TypeArray
	case map[string]any:
		return TypeObject
	default:
		return TypeString
	}
}

// Fingerprint hashes field paths and types only. Values never enter the hash.
func Fingerprint(fields []Field) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field.Path+":"+string(field.Type))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// Diff compares two accumulated registry versions.
func Diff(from, to RegistryEntry) VersionDiff {
	fromFields := make(map[string]FieldState, len(from.Fields))
	toFields := make(map[string]FieldState, len(to.Fields))
	for _, field := range from.Fields {
		fromFields[field.Path] = field
	}
	for _, field := range to.Fields {
		toFields[field.Path] = field
	}

	changes := make([]Change, 0)
	for path, previous := range fromFields {
		current, exists := toFields[path]
		if !exists {
			severity := "warning"
			if previous.Required {
				severity = "breaking"
			}
			changes = append(changes, Change{
				Kind: "field_removed", Severity: severity, Path: path,
				PreviousType: previous.Type,
				Message:      fmt.Sprintf("field %s was removed", path),
			})
			continue
		}
		if previous.Type != current.Type {
			changes = append(changes, Change{
				Kind: "type_changed", Severity: "breaking", Path: path,
				PreviousType: previous.Type, CurrentType: current.Type,
				Message: fmt.Sprintf("field %s changed type from %s to %s", path, previous.Type, current.Type),
			})
		}
	}
	for path, current := range toFields {
		if _, exists := fromFields[path]; exists {
			continue
		}
		changes = append(changes, Change{
			Kind: "field_added", Severity: "info", Path: path,
			CurrentType: current.Type,
			Message:     fmt.Sprintf("field %s was added", path),
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Severity == changes[j].Severity {
			return changes[i].Path < changes[j].Path
		}
		return severityRank(changes[i].Severity) > severityRank(changes[j].Severity)
	})

	compatibility := "compatible"
	for _, change := range changes {
		if change.Severity == "breaking" {
			compatibility = "breaking"
			break
		}
		if change.Severity == "warning" && compatibility == "compatible" {
			compatibility = "review"
		}
	}
	return VersionDiff{
		Source: from.Source, EventType: from.EventType,
		FromVersion: from.DeclaredVersion, ToVersion: to.DeclaredVersion,
		Compatibility: compatibility, Changes: changes,
	}
}

func severityRank(value string) int {
	switch value {
	case "breaking":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func semanticCandidate(path string) string {
	return strings.TrimPrefix(path, "payload.")
}
