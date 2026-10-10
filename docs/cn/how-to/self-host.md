# 构建、验证与激活

[English](../../how-to/self-host.md) · [快速开始](../quick-start.md)

DevTool 自身通过普通的 Project Extension 构建和验证，和其他项目使用同一套契约。

## 检查项目

```bash
devtool project inspect --json
devtool config validate
```

当前仓库声明 `build`、`verify`、`package`，执行时仍需满足选定 Environment/Runtime 的依赖。

## 构建与验证

```bash
devtool build
devtool verify
```

候选二进制位于 `.devtool/out/devtool-next`（Windows 为 `.exe`）；验证记录写入 `.devtool/out/devtool-next.verified.json`，包含版本、源码身份和 SHA-256。

激活前可检查：

```bash
./.devtool/out/devtool-next version --json
./.devtool/out/devtool-next project inspect --json
```

## 激活与回退

已安装 DevTool 的主机，验证通过后运行：

```bash
devtool activate
```

本地 CLI 位于 `~/.local/bin/devtool`，历史产物在 `~/.local/share/devtool`。需要撤销时：

```bash
devtool rollback
```

不要激活未验证的产物。需要发布包时才执行 `devtool package`。

## 新环境自举

尚未安装时，在 DevTool 源码仓库使用 `go run ./cmd/devtool <command>` 进入相同的扩展体系。该入口仅是自举，不允许绕过 Project Extension 自建并行流程。

参考 [CLI](../reference/cli.md) 与 [TOML](../reference/configuration.md)。
