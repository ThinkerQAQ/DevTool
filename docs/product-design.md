# DevTool 产品方案（PRD）

## 1. 产品定义

DevTool 是一个**自举式、可插拔、配置驱动的工程控制面（Engineering Control Plane）**。

它为 Human、Local Agent、Cloud Agent、CI/CD 和可选 GUI 提供同一套项目工程能力入口，把开发、代码分析、验证、构建、运行、制品、部署、环境诊断和工程控制面统一到一套协议与实现中。

一句话定义：

> **Project Provider 定义“这个项目怎么开发”，DevTool Host 提供“工程能力怎么执行”，Infrastructure Provider 决定“在什么基础设施上执行”，Control Surface 决定“这些能力如何被 CLI / GUI / Agent 呈现”。**

DevTool 不是 PhotoWaypoint 专用 CLI，也不是新的脚本框架，更不是通用 IDE。

首批目标项目：

- PhotoWaypoint
- BlogCTL
- IDFlow
- GoTiny
- DOWNKIT
- DevTool 自身

未来应能进入企业内网环境，而无需修改项目本身的工程语义。

---

## 2. 本轮设计的代码 Review 基线

本方案不是从抽象概念直接推导，而是基于现有项目代码重新 Review 后收敛。

### 2.1 PhotoWaypoint

Review 的核心代码：

- `go/cmd/devtool/main.go`
- `go/cmd/devtool/common.go`
- `go/cmd/devtool/code*.go`
- `go/cmd/devtool/device.go`
- `go/cmd/devtool/gear.go`
- `android/.../settings/SettingsModel.kt`
- `android/.../settings/SettingsRenderer.kt`
- `android/.../settings/PhotoWaypointSettingsCatalog.kt`
- `ScenePlanUiExtension.kt`
- `SceneResultUiExtension.kt`
- `ScenePlanRegistry.kt`

已经验证的模式：

1. 当前 DevTool 同时包含通用 Runtime、通用 Capability 和 PhotoWaypoint 项目编排，确实需要拆层。
2. `SettingNode -> SettingsRenderer` 已证明“类型化描述 + 通用 Renderer”可以工作。
3. `Scene*UiRegistry` 已证明“稳定主干 + Registry + Extension”适合可插拔 UI。
4. Device / Gear / Beta 等复杂流程必须留在 Project Provider，不能配置成脚本或塞进 DevTool Core。

### 2.2 BlogCTL

Review 的核心代码：

- `tools/blogctl/bridge/control.go`
- `toolDescriptor / toolConfigView / toolHealth / toolAction`
- `extension/popup/environment.js`
- `extension/popup/tasks.js`
- `extension/popup/task-ui-state.js`
- `extension/popup/popup.js`
- `extension/popup/popup.html`

已经验证的模式：

1. BlogCTL 已经实际实现了 `Descriptor -> Schema -> 动态表单/状态/Action`。
2. Environment 页面证明配置化 UI 可以显著减少项目专属前端逻辑。
3. Tasks 页面证明 Job / Progress / Logs / Retry / Pause / Resume 是可复用控制面 Feature。
4. Publishing / Indexing 等复杂页面也证明：纯通用表单 Schema 无法优雅覆盖全部 UI，需要 Feature Renderer，而不是无限扩张 UI DSL。

### 2.3 IDFlow

当前主分支尚未包含最新产品 UI，本次以当前开放的 **PR #8 `refactor/extension-ui-20261004`** 为 UI Review 基线。

Review 的核心代码：

- `extension/popup.html`
- `extension/src/popup/popup.ts`
- `extension/src/popup/tabs.ts`
- `extension/src/popup/bridge.ts`
- `go/bridge/server.go`
- `go/bridge/settings.go`
- `go/bridge/environment.go`

已经验证的模式：

1. AI Chat / Workflow / Scheduler / Capture / Settings 是清晰的产品级控制面信息架构。
2. 目前 Tab、DOM selector、Bridge endpoint 和表单仍大量硬编码，无法直接复用到其他项目。
3. Chat、Browser Capture 这类复杂交互应该成为独立 Feature Renderer。
4. Scheduler、Settings、Environment 则明显具备跨项目复用价值。

因此，DevTool UI 的目标不是把这三套 UI 代码搬进来，而是抽取三套代码中已经证明可行的模式。

---

## 3. 要解决的问题

### 3.1 工程入口碎片化

同一个项目常同时存在：

- PowerShell
- Bash
- Makefile
- npm scripts
- Go helper
- Gradle task
- Docker Compose
- CI YAML
- Agent Prompt

同一工程动作被重复定义，最终会漂移。

目标：

```text
Human ───────┐
Agent ───────┼──> DevTool ──> Project Provider
CI ──────────┤
GUI ─────────┘
```

