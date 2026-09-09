#!/usr/bin/env python3
"""Generate local TelemetryForge demo traffic with only Python's stdlib.

Normal mode emits healthy latency and occasional application events.
Use --incident to deliberately emit high latency plus an error burst.
Use --cardinality to emit identifier-shaped labels that exercise the
Cardinality Firewall and shadow-policy comparison.
Use --schema-demo to establish a schema, introduce same-version drift, and
then emit a deliberately breaking declared version.
"""

import argparse
import os
import json
import random
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone


def post(base_url: str, route: str, body: dict, api_key: str = "") -> None:
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    request = urllib.request.Request(
        f"{base_url}{route}",
        data=json.dumps(body).encode("utf-8"),
        headers=headers,
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


def metric(base_url: str, source: str, value: float, api_key: str = "", tags: dict | None = None) -> None:
    body = envelope(source, "request.duration")
    if tags:
        body["tags"].update(tags)
    body.update({"value": value, "unit": "ms"})
    post(base_url, "/api/v1/metrics", body, api_key)


def error(base_url: str, source: str, api_key: str = "", tags: dict | None = None) -> None:
    body = envelope(source, "checkout.error")
    body["tags"]["severity"] = "error"
    if tags:
        body["tags"].update(tags)
    body["payload"] = {"message": "simulated checkout dependency failure"}
    post(base_url, "/api/v1/events", body, api_key)


def schema_demo(base_url: str, api_key: str, interval: float) -> None:
    print("Schema demo enabled: baseline -> same-version drift -> declared v2 break.")
    schema_url = "https://opentelemetry.io/schemas/1.44.0"

    for index in range(24):
        body = envelope("orders-api", "order.created")
        body["schema_url"] = schema_url
        body["tags"].update({
            "service.name": "orders-api",
            "deployment.environment.name": "production",
        })
        body["payload"] = {
            "order_id": f"order-{index:04d}",
            "total": round(25.0 + index * 1.75, 2),
            "currency": "USD",
            "customer": {"tier": "standard"},
        }
        post(base_url, "/api/v1/events", body, api_key)
        time.sleep(interval)

    # Same application schema_version, but a required field disappears, a
    # number changes to a string, and an old HTTP semantic-convention name is
    # introduced. v1.1 should flag all of these without rejecting the event.
    drift = envelope("orders-api", "order.created")
    drift["schema_url"] = schema_url
    drift["tags"].update({
        "service.name": "orders-api",
        "deployment.environment.name": "production",
        "http.method": "POST",
    })
    drift["payload"] = {
        "total": "67.00",
        "currency": "USD",
        "customer": {"tier": "standard"},
    }
    post(base_url, "/api/v1/events", drift, api_key)
    time.sleep(interval)

    # Version 2 explicitly declares the breaking shape. The registry can now
    # compare v1 -> v2 rather than treating the change as accidental drift.
    version_two = envelope("orders-api", "order.created")
    version_two["schema_version"] = "2.0"
    version_two["schema_url"] = schema_url
    version_two["tags"].update({
        "service.name": "orders-api",
        "deployment.environment.name": "production",
        "http.request.method": "POST",
    })
    version_two["payload"] = {
        "total": "72.50",
        "currency": "USD",
        "customer": {"tier": "standard", "segment": "retail"},
    }
    post(base_url, "/api/v1/events", version_two, api_key)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--incident", action="store_true")
    parser.add_argument("--cardinality", action="store_true")
    parser.add_argument("--evidence-demo", action="store_true")
    parser.add_argument("--schema-demo", action="store_true")
    parser.add_argument("--api-key", default=os.getenv("TELEMETRYFORGE_DEMO_API_KEY", ""))
    parser.add_argument("--count", type=int, default=120)
    parser.add_argument("--interval", type=float, default=0.25)
    args = parser.parse_args()

    sources = ["checkout-api", "payment-service", "inventory-api"]

    print(f"Sending demo telemetry to {args.base_url}")
    if args.schema_demo:
        try:
            schema_demo(args.base_url, args.api_key, args.interval)
        except (urllib.error.URLError, RuntimeError) as exc:
            print(f"request failed: {exc}")
            print("Is `docker compose up --build` running?")
            raise SystemExit(1)
        print("Schema demo complete.")
        return
    if args.incident:
        print("Incident mode enabled: latency/error thresholds will be crossed.")
    if args.cardinality:
        print("Cardinality mode enabled: unique request_id/session_id tags will grow.")
    if args.evidence_demo:
        print("Evidence demo enabled: deployment -> latency -> errors -> recovery.")

    trace_id = f"demo-trace-{uuid.uuid4()}"

    if args.evidence_demo:
        deploy = envelope("checkout-api", "deployment.completed")
        deploy["correlation_id"] = "demo-checkout-flow"
        deploy["tags"].update({
            "deployment_id": f"deploy-{uuid.uuid4()}",
            "version": "v1-demo-change",
            "trace_id": trace_id,
        })
        post(args.base_url, "/api/v1/events", deploy, args.api_key)

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
                post(args.base_url, "/api/v1/metrics", body, args.api_key)
            elif args.evidence_demo and 8 <= index <= 18:
                metric(
                    args.base_url,
                    "checkout-api",
                    random.uniform(1200.0, 1750.0),
                    args.api_key,
                    {"trace_id": trace_id},
                )
            elif args.evidence_demo and index >= 28:
                metric(
                    args.base_url,
                    "checkout-api",
                    random.uniform(90.0, 220.0),
                    args.api_key,
                    {"trace_id": trace_id},
                )
            else:
                metric(args.base_url, source, latency, args.api_key)

            if args.incident and 28 <= index <= 33:
                error(args.base_url, "checkout-api", args.api_key)

            if args.evidence_demo and 19 <= index <= 24:
                error(
                    args.base_url,
                    "checkout-api",
                    args.api_key,
                    {"trace_id": trace_id},
                )

        except (urllib.error.URLError, RuntimeError) as exc:
            print(f"request failed: {exc}")
            print("Is `docker compose up --build` running?")
            raise SystemExit(1)

        time.sleep(args.interval)

    print("Demo traffic complete.")


if __name__ == "__main__":
    main()
