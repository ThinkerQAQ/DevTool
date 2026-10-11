# 通过 DevTool 受控修改代码

[English](../../how-to/controlled-edits.md) · [Agent 开发指南](../agent-guide.md)

DevTool 新增可选的、**只支持单个既有文本文件**的 Git 补丁执行能力：必须先针对该文件成功调用 `code_context`，才允许执行补丁。网页版 ChatGPT 负责全部分析、规划和生成修改；本地 MCP Gateway 只校验与执行，不启动 Pi 模型或其他推理服务。

## 运行条件

- 位于独立的 Git 开发分支或 Worktree；不能直接修改 `main`、`master`。
- 已安装 Git，CodeGraph 索引与 Serena/LSP 实时能力均处于就绪状态。
- 从**当前源码**构建 DevTool；已安装的旧版二进制可能没有新功能。

```bash
cd /path/to/DevTool-worktree
go build -o /tmp/devtool-policy ./cmd/devtool
DEVTOOL_PROFILES=local /tmp/devtool-policy config validate
DEVTOOL_PROFILES=local /tmp/devtool-policy code doctor
```

## 第一步：配置 Gateway

在项目根目录 `.devtool.toml` 中增加：

```toml
[agent.policy]
enabled = true
protected_branches = ["main", "master"]
max_patch_bytes = 262144
context_ttl_seconds = 600
```

未显式启用时，默认**不开放**补丁工具。TOML 负责选择和配置规则，Git 负责解析和执行补丁，不额外自研 Diff 引擎。

## 第二步：启动长期运行的 MCP 连接

```bash
DEVTOOL_PROFILES=local /tmp/devtool-policy agent mcp --context agent
```

从开发 Worktree 启动，MCP 客户端通过 stdio 保持连接；读取上下文和修改文件必须使用**同一 Gateway 进程**。也可以使用具有 Bearer Token 的 `agent serve --listen 127.0.0.1:8080`。不要在公网暴露未授权接口。

`tools/list` 应出现 `code_context` 和 `workspace_apply_patch`。

## 第三步：由网页版 ChatGPT 推理并修改

先读取某个目标文件的真实代码上下文：

```json
{"name":"code_context","arguments":{"objective":"Inspect Gateway before changing it","path":"core/agent/gateway.go"}}
```

当 CodeGraph 与 Serena 均成功返回且未降级时，Gateway 会在正常的 MCP 文本 JSON 中增加：

- `context_token`：一次性修改凭据。
- `context_path`：允许修改的文件路径。
- `context_expires_at`：凭据到期时间。

之后 ChatGPT 生成实际 Git 格式的 Unified Diff，再调用：

```json
{"name":"workspace_apply_patch","arguments":{"context_token":"<刚才返回的令牌>","patch":"diff --git a/core/agent/gateway.go b/core/agent/gateway.go\n--- a/core/agent/gateway.go\n+++ b/core/agent/gateway.go\n@@ -1 +1 @@\n-old line\n+new line\n"}}
```

这里只是参数结构示例，补丁内容应根据真实代码生成。Gateway 检查令牌、项目根目录、工作分支、Git HEAD、文件 SHA-256、补丁路径及过期时间；通过 `git apply --check` 后才应用。完成后返回修改文件的 SHA-256。

**每个令牌仅允许一次修改尝试、仅限一个既有文件。** 下一次修改必须重新调用 `code_context`。

## 第四步：构建验证并提交

后续调用 DV2 已有的 `project_build`、`project_verify`、`scm_checkpoint`、`scm_publish`。当前门禁只直接约束新补丁工具，尚未限制所有外部工具；若 Desktop Commander 能直接写文件，仍可能绕过本门禁。要做到物理不可绕过，须结合独立账户或容器权限。

## 2026-10-11 实测

通过新编译的 DEV2 自举测试：`code_context` 实际调用 CodeGraph 与 Serena，获得无降级的上下文及一次性令牌；再通过 `workspace_apply_patch` 成功修改 `core/agent/gateway.go` 的一处注释。旧令牌再次使用被拒绝。

此外已测试：主分支保护、路径越界、符号链接、文件/HEAD 变化、跨文件与多文件补丁、降级的 Provider、令牌过期、超大补丁、文件模式修改。

## 当前限制

- 令牌保存在**同一 MCP Gateway 进程的内存**中，进程重启后失效；不能跨两次独立 CLI 运行复用。
- 暂不支持新增、删除、重命名文件、修改文件权限和二进制补丁。
- 这是 Gateway 的流程校验，不是操作系统安全边界；外部 Agent 仍可通过不受限 Shell 绕过它。
- DV2 既有 MCP JSON-RPC 实现暂时保留；[官方 Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) 可用于未来协议层迁移，但本次没有必要替换已经正常运行的 MCP Server。当前静态条件也不需要额外引入 CEL 或 OPA。
