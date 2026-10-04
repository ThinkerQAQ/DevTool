# DevTool 实现方案

## 1. 实现结论

本方案基于 PhotoWaypoint、BlogCTL、IDFlow 当前代码重新 Review 后修订。

当前 DevTool 仓库仍只有 README 与 docs，没有历史实现负担，因此第一版应先把**协议边界和可验证最小实现**做正确，再迁移现有项目。

核心实现模型：

```text
Project Provider
    负责项目编排与项目 Descriptor
          |
          | bidirectional protocol
          v
DevTool Host
    负责 Runtime / Capability / Policy / Infrastructure
          |
          +--> CLI Renderer
          +--> GUI Renderer
          +--> Agent Renderer
          +--> CI
```

最重要的实现修正：

> **Project Provider 是独立进程，因此 Provider 不能直接持有 Host 内存中的 Go/Docker/ADB 等 Capability 对象。Provider 必须通过双向协议请求 Host 执行 Capability。**

也就是说，之前概念上的：

```go
ctx.Go().Test(...)
```

在 Provider 侧可以保留为 SDK API，但 SDK 的实际实现必须是 **RPC Client**，而不是直接调用 Host 对象。

这个边界必须从第一版就做对。

---

## 2. 代码 Review 后确认的可复用模式

### 2.1 PhotoWaypoint：重控制面与 Registry

已 Review：

- `go/cmd/devtool/main.go`
- `common.go`
- `code*.go`
- `device.go`
- `gear.go`
- Settings Model/Renderer
- Scene Plan/Result UI Registry

确认：

1. Process / Files / HTTP / Port / Docker discovery 等应进入 Runtime。
2. gopls / Kotlin LSP / CodeGraph / Serena 应进入通用 Capability。
3. Device / Gear / Beta / License 等必须留在 PhotoWaypoint Project Provider。
4. Settings 的类型化树模型可以作为 DevTool Settings Schema 的实现参考。
5. Scene UI Registry 可以作为 DevTool Feature Renderer Registry 的实现参考。

### 2.2 BlogCTL：Descriptor 驱动 UI

已 Review：

- `bridge/control.go`
- `toolDescriptor`
- `toolConfigView`
- `toolHealth`
- `environment.js`
- `tasks.js`
- `task-ui-state.js`
- `popup.js/html`

确认：

1. `Descriptor + Health + Config Schema + Actions` 已经实际运行，可作为 Control Surface 的直接原型。
2. Tool Card 可以由通用 Renderer 实现。
3. Jobs / Logs 可以抽成通用 Feature。
4. Publishing / Indexing 的复杂度说明不能继续无限扩展 UI Schema。
5. Secret 不回显、只返回“已配置”状态的做法应进入 DevTool 安全规则。

### 2.3 IDFlow：产品级工作台

UI Review 基线：当前开放 PR #8 `refactor/extension-ui-20261004`。

已 Review：

- `popup.html`
- `popup.ts`
- `tabs.ts`
- `bridge.ts`
- `go/bridge/server.go`
- `settings.go`
- `environment.go`

确认：

1. Chat / Workflow / Schedule / Capture / Settings 的导航结构适合 Project Control Surface。
2. 当前固定 TabID 和大量 DOM selector 不适合作为跨项目实现。
3. Scheduler 和 Settings 可以通用化。
4. Chat、Capture 需要 Feature Renderer。
5. Bridge API 已证明“本地 Host + Web UI”的运行形态可行。

---

## 3. 总体架构

```text
                         Stage-0 Bootstrap
                                |
                                v
                         +--------------+
                         | DevTool Host |
                         +------+-------+
                                |
            +-------------------+-------------------+
            |                   |                   |
            v                   v                   v
         Runtime          Capability Registry     Policy
            |                   |                   |
            +-------------------+-------------------+
                                |
                       Infrastructure
                                |
                                |
                bidirectional provider protocol
                                |
                                v
                       Project Provider
                                |
               +----------------+----------------+
               |                |                |
            Commands         Resources     Control Surface
                                |
                 +--------------+--------------+
                 |              |              |
                CLI            GUI           Agent
```

