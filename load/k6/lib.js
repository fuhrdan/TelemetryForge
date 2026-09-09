import http from "k6/http";
import { check } from "k6";

export const baseURL = __ENV.BASE_URL || "http://host.docker.internal:8080";

function eventID(prefix) {
  return `${prefix}-${__VU}-${__ITER}-${Date.now()}`;
}

export function metricPayload(source = "load-generator") {
  const value = 80 + ((__ITER * 17 + __VU * 7) % 240);
  return JSON.stringify({
    id: eventID("k6-metric"),
    source,
    type: "request.duration",
    timestamp: new Date().toISOString(),
    tags: {
      environment: "k6",
      region: `zone-${(__VU % 3) + 1}`,
      route: `/checkout/${__ITER % 8}`,
    },
    value,
    unit: "ms",
    schema_version: "1.0",
    correlation_id: `load-${__VU}-${__ITER}`,
  });
}

export function postMetric(source) {
  const response = http.post(
    `${baseURL}/api/v1/metrics`,
    metricPayload(source),
    {
      headers: { "Content-Type": "application/json" },
      tags: { endpoint: "ingest_metric" },
    },
  );

  check(response, {
    "ingest returns 202": (result) => result.status === 202,
  });
  return response;
}
