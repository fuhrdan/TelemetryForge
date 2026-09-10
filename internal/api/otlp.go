package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

type otlpAnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BytesValue  *string  `json:"bytesValue,omitempty"`
	ArrayValue  *struct {
		Values []otlpAnyValue `json:"values,omitempty"`
	} `json:"arrayValue,omitempty"`
	KvlistValue *struct {
		Values []otlpKeyValue `json:"values,omitempty"`
	} `json:"kvlistValue,omitempty"`
}
type otlpKeyValue struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}
type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes,omitempty"`
}
type otlpScope struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}
type otlpLogRequest struct {
	ResourceLogs []struct {
		Resource  otlpResource `json:"resource"`
		ScopeLogs []struct {
			Scope      otlpScope `json:"scope"`
			SchemaURL  string    `json:"schemaUrl,omitempty"`
			LogRecords []struct {
				TimeUnixNano         string         `json:"timeUnixNano,omitempty"`
				ObservedTimeUnixNano string         `json:"observedTimeUnixNano,omitempty"`
				SeverityText         string         `json:"severityText,omitempty"`
				Body                 otlpAnyValue   `json:"body,omitempty"`
				Attributes           []otlpKeyValue `json:"attributes,omitempty"`
				TraceID              string         `json:"traceId,omitempty"`
				SpanID               string         `json:"spanId,omitempty"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}
type otlpMetricRequest struct {
	ResourceMetrics []struct {
		Resource     otlpResource `json:"resource"`
		ScopeMetrics []struct {
			Scope     otlpScope `json:"scope"`
			SchemaURL string    `json:"schemaUrl,omitempty"`
			Metrics   []struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Unit        string `json:"unit,omitempty"`
				Gauge       *struct {
					DataPoints []otlpNumberPoint `json:"dataPoints"`
				} `json:"gauge,omitempty"`
				Sum *struct {
					DataPoints []otlpNumberPoint `json:"dataPoints"`
				} `json:"sum,omitempty"`
			} `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}
type otlpNumberPoint struct {
	TimeUnixNano string         `json:"timeUnixNano,omitempty"`
	Attributes   []otlpKeyValue `json:"attributes,omitempty"`
	AsDouble     *float64       `json:"asDouble,omitempty"`
	AsInt        *string        `json:"asInt,omitempty"`
}

func (server *Server) handleOTLPLogs(writer http.ResponseWriter, request *http.Request) {
	if !isJSONContentType(request.Header.Get("Content-Type")) {
		writeError(writer, http.StatusUnsupportedMediaType, "v1.8 OTLP ingestion supports application/json only")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	var payload otlpLogRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid OTLP logs JSON")
		return
	}
	accepted := 0
	for _, resourceLogs := range payload.ResourceLogs {
		resourceTags := otlpAttributesToTags(resourceLogs.Resource.Attributes)
		for _, scopeLogs := range resourceLogs.ScopeLogs {
			source := otlpSource(resourceTags, scopeLogs.Scope.Name)
			for _, record := range scopeLogs.LogRecords {
				event := domain.Event{Source: source, Type: otlpLogType(record.Attributes), Timestamp: otlpTimestamp(record.TimeUnixNano, record.ObservedTimeUnixNano), Tags: mergeOTLPTags(resourceTags, otlpAttributesToTags(record.Attributes)), SchemaVersion: "otlp-json-v1", SchemaURL: scopeLogs.SchemaURL, CorrelationID: strings.TrimSpace(record.TraceID), Payload: otlpBodyPayload(record.Body)}
				if severity := strings.TrimSpace(record.SeverityText); severity != "" {
					event.Tags["severity"] = severity
				}
				if record.TraceID != "" {
					event.Tags["trace_id"] = record.TraceID
				}
				if record.SpanID != "" {
					event.Tags["span_id"] = record.SpanID
				}
				if err := server.publishOTLPEvent(request, event, server.topics.Raw, "otlp_log"); err != nil {
					writeError(writer, http.StatusServiceUnavailable, "streaming backend unavailable")
					return
				}
				accepted++
			}
		}
	}
	server.logger.Info("OTLP logs accepted", "records", accepted)
	writeJSON(writer, http.StatusOK, map[string]any{})
}

func (server *Server) handleOTLPMetrics(writer http.ResponseWriter, request *http.Request) {
	if !isJSONContentType(request.Header.Get("Content-Type")) {
		writeError(writer, http.StatusUnsupportedMediaType, "v1.8 OTLP ingestion supports application/json only")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	var payload otlpMetricRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid OTLP metrics JSON")
		return
	}
	accepted, rejected := 0, 0
	for _, resourceMetrics := range payload.ResourceMetrics {
		resourceTags := otlpAttributesToTags(resourceMetrics.Resource.Attributes)
		for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
			source := otlpSource(resourceTags, scopeMetrics.Scope.Name)
			for _, metric := range scopeMetrics.Metrics {
				var points []otlpNumberPoint
				if metric.Gauge != nil {
					points = metric.Gauge.DataPoints
				} else if metric.Sum != nil {
					points = metric.Sum.DataPoints
				} else {
					rejected++
					continue
				}
				for _, point := range points {
					value, ok := otlpNumberValue(point)
					if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
						rejected++
						continue
					}
					tags := mergeOTLPTags(resourceTags, otlpAttributesToTags(point.Attributes))
					if metric.Description != "" {
						tags["otel.metric.description"] = metric.Description
					}
					event := domain.Event{Source: source, Type: strings.TrimSpace(metric.Name), Timestamp: otlpTimestamp(point.TimeUnixNano, ""), Tags: tags, Value: &value, Unit: metric.Unit, SchemaVersion: "otlp-json-v1", SchemaURL: scopeMetrics.SchemaURL}
					if event.Type == "" {
						rejected++
						continue
					}
					if err := server.publishOTLPEvent(request, event, server.topics.Metric, "otlp_metric"); err != nil {
						writeError(writer, http.StatusServiceUnavailable, "streaming backend unavailable")
						return
					}
					accepted++
				}
			}
		}
	}
	response := map[string]any{}
	if rejected > 0 {
		response["partialSuccess"] = map[string]any{"rejectedDataPoints": rejected, "errorMessage": "v1.8 accepts numeric gauge and sum OTLP data points"}
	}
	server.logger.Info("OTLP metrics accepted", "data_points", accepted, "rejected", rejected)
	writeJSON(writer, http.StatusOK, response)
}

func (server *Server) publishOTLPEvent(request *http.Request, event domain.Event, topic, kind string) error {
	event.TenantID = security.TenantID(request.Context())
	if event.Tags == nil {
		event.Tags = make(map[string]string)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if err := ensureEventID(&event); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := server.publisher.Publish(request.Context(), topic, event); err != nil {
		if server.observer != nil {
			server.observer.PublishFailure(topic)
		}
		return err
	}
	if server.observer != nil {
		server.observer.Accepted(kind, topic)
	}
	return nil
}
func otlpAttributesToTags(a []otlpKeyValue) map[string]string {
	r := make(map[string]string, len(a))
	for _, x := range a {
		k := strings.TrimSpace(x.Key)
		if k == "" {
			continue
		}
		if v, ok := otlpAnyValueString(x.Value); ok {
			r[k] = v
		}
	}
	return r
}
func mergeOTLPTags(a, b map[string]string) map[string]string {
	r := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		r[k] = v
	}
	for k, v := range b {
		r[k] = v
	}
	return r
}
func otlpSource(tags map[string]string, scope string) string {
	if v := strings.TrimSpace(tags["service.name"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(scope); v != "" {
		return v
	}
	return "otlp"
}
func otlpLogType(a []otlpKeyValue) string {
	tags := otlpAttributesToTags(a)
	for _, k := range []string{"event.name", "event.type", "log.type"} {
		if v := strings.TrimSpace(tags[k]); v != "" {
			return v
		}
	}
	return "otel.log"
}
func otlpTimestamp(primary, fallback string) time.Time {
	for _, raw := range []string{primary, fallback} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			return time.Unix(0, v).UTC()
		}
	}
	return time.Now().UTC()
}
func otlpNumberValue(p otlpNumberPoint) (float64, bool) {
	if p.AsDouble != nil {
		return *p.AsDouble, true
	}
	if p.AsInt != nil {
		v, err := strconv.ParseInt(strings.TrimSpace(*p.AsInt), 10, 64)
		if err == nil {
			return float64(v), true
		}
	}
	return 0, false
}
func otlpAnyValueNative(v otlpAnyValue) (any, bool) {
	switch {
	case v.StringValue != nil:
		return *v.StringValue, true
	case v.BoolValue != nil:
		return *v.BoolValue, true
	case v.IntValue != nil:
		return *v.IntValue, true
	case v.DoubleValue != nil:
		return *v.DoubleValue, true
	case v.BytesValue != nil:
		return map[string]string{"base64": *v.BytesValue}, true
	case v.ArrayValue != nil:
		items := make([]any, 0, len(v.ArrayValue.Values))
		for _, item := range v.ArrayValue.Values {
			if n, ok := otlpAnyValueNative(item); ok {
				items = append(items, n)
			}
		}
		return items, true
	case v.KvlistValue != nil:
		items := map[string]any{}
		for _, item := range v.KvlistValue.Values {
			if n, ok := otlpAnyValueNative(item.Value); ok && strings.TrimSpace(item.Key) != "" {
				items[item.Key] = n
			}
		}
		return items, true
	}
	return nil, false
}
func otlpAnyValueString(v otlpAnyValue) (string, bool) {
	n, ok := otlpAnyValueNative(v)
	if !ok {
		return "", false
	}
	if s, ok := n.(string); ok {
		return s, true
	}
	p, err := json.Marshal(n)
	if err != nil {
		return "", false
	}
	return string(p), true
}
func otlpBodyPayload(v otlpAnyValue) json.RawMessage {
	n, ok := otlpAnyValueNative(v)
	if !ok {
		return json.RawMessage(`{}`)
	}
	p, err := json.Marshal(map[string]any{"body": n})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return p
}
func isJSONContentType(v string) bool {
	return strings.ToLower(strings.TrimSpace(strings.Split(v, ";")[0])) == "application/json"
}
