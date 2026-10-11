/**
 * Experimental policy adapter for Pi -> DevTool self-hosting.
 * The model cannot execute shell/SCM/provider tools through this Pi session.
 * It is NOT a security boundary for an external agent or the OS user.
 */
import { existsSync, realpathSync } from "node:fs";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const MCP_PREFIX = "mcp__devtool__";

function contained(root: string, candidate: string): boolean {
  const rel = relative(root, candidate);
  return rel !== ".." && !rel.startsWith(".." + sep) && !isAbsolute(rel);
}

function safeSourcePath(cwd: string, input: unknown): string | undefined {
  if (typeof input !== "string" || !input.trim()) return undefined;
  const root = realpathSync(cwd);
  const target = resolve(cwd, input);
  if (!contained(root, target) || target === root) return undefined;
  const rel = relative(root, target).replaceAll("\\", "/");
  if (rel === ".git" || rel.startsWith(".git/") ||
      rel === ".pi" || rel.startsWith(".pi/") ||
      rel === ".devtool" || rel.startsWith(".devtool/") ||
      rel === ".devtool.toml" ||
      rel.startsWith("examples/pi-selfhost/")) return undefined;
  let existing = target;
  while (!existsSync(existing)) {
    const parent = dirname(existing);
    if (parent === existing) return undefined;
    existing = parent;
  }
  if (!contained(root, realpathSync(existing))) return undefined;
  return rel;
}

function parseCodeResult(content: readonly { type: string; text?: string }[]): boolean {
  for (const item of content) {
    if (item.type !== "text" || typeof item.text !== "string") continue;
    try {
      const value: unknown = JSON.parse(item.text);
      if (!value || typeof value !== "object") continue;
      const data = value as Record<string, unknown>;
      const hasIndexed = data.indexed !== undefined && data.indexed !== null;
      const hasRealtime = data.realtime !== undefined && data.realtime !== null;
      const degraded = data.degraded;
      if (hasIndexed && hasRealtime &&
          (!degraded || (typeof degraded === "object" && Object.keys(degraded).length === 0))) {
        return true;
      }
    } catch {
      // Invalid tool results never unlock edits.
    }
  }
  return false;
}

export default function register(pi: ExtensionAPI) {
  const completedScopes = new Set<string>();
  const pending = new Map<string, string>();

  pi.on("before_agent_start", () => {
    completedScopes.clear();
    pending.clear();
  });

  pi.on("tool_call", (event, ctx) => {
    if (event.toolName === "read") {
      const path = safeSourcePath(ctx.cwd, event.input.path);
      return path ? undefined : { block: true, reason: "Read is outside the assigned project scope" };
    }

    if (event.toolName === "edit" || event.toolName === "write") {
      const path = safeSourcePath(ctx.cwd, event.input.path);
      if (!path) return { block: true, reason: "Write is outside the assigned project scope" };
      if (!completedScopes.has(path)) {
        return { block: true, reason: "Call DevTool code_context successfully for this file before editing" };
      }
      return undefined;
    }

    // MCP access is derived from the effective Pi tool exposure, not a second whitelist.
    const directDevtoolTool = pi.getAllTools().some(
      (tool) => tool.name === event.toolName && tool.exposure === "direct",
    );
    if (event.toolName === MCP_PREFIX + "code_context" && directDevtoolTool) {
      const path = safeSourcePath(ctx.cwd, event.input.path);
      if (!path || typeof event.input.objective !== "string" || !event.input.objective.trim()) {
        return { block: true, reason: "Scoped code_context requires a project-relative file path and objective" };
      }
      pending.set(event.toolCallId, path);
      return undefined;
    }

    if (event.toolName.startsWith(MCP_PREFIX) && directDevtoolTool) return undefined;

    return { block: true, reason: "Only DevTool-approved tools are available; direct shell/SCM/providers are disabled" };
  });

  pi.on("tool_result", (event) => {
    if (event.toolName !== MCP_PREFIX + "code_context") return;
    const path = pending.get(event.toolCallId);
    pending.delete(event.toolCallId);
    if (path && !event.isError && parseCodeResult(event.content)) {
      completedScopes.add(path);
    }
  });
}
