# 快速开始

[English](../quick-start.md) · [中文目录](index.md)

本教程以 DevTool 自身仓库为例。其他项目通过自己的 Project Extension 复用相同控制面。

## 1. 准备环境

安装 Git 和 `go.mod` 指定的 Go 版本。具体 Provider 可能还需要外部程序和凭据；DevTool 会报告缺失依赖，但不会自行安装。

```bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
```

优先使用已安装的 `devtool`。首次自举可在 DevTool 源码仓库将以下命令前缀替换为 `go run ./cmd/devtool`。

## 2. 选择执行 Profile

仓库默认使用 Docker 环境，同时提供宿主机运行的 `local` Profile：

```bash
export DEVTOOL_PROFILES=local
devtool config path
devtool config validate
```

正常输出 `VALID <path>/.devtool.toml`。如果失败，检查 TOML 字段、Extension Loader 和 Service 绑定。

## 3. 查看项目能力

```bash
devtool project inspect --json
```

输出包括当前 Project Extension、服务绑定和项目命令。各个项目声明的命令可以不同，不要假设都能执行 `build`。

## 4. 检查 Provider

```bash
devtool code doctor
devtool init --json
```

`code doctor` 检查索引与实时语义 Provider；`init` 还可能准备代码索引，首次运行会更慢。遇到缺依赖或授权错误，先处理环境再重试。

## 5. 通过 DevTool 构建

当前仓库的 Project Extension 提供：

```bash
devtool build
devtool verify
devtool package
```

候选产物写入 `.devtool/out/`，通过验证后才考虑激活。详见[自举指南](how-to/self-host.md)。

## 6. 连接 Agent

在支持 MCP 的 Agent 中注册：

```bash
devtool agent mcp --context codex
```

还可以选 `agent` 或 `claude-code`。Agent 通过 `code_context`、`document_context`、`diagram_context`、`scm_publish` 等稳定能力工作。

接下来阅读[架构概览](architecture/overview.md)与[配置参考](reference/configuration.md)。
