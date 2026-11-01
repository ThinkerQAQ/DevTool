# DevTool

[English](README.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

开发一个项目时，经常需要在 Git、构建工具、代码搜索、语言服务器和开发环境之间切换。开发者、CI 和 AI Agent 都要分别了解这些工具的接口、配置方式和使用时机，重复集成的工作不少。

DevTool 曾尝试在现有工具之上增加一个**理解项目的轻量控制层**。每个项目通过 `.devtool.toml` 声明能力和 Provider，开发者、CI 或支持 MCP 的 AI Agent 便可以通过相对统一的入口检查项目环境、查看工程能力，并调用 Project Extension 提供的操作，而不是直接耦合每一种底层工具。

**本项目曾经归档，目前为整理文档恢复了写入；它仍是未完成产品化的实验项目，并非成熟的软件产品。** 它保留了实际实现、测试和设计取舍，但没有达到最初设想的通用、易部署工程平台目标，**没有稳定安装包、持续维护或生产兼容性承诺**。配套的 [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) 是独立的 Docker 工具链项目；下面的本地演示不依赖它。

## 2. Quick Start

先从源码运行一个最小流程：检查配置、识别当前项目、验证代码。使用仓库自带的 **local Profile**，不要求先拉取可能受限的 Docker 镜像。**这只是源码体验，不等于完成生产部署。**

### 2.1. 准备环境

安装 Git 以及 [go.mod](go.mod) 声明的 Go 工具链。以下命令面向 Linux/macOS Bash；Windows 可以使用 Git Bash 或 WSL。Local Profile 可以在宿主机执行，但某些可选 Provider 仍需要外部程序或凭据，DevTool 不会自动安装。

### 2.2. 克隆并校验配置

```bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
export DEVTOOL_PROFILES=local
go run ./cmd/devtool version --json
go run ./cmd/devtool config validate
```

版本命令显示当前源码/构建信息（源码运行可能显示 `dev`）；配置检查成功时输出 `VALID .../.devtool.toml`。**配置语法正确，不代表所有外部 Provider 已经就绪。**

### 2.3. 查看项目能力

```bash
go run ./cmd/devtool config path
go run ./cmd/devtool project inspect --json
```

JSON 应显示当前项目名称 **DevTool**、Project Extension、Service 绑定和声明的命令。确认能看到 `build`、`verify`、`package` 等命令：这些是**当前仓库**定义的功能，不是每个使用 DevTool 的项目都会拥有。

### 2.4. 验证源码

```bash
go test ./...
go -C devcontrol test ./...
```

测试检查已有实现，不会安装全局后台服务。它们也不能证明所有第三方 Provider 都可用。完成配置检查、项目识别和源码测试，就算走通本次最小验收。

### 2.5. 常见问题

- **Go 版本不符合要求：**以 `go.mod` 为准更新工具链，在仓库根目录重试。
- **找不到 `.devtool.toml`：**确认当前位于 DevTool 仓库；其他项目需要自己的配置文件。
- **`VALID` 通过但 Provider 检查失败：**`config validate` 只检查配置，`config check` 和 `init` 才会加载配置好的扩展及外部工具，详见 [Case 1](#41-case-1检查外部-provider)。
- **Docker/GHCR 报错：**首次测试应使用 local Profile，确认当前终端仍保留 `DEVTOOL_PROFILES=local`。
- **某个命令不存在：**先通过 `project inspect --json` 查看当前 Project Extension 支持的命令。

## 3. Configuration

[`.devtool.toml`](.devtool.toml) 负责声明项目身份、扩展、服务绑定及环境 Profile。Profile 决定**选择哪一种 Provider 实现**，不会替你下载或配置缺失的外部工具。

例如在执行命令前选择本地 Profile：

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool config validate
```

其他 Profile 可能依赖 Docker、Dagger、Sourcegraph 或代码智能服务，需要仓库之外的程序、网络和授权。个人凭据不应直接写入提交到 Git 的配置文件。独立的 [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) 提供可复用的工具链构建定义，DevTool 只是按配置选用这些环境。

## 4. Optional Features

### 4.1. Case 1：检查外部 Provider

如果需要进一步确认 Provider 的实际状态，保留 local Profile 并执行：

```bash
go run ./cmd/devtool config check
go run ./cmd/devtool code doctor
go run ./cmd/devtool init --json
```

`config check` 加载扩展、核对 MCP 工具注册；`code doctor` 检查索引及实时语义 Provider；`init` 检查就绪情况，某些配置下还会准备索引。缺少依赖时可能失败，应该查看报告，只配置实际准备使用的能力。**可选 Provider 未就绪，不等于上面的源码体验失败。**

### 4.2. Case 2：用 DevTool 构建自己

当前仓库的 Project Extension 提供自举流程：

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool build
go run ./cmd/devtool verify
go run ./cmd/devtool package
```

候选文件及验证信息写入 `.devtool/out/`。先检查产物，**不要把 `activate`、`rollback` 当成首次验收步骤**：它们会修改本地已安装的 DevTool。这仍然是实验性的开发流程，并非正式版本发布或一键安装。

### 4.3. Case 3：连接本地 MCP 客户端

DevTool 可以通过 **stdio** 暴露实验性的 MCP 网关：

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool agent mcp --context agent
```

在本地 MCP 客户端的服务配置中注册相应命令，并保证进程的**工作目录指向 DevTool 仓库的绝对路径**。如果客户端不支持设置工作目录，可以先运行 `go build -o devtool ./cmd/devtool` 生成可执行文件，并安排它从正确的工程目录启动。实际能看到哪些 MCP 工具，取决于当前 Provider 和权限。不要默认开启网络可访问的 HTTP 网关；远程 HTTP 需要显式认证和独立的安全评估。

### 4.4. Case 4：项目留下了什么

这次探索实际涉及**意图级 MCP 接口、声明式 Provider 切换、代码索引与实时 LSP 上下文路由、Markdown/图表分析、受限 Git Patch 授权，以及构建验证和激活/回退契约**。难点在于如何界定统一接口与不同 Provider 的边界，如何处理不同代码上下文的新鲜度和失败模式，如何限制 Agent 有副作用的操作，以及跨环境集成带来的维护成本。

项目没有解决**跨机器一致的一键初始化、成熟的发布兼容体系、系统化安全验证**等产品问题，也没有证明统一控制面的收益一定超过长期集成成本。这些是**未解决的产品化问题，不是已经证实的停更原因**。仓库保留的是工程探索和可复用经验，而不是一个宣称完成的商业级产品。

## 5. Development

使用 [go.mod](go.mod) 要求的 Go 工具链：

```bash
go test ./...
go -C devcontrol test ./...
DEVTOOL_PROFILES=local go run ./cmd/devtool project inspect --json
DEVTOOL_PROFILES=local go run ./cmd/devtool build
```

CI 覆盖单元测试和本地自举验证。依赖私有镜像或第三方服务的集成测试有专门开关，不能把缺少配置时跳过的测试写成已经通过。开发约定见 [AGENTS.md](AGENTS.md)。**仓库曾经归档，目前仅为更新文档恢复写入；仍暂停功能开发，不承诺持续维护或新增功能。**

## 6. License

原创源码与文档以 [MIT License](LICENSE) **按现状提供**，第三方工具和依赖遵循各自协议。这一实验性原型不提供生产适用性保证，也没有持续安全更新承诺。运行 Shell、Git Patch 或 MCP 相关能力前，应明确允许其访问的文件和凭据；连接 MCP 本身不代表具备完整的沙箱或权限隔离。