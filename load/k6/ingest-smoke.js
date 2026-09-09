import { sleep } from "k6";
import { postMetric } from "./lib.js";

export const options = {
  vus: 1,
  duration: "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{endpoint:ingest_metric}": ["p(95)<1000"],
  },
};

export default function () {
  postMetric("k6-smoke");
  sleep(0.25);
}
