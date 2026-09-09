#!/usr/bin/env python3
"""Generate local TelemetryForge demo traffic with only Python's stdlib.

Normal mode emits healthy latency and occasional application events.
Use --incident to deliberately emit high latency plus an error burst.
Use --cardinality to emit identifier-shaped labels that exercise the
Cardinality Firewall and shadow-policy comparison.
"""

import argparse
import json
import random
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone


def post(base_url: str, route: str, body: dict) -> None:
    request = urllib.request.Request(
        f"{base_url}{route}",
        data=json.dumps(body).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        if response.status != 202:
            raise RuntimeError(f"unexpected HTTP {response.status}")


def envelope(source: str, event_type: str) -> dict:
    return {
        "id": str(uuid.uuid4()),
        "source": source,
        "type": event_type,
        "timestamp": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "schema_version": "1.0",
        "tags": {"environment": "demo", "region": "local"},
    }


def metric(base_url: str, source: str, value: float) -> None:
    body = envelope(source, "request.duration")
    body.update({"value": value, "unit": "ms"})
    post(base_url, "/api/v1/metrics", body)


def error(base_url: str, source: str) -> None:
    body = envelope(source, "checkout.error")
    body["tags"]["severity"] = "error"
    body["payload"] = {"message": "simulated checkout dependency failure"}
    post(base_url, "/api/v1/events", body)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--incident", action="store_true")
    parser.add_argument("--cardinality", action="store_true")
    parser.add_argument("--count", type=int, default=120)
    parser.add_argument("--interval", type=float, default=0.25)
    args = parser.parse_args()

    sources = ["checkout-api", "payment-service", "inventory-api"]

    print(f"Sending demo telemetry to {args.base_url}")
    if args.incident:
        print("Incident mode enabled: latency/error thresholds will be crossed.")
    if args.cardinality:
        print("Cardinality mode enabled: unique request_id/session_id tags will grow.")

    for index in range(args.count):
        source = sources[index % len(sources)]
        latency = max(12.0, random.gauss(145.0, 42.0))

        if args.incident and 18 <= index <= 24:
            source = "checkout-api"
            latency = random.uniform(1150.0, 1800.0)

        try:
            if args.cardinality:
                body = envelope("checkout-api", "request.duration")
                body["tags"]["request_id"] = f"req-{uuid.uuid4()}"
                body["tags"]["session_id"] = f"session-{uuid.uuid4()}"
                body.update({"value": latency, "unit": "ms"})
                post(args.base_url, "/api/v1/metrics", body)
            else:
                metric(args.base_url, source, latency)

            if args.incident and 28 <= index <= 33:
                error(args.base_url, "checkout-api")

        except (urllib.error.URLError, RuntimeError) as exc:
            print(f"request failed: {exc}")
            print("Is `docker compose up --build` running?")
            raise SystemExit(1)

        time.sleep(args.interval)

    print("Demo traffic complete.")


if __name__ == "__main__":
    main()
