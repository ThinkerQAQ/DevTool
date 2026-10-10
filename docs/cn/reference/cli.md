# CLI 命令参考

[English](../../reference/cli.md) · [中文目录](../index.md)

| Core 命令 | 作用 |
| --- | --- |
| `devtool version [--json]` | 版本与源码身份 |
| `devtool config path` | 查找配置文件 |
| `devtool config validate` | 校验 TOML 和 Profile |
| `devtool project inspect [--json]` | 查看项目 Contract、服务与命令 |
| `devtool init [--json]` | 检查活跃 Provider 就绪情况 |
| `devtool code doctor` | 检查索引、实时语义 Provider |
| `devtool code verify` | 执行代码智能验证 |
| `devtool agent mcp [--context agent\|codex\|claude-code]` | 启动本地 stdio MCP |
| `devtool agent serve [--listen :8080]` | 启动 HTTP MCP，默认启用 Token 认证 |

## Project Extension 命令

下列命令由 **DevTool 仓库自身** 的 Project Extension 定义，其他项目不一定拥有。

| 命令 | 作用 |
| --- | --- |
| `devtool build` | 生成候选二进制 |
| `devtool verify` | 验证候选并记录结果 |
| `devtool package` | 打包产物 |
| `devtool activate` | 激活已验证版本 |
| `devtool rollback` | 回退上一版本 |

请先用 `devtool project inspect --json` 查看当前项目命令，不要硬编码到所有仓库。

## 源码自举

```bash
go run ./cmd/devtool config validate
go run ./cmd/devtool project inspect --json
```

## Profile

```bash
DEVTOOL_PROFILES=local devtool code doctor
DEVTOOL_PROFILES=sourcegraph devtool code verify
DEVTOOL_PROFILES=local,sourcegraph devtool project inspect --json
```

`init --json` 报告就绪问题，`code doctor` 检查可用性，`code verify` 验证 Provider。远程网关不得暴露未认证访问；以当前版本的 `devtool --help` 为准。
