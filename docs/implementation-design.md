# DevTool 实现方案

## 1. 实现结论

DevTool 的实现目标不是再造一个 CI Engine，也不是把现有各项目 DevTool 机械合并。

最终实现原则：

> **极简 Core + Extension Contract + 配置 Wiring + 可替换 Runtime + 通用 Control Surface + Self-hosting。**

其中：

- Core 只负责 Discovery、Configuration、Registry、Protocol、Routing、Event、Policy Hook 和 UI Host。
- Portable Execution 通过 Runtime Extension 接入，第一实现使用 Dagger。
- CodeGraph / LSP 通过 Code Intelligence Extension 接入，Core 只负责路由。
- ADB、Browser、Native Messaging、宿主机进程等通过 Native Extension 接入。
- 项目自己的工程语义通过 Project Extension 提供。
- GUI 通过统一 Project Descriptor 和 Feature Registry 渲染。
- DevTool 自身必须作为普通 Project Extension 使用 DevTool 开发自己。
- 不保留旧控制面长期兼容逻辑。

---

## 2. 代码 Review 与调研结论

### 2.1 PhotoWaypoint

已 Review：

- go/cmd/devtool/main.go
- common.go
- build.go
- device.go
- services.go
- postgres.go
- code*.go
- Android Settings Model/Renderer
- Scene Plan/Result Registry
- DevEnvironment Base/Android 镜像

确认：

#### Portable

适合迁入 Runtime Extension：

- Go build/test
- gomobile/gobind
- Android Gradle build
- PostgreSQL integration
- provider-gateway/recheck service
- package/artifact
- cache
- build evidence

#### Native

保留 Native Extension：

- adb devices
- adb server
- adb reverse
- APK install
- app launch
- logcat
- USB/physical device

#### Project

保留 PhotoWaypoint Project Extension：

- device.full
- device.runtime
- gear
- beta
- license
- PhotoWaypoint-specific service ordering
- acceptance semantics

### 2.2 BlogCTL

已 Review：

- bridge/control.go
- toolDescriptor/toolConfigView/toolHealth/toolAction
- environment.js
- tasks.js
- task-ui-state.js
- publishing/indexing
- Browser Extension / Bridge

确认：

- Descriptor-driven settings/environment 可以直接成为通用 Control Surface 原型。
- Jobs/Logs 是跨项目 Feature。
- Publishing domain 不进入 DevTool Core。
- Browser cookie/session/native publisher 留 Browser/Native Extension。

### 2.3 IDFlow

已 Review当前新 UI：

- popup.html
- popup.ts
- tabs.ts
- bridge.ts
- Go bridge/settings/environment
- Browser MV3 permissions

确认：

- Scheduler/Settings/Jobs 可通用化。
- Workflow/Chat/Capture 需要 Feature Renderer。
- Browser activeTab/debugger/scripting/tabs 等不能迁入普通 Web UI。
- Browser Extension 应缩成 Browser Context Adapter + Quick UI。

### 2.4 GoTiny / DOWNKIT

GoTiny 用于验证最小 Portable Project。

DOWNKIT 用于验证：

- Go build/package
- Browser Extension
- Native Messaging
- 本机 sidecar
- cross-platform package

---

## 3. 市面方案调研后的实现选择

## 3.1 Dagger

Dagger 与 DevTool Portable Execution 高度重叠：

- typed Module / Function
- Container
- Directory / File
- Service
- Secret
- Cache
- Artifact
- Module dependency
- local / CI 同构

实现决策：

~~~text
DevTool Core
    │
    ▼
PortableRuntime Contract
    │
    ▼
runtime.dagger
    │
    ▼
Dagger SDK / Engine
~~~

禁止：

~~~text
DevTool Core
    └── dagger.Container / dagger.Service / dagger.Secret
~~~

即 Dagger 类型不能出现在 Core Public Contract。

Dagger 只存在于：

