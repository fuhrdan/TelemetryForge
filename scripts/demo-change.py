#!/usr/bin/env python3
"""Generate a deterministic Change Intelligence scenario for TelemetryForge."""
import argparse
import json
import os
import random
import time
import urllib.request
import uuid


def post(base, route, body, api_key=""):
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    request = urllib.request.Request(
        f"{base}{route}", data=json.dumps(body).encode(), headers=headers, method="POST"
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode())


def metric(base, source, value, api_key="", error=False):
    body = {
        "source": source,
        "type": "request.error" if error else "request.duration",
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "schema_version": "1.0",
        "tags": {"severity": "error"} if error else {},
    }
    if not error:
        body.update({"value": value, "unit": "ms"})
        route = "/api/v1/metrics"
    else:
        body["payload"] = {"message": "simulated post-deployment failure"}
        route = "/api/v1/events"
    post(base, route, body, api_key)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--api-key", default=os.getenv("TELEMETRYFORGE_DEMO_API_KEY", ""))
    parser.add_argument("--interval", type=float, default=0.05)
    args = parser.parse_args()

    source = "checkout-api"
    peer = "payments-api"
    change_id = f"DEP-{uuid.uuid4().hex[:10]}"

    print("Generating baseline telemetry...")
    for _ in range(24):
        metric(args.base_url, source, random.uniform(140, 240), args.api_key)
        metric(args.base_url, peer, random.uniform(100, 190), args.api_key)
        time.sleep(args.interval)

    print("Recording deployment", change_id)
    result = post(args.base_url, "/api/v1/changes", {
        "change_id": change_id,
        "source": source,
        "kind": "deployment",
        "status": "completed",
        "environment": "production",
        "version": "4.12.7-demo",
        "previous_version": "4.12.6-demo",
        "git_sha": uuid.uuid4().hex,
        "build_id": f"build-{uuid.uuid4().hex[:8]}",
        "actor": "demo-pipeline",
        "summary": "Demo checkout deployment used to exercise Change Intelligence"
    }, args.api_key)
    print(result)

    print("Generating post-change regression across checkout + payments...")
    for index in range(28):
        metric(args.base_url, source, random.uniform(1200, 1800), args.api_key, error=index % 3 == 0)
        metric(args.base_url, peer, random.uniform(650, 1100), args.api_key, error=index % 5 == 0)
        time.sleep(args.interval)

    rollback_id = f"RB-{uuid.uuid4().hex[:10]}"
    print("Recording rollback", rollback_id)
    post(args.base_url, "/api/v1/changes", {
        "change_id": rollback_id,
        "source": source,
        "kind": "rollback",
        "status": "completed",
        "environment": "production",
        "version": "4.12.6-demo",
        "previous_version": "4.12.7-demo",
        "rollback_of": change_id,
        "actor": "demo-pipeline",
        "summary": "Demo rollback after simulated regression"
    }, args.api_key)

    print("Generating recovery telemetry...")
    for _ in range(24):
        metric(args.base_url, source, random.uniform(130, 260), args.api_key)
        metric(args.base_url, peer, random.uniform(110, 220), args.api_key)
        time.sleep(args.interval)

    print("\nAnalyze with:")
    print(f"  go run ./cmd/telemetryctl change analyze --id {change_id} --before 15m --after 15m")
    print("or select the deployment in the dashboard Change Intelligence panel.")


if __name__ == "__main__":
    main()
