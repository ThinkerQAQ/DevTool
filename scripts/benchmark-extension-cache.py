#!/usr/bin/env python3
"""A/B of DevTool local Go Provider discovery; excludes network latency."""
import argparse
import json
import os
from pathlib import Path
import shutil
import statistics
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser()
parser.add_argument("--project", required=True)
parser.add_argument("--baseline", required=True)
parser.add_argument("--candidate", required=True)
parser.add_argument("--repeats", type=int, default=6)
parser.add_argument("--output")
args = parser.parse_args()
if not 3 <= args.repeats <= 20:
    parser.error("repeats must be 3..20")
root = Path(args.project).resolve()
go = shutil.which("go")
if not go:
    raise SystemExit("go not found")
tools = {"baseline": str(Path(args.baseline).resolve()),
         "candidate": str(Path(args.candidate).resolve())}
results = []

with tempfile.TemporaryDirectory(prefix="dv2-dep-bench-") as temp:
    shim = Path(temp, "go")
    shim.write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> "$DV2_BENCH_LOG"\nexec "$DV2_REAL_GO" "$@"\n')
    shim.chmod(0o700)

    def run(mode, action, warmup=False):
        log = Path(temp, "go-" + mode + "-" + action.replace(" ", "_") + ".log")
        log.write_text("")
        env = dict(os.environ, PATH=temp + os.pathsep + os.environ.get("PATH", ""),
                   DEVTOOL_PROFILES="local", DV2_BENCH_LOG=str(log), DV2_REAL_GO=go)
        start = time.perf_counter()
        outcome = subprocess.run([tools[mode], *action.split()], cwd=root, env=env,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                 text=True, timeout=120)
        elapsed = round((time.perf_counter() - start) * 1000)
        if outcome.returncode:
            raise RuntimeError(mode + " " + action + ": " + outcome.stderr[-900:])
        calls = log.read_text().splitlines()
        item = {"mode": mode, "action": action, "ms": elapsed,
                "go_list": calls.count("list"), "go_build": calls.count("build")}
        if not warmup:
            results.append(item)
            print("SAMPLE", json.dumps(item), flush=True)

    for action in ("config check", "project inspect --json"):
        for mode in tools:
            run(mode, action, warmup=True)
        for i in range(args.repeats):
            order = ("baseline", "candidate") if i % 2 == 0 else ("candidate", "baseline")
            for mode in order:
                run(mode, action)

summaries = []
for action in ("config check", "project inspect --json"):
    subset = {mode: [s for s in results if s["action"] == action and s["mode"] == mode] for mode in tools}
    before = statistics.median(row["ms"] for row in subset["baseline"])
    after = statistics.median(row["ms"] for row in subset["candidate"])
    summaries.append({
        "action": action, "repeats_per_variant": args.repeats,
        "baseline_median_ms": round(before), "candidate_median_ms": round(after),
        "speedup_factor": round(before / after, 2),
        "reduction_percent": round((1-after / before) * 100, 1),
        "baseline_go_list_counts": [s["go_list"] for s in subset["baseline"]],
        "candidate_go_list_counts": [s["go_list"] for s in subset["candidate"]],
    })
report = {"project": str(root), "summary": summaries, "samples": results,
          "scope": "local CLI/provider discovery only; excludes remote transport"}
print("SUMMARY", json.dumps(summaries), flush=True)
if args.output:
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