~~~text
extensions/runtime/dagger
~~~

内部。

### 为什么不复制或 fork Dagger

因为以下能力已经是成熟基础设施：

- container DAG execution
- cache
- service lifecycle
- secret handling
- artifact movement

DevTool 的差异化在：

- Project Control Plane
- Native Host Integration
- Browser Integration
- Control Surface
- Agent / CI / Human 统一协议
- Extension Wiring

因此使用 Adapter，而不是维护第二套 Dagger。

---

## 3.2 Backstage

借鉴：

- Plugin
- Extension Point
- Registry
- indirect routing
- utility API

不直接依赖 Backstage Runtime。

DevTool 是 local-first 控制面，后续企业已有 Backstage 时，可以：

~~~text
Backstage Plugin
    │
    ▼
DevTool API
~~~

接入。

---

## 3.3 JSON Forms

只用于：

- Command Parameters
- Settings
- Config

不作为整个 UI DSL。

---

## 4. 总体架构

~~~text
                       Human / Agent / CI / GUI
                                 │
                                 ▼
                         +----------------+
                         |  DevTool Core  |
                         +--------+-------+
                                  │
                     Extension / Service Registry
                                  │
          ┌──────────────────┬──────────────────┬──────────────────┐
          ▼                  ▼                  ▼                  ▼
   Project Extension  Runtime Extension  Code Intelligence   Native Extension
          │                  │             Extension               │
   project semantics       Dagger        CodeGraph / LSP      ADB / Browser / OS
          │
          ├──────── Infrastructure Extension
          │
          ├──────── Policy Extension
          │
          └──────── UI Feature Extension
~~~

---

## 5. Core

Core 必须保持极小。

建议目录：

~~~text
core/
├── project/
├── config/
├── extension/
├── registry/
├── command/
├── resource/
├── event/
├── policy/
├── surface/
└── host/
~~~

Core 只提供：

### Project Discovery

定位当前项目和 devtool.toml。

### Configuration

解析 Extension 选择和 wiring。

### Extension Lifecycle

负责：

- discover
- load
- validate
- start
- stop

### Registry

注册：

- Service
- Command
- Resource
- UI Feature
- Policy Hook

### Router

把 Project Extension 对 Service Contract 的调用路由到具体 Extension。

### Event Bus

统一：

- progress
- log
- warning
- error
- resource.changed
- job.changed
- artifact
- evidence
- result

### Control Surface

提供 Project Descriptor。

Core 不直接拥有：

- Go builder
- Docker
- Dagger
- GitHub
- ADB
- Blog Publisher
- IDFlow Browser Capture

---

## 6. Extension Contract

所有 Extension 统一具备最小生命周期：

~~~go
type Extension interface {
    ID() string
    Describe() Descriptor
    Register(Registry) error
    Start(Context) error
    Stop(Context) error
}
~~~

实际 Go API 应拆成小接口，不制造一个巨型 Extension interface。

统一概念不意味着所有 Extension 必须实现全部能力。

---

## 7. Service Contract

Extension 之间通过 Service Contract 解耦。

例如：

~~~text
PortableRuntime
CodeGraph
LanguageServer
ADB
BrowserSession
VCS
Registry
Deployment
Secrets
~~~

Project Extension 只依赖 Contract。

例如 PhotoWaypoint：

~~~go
func DeviceFull(ctx Context, req DeviceFullRequest) error {
    artifact, err := ctx.Service("portable-runtime").Build(...)
    if err != nil {
        return err
    }

    adb := ctx.Service("adb")
    device, err := adb.RequireDevice(req.Serial)
    if err != nil {
        return err
    }

    return adb.Install(device, artifact)
}
~~~

这里 Project 不知道 Portable Runtime 是 Dagger。

---

## 8. Runtime Extension

### 8.1 第一实现：runtime.dagger

runtime.dagger 负责：

- build
- test
- package
- service
- artifact
- cache
- secret injection

