#!/usr/bin/env python3
"""Verify a real configured DevTool MCP server using a persistent stdio session."""

import json
import os
import subprocess
import sys

process = subprocess.Popen(
    [sys.argv[1], "agent", "mcp", "--context", "agent"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    text=True,
    bufsize=1,
    env=os.environ.copy(),
)
next_id = 0


def request(method, params=None):
    global next_id
    next_id += 1
    process.stdin.write(
        json.dumps({"jsonrpc": "2.0", "id": next_id, "method": method, "params": params or {}}) + "\n"
    )
    process.stdin.flush()
    while True:
        line = process.stdout.readline()
        if not line:
            raise RuntimeError(f"MCP closed before replying to {method}: {process.poll()}")
        reply = json.loads(line)
        if reply.get("id") == next_id:
            if reply.get("error"):
                raise AssertionError(reply)
            return reply["result"]


try:
    init = request(
        "initialize",
        {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {"name": "devtool-ci-check", "version": "1"},
        },
    )
    assert init["serverInfo"]["name"] == "devtool-mcp", init
    process.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
    process.stdin.flush()

    tools = request("tools/list")["tools"]
    names = {tool["name"] for tool in tools}
    for required in (
        "scm_publish",
        "code_context",
        "project_build",
        "project_verify",
        "project_activate",
        "project_rollback",
    ):
        assert required in names, (required, names)
    assert not names.intersection(
        {"code_search", "code_symbols", "code_references", "code_diagnostics"}
    ), names
    assert "find_symbol" not in names, names
    assert not any(name.startswith(("codegraph_", "sourcegraph_")) for name in names)

    result = request(
        "tools/call",
        {
            "name": "code_context",
            "arguments": {
                "objective": "understand Registry routing",
                "symbol": "Registry",
                "path": "core/registry/registry.go",
                "limit": 5,
            },
        },
    )
    assert not result.get("isError"), result
    data = json.loads(result["content"][0]["text"])
    assert data.get("indexed") is not None, data
    assert data.get("realtime") is not None, data
    print(f"Real MCP context passed: {len(names)} exposed tools")
finally:
    process.stdin.close()
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)
