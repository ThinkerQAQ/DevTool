# Pi 作为 DevTool V2 的统一受控 Agent 入口

[English](../../architecture/pi-agent-gateway.md) · [架构概览](overview.md) · [Agent 开发指南](../agent-guide.md)

> **状态：方案（2026-10-11）。** 本文讨论拟议架构，不能将尚未实现的 Pi/DV2 集成写成已有功能。本次分支只修改文档。

## 决策

优先使用上游 **Pi** 承担统一 Coding Agent 执行层，保留 **DevTool V2（DV2）** 的工程控制面职责。先通过 Pi Extension + 原生 MCP 做最小验收，**不 Fork Pi、不重写 DevTool Core，也不再新建一个调度系统**。

```text
本地开发者 / Codex ────> Pi CLI 或 Headless ──┐
                                             ├──> Pi Policy Adapter
网页 ChatGPT ──> Desktop Commander ──> Pi ───┘          |
                                                      DevTool MCP
                                                 /       |        \
                                           CodeGraph  Serena/LSP  SCM/Project
```

- **本地入口**：可以直接使用 Pi 的终端交互，也可以由本地 Codex 把范围明确的实现任务委派给 Pi。
- **远程入口**：网页 ChatGPT 负责讨论、拆分任务与验收，通过 Desktop Commander 在授权 Ubuntu 机器上的指定 Worktree 启动 Pi；传入任务和执行边界。
- **统一执行**：Pi 决定何时使用工具、执行代码编辑；DV2 以 Capability/Service/Provider 提供代码上下文、验证、工作区和 SCM。
- **限制**：如果外层 Codex 或 ChatGPT 的 Desktop Commander 仍能不受限制地执行 shell、修改文件、直接操作 GitHub，就仍能绕过 Pi。仅仅约定“先调用 Pi”并不是系统级强制。

## 本地源码与实际验证结果

| 项目 | 已核实事实 |
| --- | --- |
| DevTool 源码 | `/home/zsk/code/DevTool`；检查时 `main` 干净，HEAD 为 `388790d` |
| 本地已安装 DV2 | `0.1.0-dev`，内置 Commit `89a45fa`；**与当前源码不一致**，PoC 前必须处理 |
| DV2 Provider | `devtool code doctor`：CodeGraph `0.21.0` READY；Serena `1.7.0` READY |
| DV2 MCP 契约 | 实际列出 `code_context`、`workspace_create`、`scm_checkpoint`、`scm_publish` 等 |
| DevTool 源码分析 | 真实调用过 DV2 `code_context`，返回索引结果及 LSP 诊断；查询不存在的符号时部分 references 降级，不能把所有 Provider 都算作成功 |
| Pi 源码 | `/home/zsk/code/pi`，克隆自 `earendil-works/pi`，Commit `ea448f4`（2026-10-10）；不是 Fork |
| Pi 源码分析 | 使用**临时本地研究配置**为 Pi 引入 DV2 Provider；`config validate`、`project inspect`、`code doctor`、真实 `code_context` 调用成功 |
| Pi 可执行 Agent | **未安装 Pi CLI、未配置/验证模型授权、未运行 Pi Agent 与策略拦截**；以上不等于端到端集成完成 |

已分析源码位置：