第一阶段不要设计 Universal Container API。

DevTool Contract 只抽真实项目需要的语义。

建议最小接口：

~~~go
type PortableRuntime interface {
    Invoke(ctx context.Context, req Invocation) (Result, error)
}
~~~

Invocation 描述：

- module
- function
- typed args
- input source/artifact refs
- requested outputs

Dagger Adapter 内部负责映射到 Dagger Module/Function。

`Invocation.Module` 使用相对于 `Invocation.Workspace` 的项目路径语义；`runtime.dagger` 负责将其解析为 Dagger CLI 可消费的绝对模块路径。Project Extension 不感知 Dagger CLI 的参数位置或 cwd 解析规则。

### 8.2 Project Dagger Module

项目自己的 Portable 工程逻辑可以放项目 Dagger Module。

例如 PhotoWaypoint：

~~~text
PhotoWaypoint
├── Project Extension
│   └── device.full / gear / deploy semantics
│
└── Dagger Module
    ├── build
    ├── verify
    ├── postgres-test
    └── package
~~~

这样 portable build logic 仍然属于项目，不进入 DevTool 仓库。

### 8.3 DevEnvironment / Environment Service

DevEnvironment 继续作为 **Shared Toolchain Image**，但 DevTool 不再让 CodeGraph/Serena 直接操作 Docker。

正式边界：

~~~text
CodeGraph / Serena / future tools
              │
              ▼
       environment Service
              │
              ▼
      environment.docker
              │
              ▼
 project-scoped reusable container
~~~

当前 Docker Provider 以 project + image 为边界创建一个可复用容器；后续命令通过 docker exec 进入已有环境。镜像变化时重建该 project container。

Dagger 仍可使用同一 DevEnvironment 镜像作为 build base：

~~~text
DevEnvironment image
      ├── environment.docker -> long-lived development workspace
      └── runtime.dagger     -> portable build/test/package execution
~~~

工具版本继续由 DevEnvironment 管理，不在 Dagger Module、CodeGraph Extension 或 Serena Extension 内复制一份。

Environment 是可替换 Service。未来可以增加 Podman/Kubernetes/Remote Provider，而不修改 CodeGraph、Serena 或 Core。


---

## 9. Code Intelligence Extension

Code Intelligence 是通用工程能力，不属于 Project Extension，也不进入 Core 实现。

第一批实现：

~~~text
extensions/intelligence/
├── codegraph/
└── serena/
~~~

Service Contract：

~~~text
code-graph
├── doctor
├── mcp
├── sync
└── query

code-lsp
├── doctor
├── verify
└── mcp
~~~

职责边界：

~~~text
Agent / Human
      │
      ▼
devtool code ...
      │
      ▼
Core Router
      │
      ├── code-graph -> intelligence.codegraph -> CodeGraph
      └── code-lsp   -> intelligence.lsp.serena -> Serena -> gopls / Kotlin LSP
~~~

Core 不引用 CodeGraph、Serena、gopls 或 Kotlin language server 类型。

项目只配置 workspace composition：

~~~toml
[service.code-graph]
provider = "intelligence.codegraph"

[service.code-lsp]
provider = "intelligence.lsp.serena"

[code]
workspaces = [".", "./devcontrol"]
~~~

工具链由 DevEnvironment 提供。DevTool 负责 discovery、workspace/branch 上下文、生命周期、路由和 doctor；项目仓库不再各自下载或固定 CodeGraph / Serena 二进制。

Environment Provider 为每个 project/worktree 维护一个可复用的开发容器，并将 `.devtool/cache/devenv-home` 挂载为 `/tmp/devenv-home`。CodeGraph graph DB、Serena/语言服务运行态与工具缓存可以持续复用，同时保持 project/worktree 隔离；Agent 不需要手工安装、创建缓存目录或直接启动依赖。

Agent 使用规则：

