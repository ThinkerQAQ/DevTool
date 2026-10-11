import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

// No-model RPC smoke probe. Report actual Pi-registered tool names.
export default function (pi: ExtensionAPI) {
  pi.registerCommand("dv2probe", {
    description: "List connected DevTool tools for the integration smoke test",
    handler: async (_args, ctx) => {
      const registered = pi.getAllTools().filter((tool) => tool.name.startsWith("mcp__devtool__"));
      const active = pi.getActiveTools().filter((name) => name.startsWith("mcp__devtool__"));
      const exposure = Object.fromEntries(registered.map((t) => [t.name, t.exposure]));
      ctx.ui.notify("DV2_TOOL_STATE=" + JSON.stringify({ active, exposure }), "info");
    },
  });
}
