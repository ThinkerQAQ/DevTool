# Go Provider 加载 Benchmark

DevTool 的 Go Extension 是独立进程。虽然编译出的可执行文件已经缓存，每次打开项目仍需要判断源码与传递依赖是否变更。

优化后，同一次项目初始化会将相同 Go Module 下的 Provider 包汇总，以一次 `go list -deps -json` 获取依赖信息，但每个 Provider **仍保留独立的依赖集合**，继续按照各自源码和 `go.mod` / `go.sum` 的时间戳判断缓存是否失效。远程连接和 MCP 传输协议不变。

## 如何复现

分别构建旧版 DevTool 和本次分支的 DevTool，并让二者指向**相同项目和热缓存**：

```bash
python3 scripts/benchmark-extension-cache.py \
  --project /path/to/DevTool \
  --baseline /tmp/devtool-baseline \
  --candidate /tmp/devtool-candidate \
  --repeats 6 \
  --output /tmp/devtool-go-provider-benchmark.json
```

脚本交错运行 A/B、B/A，测量 `config check`、`project inspect --json`，以及真实 `agent mcp --context agent` 初始化和工具列表获取，通过临时 PATH 包装器记录实际 `go list` 启动次数。不修改项目文件、不依赖新的网络服务，也不替代原有 Go 工具链。任意验证命令失败，基准随之失败。

## 2026-10-11 WSL 热缓存实测

两个 CLI 场景每组 6 次，MCP 初始化每组 3 次；基准版本 `82910ce`：

| 操作 | 原实现中位数 | 优化后中位数 | 耗时下降 |
| --- | ---: | ---: | ---: |
| `config check` | 2,154 ms | 356 ms | 83.5% |
| `project inspect --json` | 2,194 ms | 301 ms | 86.3% |
| `agent mcp --context agent`（3 次） | 2,294 ms | 332 ms | 85.5% |
| 每次执行的 `go list` | 25 次 | 2 次 | 92% |

这些数据只表示**本地 DevTool 启动和 Provider 依赖发现性能**，不能作为 ChatGPT 到 Desktop Commander 远程网络加速结论。首次编译和缓存失效时仍可能触发 Go 构建。

## 正确性

自动化回归会在临时 Go Module 中构造相互独立的 Provider，比较批量扫描与旧版单独扫描的依赖集合：修改无关包不应触发重建；修改传递依赖必须触发重建。若批量扫描发生错误，则退回原有的单个 Provider 依赖扫描和构建路径，不会把不完整的依赖信息当成有效缓存。
