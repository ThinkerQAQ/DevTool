# Pi + DevTool V2 自举实验

[English](README.md)

**状态：实验，不是正式支持的功能。** 目标是让上游 Pi 在 DevTool 的独立 Git Worktree 中开发 DevTool 本身，验证 Pi Hook 强制工程工具调用，同时保持 DV2 的独立性。

## 架构决策

- Pi 负责 Agent Loop、工具选择和执行前的策略 Hook。
- DV2 继续负责 CodeGraph/Serena、Build/Verify/Package、Workspace 和 SCM。
- `../../.pi/mcp.json` 声明哪些 DV2 MCP 工具对 Pi 可见。策略扩展从 Pi 的**实际工具曝光配置**读取名单，不再维护第二份硬编码工具白名单。
- `policy.ts` 强制每个待编辑文件先取得该路径的 `code_context` 成功结果，并同时包含 Indexed 和 Realtime 信息；禁止直接 Shell/Provider/SCM、工作区外写入以及修改策略目录。
- 本实验刻意隐藏 `scm_publish`、`scm_checkpoint`、`project_activate`、`project_rollback`；通过可信 DV2 外层操作提交实验分支。

## 2026-10-11 实测

| 项目 | 结果 |
| --- | --- |
| Pi | Ubuntu 上已安装 1.1.0，源码另位于 `/home/zsk/code/pi` |
| DV2 | 利用 `workspace_create` 创建本实验 Worktree；主分支不受影响 |
| MCP | Pi RPC 最终识别 DV2 的 15 个 MCP 工具；只有配置明确开放的工具为 direct |
| 代码上下文 | DV2 在实验 Worktree 对自身 Go 文件实际返回 CodeGraph Indexed + Serena Realtime，未发生降级 |
| 策略 | 通过 Node 模拟 Pi 的 Hook 事件进行 5 项拒绝/放行测试 |
| 模型执行 | **未执行：** `pi auth check --provider openai-codex` 返回 `not_ready` |
| 版本一致性 | 已安装 DV2 `89a45fa` 与当前源码 `388790d` 不一致，正式验收前需对齐 |

**连接时间问题：** 立即向刚启动的 Pi RPC 发送探针时，DV2 工具列表为空；待 MCP 连接完成后 15 个工具均已注册。因此未来 Runner 要等待 MCP 就绪信号，而不是用固定 Sleep 作为解决方案。

## 复现

```bash
cd /path/to/experiment/worktree
node --test examples/pi-selfhost/policy.test.mjs
DEVTOOL_PROFILES=local devtool code doctor

pi --mode rpc --no-session --approve \
  --extension ./examples/pi-selfhost/policy.ts \
  --extension ./examples/pi-selfhost/probe.ts \
  --tools 'read,edit,write,mcp__devtool__*' \
  --exclude-tools 'bash,powershell,codemode,tool_search'
```

确认 MCP 连接完成后向 RPC 输入 `{"type":"prompt","message":"/dv2probe","id":"probe"}`，检查 `DV2_TOOL_STATE`。这是无需模型的工具注册探针，不等于实际 Agent 已遵守策略。不要加 `--no-extensions`，它会同时关闭 Pi 内置 MCP 扩展。

## 尚未通过的门槛

1. 配置 Pi 自己的模型授权，完成真实的“DV2 理解源码→Pi 编辑→DV2 Build/Verify”工作流。
2. 在真实 Agent 工具调用上验证 Context Gate、断网/Provider 失败、策略修改、Worktree 切换和并发；当前 5 个测试属于模拟 Hook 单测。
3. 处理工作区版本/文件变更后的上下文失效，以及新增文件、跨文件编辑。
4. 防止外层 ChatGPT/Codex 绕过 Pi，需要独立权限边界；Pi Extension 无法限制外层 Shell 或 Desktop Commander。

方案详见单独文档分支的[中文架构提案](https://github.com/ThinkerQAQ/DevTool/blob/docs/pi-devtool-integration-20261011/docs/cn/architecture/pi-agent-gateway.md)。
