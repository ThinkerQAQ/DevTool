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
| Core | 发现、扩展注册/加载、路由、协议、生命周期 | 厂商细节与项目业务 |
| Project Extension | build、verify、package 等项目命令 | 具体基础设施实现 |
| Capability | code_context、scm_publish 等稳定开发意图 | 原始 Provider 方法 |
| Service | 可替换能力的 Contract | 项目 UI 和业务流程 |
| Provider | CodeGraph、Serena、Dagger、GitHub 等集成 | 跨 Provider Core 策略 |

## 代码智能

`code-indexed` 提供仓库索引/结构信息，默认 CodeGraph，也可切换 Sourcegraph；`code-realtime` 通过 Serena/LSP 获取实时语义。Agent 使用 `code_context`，由 Capability 调度服务，Provider 切换不影响调用方。

文档和图表能力采用相同分层。

## 环境和生命周期

构建、测试、打包等可移植工作通过环境或运行时 Provider 执行；USB、ADB、物理设备属于 Native 能力。谁申请资源谁负责取消和清理。

## 自举

DevTool 使用普通 Project Extension 构建、验证、打包自己，不存在独立的隐藏通道。候选版本携带版本/源码身份，验证后才激活。

修改功能前优先复用 SDK 或既有 Provider；配置不够时扩展 Provider；项目行为在 Project Extension；只有通用机制才改 Core。详见[架构原则](principles.md)和[自举指南](../how-to/self-host.md)。
