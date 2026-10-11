# 架构概览

[English](../../architecture/overview.md) · [中文目录](../index.md)

## 一句话定义

DevTool 是工程开发控制面：上层表达开发意图，配置将稳定能力绑定到具体实现。

```text
开发者 / Agent / CI
        |
      CLI / MCP
        |
 Project Extension / Capability
        |
    Service Contract
        |
  配置选中的 Provider
        |
 Git / LSP / Docker / Dagger / ...
```

## 分层职责

| 层 | 负责 | 不负责 |
| --- | --- | --- |
| Core | 项目发现、扩展注册/加载、路由、策略、生命周期 | MCP 底层协议、厂商细节与项目业务 |
| Project Extension | build、verify、package 等项目命令 | 具体基础设施实现 |
| Capability | code_context、scm_publish 等稳定开发意图 | 原始 Provider 方法 |
| Service | 可替换能力的 Contract | 项目 UI 和业务流程 |
| Provider | CodeGraph、LSP、Dagger、GitHub 等集成 | 跨 Provider Core 策略 |

## MCP 协议与传输

[官方 Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) 负责 Server、stdio/Streamable HTTP、Client Bridge、协议版本协商、请求关联与取消。DevTool 自身负责**开发意图、项目感知路由、可配置 Provider 及上下文修改策略**，不再额外实现 MCP JSON-RPC 协议层。

旧版 `2025-06-18` 客户端与新版 SDK 客户端可协商兼容。一次性修改令牌仍存储于活动 DevTool Gateway 的内存；远程 HTTP 入口保留 Bearer Token 认证。

## 代码智能

`code-indexed` 提供仓库索引/结构信息，默认 CodeGraph，也可切换 Sourcegraph；`code-realtime` 是可选 LSP 服务，默认 Profile 当前没有配置。现有 `code_context` 仅返回索引结果，并明确标记实时能力未配置；后续将改为正交代码工具，避免固定编排多种操作。

文档和图表能力采用相同分层。Merman 图表智能复用成熟的 [jrpc2](https://github.com/creachadair/jrpc2) LSP 传输与 JSON-RPC Client，负责消息分帧、请求关联和服务端回调；DevTool 仅保留图表专用的符号与诊断映射。底层 `merman-lsp` 可执行文件仍通过 `.devtool.toml` 配置替换，不再维护自研 LSP 帧解析器。

## 环境和生命周期

构建、测试、打包等可移植工作通过环境或运行时 Provider 执行；USB、ADB、物理设备属于 Native 能力。谁申请资源谁负责取消和清理。

## 自举

DevTool 使用普通 Project Extension 构建、验证、打包自己，不存在独立的隐藏通道。候选版本携带版本/源码身份，验证后才激活。

修改功能前优先复用 SDK 或既有 Provider；配置不够时扩展 Provider；项目行为在 Project Extension；只有通用机制才改 Core。详见[架构原则](principles.md)和[自举指南](../how-to/self-host.md)。
