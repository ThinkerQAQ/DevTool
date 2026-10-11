#!/usr/bin/env python3
"""A/B: repeated DevTool CLI starts vs one persistent, standard MCP session.

Exactly the same project 'verify' command is invoked in both modes.
This measures DevTool process reuse, NOT the ChatGPT->Desktop Commander network.
"""
import argparse
import json
import os
import pathlib
import statistics
import subprocess
import time

args = argparse.ArgumentParser()
args.add_argument("--project", required=True)
args.add_argument("--devtool", required=True)
args.add_argument("--repeats", type=int, default=3)
options = args.parse_args()
if options.repeats < 2 or options.repeats > 6:
    raise SystemExit("repeats must be 2..6")
root = pathlib.Path(options.project).resolve()
executable = str(pathlib.Path(options.devtool).resolve())
env = dict(os.environ, DEVTOOL_PROFILES="local")
data = {"baseline_cli": [], "persistent_mcp": []}

def cli_once():
    started = time.monotonic()
    response = subprocess.run([executable, "verify"], cwd=root, env=env, capture_output=True, text=True, timeout=180)
    elapsed = round((time.monotonic()-started)*1000)
    if response.returncode != 0:
        raise RuntimeError("CLI verify failed: " + response.stderr[-500:])
    print("SAMPLE", json.dumps({"mode":"baseline_cli","elapsed_ms":elapsed,"exit_code":response.returncode}), flush=True)
    data["baseline_cli"].append(elapsed)

start = time.monotonic()
mcp = subprocess.Popen([executable, "agent", "mcp", "--context", "agent"],cwd=root,env=env,
                       stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True,bufsize=1)
index = 0
def rpc(method,params={}):
    global index
    index += 1
    mcp.stdin.write(json.dumps({"jsonrpc":"2.0","id":index,"method":method,"params":params})+"\n")
    mcp.stdin.flush()
    while True:
        line = mcp.stdout.readline()
        if not line: raise RuntimeError("MCP exited before returning "+method)
        result=json.loads(line)
        if result.get("id")==index:
            if result.get("error"):raise RuntimeError(result["error"])
            return result["result"]
try:
    rpc("initialize",{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"DevTool-project-invocation-benchmark","version":"1"}})
    mcp.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
    mcp.stdin.flush()
    tools=rpc("tools/list")["tools"]
    if "project_verify" not in {tool["name"] for tool in tools}:raise RuntimeError("project_verify not registered")
    init_ms=round((time.monotonic()-start)*1000)
    print("MCP_INITIALIZATION_MS",init_ms,flush=True)
    def mcp_once():
        started=time.monotonic()
        result=rpc("tools/call",{"name":"project_verify","arguments":{}})
        elapsed=round((time.monotonic()-started)*1000)
        if result.get("isError"):raise RuntimeError(str(result.get("content"))[-500:])
        print("SAMPLE",json.dumps({"mode":"persistent_mcp","elapsed_ms":elapsed,"error":False}),flush=True)
        data["persistent_mcp"].append(elapsed)
    for iteration in range(options.repeats):
        if iteration%2==0:
            cli_once()
            mcp_once()
        else:
            mcp_once()
            cli_once()
finally:
    mcp.stdin.close()
    mcp.terminate()
    try: mcp.wait(timeout=5)
    except subprocess.TimeoutExpired:mcp.kill()

print("SUMMARY",json.dumps({
    "project":str(root),
    "workload":"same project verify operation",
    "repeats":options.repeats,
    "cli_process_starts":options.repeats,
    "mcp_process_starts":1,
    "mcp_init_ms":init_ms,
    "baseline_cli_median_ms":round(statistics.median(data["baseline_cli"])),
    "persistent_mcp_median_ms":round(statistics.median(data["persistent_mcp"])),
    "baseline_cli_total_ms":sum(data["baseline_cli"]),
    "mcp_total_including_init_ms":sum(data["persistent_mcp"])+init_ms,
    "note":"This is local process-start and project execution cost. ChatGPT and Remote Desktop Commander network latency is not included.",
},sort_keys=True),flush=True)