依赖规则：

```text
Project -> DevTool SDK/Protocol

DevTool Core -X-> Project packages
Capability -X-> Project semantics
Project Provider -X-> concrete GitHub/Railway/GHCR API
UI Renderer -X-> project-specific business code
```

---

## 4. 仓库结构

建议第一版：

```text
DevTool/
├── cmd/
│   ├── devtool/
│   └── devtool-bootstrap/
│
├── protocol/
│   ├── envelope.go
│   ├── provider.go
│   ├── capability.go
│   ├── resource.go
│   ├── event.go
│   └── version.go
│
├── sdk/
│   ├── provider/
│   ├── command/
│   ├── resource/
│   ├── surface/
│   └── capability/
│
├── runtime/
│   ├── discovery/
│   ├── provider/
│   ├── process/
│   ├── filesystem/
│   ├── environment/
│   ├── network/
│   ├── cache/
│   ├── evidence/
│   └── artifact/
│
├── capability/
│   ├── git/
│   ├── go/
│   ├── node/
│   ├── java/
│   ├── docker/
│   ├── postgres/
│   ├── android/
│   ├── adb/
│   ├── gradle/
│   ├── lsp/
│   ├── codegraph/
│   └── serena/
│
├── infrastructure/
│   ├── contract/
│   └── personal/
│
├── policy/
│
├── ui/
│   ├── server/
│   ├── web/
│   ├── primitive/
│   └── feature/
│       ├── environment/
│       ├── settings/
│       ├── jobs/
│       ├── logs/
│       ├── scheduler/
│       ├── workflow/
│       ├── chat/
│       └── browsercapture/
│
├── devcontrol/
│   ├── project.go
│   ├── build.go
│   ├── verify.go
│   ├── package.go
│   └── install.go
│
├── docs/
├── go.mod
└── README.md
```

注意：这是最终边界，不要求 Phase A 创建所有空 package。

---

## 5. Project Discovery

项目根目录使用最小：

```text
.devtool.toml
```

示例：

```toml
version = 1

[project]
name = "PhotoWaypoint"

[provider]
type = "go"
module = "./go"
package = "./cmd/devprovider"
protocol = 1
```

该文件只负责：

- project identity
- provider source
- provider type
- protocol requirement
- optional infrastructure profile

禁止承载：

- build workflow
- verify workflow
- shell command
- UI layout DSL

项目编排和 Control Surface 都由 Provider 正式代码声明。

---

## 6. Project Provider

### 6.1 Provider 是独立 executable

不使用 Go 标准库 `plugin`。

原因：

- Windows 必须支持
- 独立故障域
- 协议可版本化
- Host 与 Provider 可独立升级
- 避免 Go plugin ABI/build tag 限制
- 后续企业环境更容易审计

运行：

```text
devtool.exe
   |
   +---- stdin/stdout protocol ---- project-provider.exe
```

### 6.2 Provider 只负责项目语义

例如 PhotoWaypoint Provider：

```text
build
verify
device.full
device.runtime
gear.sync
beta.build
deploy
```

它不重新实现：

- command execution
- HTTP client
- Docker detection
- Git helpers
- ADB generic helpers
- LSP / CodeGraph lifecycle

这些由 Host Capability 提供。

---

## 7. 双向 Host / Provider Protocol

### 7.1 为什么必须双向

执行：

```text
devtool device full
```

Host 请求 Provider：

```text
provider.execute(device.full)
```

Provider 编排到：

```text
需要 adb device
需要 postgres
需要 gradle
```

Provider 必须反向请求 Host：

```text
capability.invoke(adb.require-device)
capability.invoke(postgres.ensure)
capability.invoke(gradle.run)
```

因此协议不是单向 command runner，而是**双向 RPC + Event Stream**。

---

### 7.2 Envelope

第一版使用 newline-delimited JSON。

每行一个完整 JSON envelope。

概念结构：

