import { postMetric } from "./lib.js";

const rate = Number(__ENV.RATE || 100);
const duration = __ENV.DURATION || "2m";

export const options = {
  discardResponseBodies: true,
  scenarios: {
    sustained_ingest: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 250),
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{endpoint:ingest_metric}": ["p(95)<1500"],
  },
};

export default function () {
  postMetric("k6-sustained");
}
