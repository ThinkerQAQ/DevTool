# 代码智能：原生工具优先，不再自研组合路由

[English](../../architecture/code-intelligence.md) · [架构原则](principles.md)

## 当前状态

DevTool 默认通过 CodeGraph 提供索引式代码智能。Serena 已移除；默认配置暂未接入新的实时 LSP。`code_context` 仍可返回索引结果，并在 `not_applied` 中明确提示实时 Provider 未配置。源码补丁门禁仍要求索引与实时上下文同时有效，因此默认配置暂不能授权源码补丁。

这只是迁移的中间状态，不代表 CodeGraph 可以取代 LSP。

## `code_context` 的根本问题

当前 Composite 将仓库发现、符号查找、引用和诊断捆在一起，还自行切词并重排 CodeGraph 搜索结果。这改变了 Provider 原生语义，可能让 `Find`、`Definition` 等泛化符号排到真正目标前面；LSP 初始化失败时，又向本来成功的索引查询附加大段错误文本。

继续补停用词、特殊判断或 fallback 规则，只会增加维护成本。

## 目标架构

- **索引与代码图：** 直接暴露所选 Provider 的原生 MCP 工具，包括 CodeGraph 的符号搜索、Curated Context、调用图和影响分析；DevTool 不改写查询、不自行打分。
- **实时语义：** 复用成熟的通用 MCP–LSP Bridge 管理语言服务器、文件同步、定义、引用和诊断；语言服务器分别隔离故障。Go 使用 `gopls`，Kotlin 单独验证。
- **DevTool 边界：** 仅负责配置化 MCP 发现、命名空间、调用、生命周期及传输。复用现有 `core/agent/mcpbridge` 的动态 `ListTools` / `CallTool`，不重新实现自然语言检索、不手工映射每个命令，也不隐式 fallback 到 Shell。
- **工具暴露：** 通过配置选择 Provider 或其原生 Tool Profile，不在 DevTool 源码、各个项目的 Agent 文档里逐条登记底层方法；保留原生 Schema。
- **源码修改：** 在移除 Composite 前，将现有 `code_context` 令牌门禁解耦为明确的代码操作证据，不能悄悄降低已有校验。

```text
ChatGPT / Local Agent
        |
   DevTool MCP + Tunnel
        |
   配置化原生 MCP
      /          \
  CodeGraph     LSP–MCP Bridge
  原生工具       原生 LSP 能力
                    |
             gopls / Kotlin / ...
```

## 迁移顺序

1. 删除 Serena 和相关配置，保留索引检索，并明确提示 LSP 缺席。**本分支已完成。**
2. 在真实 Go/Kotlin 仓库验证成熟的通用 LSP–MCP Bridge；不在 DevTool 中重写语言服务器管理器。
3. 通过已有 MCP Bridge 动态暴露原生工具，Provider 和工具集合由配置决定。
4. 新链路实际验收通过、源码修改门禁完成解耦后，再删除 `code_context`、自研 CodeGraph `discover` 排序逻辑和 Composite 服务。

## 验收标准

以同一组原生工具调用比较 ChatGPT → Tunnel → DevTool 与 ChatGPT → Desktop Commander → 本机 Provider。分别计量端到端与本机执行耗时，区分冷启动/热态，并验证符号定义、跨文件引用、诊断及不同语言服务器之间的故障隔离。整个本地验收不依赖 GitHub Actions。