```go
type Envelope struct {
    ID      string          `json:"id,omitempty"`
    ReplyTo string          `json:"reply_to,omitempty"`
    Type    string          `json:"type"`
    Method  string          `json:"method,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}
```

Type：

```text
request
response
event
error
```

---

### 7.3 Host -> Provider

核心方法：

```text
provider.handshake
provider.describe
provider.execute
resource.read
provider.shutdown
```

### 7.4 Provider -> Host

核心方法：

```text
capability.invoke
event.emit
artifact.register
evidence.register
```

---

### 7.5 并发实现要求

Provider 在处理 `provider.execute` 时会同步等待 `capability.invoke` 的结果。

因此 Host 和 Provider 两端都必须：

1. 独立 goroutine 持续读取输入流。
2. 根据 `id/reply_to` 路由 response。
3. 使用 pending request map。
4. 不允许“发请求后停止读输入”。

否则会产生经典双向 RPC deadlock。

这是 Phase A 必测项。

---

### 7.6 stdout 规则

Provider stdout **只允许 Protocol Frame**。

禁止：

```go
fmt.Println("debug")
```

直接写 stdout。

Provider debug：

- stderr
- 或 `event.emit`

Host 执行外部命令时负责捕获 stdout/stderr，并转换为结构化 log event。

否则任意第三方命令输出都可能破坏 JSON 协议。

---

## 8. Provider SDK

Provider 中可以写自然 API：

```go
func DeviceFull(ctx devtool.Context, req DeviceRequest) error {
    device, err := ctx.ADB().RequireDevice(req.Serial)
    if err != nil {
        return err
    }

    if err := ctx.Postgres().Ensure(...); err != nil {
        return err
    }

    if err := buildCore(ctx); err != nil {
        return err
    }

    return installAndroid(ctx, device)
}
```

但：

```text
ctx.ADB()
ctx.Postgres()
ctx.Go()
ctx.Docker()
```

都是 Provider SDK 中的 RPC Proxy。

实际执行发生在 Host。

---

## 9. Provider Build 与 Cache

不能只 hash provider source。

Cache Key 至少包括：

```text
provider source tree hash
go.mod
go.sum
DevTool SDK version
protocol version
GOOS
GOARCH
Go toolchain version
build tags/options
```

建议：

```text
.devtool/cache/provider/
└── <cache-key>/
    ├── provider.exe
    └── metadata.json
```

metadata 记录：

- project
- source SHA
- sdk version
- protocol version
- build timestamp
- executable SHA256

这样 SDK/Protocol 升级时不会错误复用旧 Provider。

---

## 10. Capability

Capability 是 Host 拥有的通用工程原语。

### 10.1 粒度原则

不要设计成：

```text
PhotoWaypointDeviceCapability
BlogPublishCapability
```

应该是：

```text
process.run
git.status
go.test
go.build
docker.ensure
postgres.ensure
adb.devices
adb.reverse
gradle.run
codegraph.sync
lsp.start
```

Project Provider 决定如何组合。

---

### 10.2 Runtime Capability

从 PhotoWaypoint `common.go` 可直接提炼：

- process execution
- output capture
- environment merge
- executable lookup
- filesystem helpers
- hash
- temp path
- HTTP
- health wait
- free port
- background process
- PID lifecycle
- OS-specific signal handling

这些应先迁，避免后续 Capability 重复造轮子。

---

### 10.3 Code Intelligence

从 PhotoWaypoint `code*.go` 抽：

- gopls
- Kotlin LSP
- Serena
- CodeGraph
- bootstrap
- doctor
- sync
- MCP integration
- workspace lifecycle

最终通用命令：

```text
devtool code doctor
devtool code graph sync
devtool code graph query ...
devtool code lsp ...
```

---

## 11. Project Descriptor

`provider.describe` 返回稳定 Project Descriptor。

概念模型：

```go
type ProjectDescriptor struct {
    Identity     ProjectIdentity
    Commands     []CommandDescriptor
    Resources    []ResourceDescriptor
    Navigation   []NavItem
    Views        []ViewDescriptor
    Features     []FeatureBinding
}
```

这个 Descriptor 是 CLI / GUI / Agent 的共同事实来源。

---

## 12. Command Descriptor

```go
type CommandDescriptor struct {
    ID          string
    Title       string
    Description string
    Parameters  []Field
    SideEffect  SideEffect
    Confirm     *Confirmation
}
```

Field 第一版支持：

```text
string
integer
number
boolean
select
multi-select
secret
file
directory
date
time
datetime
device
```

同一份 Parameter Schema：

- CLI 生成 flags/validation
- GUI 生成 form
- Agent 生成 tool input schema

Command 的执行实现只有 Provider 一份。

---

## 13. Resource

Resource 是 UI/Agent 可读取的状态，不是命令。

例如：

```text
environment
devices
jobs
workflows
scheduled-tasks
publications
artifacts
```

协议：

```text
resource.read(resource-id, query)
```

Provider 可以在读取 Resource 时通过 Capability RPC 获取底层状态。

### 13.1 ResourceDescriptor

建议字段：

```go
type ResourceDescriptor struct {
    ID          string
    Title       string
    Schema      ValueSchema
    Refresh     RefreshPolicy
}
```

RefreshPolicy 第一版：

```text
manual
event-driven
interval fallback
```

优先事件驱动。

---

## 14. Event

标准事件：

```text
progress
log
warning
error
artifact
evidence
resource.changed
job.changed
result
```

BlogCTL 当前大量 2 秒轮询、IDFlow 15 秒刷新可以逐步变成：

```text
command execution
   |
   v
