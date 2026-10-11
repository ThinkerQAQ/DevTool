#!/usr/bin/env python3
"""Summarize opt-in DevTool timing spans from DEVTOOL_TRACE_FILE JSONL.

This is an analysis tool, not a new logging format. Aggregate nested parent
and child spans separately; their durations are not additive.
"""
import argparse
import collections
import json
import statistics
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("trace", nargs="+", help="One or more DEVTOOL_TRACE_FILE JSONL files")
args = parser.parse_args()
spans = collections.defaultdict(list)
for filename in args.trace:
    for line in Path(filename).read_text().splitlines():
        if not line.strip():
            continue
        entry = json.loads(line)
        name, duration = entry.get("name"), entry.get("duration_ms")
        if isinstance(name, str) and isinstance(duration, (int, float)):
            spans[name].append(float(duration))
print(f"{'stage':<28} {'count':>6} {'median_ms':>11} {'p90_ms':>10} {'max_ms':>10}")
for name, values in sorted(spans.items()):
    ordered = sorted(values)
    p90 = ordered[min(len(ordered)-1, int(0.9 * len(ordered)))]
    print(f"{name:<28} {len(ordered):>6} {statistics.median(ordered):>11.1f} {p90:>10.1f} {ordered[-1]:>10.1f}")
print("Note: nested project.open and extension.resolve spans overlap with child stages.")
