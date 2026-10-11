# 远程执行性能与结构化日志

DevTool 负责开发意图：构建、验证、Code/Document/Diagram/Log Context、SCM、Git Worktree。Desktop Commander 负责经授权的**远程操作系统和 GUI**。能够使用现有 DevTool Capability 时，优先复用该能力，而不是不断调用 `start_process` 和 `read_process_output`。目前经 Desktop Commander 使用 DV2 仍需要外层远程调用；这里**并不声称**已经直连 ChatGPT DevTool MCP。

## 用 DevTool 分析 Desktop Commander

```json
{"name":"log_context","arguments":{"objective":"Analyze remote tool invocation counts and latency","source":{"kind":"journald","scope":"user","unit":"desktop-commander.service","since":"2026-10-11T09:00:00+08:00","max_entries":14000},"limit":10}}
```

`intelligence.log.local` 继续读取**未经删减的详细 journald**。识别到 Desktop Commander Remote 事件后，额外返回 `log.tool_calls`：

- 每类工具的 `received`、`completed`、`failed`，反映远程传输事件，不是 shell 命令的退出码。
- `duration_samples`、`p50_ms`、`p90_ms`、`max_ms`，统计请求到完成事件的耗时。
- `correlation=approximate_fifo_by_tool_name` 表示**近似匹配**：完成日志没有 Call ID，只能按同名工具先进先出匹配；并发调用无法确保精准对应。时间窗口内的完成次数也可能大于该窗口的发起次数。
- 请求参数包含 `timeout_ms` 不等于真正超时。关键词搜索、日志等级都不能直接代表工具调用失败次数。

**保留 Desktop Commander 详细日志**，不修改其日志级别、保留周期或鉴权。结构化统计不额外输出完整命令参数，以免扩大敏感内容暴露范围。

## 可复现 A/B

```bash
python3 scripts/benchmark-project-invocation.py \
  --project /path/to/DownKit-worktree \
  --devtool /path/to/compiled/devtool \
  --repeats 3
```

A 每次新建 DevTool CLI 进程执行 `verify`；B 只建立一个官方 MCP stdio 会话，连续调用相同项目的 `project_verify`。两组调用交错，任何验证失败都视为基准失败。

2026-10-11 WSL 实测：

| 指标 | 重启 CLI（A） | 持久 MCP（B） |
| --- | ---: | ---: |
| 次数 | 3 | 3 |
| DevTool 进程启动次数 | 3 | 1 |
| 热缓存中位验证耗时 | 2,145 ms | 2,055 ms |
| 三次合计（B 含 114 ms 初始化） | 6,627 ms | 6,445 ms |

中位改善约 **4.2%**，总耗时改善约 **2.7%**。这是**本地流程耗时**，不是 ChatGPT 到 Desktop Commander 的网络耗时。第一次冷缓存 CLI 验证花了 44.61 秒，因缓存/初始化影响未纳入热缓存 A/B。其他项目并发可能影响测量结果。

同一天的近 3 小时日志包含约 1,379 次工具请求：`start_process` 822 次（59.6%）、`read_process_output` 375 次（27.2%），其他 182 次（13.2%）。这是多个并行项目的历史流量画像，**不能当成优化前后的因果对比**。

## 真实进程盘点（2026-10-11）

Desktop Commander Remote 当前只有两个必要的 Node 进程（远程连接与本地 MCP）。另发现 BlogContent 的 `bash → rg → head` 三进程链，已无进展超过 65 分钟：`rg -l` 没有给定搜索路径，在 Desktop Commander 的非交互 Shell 中从**未关闭的 socket 标准输入**等待数据，`head -40` 则一直等上游输出。确认 PID 与父子关系后只结束这三个无效进程，未重启服务，也没有对 Desktop Commander 的共享进程组发信号。systemd 任务数从 51 降至 48。

非交互命令应明确搜索目录，并为可能长期执行的操作设置超时：

```bash
timeout 10s rg -l 'some heading' docs
```

不需要 stdin 的操作可使用 `</dev/null`，但不能套用到真实交互任务；停止任务之前确认其拥有的 PID 或独立进程组，**禁止结束 Desktop Commander 共享进程组，也不要全局 `pkill chrome`**。

最近三小时的另一组抽样中，约有 817 次 `start_process` 和 366 次 `read_process_output`（分布在 249 个 PID 上，其中 178 个 PID 只查询一次）。大量命令已经使用 DevTool，不能将所有开销归因于重复轮询，更没有数据支持再造一套远程 MCP 进程管理服务。

## 建议工作流

1. 开发操作优先调用现有 `project_verify`、`project_build`、`workspace_create`、`scm_publish` 及各 Context。
2. 现有本地 DevTool 会话可复用时再复用，不为此新增一层远程 MCP；有同步结果时不要额外轮询 PID。长任务仍需合理读取状态。
3. Desktop Commander 保留操作系统、GUI、环境诊断等 DV2 没有抽象的能力。
4. 不为减少几次 Shell 调用而自研新的通用 Agent Runner 或 MCP 协议层。直接接入 ChatGPT 需要用户明确配置对应 MCP 连接。

端到端网络性能还需单独埋点，当前本地 A/B **不支持宣称远程加速几十倍**。

## 真实远程往返与命令耗时（2026-10-11）

使用 Desktop Commander 自带的 `verbose_timing` 测量简单 `start_process` 内部耗时，同时独立记录外层工具调用往返。连续五次运行 `printf 'REMOTE_PROBE_OK'`，内部执行耗时为 8–9 ms；外层请求到响应为 2,577–3,388 ms（中位数 2,731 ms）。外层耗时包含客户端编排、传输及响应处理，**不能全部认定为网络延迟**。

另外将三个相同的只读操作（输出标记、检查 DevTool 配置文件、读取 main 提交号）交错 A/B。A 为三个独立的 Desktop Commander `start_process` 调用；B 为单次远程调用执行这三个很短的检查，输出结果一致。

| 轮次 | 3 次独立远程调用 | 1 次合并远程调用 |
| --- | ---: | ---: |
| AB | 7,781 ms | 2,571 ms |
| BA | 7,518 ms | 2,560 ms |
| 中位数 | 7,650 ms | 2,566 ms |

本次极短任务中合并调用约 **2.98 倍快**（总耗时降低 66.5%）。这是实际 ChatGPT/Desktop Commander 环境中的整条外层调用链路耗时，不是本地单机微基准；并发和网络状况不同会影响结果。

**建议：** 只合并无需 Agent 在中间判断的连续确定性步骤，设置明确目录、有限输出和合理的 `set -e` / `pipefail` 错误处理；构建、验证、SCM 仍使用已有 DevTool 项目命令。不要将任意长任务塞入不透明 Shell 脚本，不必合并 Desktop Commander 的两个正常 Node 进程，更不新增第二套远程 MCP 服务。原始 journald 日志维持不变。