resource.changed
   |
   v
UI refresh target resource
```

对于外部系统可能发生变化的 Resource，可以保留 interval fallback。

---

## 15. Control Surface

Control Surface 不描述 HTML/CSS。

只描述：

```text
Navigation
View
Resource binding
Action binding
Feature binding
```

### 15.1 Action 与 Command 的关系

Action 不实现逻辑：

```go
type Action struct {
    CommandID string
    Label     string
    Style     ActionStyle
}
```

最终执行仍然是 Command。

这样不会出现 UI 和 CLI 两套业务实现。

---

## 16. UI Primitive

第一版只保留有限 Primitive：

### Layout

- Section
- Group
- Grid

### Display

- Text
- Status
- KeyValue
- List
- Table
- Progress
- Log

### Input

- Form
- Field

### Control

- Action

不要把业务语义做成 Primitive。

例如：

```text
PublicationCard
MoonPlanWidget
CapturePanel
```

都不属于 Primitive。

---

## 17. Feature Renderer Registry

复杂但可复用 UI 使用 Feature Renderer。

实现思想来自 PhotoWaypoint 的 `Scene*UiRegistry`。

概念：

```go
type FeatureRenderer interface {
    ID() string
    Validate(binding FeatureBinding) error
}
```

Web UI 中对应：

```text
feature id
   |
   v
Feature Registry
   |
   v
Renderer Component
```

第一版只允许 DevTool 内置/第一方 Renderer。

---

## 18. 首批 Feature

### 18.1 Environment / Settings

直接参考：

- BlogCTL `toolDescriptor/toolConfigView/toolHealth`
- PhotoWaypoint `SettingNode/SettingsRenderer`

标准模型：

```text
Tool/Setting
  identity
  required
  health
  description
  config schema
  configured values/state
  actions