- definition / references / diagnostics / rename 等 compiler-grade 语义优先走 LSP。
- callers / callees / dependency topology / impact analysis 等结构问题优先走 CodeGraph。
- 源码修改后，如果图查询用于正确性结论，必须重新执行 one-shot devtool code graph query，确保读取当前 worktree。
- devtool code verify 同时验证 CodeGraph 当前图和 LSP workspace health。

DevTool 自己必须通过同一 devtool code 路径完成 dogfood。

## 10. Native Extension

Native Extension 负责不能自然容器化的宿主能力。

首批：

~~~text
native.process
native.filesystem
native.adb
native.browser
~~~

### native.adb

提供：

- devices
- server state
- reverse
- install
- shell
- launch
- logcat

### native.browser

提供：

- Browser session registry
- Extension Bridge
- Native Messaging
- local browser coordination

具体 Chrome API 仍发生在 Browser Extension 内。

---

## 11. Infrastructure Extension

第一批实际需要时再实现：

~~~text
vcs.github
registry.ghcr
deploy.railway
~~~

Contract 不允许直接以 GitHub/Railway 命名。

例如：

~~~text
VCS
ArtifactRegistry
DeploymentTarget
~~~

只有出现真实第二实现时，再稳定更深层通用接口。

避免提前设计万能 Enterprise SDK。

---

## 12. 配置与 Wiring

项目根：

~~~text
.devtool.toml
~~~

只包含：

- project identity
- Extension loader/build metadata
- Service provider mapping
- UI Feature enablement
- optional profile

当前 Go Extension 示例：

~~~toml
version = 1

[project]
name = "PhotoWaypoint"

[extension.project]
type = "go"
module = "./go/devcontrol"
package = "./cmd/provider"

[extension.environment]
type = "go"
module = "."
package = "./extensions/environment/docker/cmd/provider"

[service.environment]
provider = "environment.docker"

[service.portable-runtime]
provider = "runtime.dagger"

[service.adb]
provider = "native.adb"

[ui]
features = ["environment", "settings", "jobs", "logs"]
~~~

配置选择的是 loader/provider，Core 不枚举具体 Provider ID。Go 的 build/cache 由 adapters/extensionloader 负责，Core 只接收解析后的 executable。

不允许 executable workflow。

---

## 13. Project Extension

Project Extension 是项目工程语义唯一事实来源。

建议 PhotoWaypoint：

~~~text
go/
└── devcontrol/
    ├── project.go
    ├── build.go
    ├── verify.go
    ├── device.go
    ├── gear.go
    ├── beta.go
    └── deploy.go
~~~

DevTool：

~~~text
devcontrol/
├── project.go
├── build.go
├── verify.go
├── package.go
├── install.go
└── release.go
~~~

BlogCTL / IDFlow / GoTiny / DOWNKIT 同理。

---

## 14. Command / Resource / Event Contract

### Command

~~~go
type CommandDescriptor struct {
    ID          string
    Title       string
    Description string
    Parameters  []Field
    SideEffect  SideEffect
}
~~~

Command 参数 Schema 同时服务：

- CLI
- GUI
- Agent Tool

### Resource

例如：

- environment
- devices
- jobs
- workflows
- publications
- artifacts

Resource 是状态，不是命令。

### Event

标准事件：

~~~text
progress
log
warning
error
resource.changed
job.changed
artifact
evidence
result
~~~

---

## 15. Control Surface

Project Descriptor：

~~~text
ProjectDescriptor
├── Identity
├── Commands
├── Resources
├── Navigation
├── Views
└── FeatureBindings
~~~

核心 UI Primitive：

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

不把业务概念做 Primitive。

---

## 16. UI 实现

第一版技术栈：

~~~text
React + TypeScript
TanStack Query
JSON Forms
SSE
Feature Registry
~~~

### 为什么 React

DevTool UI 是复杂控制面，不是简单 Extension popup。

它需要：

