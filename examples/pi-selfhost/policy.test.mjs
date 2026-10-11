import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import register from "./policy.ts";

function mockSession() {
  const hooks = new Map();
  const settings = JSON.parse(readFileSync(new URL("../../.pi/mcp.json", import.meta.url), "utf8"));
  const exposure = settings.mcpServers.devtool.toolExposure;
  register({
    on(name, fn) {
      hooks.set(name, fn);
    },
    getAllTools() {
      return [
        ...Object.entries(exposure).map(([name, mode]) => ({ name: "mcp__devtool__" + name, exposure: mode })),
        { name: "mcp__devtool__scm_publish", exposure: "hidden" },
      ];
    },
  });
  const cwd = new URL("../../", import.meta.url).pathname.replace(/\/$/, "");
  let sequence = 0;
  const invoke = (toolName, input = {}) => {
    const toolCallId = "call-" + ++sequence;
    return {
      toolCallId,
      result: hooks.get("tool_call")({ toolName, input, toolCallId }, { cwd }),
    };
  };
  const complete = (id, payload, isError = false) => {
    hooks.get("tool_result")({
      toolName: "mcp__devtool__code_context",
      toolCallId: id,
      content: [{ type: "text", text: JSON.stringify(payload) }],
      isError,
    });
  };
  return { hooks, invoke, complete };
}

test("Pi connects only through configured DevTool MCP", () => {
  const { devtool } = JSON.parse(readFileSync(new URL("../../.pi/mcp.json", import.meta.url), "utf8")).mcpServers;
  assert.equal(devtool.command, "devtool");
  assert.deepEqual(devtool.args, ["agent", "mcp", "--context", "agent"]);
  assert.equal(devtool.toolExposure.code_context, "direct");
  assert.equal(devtool.toolExposure.scm_publish, undefined);
});

test("first edit is denied until a real successful scoped context result arrives", () => {
  const x = mockSession();
  const path = "extensions/capability/code/code.go";
  assert.equal(x.invoke("edit", { path }).result.block, true);
  const call = x.invoke("mcp__devtool__code_context", { objective: "Inspect code", path });
  assert.equal(call.result, undefined);
  assert.equal(x.invoke("edit", { path }).result.block, true);
  x.complete(call.toolCallId, { indexed: { results: [] }, realtime: { diagnostics: {} } });
  assert.equal(x.invoke("edit", { path }).result, undefined);
  assert.equal(x.invoke("write", { path: "cmd/devtool/main.go" }).result.block, true);
});

test("provider error and indexed-only result do not unlock editing", () => {
  const x = mockSession(), path = "cmd/devtool/main.go";
  let call = x.invoke("mcp__devtool__code_context", { objective: "Inspect", path });
  x.complete(call.toolCallId, { indexed: {}, realtime: {} }, true);
  assert.equal(x.invoke("edit", { path }).result.block, true);
  call = x.invoke("mcp__devtool__code_context", { objective: "Inspect", path });
  x.complete(call.toolCallId, { indexed: {} });
  assert.equal(x.invoke("edit", { path }).result.block, true);
  call = x.invoke("mcp__devtool__code_context", { objective: "Inspect", path });
  x.complete(call.toolCallId, { indexed: {}, realtime: {}, degraded: { realtime: "offline" } });
  assert.equal(x.invoke("edit", { path }).result.block, true);
});

test("direct shell, Git, providers, and policy overwrite are blocked", () => {
  const x = mockSession();
  for (const name of ["bash", "powershell", "codemode", "tool_search", "mcp__other__shell", "mcp__devtool__scm_publish", "mcp__devtool__scm_checkpoint"]) {
    assert.equal(x.invoke(name, { command: "git push" }).result.block, true, name);
  }
  for (const path of ["../../secrets.txt", ".devtool.toml", "examples/pi-selfhost/policy.ts"]) {
    assert.equal(x.invoke("write", { path }).result.block, true, path);
  }
});

test("new agent turn clears previous context attestation", () => {
  const x = mockSession(), path = "cmd/devtool/main.go";
  const call = x.invoke("mcp__devtool__code_context", { objective: "Inspect", path });
  x.complete(call.toolCallId, { indexed: {}, realtime: {} });
  assert.equal(x.invoke("edit", { path }).result, undefined);
  x.hooks.get("before_agent_start")();
  assert.equal(x.invoke("edit", { path }).result.block, true);
});
