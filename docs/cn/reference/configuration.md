# TOML 配置参考

[English](../../reference/configuration.md) · [中文目录](../index.md)

DevTool 从项目根目录解析 `.devtool.toml`。TOML 负责声明与装配，Extension/Provider 代码负责行为。

## 项目信息

```toml
version = 1

[project]
name = "DevTool"
```

## 扩展注册

```toml
[extension.project]
loader = "go"

[extension.project.loader_config]
module = "./devcontrol"
package = "./cmd/provider"
```

扩展参数使用 `[extension.<name>.settings]`。注册扩展不等于启用它，还需要绑定 Service。

## Service 绑定

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

# 可选。安装 LSP Provider 扩展后再配置。
# [service.code-realtime]
# provider = "<你的 LSP Provider ID>"
```

仓库也定义 workspace、environment、tooling-environment、portable-runtime、document-context、diagram-context、credential、scm 等服务。具体以[实际 TOML](../../../.devtool.toml)为准。

## Profile

```toml
[profile.local.service.environment]
provider = "environment.local"

[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

```bash
DEVTOOL_PROFILES=local devtool project inspect --json
DEVTOOL_PROFILES=local,sourcegraph devtool config validate
```

## Markdown / 图表覆盖范围

项目把已有的 Markdown 和 Marksman Provider 的作用域扩展至整个仓库，根目录的 `README.md`、`AGENTS.md` 也可以先通过 `document_context` 检查，再执行受控修改：

```toml
[extension.document-markdown.settings]
roots = ["."]

[extension.document-realtime.settings]
workspace_roots = ["."]
```

Markdown 修改必须使用 `document_context`；Markdown 内 Mermaid/PlantUML 图的修改还须调用 `diagram_context`。查询项目日志和 journald 使用只读 `log_context`。

## 代码工作区

```toml
[code]
workspaces = ["."]
```

CodeGraph 负责索引/结构信息。实时语义需要额外安装并配置 LSP Provider；默认 Profile 尚未启用。更换 Provider 时仍保留稳定 Capability。

## 受控 Agent 修改

```toml
[agent.policy]
enabled = true
protected_branches = ["main", "master"]
max_patch_bytes = 262144
context_ttl_seconds = 600
```

默认关闭；启用后，现有 MCP Gateway 新增 `workspace_apply_patch`。修改既有文件前必须通过**同一个 Gateway 进程**取得一次性 `code_context` 凭据。`main` 和 `master` 始终受保护。操作方法参见[受控代码修改](../how-to/controlled-edits.md)。

## 验证

```bash
devtool config path
devtool config validate
devtool config check
devtool project inspect --json
devtool init --json
```

`config validate` 是不加载扩展的快速 TOML 和基础规则校验。`config check` 使用**与 `agent mcp` 相同的 Host 和官方 MCP Gateway**，实际加载配置的 Provider、检查已选服务与依赖、检测 MCP 工具名冲突及工具定义错误；不执行项目构建/验证，不测试凭据的可用性，不运行 Provider readiness，也不会监听端口。需要完整环境就绪检查时再用 `init`。

首次 `config check` 可能触发 Go-module 扩展编译而较慢，后续可使用 Go 缓存。Provider 设置与凭据仍由对应 Provider 负责，不在 Core 新建依赖管理器。

不要在 TOML 中写 Shell 管道、业务算法、明文密钥或厂商特例；业务属于 Project Extension，集成属于 Provider。
