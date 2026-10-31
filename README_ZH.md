# DevTool

**面向开发者、AI Agent 与 CI 的配置驱动工程控制面。**

**项目状态：实验性原型 · 已暂停维护 · 不保证生产可用。** 该仓库保留了一次高复杂度工程探索的源码、设计取舍及验证记录。它没有达成最初设想的完整产品目标，不能当作成熟的开箱即用工具。

**配套开发环境：** [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) 提供可复用的 Docker 工具链镜像；DevTool 的环境 Provider 通过 [\`.devtool.toml\`](.devtool.toml) 引用固定 digest 的 \`dev-base\` 镜像。两者独立成仓，DevEnvironment 无须依赖 DevTool 即可构建。

[English](README.md) · [中文文档](docs/cn/index.md) · [技术复盘](docs/cn/retrospective.md) · [MIT License](LICENSE)

## 试图解决什么问题

Git、语言服务器、代码索引、容器运行时和项目构建系统各有接口。人、Agent 和 CI 如果直接对接各个工具，往往需要重复处理项目发现、配置、能力选择、权限和生命周期。

DevTool 尝试在这些工具之上构建一个**薄控制层**，而不是重写它们：

- **统一工程操作**：Project Extension 定义项目检查、构建、验证与打包等操作。
- **代码智能**：把索引式仓库检索（CodeGraph / Sourcegraph）与实时语言服务器语义（Serena / LSP）分开建模。
- **文档与图表**：通过配置好的 Provider 获取 Markdown、Mermaid 和 PlantUML 上下文。
- **MCP Agent 入口**：让 Agent 调用稳定的意图级能力，而不是依赖某个 Provider 的专有工具名。
- **可替换实现**：通过 `.devtool.toml` 声明扩展、服务绑定与执行环境 Profile。

这些是仓库实际探索和实现过的方向，不代表全部 Provider 在所有环境中都可立即使用。

## 架构

~~~text
开发者 / AI Agent / CI
          |
       CLI / MCP
          |
  Project Extension / Capability
          |
     Service Contract
          |
   配置选择的 Provider
          |
 Git / LSP / Docker / Dagger / ...
~~~

核心原则是 **Minimal Core + Extension + Service/Provider + Declarative Configuration**。核心负责发现、路由与生命周期；具体实现由扩展和 Provider 承担。

详见 [架构概览](docs/cn/architecture/overview.md) 与 [架构原则](docs/cn/architecture/principles.md)。

## 从源码体验

前提：Git、`go.mod` 声明的 Go 版本，以及所选 Provider 所需的外部依赖。下面使用仓库提供的本地 Profile；该命令不代表所有 Provider 已就绪。

~~~bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
DEVTOOL_PROFILES=local go run ./cmd/devtool config validate
DEVTOOL_PROFILES=local go run ./cmd/devtool project inspect --json
DEVTOOL_PROFILES=local go run ./cmd/devtool init --json
~~~

Provider 依赖、构建和 MCP 示例见 [快速开始](docs/cn/quick-start.md)。没有稳定发行版或一键安装承诺；当前源码版本标记为 **0.1.0-dev**。

## 文档导航

| 主题 | 中文 | English |
| --- | --- | --- |
| 快速开始 | [指南](docs/cn/quick-start.md) | [Guide](docs/quick-start.md) |
| 系统架构 | [概览](docs/cn/architecture/overview.md) | [Overview](docs/architecture/overview.md) |
| Agent 接入 | [指南](docs/cn/agent-guide.md) | [Guide](docs/agent-guide.md) |
| Provider 扩展 | [指南](docs/cn/how-to/replace-or-add-provider.md) | [Guide](docs/how-to/replace-or-add-provider.md) |
| CLI / 配置 | [CLI](docs/cn/reference/cli.md) / [TOML](docs/cn/reference/configuration.md) | [CLI](docs/reference/cli.md) / [TOML](docs/reference/configuration.md) |
| 技术复盘 | [复盘](docs/cn/retrospective.md) | [Retrospective](docs/retrospective.md) |

## 项目结果

DevTool 并没有成为最初设想中稳定、通用、易部署的工程控制平台，但留下了可研究的架构与实现：意图级 MCP 接口、配置驱动 Provider、代码上下文路由、自举构建与验证等。

[技术复盘](docs/cn/retrospective.md) 将已探索的技术问题与尚未解决的产品化挑战分开说明。暂停维护不等于否定探索价值，也不代表源码已经生产验证。

## 许可与使用

源码按 [MIT License](LICENSE) 提供，第三方依赖遵循各自许可证。按现状提供，不含维护、兼容性、安全更新或商业支持承诺。涉及远程 MCP、Shell、凭据或补丁操作时，请先在隔离环境中检查配置和权限。

贡献者可参考 [AGENTS.md](AGENTS.md)；当前不承诺审查或接受新功能。
