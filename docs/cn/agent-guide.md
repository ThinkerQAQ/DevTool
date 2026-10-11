# Agent 开发指南

[English](../agent-guide.md) · [仓库 Agent 约定](../../AGENTS.md)

DevTool 是统一的开发能力入口，不是让 Agent 直接调用所有底层 Provider 的工具集合。

## 发现项目

```bash
DEVTOOL_PROFILES=local devtool config validate
DEVTOOL_PROFILES=local devtool project inspect --json
DEVTOOL_PROFILES=local devtool init --json
```

先看 Project Extension、Service 绑定和可用命令。就绪检查出错时优先修复环境或选择已有 Profile。

## 连接 MCP

```bash
devtool agent mcp --context codex
```

主要工具：

| 能力 | 用途 |
| --- | --- |
| `code_context` | 组合仓库索引与实时代码语义 |
| `workspace_apply_patch` | 可选：基于代码上下文凭据对一个已有文件执行 Git 补丁 |
| `document_context` | Markdown 结构、分段审查和关联上下文 |
| `diagram_context` | Markdown 内嵌图（配合 Document Context）或独立 Mermaid/PlantUML 源码的分析与渲染校验 |
| `log_context` | 对文件日志、journald 进行只读结构化查询 |
| `scm_publish` | 通过选定 SCM Provider 提交发布 |
| `project_<command>` | 调用 Project Extension 定义的命令 |

修改 Markdown 先调用 `document_context`；修改 Markdown 内的 Mermaid/PlantUML 图同时调用 `document_context` 和 `diagram_context`；独立 `.mmd` / `.mermaid` / `.puml` / `.plantuml` 源码只需 `diagram_context`；新图或变更图还必须在临时 Git 预览中通过已有 `diagram-render` Provider 的渲染校验；查询日志必须通过 `log_context`，不能用源码编辑工具替代。可选的 `workspace_apply_patch` 在执行前校验相应上下文。

实际参数应以 MCP 返回的 Tool Schema 为准。

查询特定文件或目录时，为 `code_context` 指定项目相对路径，例如 `"path":"go"`。索引检索先按目录过滤，再排序和截断，不再返回 Android 等目录外的结果。查询符号引用时还应传入 `symbol` 和具体文件路径。CodeGraph Provider 会拒绝绝对路径、无效路径及逃出项目根目录的路径。

## Agent 一次性接入，项目独立配置

每个 Agent 客户端只需要配置所连接的 DevTool MCP 实例，以及一次工具优先级偏好。不要在 PhotoWaypoint、IDFlow、BlogCTL 等项目的 `AGENTS.md` 里复制 Provider 名称或工具映射表。

在 ChatGPT Project Instructions 或其他 Agent 客户端的全局指令中配置一次：

> 在已连接 DevTool 的软件工程任务中，优先选择匹配的 DevTool MCP Capability。以动态发现的工具描述与 Schema 为准，不维护硬编码 Provider 列表。查询无结果时先调整参数；能力不可用或无法满足需求时再使用 Desktop Commander。系统与 GUI 操作直接使用适合的工具，不要无限重试。

**项目侧**只需通过 `.devtool.toml` 声明 Extension 与 Provider，在项目目录按所需 Profile 启动 DevTool MCP；客户端通过 `tools/list` 动态发现能力。CodeGraph、Serena、Sourcegraph 或 SCM Provider 由项目配置选择。项目 `AGENTS.md` 只写项目特有约定，不承担 DevTool 工具目录职责。

注意：这是一条**工具选择偏好**，不是强制权限。Desktop Commander 可用时，指令和 Tool Description 不能保证 Agent 绝对不绕过 DV2。应使用不带逐项提示的真实工程任务衡量首选率、结果正确性、耗时和无必要的 Fallback。

## 开发循环

1. 明确开发意图，使用 DevTool 查询相关代码与 Contract。
2. 优先选择成熟组件或已有 Provider，检查配置能否直接实现。
3. 项目逻辑放 Project Extension，集成逻辑放 Provider，跨项目机制才放 Core。
4. 每次完成一项连贯修改后检查 diff、commit、push。
5. 阶段完成后运行项目声明的验证命令。
6. 报告真实验证结果，明确未执行的步骤。

## 问题处理

缺程序时检查环境或切换 Profile；授权不足时使用 Credential Provider，不得提交密钥；索引与分支不一致时重新检查活动工作区并参考 LSP 实时诊断；缺项目命令时重新执行 `project inspect`。远程网关必须启用恰当认证。

CodeGraph/Sourcegraph 和 Serena/LSP 只是 Provider；切换它们不应改变 Agent 的 `code_context` 接口。

参阅[架构原则](architecture/principles.md)和[CLI 命令](reference/cli.md)。
