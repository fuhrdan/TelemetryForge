package connectors

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
)

type prometheusRemoteWriteConnector struct {
	spec   Spec
	client *http.Client
}

func newPrometheusRemoteWriteConnector(spec Spec, f FactoryConfig) (Connector, error) {
	if err := requireSecretEnvironment(spec); err != nil {
		return nil, err
	}
	return &prometheusRemoteWriteConnector{spec: spec, client: clientFor(spec, f)}, nil
}
func (c *prometheusRemoteWriteConnector) Kind() string { return KindPrometheusRemoteWrite }
func (c *prometheusRemoteWriteConnector) Capabilities() Capabilities {
	return capability(KindPrometheusRemoteWrite)
}
func (c *prometheusRemoteWriteConnector) Send(ctx context.Context, e domain.Event) error {
	if e.Value == nil {
		return &DeliveryError{Err: errors.New("Prometheus remote write accepts only numeric canonical events"), Retryable: false}
	}
	payload := encodePrometheusWriteRequest(e, c.spec.PrometheusLabels)
	compressed := snappyLiteralBlock(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.spec.Endpoint, bytes.NewReader(compressed))
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("User-Agent", "TelemetryForge/"+ProductVersion)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("Prometheus remote-write request: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func (c *prometheusRemoteWriteConnector) Ready(ctx context.Context) error {
	return readinessGET(ctx, c.client, c.spec)
}
func (c *prometheusRemoteWriteConnector) Close() {}
func encodePrometheusWriteRequest(e domain.Event, allowed []string) []byte {
	labels := map[string]string{"__name__": sanitizeMetricName(e.Type), "source": e.Source, "tenant_id": e.TenantID}
	allow := map[string]struct{}{}
	for _, k := range allowed {
		allow[k] = struct{}{}
	}
	for k, v := range e.Tags {
		if _, ok := allow[k]; ok {
			labels[sanitizeLabelName(k)] = v
		}
	}
	names := make([]string, 0, len(labels))
	for n := range labels {
		names = append(names, n)
	}
	sort.Strings(names)
	var ts []byte
	for _, n := range names {
		var l []byte
		l = appendProtoString(l, 1, n)
		l = appendProtoString(l, 2, labels[n])
		ts = appendProtoBytes(ts, 1, l)
	}
	var sample []byte
	sample = appendProtoFixed64(sample, 1, math.Float64bits(*e.Value))
	sample = appendProtoVarint(sample, 2, uint64(e.Timestamp.UTC().UnixMilli()))
	ts = appendProtoBytes(ts, 2, sample)
	return appendProtoBytes(nil, 1, ts)
}
func snappyLiteralBlock(p []byte) []byte {
	r := appendUvarint(nil, uint64(len(p)))
	if len(p) == 0 {
		return r
	}
	n := len(p) - 1
	switch {
	case len(p) < 61:
		r = append(r, byte(n<<2))
	case n <= 0xff:
		r = append(r, 60<<2, byte(n))
	case n <= 0xffff:
		r = append(r, 61<<2, byte(n), byte(n>>8))
	case uint64(n) <= 0xffffff:
		r = append(r, 62<<2, byte(n), byte(n>>8), byte(n>>16))
	default:
		r = append(r, 63<<2, byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
	}
	return append(r, p...)
}
func appendProtoString(d []byte, f int, v string) []byte { return appendProtoBytes(d, f, []byte(v)) }
func appendProtoBytes(d []byte, f int, v []byte) []byte {
	d = appendUvarint(d, uint64(f<<3|2))
	d = appendUvarint(d, uint64(len(v)))
	return append(d, v...)
}
func appendProtoVarint(d []byte, f int, v uint64) []byte {
	d = appendUvarint(d, uint64(f<<3))
	return appendUvarint(d, v)
}
func appendProtoFixed64(d []byte, f int, v uint64) []byte {
	d = appendUvarint(d, uint64(f<<3|1))
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return append(d, b...)
}
func appendUvarint(d []byte, v uint64) []byte {
	for v >= 0x80 {
		d = append(d, byte(v)|0x80)
		v >>= 7
	}
	return append(d, byte(v))
}
func sanitizeMetricName(v string) string { return sanitizePrometheusName(v, true) }
func sanitizeLabelName(v string) string  { return sanitizePrometheusName(v, false) }
func sanitizePrometheusName(v string, metric bool) string {
	v = strings.TrimSpace(v)
	if v == "" {
		if metric {
			return "telemetryforge_metric"
		}
		return "label"
	}
	r := []rune{}
	for i, c := range v {
		valid := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9')
		if metric && c == ':' {
			valid = true
		}
		if valid {
			r = append(r, c)
		} else {
			r = append(r, '_')
		}
	}
	first := r[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_' || (metric && first == ':')) {
		r = append([]rune{'_'}, r...)
	}
	return string(r)
}
