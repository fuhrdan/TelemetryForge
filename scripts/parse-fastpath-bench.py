#!/usr/bin/env python3
from __future__ import annotations
import argparse, json, re, statistics
from pathlib import Path

LINE = re.compile(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([0-9.]+)\s+ns/op(?:\s+([0-9.]+)\s+B/op)?(?:\s+([0-9.]+)\s+allocs/op)?$')

def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('input')
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    samples: dict[str, list[dict[str, float]]] = {}
    meta: dict[str, str] = {}
    for raw in Path(args.input).read_text().splitlines():
        line = raw.strip()
        if line.startswith(('goos:', 'goarch:', 'cpu:', 'pkg:')):
            key, value = line.split(':', 1)
            meta[key] = value.strip()
        match = LINE.match(line)
        if not match:
            continue
        name, ns, bytes_op, allocs = match.groups()
        samples.setdefault(name, []).append({
            'ns_per_op': float(ns),
            'bytes_per_op': float(bytes_op or 0),
            'allocs_per_op': float(allocs or 0),
        })
    if not samples:
        raise SystemExit('no Go benchmark samples found')
    benchmarks = []
    for name, rows in sorted(samples.items()):
        benchmarks.append({
            'name': name,
            'samples': rows,
            'median_ns_per_op': statistics.median(r['ns_per_op'] for r in rows),
            'median_bytes_per_op': statistics.median(r['bytes_per_op'] for r in rows),
            'median_allocs_per_op': statistics.median(r['allocs_per_op'] for r in rows),
        })
    report = {'format': 'telemetryforge-fastpath-benchmark', 'format_version': 1, 'environment': meta, 'benchmarks': benchmarks}
    Path(args.out).write_text(json.dumps(report, indent=2) + '\n')

if __name__ == '__main__':
    main()