```

Secret 规则：

- Resource/Descriptor 不返回 secret 原值。
- 只返回 `configured=true/false` 或 placeholder state。
- 更新 secret 使用 Action/Command 参数。

---

### 18.2 Jobs

参考 BlogCTL `tasks.js`。

标准 Job：

```go
type Job struct {
    ID         string
    Title      string
    Kind       string
    State      JobState
    Progress   *Progress
    StartedAt  *time.Time
    FinishedAt *time.Time
    Actions    []ActionRef
    Details    []Block
    LogRef     string
    Artifacts  []ArtifactRef
}
```

State：

```text
queued
running
paused
completed
failed
cancelled
```

Action：

- retry
- pause
- resume
- cancel
- delete/clear where allowed

BlogCTL 的 platform result 可以通过 Details 中的 Table/List 表达，不需要在 Job Core 中加入 publishing 专用字段。

---

### 18.3 Logs

统一：

- structured log entry
- level
- timestamp
- component
- message
- fields
- query/filter
- follow
- clear（由 policy/action 控制）

---

### 18.4 Scheduler

参考 IDFlow：

```text
once
interval
daily
```

第一版足够覆盖现有 IDFlow。

模型：

- runnable id
- name
- enabled
- schedule
- next_run_at
- actions

后续 cron 等复杂规则再扩展，不进入 v1。

---

### 18.5 Workflow / Chat / Browser Capture

它们不是基础 Primitive。

第一版作为可选内置 Feature：

- `workflow`
- `chat`
- `browser-capture`

IDFlow Provider 通过 Binding 启用。

UI Renderer 本身位于 DevTool UI 包，不由项目动态注入 JavaScript。

---

## 19. DevTool UI Host

第一版建议实现：

```text
devtool ui
```

行为：

1. DevTool Host discovery 当前项目。
2. 启动/连接 Project Provider。
3. 获取 Project Descriptor。
4. 在 localhost 启动 UI API。
5. 使用 Go `embed` 提供 DevTool Web UI 静态资源。
6. 打开浏览器。

这样：

- Windows/Linux/macOS/Codespaces 模型一致。
- 不需要第一版引入 Electron/Tauri。
- UI 与 Host 生命周期一致。
- 后续可以再套 Desktop Shell。

---

## 20. UI Local API

建议：

```text
GET  /api/project
GET  /api/resources/{id}
POST /api/commands/{id}
GET  /api/events
```

Event 第一版可用 SSE。

原因：

- GUI 主要需要 Host -> Browser 的事件流。
- HTTP POST 足够执行 Action。
- 实现复杂度低于 WebSocket。
- 浏览器原生支持。

如果未来出现双向高频交互，再评估 WebSocket。

---

## 21. UI 安全

`devtool ui`：

- 只 bind `127.0.0.1`
- 使用随机端口
- 启动时生成 session token
- Mutation request 必须携带 token
- 校验 Origin
- Secret 永不通过 Resource 回显
- destructive Action 支持 confirmation
- 最终仍经过 Policy

不能因为是 localhost 就跳过基本控制。

---

## 22. UI 本地状态

以下状态属于 Renderer，不属于 Project Provider：

- active tab
- expanded cards
- expanded logs
- table sort/filter
- selected row
- local search query

参考 BlogCTL `task-ui-state.js`。

存储 key 必须带：

```text
project-id
surface-id
view-id
```

避免多个项目相互污染。

---

## 23. Infrastructure Provider

Project Provider 不直接依赖：

- GitHub
- GHCR
- Railway
- GitLab
- Harbor
- Kubernetes
- npmjs
- proxy.golang.org

Host 通过小接口提供：

- VCS
- Registry
- Packages
- Secrets
- Network
- Deployment
- Identity
- Observability

第一版只实现实际需要的 Personal Provider。

Enterprise 接口预留边界，不在 Phase A 造空泛大接口。

原则：

> **只有出现真实第二实现需求时，再稳定对应 interface。**

避免提前设计企业万能抽象。

---

## 24. Policy

Policy 位于 Capability/Infrastructure side effect 前。

Action 元数据：

```text
side_effect = read | write | deploy | destructive
```

Policy 可以决定：

- allow
- deny
- require confirmation
- require approval

第一阶段 Personal Policy 简单实现。

---

## 25. Self-hosting

### 25.1 Stage 0

第一次 clone：

```text
Go toolchain
   |
   v
devtool-bootstrap
   |
   v
go build ./cmd/devtool
```

Stage 0 只负责获得 Host。

### 25.2 Stage 1

```text
DevTool N
   |
   v
build DevTool N+1
   |
   v
