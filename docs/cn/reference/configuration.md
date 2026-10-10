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

[service.code-realtime]
provider = "intelligence.lsp.serena"
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

## 代码工作区

```toml
[code]
workspaces = ["."]
```

CodeGraph 负责索引/结构信息，Serena/LSP 负责实时语义。更换 Provider 时仍保留稳定 Capability。

## 验证

```bash
devtool config path
devtool config validate
devtool project inspect --json
devtool init --json
```

前三项检查配置与绑定，`init` 进一步检查 Provider 就绪情况。缺少可执行程序或凭据时，应修复环境而非改 Core 自行安装。

不要在 TOML 中写 Shell 管道、业务算法、明文密钥或厂商特例；业务属于 Project Extension，集成属于 Provider。
