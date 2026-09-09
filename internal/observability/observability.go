// Package observability instruments TelemetryForge itself.
package observability

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Metrics owns one process-local Prometheus registry.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests      *prometheus.CounterVec
	httpDuration      *prometheus.HistogramVec
	accepted          *prometheus.CounterVec
	publishFailure    *prometheus.CounterVec
	workerCompleted   *prometheus.CounterVec
	workerDuration    prometheus.Histogram
	workerRetries     *prometheus.CounterVec
	queueDepth        prometheus.Gauge
	queueCapacity     prometheus.Gauge
	consumerLag       *prometheus.GaugeVec
	consumerLagTotal  prometheus.Gauge
	deadLetters       *prometheus.CounterVec
	routingDeliveries *prometheus.CounterVec
}

// NewMetrics creates a registry with low-cardinality TelemetryForge metrics.
func NewMetrics(service string) *Metrics {
	const namespace = "telemetryforge"
	labels := prometheus.Labels{"service": service}
	metrics := &Metrics{registry: prometheus.NewRegistry()}
	metrics.httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "http_requests_total", Help: "HTTP requests by method, route pattern, and status.", ConstLabels: labels,
	}, []string{"method", "route", "status"})
	metrics.httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Name: "http_request_duration_seconds", Help: "HTTP request duration by method and route pattern.", ConstLabels: labels,
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
	metrics.accepted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "accepted_events_total", Help: "Telemetry accepted by event class and Kafka topic.", ConstLabels: labels,
	}, []string{"kind", "topic"})
	metrics.publishFailure = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "kafka_publish_failures_total", Help: "Kafka publishing failures by topic.", ConstLabels: labels,
	}, []string{"topic"})
	metrics.workerCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "worker_jobs_total", Help: "Worker jobs completed by outcome.", ConstLabels: labels,
	}, []string{"outcome"})
	metrics.workerDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Name: "worker_job_duration_seconds", Help: "End-to-end processing time for worker jobs.", ConstLabels: labels,
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	})
	metrics.workerRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "worker_retries_total", Help: "Worker retry attempts by failure classification.", ConstLabels: labels,
	}, []string{"classification"})
	metrics.queueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "worker_queue_depth", Help: "Current bounded worker queue depth.", ConstLabels: labels,
	})
	metrics.queueCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "worker_queue_capacity", Help: "Configured bounded worker queue capacity.", ConstLabels: labels,
	})
	metrics.consumerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Name: "kafka_consumer_lag", Help: "Broker-derived committed consumer lag by topic and partition.", ConstLabels: labels,
	}, []string{"topic", "partition"})
	metrics.consumerLagTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "kafka_consumer_lag_total", Help: "Total broker-derived committed consumer lag for the worker group.", ConstLabels: labels,
	})
	metrics.deadLetters = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "dead_letters_total", Help: "Events published to the DLQ by failure classification.", ConstLabels: labels,
	}, []string{"classification"})
	metrics.routingDeliveries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "routing_deliveries_total", Help: "Telemetry Router delivery outcomes by configured destination.", ConstLabels: labels,
	}, []string{"destination", "outcome"})

	metrics.registry.MustRegister(
		metrics.httpRequests, metrics.httpDuration, metrics.accepted,
		metrics.publishFailure, metrics.workerCompleted, metrics.workerDuration,
		metrics.workerRetries, metrics.queueDepth, metrics.queueCapacity,
		metrics.consumerLag, metrics.consumerLagTotal, metrics.deadLetters, metrics.routingDeliveries,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return metrics
}

// Handler exposes the process registry in Prometheus text format.
func (metrics *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

// Accepted records one successfully durable Kafka handoff.
func (metrics *Metrics) Accepted(kind, topic string) {
	metrics.accepted.WithLabelValues(kind, topic).Inc()
}
func (metrics *Metrics) PublishFailure(topic string) {
	metrics.publishFailure.WithLabelValues(topic).Inc()
}

// QueueDepth implements the worker observer contract.
func (metrics *Metrics) QueueDepth(current, capacity int) {
	metrics.queueDepth.Set(float64(current))
	metrics.queueCapacity.Set(float64(capacity))
}

// JobCompleted implements the worker observer contract.
func (metrics *Metrics) JobCompleted(duration time.Duration, attempts int, outcome string) {
	metrics.workerCompleted.WithLabelValues(outcome).Inc()
	metrics.workerDuration.Observe(duration.Seconds())
}

// Retry records a retry by classification.
func (metrics *Metrics) Retry(classification string) {
	metrics.workerRetries.WithLabelValues(classification).Inc()
}

// SetConsumerLag implements the stream lag observer contract.
func (metrics *Metrics) SetConsumerLag(topic string, partition int32, lag int64) {
	if lag < 0 {
		return
	}
	metrics.consumerLag.WithLabelValues(topic, strconv.Itoa(int(partition))).Set(float64(lag))
}
func (metrics *Metrics) SetConsumerLagTotal(lag int64) {
	if lag >= 0 {
		metrics.consumerLagTotal.Set(float64(lag))
	}
}
func (metrics *Metrics) DeadLetter(classification string) {
	metrics.deadLetters.WithLabelValues(classification).Inc()
}

// RoutingDelivery records a bounded destination/outcome routing metric.
// Destination names come only from reviewed routing configuration.
func (metrics *Metrics) RoutingDelivery(destination, outcome string) {
	metrics.routingDeliveries.WithLabelValues(destination, outcome).Inc()
}

// Middleware records Prometheus HTTP metrics and OpenTelemetry spans.
// Route labels come from net/http patterns, never raw URL paths.
func (metrics *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/metrics" {
			next.ServeHTTP(writer, request)
			return
		}
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		tracer := otel.Tracer("github.com/fuhrdan/TelemetryForge/http")
		ctx, span := tracer.Start(request.Context(), "HTTP "+request.Method)
		request = request.WithContext(ctx)

		next.ServeHTTP(recorder, request)

		route := request.Pattern
		if route == "" {
			route = "unmatched"
		}
		duration := time.Since(started)
		status := strconv.Itoa(recorder.status)
		metrics.httpRequests.WithLabelValues(request.Method, route, status).Inc()
		metrics.httpDuration.WithLabelValues(request.Method, route).Observe(duration.Seconds())
		span.SetAttributes(
			attribute.String("http.request.method", request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", recorder.status),
		)
		if recorder.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(recorder.status))
		}
		span.End()
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (writer *statusRecorder) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}
func (writer *statusRecorder) Unwrap() http.ResponseWriter { return writer.ResponseWriter }
func (writer *statusRecorder) Flush() {
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// InitTracing installs an OTLP/HTTP trace provider. An empty endpoint keeps the
// default no-op provider, which makes tracing optional in local/custom deploys.
func InitTracing(ctx context.Context, service, version, endpoint string) (func(context.Context) error, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithMaxRequestSize(4*1024*1024),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", service),
		attribute.String("service.version", version),
	))
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.10))),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