- DV2：`core/project/discovery.go`、`extensions/capability/code/code.go`、`extensions/context/code/code.go`、`extensions/capability/workspace/workspace.go`、`extensions/capability/scm/scm.go`、`adapters/extensionloader/go.go`。
- Pi：`packages/coding-agent/docs/{mcp,extensions,cli,cli-integration,rpc,security}.md`、`packages/agent/src/agent-loop.ts`。
- 上游：[Pi 源码](https://github.com/earendil-works/pi/tree/ea448f454af4)、[DevTool](https://github.com/ThinkerQAQ/DevTool)。

## 当前能力与架构缺口

**DV2 已实现：** `devtool agent mcp --context agent` 通过 MCP 暴露稳定意图工具。`code_context` 接受必填的 `objective`，以及可选的 `symbol`、项目相对路径 `path`。工作区来自**当前 DV2 项目会话**，不能随便传一个根目录切换到其他仓库。源码显示，当 `symbol` 与 `path` 都未指定时，只返回索引发现结果；指定范围后才可能组合调用实时 LSP。因此不能规定每次都必须调用 CodeGraph + LSP 两者。

**Pi 已实现：** 当前上游源码支持原生 MCP（项目级 `.pi/mcp.json`、用户级 MCP 文件）、`exposure`、`toolExposure`、`tool_call` 拦截、`--tools`/`--exclude-tools`、Print/JSON/RPC/SDK。但 Pi 的扩展与进程拥有同样的系统权限，**它没有内建的操作系统级权限隔离**。即使某 MCP 工具没有在模型可见列表中暴露，也可能经 `codemode` 被间接调用，不能把“隐藏”误认为“禁止”。

**尚未实现：** DV2 专用 Pi Policy Extension、编辑前必须取得上下文、外层 Agent 的权限隔离、可信调用凭据、持久 Runner 与并发 Worktree 协调。

## 目标契约

1. **工作区身份**：每次任务绑定 Project Root、Git Common Directory、Worktree 路径、Branch/HEAD、Task ID、允许修改的路径范围。每个会话只能写自己的 Worktree；拒绝符号链接/路径逃逸。相关文件、分支变化应使先前上下文失效。
2. **上下文门禁**：修改源码前必须有同一任务、同一范围内成功的 DV2 `code_context` 调用；文档修改可走 `document_context`。记录 CodeGraph/LSP 的实际调用、降级与跳过原因；强依赖失败时 Fail Closed，明确声明为可选的服务才能按策略降级。
3. **工具门禁**：禁止直连底层 Provider、绕过 DV2 的 Git/构建命令。严格 PoC 先不开放 Pi `bash` 及未经审计的 `codemode`，仅开放必要的文件编辑工具与 DV2 MCP 工具。由 Pi `tool_call` Hook 在执行前拦截违规调用，包括修改策略本身。
4. **发布控制**：小步提交推送使用 DV2 `scm_checkpoint`；创建 PR 使用 `scm_publish`。合并须显式指定，GitHub 的分支保护/CI 是独立防线。
5. **调用证明**：审计 Task/Worktree、Pi Session、Tool Call ID、DV2 Capability、最终 Provider、返回状态、降级原因、Commit、PR URL。Agent 自述“已通过 DV2”不算真实证据。
6. **外层权限**：要让本地 Codex、网页 ChatGPT 真正不能绕开 Pi，就必须收窄它们经 Desktop Commander 获得的文件与命令能力，只开放受控 Runner 或使用隔离账户/容器；Pi 内部 Hook 不能约束它们本身。

策略做成一个可复用的轻量 Pi Extension + 声明式配置；项目 Provider 仍由 `.devtool.toml` 选择。TOML 不承载动态工作流脚本。

## Pi MCP 配置示例（仅方案，尚未部署）

项目级 `.pi/mcp.json` 只应用于已授信项目，也可改用用户级配置。Pi 会检查项目授信后才加载项目 MCP 配置。

```json
{
  "mcpServers": {
    "devtool": {
      "command": "devtool",
      "args": ["agent", "mcp", "--context", "agent"],
      "env": { "DEVTOOL_PROFILES": "local" },
      "exposure": "hidden",
      "toolExposure": {
        "code_context": "direct",
        "document_context": "direct",
        "workspace_create": "direct",
        "scm_checkpoint": "direct",
        "scm_publish": "direct",
        "project_verify": "direct"
      }
    }
  }
}
```

注意：`project_verify` 属于 Project Extension 提供的命令，不一定在所有仓库存在。这里的 `local` Profile、工具清单也必须和目标项目实际配置一致；Pi 源码仓库临时研究配置并没有 `local` Profile，不能照抄。不要将此配置永久写入上游 Pi 源码。

在 **Extension 已实现并验证后**，可以用以下命令测试严格模式：

```bash
cd /path/to/assigned/worktree
pi --mode json --no-session \
  --tools 'read,edit,write,mcp__devtool__*' \
  --exclude-tools 'codemode,tool_search,bash,powershell' \
  --extension /path/to/dv2-policy.ts \
  "通过 DevTool 完成已批准的、限定范围内的任务"
```

Hook 仍需校验生效的工具列表、禁止策略配置被修改，并跟踪直接及嵌套调用。命令行参数自身不能保证编辑前有代码上下文，也不能阻止外层启动另一个不带 Extension 的 Pi。一次性远程任务优先 JSON 事件流；只有确实需要复用会话时才启用 RPC。RPC 的 `prompt` 被接受**不代表任务完成**，应以 `agent_settled` 为准。

## 分阶段落地与验收

| 阶段 | 范围 | 通过标准 |
| --- | --- | --- |
| 0：基线 | 对齐 DV2 本地版本、固定 Pi 源码/发行版、检查项目授信、模型授权及真实 MCP Schema | 版本、Project/Provider、工具契约可复现 |
| 1：策略扩展 | 仅做一个 Pi Extension；失败关闭的编辑门禁、工具白名单、Worktree 隔离和结构化日志 | 上下文→编辑→DV2 验证成功，直接绕过调用失败 |
| 2：双入口 | 本地 Pi / Codex 委派；网页 ChatGPT→Desktop Commander→Pi | 两条入口产出相同的调用证明，禁止无管理的源码写入 |
| 3：并发 | Worktree 单写者、Task ID、并发上限、取消与清理 | 两个 Worktree 无串写，重试/取消/重复任务行为明确 |
| 4：推广 | 复用到 PhotoWaypoint、IDFlow；按需评估 RPC | 用实测成本、耗时、错误率决定是否推广 |

在阶段 0–2 **不引入** 自研常驻守护进程、队列、调度平台或 Pi Fork。GoTiny 已覆盖其他 Agent Runner 语义，任何持久执行能力都要先检查是否重复建设。

## 必须通过的负向测试

- 未经过 DV2 `code_context` 就编辑 Go/TypeScript → 阻止；同一范围正确获取上下文后 → 允许。
- 请求直接执行 `git push`、`go test`、CodeGraph/Serena、shell 逃逸或修改策略文件 → 阻止，或仅在已配置相应 DV2 Capability 时走 DV2。
- DV2 MCP 断连、索引/实时 Provider 失败 → 明确报错；绝不悄悄回退到普通 shell。
- 更换 Branch、Worktree、相关源码 → 必要时失效上下文；严禁沿用其他工作区的凭据。
- 修改 `--tools`、加载 `codemode`、关闭 Extension 或通过 Desktop Commander 直接编辑 → 单独校验**外层权限边界**，不能假称 Pi 能约束这些路径。
- 两个并发任务与一个重复请求 → 不跨 Worktree 修改、不重复发布。
- 中途取消或杀掉 Pi → 不产生虚假的成功记录，不得无意发布半成品。

## 是否值得继续

先衡量两条入口的正负测试结果、额外模型调用、耗时及费用。对于构建、验证、信息查询等确定性操作，可以继续直接调用 DV2，不必再增加一轮 Pi Agent 推理。只有在扩展机制存在**具体且无法绕开的限制**时，才考虑 Fork Pi。达到验收门槛之前，这篇文档只是方案，不意味着所有仓库应该立刻迁移。