- dynamic navigation
- resource rendering
- jobs
- logs
- chat
- workflow
- schema form
- feature registry

React 适合作为 Renderer Host。

### 为什么现有 Extension 不统一迁 React

IDFlow 当前是 TypeScript + DOM。

BlogCTL / DOWNKIT 当前以原生 JS/HTML 为主。

未来 Browser Extension 会变薄，因此没有必要为了技术统一增加 React 依赖。

原则：

~~~text
DevTool Full Control Plane -> React
Browser Context Adapter    -> lightweight TS/JS
~~~

### UI Host

第一版：

~~~text
devtool ui
~~~

Go Host：

1. discovery current project
2. load Project Extension
3. expose Local API
4. serve embedded React SPA
5. open browser

HTTP API：

~~~text
GET  /api/project
GET  /api/resources/{id}
POST /api/commands/{id}
GET  /api/events
~~~

Event 使用 SSE。

第一版不需要 Tauri/Electron。

---

## 17. Feature Renderer Registry

复杂控制面通过 Feature Renderer：

~~~text
ui.environment
ui.settings
ui.jobs
ui.logs
ui.scheduler
ui.workflow
ui.chat
ui.browser-capture
~~~

第一版只允许 DevTool first-party Renderer。

不允许项目动态注入任意 JS/CSS/React Bundle。

如果未来确实出现通用 Feature 无法覆盖的场景，再单独设计 sandboxed custom renderer。

---

## 18. Browser Extension 边界

### IDFlow

Browser Extension 保留：

- current tab
- debugger
- capture
- browser session
- quick actions

DevTool UI 接管：

- workflow management
- scheduler
- settings
- jobs
- logs

### BlogCTL

Browser Extension 保留：

- cookies/session
- browser login state
- browser-specific publishing handoff
- quick publish

DevTool UI 接管：

- publishing control
- jobs
- logs
- indexing
- environment
- settings

### DOWNKIT

Browser Extension 保留媒体检测和页面上下文。

完整任务/配置控制面可以逐步进入 DevTool UI。

---

## 19. Self-hosting

Self-hosting 是实现方案的硬约束。

### Stage 0

裸环境第一次没有 DevTool：

~~~text
OS + Go Toolchain
      │
      ▼
minimal bootstrap
      │
      ▼
build devtool binary
~~~

Stage 0 只做这一件事。

不允许 Stage 0 逐步长成第二套控制面。

### Stage 1

从有 DevTool 开始：

~~~text
DevTool N
    │
    ▼
DevTool Project Extension
    │
    ▼
PortableRuntime Contract
    │
    ▼
runtime.dagger
    │
    ├── build N+1 artifact
    ├── verify source + build N+1 + N+1 inspect itself
    └── package verified artifact
    │
    ▼
N+1 can invoke the same Project Extension + Runtime Contract again
    │
    ▼
N+1 -> build -> N+2, then N+2 inspects itself
~~~

### Windows

当前运行中的 devtool.exe 不直接覆盖自己。

流程：

~~~text
devtool.exe
    │
    ▼
build devtool-next.exe
    │
    ▼
verify devtool-next.exe
    │
    ▼
current process exits
    │
    ▼
minimal installer/bootstrap replaces binary
~~~

### Dogfooding Rule

任何新增通用能力都必须回答：

> DevTool 自己能否通过这套能力完成同样动作？

如果不能，需要重新检查能力所在层。

---

## 20. Provider / Extension 进程边界

第一阶段 Project Extension 可以采用独立 executable，以获得：

- Windows 支持
- crash isolation
- independent versioning
- protocol compatibility
- future non-Go implementation possibility

Host / Project Extension 使用双向 RPC。

原因：

Project 在执行 command 时，需要反向请求 Host Service，例如：

~~~text
provider.execute(device.full)
        │
        ▼
Project Extension
        │
        ├── portable-runtime.invoke
        └── adb.install
~~~

