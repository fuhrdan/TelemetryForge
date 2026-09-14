#!/usr/bin/env python3
"""Validate lightweight documentation invariants without third-party packages.

The goal is not to enforce a writing style. It catches the mistakes that make a
GitHub repository frustrating to review: broken relative links, missing core
project documents, duplicate/skipped ADR numbers, and leaked local artifact
paths.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]

REQUIRED = [
    ROOT / "README.md",
    ROOT / "ARCHITECTURE.md",
    ROOT / "ROADMAP.md",
    ROOT / "CONTRIBUTING.md",
    ROOT / "SECURITY.md",
    ROOT / "CODE_OF_CONDUCT.md",
    ROOT / "LICENSE",
    ROOT / "CHANGELOG.md",
    ROOT / "docs" / "README.md",
    ROOT / "docs" / "reference" / "configuration.md",
    ROOT / "docs" / "operations" / "troubleshooting.md",
    ROOT / "docs" / "development" / "testing.md",
    ROOT / "docs" / "development" / "release-process.md",
    ROOT / "docs" / "policy" / "cardinality-firewall.md",
    ROOT / "docs" / "policy" / "policy-as-code.md",
    ROOT / "docs" / "policy" / "shadow-pipeline.md",
    ROOT / "docs" / "deployment" / "kubernetes.md",
    ROOT / "docs" / "deployment" / "terraform.md",
    ROOT / "docs" / "incidents" / "replay.md",
    ROOT / "docs" / "cost" / "cost-simulator.md",
    ROOT / "docs" / "observability" / "self-observability.md",
    ROOT / "docs" / "observability" / "metrics.md",
    ROOT / "docs" / "observability" / "tracing.md",
    ROOT / "docs" / "performance" / "benchmark-methodology.md",
    ROOT / "docs" / "evidence" / "evidence-graph.md",
    ROOT / "docs" / "security" / "authentication.md",
    ROOT / "docs" / "security" / "tenant-isolation.md",
    ROOT / "docs" / "security" / "redaction.md",
    ROOT / "docs" / "security" / "dashboard-proxy.md",
    ROOT / "docs" / "deployment" / "production-profile.md",
    ROOT / "docs" / "operations" / "upgrade-v1.md",
    ROOT / "docs" / "operations" / "rollback-v1.md",
    ROOT / "docs" / "demo" / "v1-portfolio-demo.md",
    ROOT / "docs" / "schema" / "schema-intelligence.md",
    ROOT / "docs" / "schema" / "opentelemetry-semantic-conventions.md",
    ROOT / "docs" / "roadmap" / "v1.1.0.md",
    ROOT / "docs" / "roadmap" / "v1.2.0.md",
    ROOT / "docs" / "cardinality" / "distributed-cardinality.md",
    ROOT / "docs" / "cardinality" / "budgets.md",
    ROOT / "docs" / "routing" / "telemetry-router.md",
    ROOT / "docs" / "routing" / "shadow-routing.md",
    ROOT / "docs" / "roadmap" / "v1.3.0.md",
    ROOT / "docs" / "shaping" / "adaptive-sampling.md",
    ROOT / "docs" / "shaping" / "shaping-policy.md",
    ROOT / "docs" / "shaping" / "visibility-preview.md",
    ROOT / "docs" / "roadmap" / "v1.4.0.md",
    ROOT / "docs" / "incidents" / "portable-archive.md",
    ROOT / "docs" / "security" / "archive-encryption.md",
    ROOT / "docs" / "operations" / "archive-workflow.md",
    ROOT / "docs" / "roadmap" / "v1.5.0.md",
    ROOT / "docs" / "change-intelligence" / "change-intelligence.md",
    ROOT / "docs" / "change-intelligence" / "change-api.md",
    ROOT / "docs" / "operations" / "change-analysis.md",
    ROOT / "docs" / "roadmap" / "v1.7.0.md",
    ROOT / "docs" / "connectors" / "connector-platform.md",
    ROOT / "docs" / "connectors" / "connector-sdk.md",
    ROOT / "docs" / "connectors" / "otlp.md",
    ROOT / "docs" / "connectors" / "prometheus-remote-write.md",
    ROOT / "docs" / "connectors" / "vendor-adapters.md",
    ROOT / "docs" / "operations" / "connectors.md",
    ROOT / "docs" / "roadmap" / "v1.8.0.md",
    ROOT / "docs" / "proof" / "operational-proof.md",
    ROOT / "docs" / "proof" / "ha-scenarios.md",
    ROOT / "docs" / "proof" / "benchmark-methodology-v1.9.md",
    ROOT / "docs" / "roadmap" / "v1.9.0.md",
    ROOT / "docs" / "intelligence" / "evidence-first-investigator.md",
    ROOT / "docs" / "intelligence" / "incident-comparison.md",
    ROOT / "docs" / "intelligence" / "policy-recommendations.md",
    ROOT / "docs" / "roadmap" / "v2.0.0.md",
    ROOT / "docs" / "roadmap" / "v2.1.0.md",
    ROOT / "docs" / "roadmap" / "v2.2.0.md",
    ROOT / "docs" / "edge" / "durable-edge.md",
    ROOT / "docs" / "edge" / "replicated-durability.md",
    ROOT / "docs" / "mesh" / "global-routing-mesh.md",
    ROOT / "docs" / "roadmap" / "v2.3.0.md",
    ROOT / "docs" / "roadmap" / "v2.4.0.md",
    ROOT / "docs" / "security" / "cryptographic-lineage.md",
    ROOT / "docs" / "roadmap" / "v2.5.0.md",
    ROOT / "docs" / "formal" / "formal-verification.md",
    ROOT / "docs" / "roadmap" / "v2.6.0.md",
    ROOT / "docs" / "performance" / "fast-path-v2.6.md",
    ROOT / "docs" / "roadmap" / "v2.7.0.md",
    ROOT / "docs" / "autonomy" / "autonomous-control.md",
    ROOT / "docs" / "autonomy" / "wasm-plugins.md",
    ROOT / "docs" / "roadmap" / "v2.8.0.md",
    ROOT / "docs" / "roadmap" / "v2.9.0.md",
    ROOT / "docs" / "roadmap" / "v3.0.0.md",
    ROOT / "docs" / "ebpf" / "edge-collection.md",
    ROOT / "docs" / "fabric" / "global-edge-fabric.md",
    ROOT / "docs" / "operations" / "upgrade-v3.md",
]

LINK = re.compile(r"(?<!!)\[[^\]]+\]\(([^)]+)\)")
ADR = re.compile(r"^(\d{4})-[^.]+\.md$")


def markdown_files() -> list[Path]:
    return sorted(path for path in ROOT.rglob("*.md") if ".git" not in path.parts)


def check_required(errors: list[str]) -> None:
    for path in REQUIRED:
        if not path.is_file():
            errors.append(f"missing required document: {path.relative_to(ROOT)}")


def check_links(errors: list[str]) -> None:
    for path in markdown_files():
        text = path.read_text(encoding="utf-8")
        for match in LINK.finditer(text):
            raw = match.group(1).strip()
            # Optional Markdown title: path "title". All project links are
            # expected to use simple paths, but splitting here avoids false
            # positives if a title is added later.
            target = raw.split(" ", 1)[0].strip("<>")
            if not target or target.startswith(("http://", "https://", "mailto:", "#")):
                continue

            target = unquote(target.split("#", 1)[0])
            if not target:
                continue
            resolved = (path.parent / target).resolve()
            try:
                resolved.relative_to(ROOT.resolve())
            except ValueError:
                errors.append(f"link escapes repository: {path.relative_to(ROOT)} -> {raw}")
                continue
            if not resolved.exists():
                errors.append(f"broken link: {path.relative_to(ROOT)} -> {raw}")


def check_adrs(errors: list[str]) -> None:
    adr_dir = ROOT / "docs" / "adr"
    numbers: list[int] = []
    for path in sorted(adr_dir.glob("*.md")):
        match = ADR.match(path.name)
        if not match:
            errors.append(f"ADR filename does not follow NNNN-name.md: {path.name}")
            continue
        numbers.append(int(match.group(1)))

    if not numbers:
        errors.append("no ADRs found")
        return

    expected = list(range(1, max(numbers) + 1))
    if numbers != expected:
        errors.append(f"ADR sequence is not contiguous: found {numbers}, expected {expected}")


def check_repository_leaks(errors: list[str]) -> None:
    forbidden = ("sandbox:/", "/mnt/data/", "<<<<<<<", ">>>>>>>")
    for path in markdown_files():
        text = path.read_text(encoding="utf-8")
        for marker in forbidden:
            if marker in text:
                errors.append(f"forbidden repository-local marker {marker!r} in {path.relative_to(ROOT)}")


def check_nonempty(errors: list[str]) -> None:
    for path in markdown_files():
        if not path.read_text(encoding="utf-8").strip():
            errors.append(f"empty documentation file: {path.relative_to(ROOT)}")


def main() -> int:
    errors: list[str] = []
    check_required(errors)
    check_links(errors)
    check_adrs(errors)
    check_repository_leaks(errors)
    check_nonempty(errors)

    if errors:
        print("Documentation check failed:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1

    print(f"Documentation check passed ({len(markdown_files())} Markdown files).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
