package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestHTTPMetricsUseRoutePatternInsteadOfRawID(t *testing.T) {
	metrics := NewMetrics("test")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/items/very-high-cardinality-id", nil)
	response := httptest.NewRecorder()
	metrics.Middleware(mux).ServeHTTP(response, request)

	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	foundRoute := false
	for _, family := range families {
		if family.GetName() != "telemetryforge_http_requests_total" {
			continue
		}
		for _, metric := range family.Metric {
			labels := labelMap(metric.Label)
			if labels["route"] == "GET /items/{id}" {
				foundRoute = true
			}
			if labels["route"] == "/items/very-high-cardinality-id" ||
				labels["route"] == "GET /items/very-high-cardinality-id" {
				t.Fatal("raw request ID leaked into Prometheus route label")
			}
		}
	}
	if !foundRoute {
		t.Fatal("expected registered route pattern in HTTP metrics")
	}
}

func TestNegativeConsumerLagIsIgnored(t *testing.T) {
	metrics := NewMetrics("worker")
	metrics.SetConsumerLag("telemetry.raw", 1, -1)

	families, err := metrics.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "telemetryforge_kafka_consumer_lag" &&
			len(family.Metric) != 0 {
			t.Fatal("negative/unavailable Kafka lag should not create a gauge series")
		}
	}
}

func labelMap(labels []*dto.LabelPair) map[string]string {
	result := make(map[string]string, len(labels))
	for _, label := range labels {
		result[label.GetName()] = label.GetValue()
	}
	return result
}