因此协议必须支持嵌套请求。

建议第一版：

~~~text
newline-delimited JSON
~~~

Envelope：

~~~go
type Envelope struct {
    ID      string
    ReplyTo string
    Type    string
    Method  string
    Payload json.RawMessage
}
~~~

Host 和 Extension 都必须持续读取输入并通过 pending request map 路由 response，避免双向 RPC deadlock。

stdout 只允许协议 frame；日志走 stderr 或 Event。

---

## 21. 安全与 Policy

Command 必须标记：

~~~text
read
write
deploy
destructive
~~~

Policy Hook 可以：

- allow
- deny
- require confirmation
- require approval

Secret：

- Resource 不返回原值
- Descriptor 不返回原值
- UI 只显示 configured/unconfigured
- 修改 Secret 通过 typed Command

Local UI：

- bind 127.0.0.1
- random port
- session token
- Origin validation
- mutation token
- destructive confirmation

---

## 22. Contract Validation

加载项目后先验证：

1. Command ID 唯一。
2. Resource ID 唯一。
3. Navigation 引用 View 存在。
4. View 引用 Resource 存在。
5. Action 引用 Command 存在。
6. Feature Renderer 已注册。
7. Field type 受支持。
8. Secret 不泄漏。
9. SideEffect 已声明。
10. Extension Service dependency 可满足。

错误 Descriptor：

~~~text
fail closed
~~~

---

## 23. 测试要求

### Core

- extension discovery
- service resolution
- missing provider
- dependency cycle
- duplicate service provider
- config validation

### RPC

- handshake
- incompatible protocol
- nested request
- concurrent requests
- timeout
- extension crash
- malformed frame
- stdout contamination

### Runtime Adapter

- Dagger unavailable
- Dagger function error
- artifact mapping
- secret mapping
- service lifecycle
- cancellation

### Control Surface

使用真实 fixture：

PhotoWaypoint：

- environment
- device actions
- jobs

BlogCTL：

- dynamic settings
- jobs
- logs

IDFlow：

- navigation
- scheduler
- workflow/chat/capture binding

### Self-host

必须有真实验收：

~~~text
DevTool N builds N+1
N+1 executes project inspect
N+1 verifies itself
~~~

---

## 24. 架构约束

后续 PR 必须满足：

1. Core 不 import 项目代码。
2. Core 不 import Dagger SDK。
3. Project Extension 不直接调用 Dagger SDK。
4. Project Extension 只依赖 Service Contract。
5. Runtime Extension 不包含项目语义。
6. Native Extension 不包含项目编排。
7. UI 不出现项目专属 Page package。
8. Browser Extension 不重新承载完整工程控制面。
9. devtool.toml 不承载 executable workflow。
10. CI 不复制 Project Extension 逻辑。
11. 不增加 Legacy Alias / fallback / dual-write。
12. DevTool 自身必须持续 self-host。

---

## 25. 最终调用链

~~~text
Human / Agent / CI / GUI
          │
          ▼
      DevTool Core
          │
          ▼
   Project Extension
          │
          ▼
     Service Contract
          │
      ┌───┴───────────────┐
      ▼                   ▼
Runtime Extension    Native Extension
      │                   │
    Dagger             ADB/Browser
      │
Infrastructure Extension
          │
          ▼
       Result
          │
          ├── Event
          ├── Resource
          ├── Artifact
          └── Evidence
~~~

长期目标不是让 DevTool 越来越大，而是让 Core 越来越稳定，让变化全部发生在可替换 Extension 中。


---

## 26. 调研参考

本方案中的外部产品边界主要参考以下成熟实现：

### Dagger

用于验证“Portable Runtime 不应由 DevTool 自己重新实现”的判断。

- https://docs.dagger.io/
- https://docs.dagger.io/reference/sdks/go/
- https://docs.dagger.io/reference/modules/
- https://docs.dagger.io/using/services/
- https://docs.dagger.io/reference/api/host/

