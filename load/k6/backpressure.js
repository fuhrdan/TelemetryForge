import { postMetric } from "./lib.js";

const rate = Number(__ENV.RATE || 500);
const duration = __ENV.DURATION || "90s";

export const options = {
  discardResponseBodies: true,
  scenarios: {
    burst_into_bounded_pipeline: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 100),
      maxVUs: Number(__ENV.MAX_VUS || 500),
    },
  },
  thresholds: {
    // This scenario is intended to create Kafka lag on modest machines. The
    // correctness threshold is intentionally looser than the smoke test.
    http_req_failed: ["rate<0.05"],
    "http_req_duration{endpoint:ingest_metric}": ["p(95)<2500"],
  },
};

export default function () {
  postMetric("k6-backpressure");
}
