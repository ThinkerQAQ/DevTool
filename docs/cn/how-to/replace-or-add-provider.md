# 替换或新增 Provider

[English](../../how-to/replace-or-add-provider.md) · [中文目录](../index.md)

Provider 在稳定的 Service Contract 背后实现功能。TOML 负责选择，代码负责行为。

## 切换已有实现

仓库默认使用 CodeGraph：

```toml
[service.code-indexed]
provider = "intelligence.codegraph"
```

Sourcegraph 可通过 Profile 替换：

```toml
[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

验证：

```bash
DEVTOOL_PROFILES=sourcegraph devtool config validate
DEVTOOL_PROFILES=sourcegraph devtool project inspect --json
DEVTOOL_PROFILES=sourcegraph devtool code doctor
```

Sourcegraph 本身需配置正确并可访问。切换不应改变上层 `code_context` 接口。

## 新增实现

1. 阅读现有 Service/SDK Contract，调查是否已有成熟 SDK、Provider 或标准协议。
2. 编写实现该 Contract 的 Extension，把工具参数、凭据和故障处理留在 Provider。
3. 在 `.devtool.toml` 注册 Extension 和设置。
4. 通过基础配置或 Profile 绑定 Service。
5. 执行 `config validate`、`project inspect --json` 和对应能力验证。
6. 确认 MCP Tool 名称和语义没有改变。

示例（必须提供真实实现；单纯写 TOML 不会生成 Provider）：

```toml
[extension.example]
loader = "go"

[extension.example.loader_config]
module = "."
package = "./extensions/example/cmd/provider"

[profile.example.service.code-indexed]
provider = "intelligence.example"
```

## 何时改 Core

只有缺失的是所有 Provider 都共享的路由、发现、生命周期或协议机制时才改 Core。不要在 Core 增加按 Provider ID 分支的 `switch`。

参考[配置参考](../reference/configuration.md)和[架构原则](../architecture/principles.md)。