重点借鉴：

- typed Module / Function
- Service
- Secret
- Cache
- Artifact
- Module dependency
- Local / CI 同构执行

### Backstage

用于验证“极简 App Shell + Plugin / Extension Point + Registry”的 UI/平台扩展模式。

- https://backstage.io/docs/frontend-system/architecture/
- https://backstage.io/docs/frontend-system/architecture/plugins/
- https://backstage.io/docs/overview/architecture-overview/

重点借鉴：

- Plugin / Extension
- Extension Point
- App wiring
- Utility API
- indirect routing

DevTool 不直接引入 Backstage Runtime。

### JSON Forms

用于验证 Settings / Command Parameters 的 Schema-driven Form 模式。

- https://jsonforms.io/docs/architecture

只借鉴和复用 Form Renderer，不把整个 Control Surface 建成 JSON Forms DSL。

### TanStack Query

用于 DevTool Web UI 的 Server State 管理。

- https://tanstack.com/query/latest/docs/framework/react/overview

Resource 更新通过 Event/SSE 驱动 query invalidation，而不是每个 Feature 自己维护一套 polling/loading/error 状态。

### React

DevTool 完整 Control Surface 使用 React + TypeScript；Browser Extension 保持轻量 TS/JS。

- https://react.dev/

外部产品只作为 Extension 实现或设计参考，任何第三方产品都不能成为 DevTool Core 的不可替换依赖。


---

## 18. Process Extension 与 Loader 收口（2026-10-04）

当前 Extension 装载路径已经统一为：

~~~text
.devtool.toml
     │
     ▼
Extension Loader Adapter
     │ resolve executable
     ▼
Core Process Host
     │ bidirectional RPC
     ├── provider service/tool calls
     └── provider -> Host service callback
~~~

Environment、Dagger、CodeGraph、Serena、SCM 都以独立 Process Extension 运行。

关键结果：

- 删除中央 extensions/catalog.go，不再按 Provider ID 写 switch；
- Core 不再执行 go build；
- Project Extension 与普通 Capability Extension 共用 executable-resolution 边界；
- Go-specific build/cache 位于 adapters/extensionloader；
- CodeGraph/Serena 作为进程 Extension，仍可通过双向 RPC 调用 environment Service；
- Provider 进程在 ProjectHost 生命周期内复用并显式关闭；
- Agent MCP 子进程继续由 MCP bridge 在同一 session 内复用。

这保持了“极简 Core + 可插拔 + 配置化”的边界，同时没有引入全局 daemon、插件市场或集群调度器。

完整生命周期、性能动机与 Kubernetes 边界见 docs/runtime-lifecycle.md。


---

## 19. Project Command Agent Adapter

Project Extension commands are automatically projected into the Agent Gateway instead of requiring a second agent-specific implementation.

~~~text
Project Extension
  -> ProjectDescriptor.Commands
  -> ProjectCommandProvider
  -> Agent Tool Registry
  -> MCP tools/list + tools/call
~~~

The adapter is generic and lives in the Agent layer. It does not know DevTool, PhotoWaypoint, BlogCTL or any project-specific command ID.

Mapping rules:

- tool name: `project_<normalized command id>`;
- title/description: copied from `CommandDescriptor`;
- input schema: derived from `FieldDescriptor`;
- required fields: preserved;
- select/multi-select options: exposed as JSON Schema enum values;
- side-effect metadata: exposed through MCP read-only/destructive annotations.

Invocation uses the same long-lived Project Extension process as CLI execution:

~~~text
Agent tools/call
  -> ProjectHost.ExecuteCommand(structured args)
  -> ProjectProcess.Execute
  -> Project Extension
~~~

CLI execution remains a thin compatibility surface for positional command invocation; structured Agent execution passes typed argument maps directly.

This preserves one authoritative project-command implementation across Human CLI, Agent MCP and future UI surfaces.