工程编排只保留一份事实来源。

---

### 3.2 通用能力和项目逻辑混杂

以 PhotoWaypoint 为例：

通用能力：

- Process / Filesystem / Network
- Git
- CodeGraph / LSP / Serena
- Docker / PostgreSQL
- Android / ADB / Gradle

项目逻辑：

- Device full/runtime
- Gear
- Beta
- License
- provider/recheck 编排
- PhotoWaypoint 部署语义

DevTool 必须让两者分离。

---

### 3.3 CLI、GUI、Agent 各写一套控制逻辑

理想模型不是：

```text
CLI implementation
GUI implementation
Agent tool implementation
CI implementation
```

而是：

```text
             Project Provider
                    |
          Project / Command Descriptor
                    |
      +-------------+-------------+
      |             |             |
     CLI           GUI          Agent
```

一份 Command/Parameter 契约，被不同 Renderer 使用。

---

### 3.4 UI 无法跨项目复用

PhotoWaypoint、BlogCTL、IDFlow 的控制面完全不同。

因此 DevTool 不应该硬编码：

```text
PhotoWaypointPage
BlogCTLPage
IDFlowPage
```

而应该提供：

> **Control Surface Protocol**

项目声明：

- Navigation
- View
- Resource
- Action
- Event

DevTool UI 负责渲染。

---

### 3.5 个人环境和企业内网不一致

个人环境可能使用：

- GitHub
- GitHub Actions
- GHCR
- Railway
- 公网 package registry

企业环境可能使用：

- GitLab / Gerrit
- Jenkins / GitLab CI / Tekton
- Harbor
- Kubernetes / Internal PaaS
- Nexus / Artifactory
- 企业 Secrets / SSO / Proxy

Project Provider 不应该感知这些具体实现。

---

## 4. 核心产品原则

### 4.1 极简主干

DevTool Core 只保留稳定、跨项目的最小能力：

- Project Discovery
- Provider Lifecycle
- Protocol
- Runtime
- Capability Registry
- Policy
- Control Surface
- Structured Event
- Artifact / Evidence

项目领域不进入 Core。

---

### 4.2 可插拔

三个独立可插拔维度：

```text
Project Provider
    项目怎么开发

Capability
    通用工程动作怎么执行

Infrastructure Provider
    具体基础设施怎么实现
```

UI 另有：

```text
Feature Renderer Registry
    复杂控制面怎么显示
```

---

### 4.3 配置化，但不创造新的脚本 DSL

“配置化”指：

- Project Descriptor
- Command Descriptor
- Resource Descriptor
- Control Surface Descriptor
- Settings Schema

这些都是**结构化数据**。

`.devtool.toml` 只负责发现 Provider 和少量静态元数据。

禁止把 TOML 变成：

```toml
build = "go build ..."
verify = "./verify.ps1"
deploy = "npm run deploy"
```

工程编排必须由正式代码实现。

---

### 4.4 控制面代码化，不脚本化

项目工程控制面使用 Project Provider 实现。

PowerShell/Bash/Make/npm scripts 可以作为第三方生态工具的内部实现细节，但不再作为项目工程流程的 Single Source of Truth。

---

### 4.5 UI 采用两级模型

第一版只提供：

#### Level 1：通用 Schema

覆盖高频、稳定 UI：

- Status
- KeyValue
- List
- Table
- Progress
- Log
- Form
- Action
- Settings
- Environment

#### Level 2：内置 Feature Renderer

覆盖复杂但具有复用价值的交互：

- Jobs
- Logs
- Scheduler
- Workflow
- Chat
- Browser Capture
- Environment / Settings

第一版**不开放项目任意注入 JavaScript/React bundle**。

如果未来确实出现无法由 Schema + 内置 Feature 表达的场景，再设计隔离的 Custom Renderer。

---

### 4.6 CLI / GUI / Agent 共用同一契约

例如：

```text
Command: device.full
Parameters:
  serial: device
  skipTests: boolean
```

可以同时生成：

CLI：

```text
devtool device full --serial xxx --skip-tests
```

GUI：

```text
Device [ xxx ▼ ]
☐ Skip tests
[完整构建并安装]
```

Agent：

```json
{
  "tool": "device.full",
  "arguments": {
    "serial": "xxx",
    "skipTests": true
  }
}
```

---

### 4.7 Build Once, Promote Many

同一个 artifact：

```text
Build
  |
  v
artifact@source-sha
  |
  +--> DEV
  +--> STAGING
  +--> PROD
```

环境晋级不重新构建。

---

### 4.8 Self-hosting

DevTool 必须能够开发 DevTool 自己：

```text
DevTool N
  -> build N+1
  -> N+1 verify itself
  -> install / publish
```

