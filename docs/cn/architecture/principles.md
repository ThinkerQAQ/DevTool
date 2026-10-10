# 架构原则

[English](../../architecture/principles.md) · [架构概览](overview.md)

## 极简 Core

Core 负责项目发现、配置、扩展加载、路由和生命周期，不承载 Docker、GitHub 或特定项目业务。

**判断：** 换一个 Provider 就不需要的 Core 修改，大概率放错了层次。

## 成熟组件优先复用

新功能先调研开源代码、SDK、框架、服务和标准协议。比较功能覆盖、维护情况、许可证、可移植性和扩展点。没有合适的成熟方案时才自研。

## 配置选择，代码实现

`.devtool.toml` 只负责扩展、Provider、Service、Profile 的声明式绑定，不成为 Shell 工作流或第二套编程语言。

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

## 稳定能力、可替换实现

Capability 表达开发意图，Service 定义 Contract，Provider 实现 Contract。底层多一个 API 不应自动多一个 Agent Tool。

## 项目语义由 Project Extension 管理

`build`、`verify`、`package` 是项目声明的能力。DevTool 自身不能享受特殊开发通道。先通过 `project inspect` 发现命令。

## Portable 与 Native 分离

构建、容器与打包由可移植 Runtime 管理；USB、ADB、设备与操作系统集成归 Native。不为了统一抽象而抹去边界。

## 生命周期和副作用

谁创建进程/资源，谁负责取消与清理。密钥和外部写操作必须经过明确的 Provider，不引入隐藏回退路线。

## 小步修改

优先根因修复，拒绝补丁式历史兼容。改一项、检查 diff、commit、push，阶段完成后使用 DevTool 集中验证。

## 文档

写当前已验证行为；教程、操作指南、概念、参考手册分别组织。英文默认，中文同步。
