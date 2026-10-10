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
| `document_context` | Markdown 结构、分段审查和关联上下文 |
| `diagram_context` | Mermaid/PlantUML 图表分析与验证 |
| `scm_publish` | 通过选定 SCM Provider 提交发布 |
| `project_<command>` | 调用 Project Extension 定义的命令 |

实际参数应以 MCP 返回的 Tool Schema 为准。

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