Self-hosting 是架构正确性的长期验证。

---

## 5. Control Surface 产品模型

Control Surface 只保留五个核心概念：

```text
Navigation
View
Resource
Action
Event
```

### Navigation

定义当前项目有哪些控制面入口。

例如 IDFlow：

```text
AI Chat
Workflow
Schedule
Capture
Settings
```

### View

定义页面如何组织 Resource 和 Action。

### Resource

只读状态模型，例如：

- environment
- devices
- jobs
- workflows
- publications
- artifacts

### Action

对一个 Command 的 UI 引用。

Action 自己不实现业务逻辑。

### Event

描述运行中的：

- progress
- log
- warning
- error
- artifact
- evidence
- resource.changed
- result

---

## 6. 内置 Feature

### 6.1 Environment / Settings

来源于 BlogCTL 动态工具卡和 PhotoWaypoint Settings 模型。

应支持：

- required / optional
- health
- version
- path
- description
- configurable fields
- secret configured state
- actions
- searchable settings

### 6.2 Jobs

来源于 BlogCTL Tasks 和 PhotoWaypoint 长流程。

标准能力：

- queued / running / paused / completed / failed
- progress
- logs
- retry
- pause
- resume
- cancel
- details
- artifacts

### 6.3 Scheduler

来源于 IDFlow Scheduled Tasks。

标准能力：

- once
- interval
- daily
- enable/disable
- next run
- create/edit/delete

### 6.4 Logs

统一结构化日志和过滤。

### 6.5 Chat / Workflow / Browser Capture

这些是复杂 Feature，不进入最小 Primitive。

由可选内置 Feature Renderer 提供。

---

## 7. 与 DevEnvironment 的关系

```text
DevEnvironment
    |
    | provides executable/toolchain
    v
DevTool Capability
    |
    v
Project Provider
    |
    +--> CLI
    +--> GUI
    +--> Agent
    +--> CI
```

DevEnvironment 回答：

> 环境里有什么？

DevTool 回答：

> 当前项目如何组合这些能力？

---

## 8. 企业内网适配

Infrastructure Provider 提供抽象：

- VCS
- Registry
- Packages
- Secrets
- Network
- Deployment
- Identity
- Observability

Policy 负责：

- 公网访问限制
- Package Source 限制
- Production Approval
- Secret policy
- Artifact scan
- Branch policy
- Log redaction

增加 Enterprise Infrastructure Provider 不应要求修改 Project Provider。

---

## 9. 非目标

当前阶段不做：

- 通用 IDE
- 替代 Git / Docker / Kubernetes / CI
- 将产品业务 UI 全部迁进 DevTool
- 用 TOML/YAML 定义可执行工作流
- Go 原生 `plugin` 动态加载
- 项目任意注入 JS/CSS/React 到 DevTool UI
- 一开始实现“万能 UI DSL”
- 一开始适配具体公司的私有平台

---

## 10. 第一阶段成功标准

1. DevTool 可以 self-host。
2. Project Provider 可以通过稳定协议调用 Host Capability。
3. PhotoWaypoint 的通用 DevTool 逻辑可以迁入 DevTool，Device/Gear 等项目逻辑保留在 Provider。
4. Local / Agent / CI 使用相同 Command。
5. Command Parameter Schema 可以同时被 CLI、GUI、Agent 使用。
6. DevTool GUI 能由 Project Descriptor 自动生成 Navigation。
7. BlogCTL 的 tool descriptor 模式可由通用 Environment/Settings Renderer 表达。
8. BlogCTL 的任务模型可由通用 Jobs Feature 表达主要行为。
9. IDFlow 的 Settings/Scheduler 可由通用 Feature 表达，Chat/Capture 可通过内置 Feature Renderer 表达。
10. 第一阶段无需项目编写 DevTool 专属前端代码。
11. 项目不能通过任意 JS 注入扩展 DevTool UI。
12. DevTool 接口不硬编码 GitHub / GHCR / Railway。
13. Personal Infrastructure 可运行。
14. 架构允许未来接入 Enterprise Infrastructure，而无需改项目工程语义。

---

## 11. 最终产品形态

```text
                    Project Provider
                           |
        +------------------+------------------+
        |                  |                  |
     Commands           Resources       Control Surface
        |                  |                  |
        +------------------+------------------+
                           |
                       DevTool Host
                           |
        +------------------+------------------+
        |                  |                  |
       CLI                GUI               Agent
        |                  |                  |
        +------------------+------------------+
                           |
                         Policy
                           |
                      Capability
                           |
                Infrastructure Provider
                           |
             Personal / Enterprise
```

DevTool 的长期核心是：

> **一个项目工程模型，多种控制面 Renderer；一个极简主干，能力按需插拔。**
