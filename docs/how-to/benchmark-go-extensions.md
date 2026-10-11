# Benchmark local Go Provider loading

DevTool builds local Go extensions as independent processes. Compiled binaries are cached, but the host still checks whether their source or transitive dependencies changed before running them.

To reduce repeated Go dependency discovery, a project opening now collects the configured local Go extension packages per module and uses one `go list -deps -json` per module. Every Provider keeps its individual dependency set for cache invalidation. Remote access and MCP transports are unchanged.

## Reproduce the benchmark

Build the baseline from an older DevTool commit and the candidate from the working branch. Run both binaries against the **same** checkout with a warm local extension cache:

```bash
python3 scripts/benchmark-extension-cache.py \
  --project /path/to/DevTool \
  --baseline /tmp/devtool-baseline \
  --candidate /tmp/devtool-candidate \
  --repeats 6 \
  --output /tmp/devtool-go-provider-benchmark.json
```

The script alternates A/B and B/A, times `config check`, `project inspect --json`, and a real `agent mcp --context agent` initialize/tools-list exchange, and records the number of `go list` invocations using a temporary PATH shim. It does not mutate the project, modify Desktop Commander, use a network service, or bundle a custom Go toolchain. It fails if any tested command fails.

## Observed warm-cache comparison

WSL Ubuntu, October 11, 2026; 6 runs per variant for the two CLI commands and 3 runs per variant for MCP initialization. Original baseline `82910ce` versus the batched implementation:

| Command | Baseline median | Batched median | Relative reduction |
| --- | ---: | ---: | ---: |
| `config check` | 2,154 ms | 356 ms | 83.5% |
| `project inspect --json` | 2,194 ms | 301 ms | 86.3% |
| `agent mcp --context agent` (3 runs) | 2,294 ms | 332 ms | 85.5% |
| `go list` per invocation | 25 | 2 | 92% |

This measures **local DevTool startup/provider discovery**, not ChatGPT/Remote Desktop Commander round trips or actual build/download throughput. Cold build/cache misses may still require full Go compilation.

## Cache correctness

The per-Provider dependency snapshot includes its transitive Go imports and local module roots. Each Provider still checks file timestamps and local `go.mod`/`go.sum` against its own cached binary. Modifying another Provider's unrelated source must not invalidate it; modifying a transitive dependency must invalidate it. The automated regression test creates independent fixture providers to verify both conditions against the previous individual dependency scan.

If an aggregate dependency scan fails, DevTool falls back to the original per-Provider resolution instead of silently accepting an incomplete dependency graph.