N+1 build/load its own Provider
   |
   v
N+1 verify
   |
   v
install
```

### 25.3 Windows

不能覆盖正在运行的 exe。

生成：

```text
devtool-next.exe
```

验证完成后通过退出后的 bootstrap/install helper 原子替换。

---

## 26. PhotoWaypoint 迁移

### 26.1 迁到 Runtime/Capability

从当前 DevTool 迁：

- command/process
- env merge
- file helpers
- hash
- network
- free port
- background process
- Git generic
- Docker generic
- PostgreSQL generic
- Android SDK generic
- ADB generic
- Gradle generic
- CodeGraph
- LSP
- Serena

### 26.2 留在 Provider

- gomobile/AAR 特殊构建
- Device full/runtime
- Gear
- Beta
- License
- Firebase 项目语义
- provider/recheck 项目服务编排
- PhotoWaypoint-specific acceptance
- deployment semantics

### 26.3 UI

PhotoWaypoint Provider 先提供：

```text
Overview
Environment
Device
Build
Jobs
Logs
```

Settings/Device Action 由 Control Surface 描述。

不要迁 Android 产品 UI 到 DevTool。

---

## 27. BlogCTL 迁移

### 27.1 直接复用模式

`toolDescriptor` 可以映射到 DevTool：

```text
Environment Resource
Settings Schema
Action Descriptor
Health
```

### 27.2 Jobs

现有 Tasks 映射到通用 Jobs Feature。

Publishing-specific platform details 用通用 Details blocks 表达。

### 27.3 保留 Blog Domain

以下仍属于 BlogCTL：

- article
- bindings
- publishing
- search/indexing domain
- browser session
- platform adapters

DevTool 只提供控制面容器，不吞掉 Blog Domain。

---

## 28. IDFlow 迁移

当前最新 UI 中：

```text
Chat
Workflow
Schedule
Capture
Settings
```

迁移映射：

```text
Chat      -> chat Feature
Workflow  -> workflow Feature + Resource/Action
Schedule  -> scheduler Feature
Capture   -> browser-capture Feature
Settings  -> settings Feature
```

Provider Navigation 动态声明以上 View。

最终不再需要 DevTool UI 中硬编码 IDFlow TabID。

IDFlow 产品扩展本身仍可保留自己的 UI；DevTool Control Surface 是工程/控制面视图，两者不强制合并。

---

## 29. Contract Validation

任何 Provider Descriptor 在显示 UI 前必须验证。

至少检查：

1. ID 唯一。
2. Navigation 引用的 View 存在。
3. View 引用的 Resource 存在。
4. Action 引用的 Command 存在。
5. Field type 受支持。
6. Feature renderer 已注册。
7. Secret Resource 不包含原值。
8. Command parameter schema 与 execute request 一致。
9. SideEffect 必须声明。
10. Descriptor protocol version 兼容。

错误 Descriptor：

```text
fail closed
```

不渲染半残 UI。

---

## 30. 必须有的测试

为了避免抽象迁移后产生大量隐蔽 bug，Phase A/B 必须有以下测试。

### 30.1 Protocol

- handshake
- incompatible protocol
- concurrent requests
- request timeout
- provider crash
- malformed frame
- Provider execute -> capability.invoke -> response -> event 的嵌套调用
- stdout contamination detection

### 30.2 Provider Cache

- source change invalidates
- go.mod/go.sum change invalidates
- SDK version invalidates
- protocol version invalidates
- GOOS/GOARCH invalidates

### 30.3 Descriptor

- invalid references
- duplicate IDs
- unsupported field
- missing feature
- secret leak rejection

### 30.4 Renderer Golden Fixtures

从真实项目建立 fixture：

#### PhotoWaypoint fixture

- environment
- device actions
- settings

#### BlogCTL fixture

- tool descriptors
- job with platform details
- logs

#### IDFlow fixture

- navigation
- scheduler
- chat/workflow/capture feature binding

UI contract test只需要保证 Descriptor 能稳定渲染，不复制项目业务实现。

---

## 31. 第一阶段实施顺序

### Phase A — Host + Protocol + Self-host

实现：

- Host
- discovery
- Stage-0 bootstrap
- Provider build/cache
- bidirectional protocol
- Provider SDK RPC proxy
- process/files/environment runtime
- structured event
- DevTool 自己的 Provider

验收：

```text
devtool can discover itself
devtool can build its provider
provider can call host capability
devtool can verify itself
```

---

### Phase B — 最小 Capability + PhotoWaypoint 验证

先迁：

- Git
- Go
- Process
- CodeGraph
- LSP
- Docker/Postgres 最小能力

将 PhotoWaypoint 的一小段真实流程迁成 Provider。

不要先迁全部 Device。

目的：

> 验证 Host/Provider 边界，而不是快速“搬完代码”。

---

### Phase C — Control Surface Contract

实现：

- Project Descriptor
- Command Descriptor
- Resource
- Navigation
- View
- Action
- Event
- contract validation

用三套真实 fixture 验证。

---

### Phase D — 最小 GUI

实现：

- `devtool ui`
- localhost HTTP
- SSE Event
- Primitive Renderer
- Environment
- Settings
- Jobs
- Logs

先让 PhotoWaypoint/BlogCTL fixture 能完整显示。

---

### Phase E — IDFlow Feature

实现：

- Scheduler
- Workflow
- Chat
- Browser Capture

按 IDFlow PR #8 的真实交互需求驱动，不凭空扩 UI DSL。

---

### Phase F — PhotoWaypoint 完整迁移

迁：

- Device
- Gear
- Build
- Beta
- service orchestration

旧 `go/cmd/devtool` 完全退出后再删除。

迁移期允许双跑验证：

```text
old devtool output
vs
new provider output
```

关键流程确认等价再切换。

---

### Phase G — BlogCTL / GoTiny / DOWNKIT

顺序：

1. GoTiny：验证最小 Go Provider。
2. BlogCTL：验证 Descriptor/Jobs/Settings 的真实复用。
3. DOWNKIT：验证 Extension + packaging + cross-platform。

---

### Phase H — Infrastructure

只在前面模型稳定后接：

- GitHub
- Registry
- Deployment
- Personal secrets

企业 Provider 等有真实公司环境时再实现。

---

## 32. Code Review Rule

后续 PR 必须满足：

1. Core 不 import project package。
2. Provider 不直接实现通用 Runtime helper。
3. Provider 不直接依赖具体 cloud/VCS vendor，除非属于项目业务本身且经过明确边界审查。
4. Capability 不出现 PhotoWaypoint/BlogCTL/IDFlow 领域词。
5. `.devtool.toml` 不出现 executable workflow。
6. GUI Action 必须引用 Command，不允许另写业务处理。
7. Agent Tool 必须来自同一 Command Descriptor。
8. Project 不允许注入任意 JS/CSS 到 DevTool UI。
9. Secret 不从 Resource/Descriptor 回显。
10. Provider stdout 只传协议。
11. 长流程必须发结构化 Event。
12. UI 通用性不足时，先评估 Feature Renderer，不扩万能 DSL。
13. 一个新抽象至少由两个真实场景证明，或明确只留在 Project Provider。
14. DevTool 自身必须持续 dogfooding。

---

## 33. 最终调用链

```text
Human / Agent / CI / GUI
          |
          v
      DevTool Host
          |
          +--> Project Descriptor
          |
          +--> provider.execute
                  |
                  v
            Project Provider
                  |
                  | capability.invoke
                  v
             DevTool Host
                  |
            Policy / Capability
                  |
          Infrastructure / OS
                  |
                  v
               Result
                  |
                  +--> Event
                  +--> Resource change
                  +--> Artifact
                  +--> Evidence
```

最终目标不是做一个庞大的开发平台，而是保持：

> **最小 Host + 稳定协议 + 可插拔 Provider/Capability + 配置驱动 Control Surface。**

所有复杂度都必须有明确归属，不允许重新堆回一个巨型 DevTool。
