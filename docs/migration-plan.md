# DevTool 迁移方案

## 1. 迁移目标

本迁移不是把现有多个 devtool CLI 合并到一个仓库，而是一次性切换到新的通用架构：

> **极简 Core + Extension Contract + 配置 Wiring + 可替换 Runtime + 通用 Control Surface + Self-hosting。**

最终状态必须满足：

~~~text
DevTool Core
    │
    ├── Project Extension
    ├── Runtime Extension
    ├── Native Extension
    ├── Infrastructure Extension
    ├── Policy Extension
    └── UI Feature Extension
~~~

所有项目只通过新架构接入。

---

## 2. 迁移原则

### 2.1 不保留历史兼容逻辑

最终主分支禁止存在：

- old devtool command alias
- legacy config reader
- old path fallback
- shell wrapper fallback
- old/new dual-write
- old/new runtime selection
- compatibility adapter
- deprecated command forwarding
- “先尝试新实现，失败就走旧实现”

迁移分支可以临时保留旧代码用于行为对比，但在 merge 前必须删除。

### 2.2 一次只迁一条真实链路

不先设计一个“大而全”框架再等待项目验证。

每个抽象必须由真实项目驱动。

顺序：

~~~text
DevTool Self-host
    ↓
GoTiny Portable Runtime
    ↓
PhotoWaypoint Portable + Native
    ↓
Control Surface
    ↓
BlogCTL
    ↓
IDFlow
    ↓
DOWNKIT
~~~

### 2.3 先建立新事实来源，再删除旧事实来源

每个项目迁移过程：

~~~text
新 Contract
    ↓
新 Extension
    ↓
真实执行验证
    ↓
CI/Agent/UI 切换
    ↓
删除旧实现
    ↓
merge
~~~

不是长期并存。

### 2.4 配置不承载行为

所有迁移后的 devtool.toml 只允许：

- project identity
- extension source
- service provider wiring
- profile
- UI Feature enablement

不允许迁移 shell command 到 TOML/YAML。

### 2.5 DevTool 必须最先 Dogfood

DevTool 自己不是最后迁移对象，而是第一验证项目。

如果 DevTool 不能用 DevTool 开发自己，不继续向其他项目扩散。

---

## 3. 目标仓库结构

~~~text
DevTool/
├── cmd/
│   ├── devtool/
│   └── devtool-bootstrap/
│
├── core/
│   ├── project/
│   ├── config/
│   ├── extension/
│   ├── registry/
│   ├── command/
│   ├── resource/
│   ├── event/
│   ├── policy/
│   ├── surface/
│   └── host/
│
├── protocol/
│
├── sdk/
│
├── extensions/
│   ├── runtime/
│   │   └── dagger/
│   ├── native/
│   │   ├── process/
│   │   ├── filesystem/
│   │   ├── adb/
│   │   └── browser/
│   ├── infrastructure/
│   └── ui/
│
├── ui/
│   └── web/
│
├── devcontrol/
│
└── docs/
~~~

不要在第一阶段创建没有实现的空 package；目录随真实能力逐步出现。

---

# Phase 0 — 固化 Contract

## 目标

先实现最小、无项目依赖的 Core Contract。

### 实现

- Project Discovery
- devtool.toml parser
- Extension Descriptor
- Extension Registry
- Service Registry
- Command Descriptor
- Resource Descriptor
- Event
- Project Descriptor
- Contract Validation

### 暂时不实现

- Dagger
- ADB
- React UI
- GitHub/Railway
- 企业抽象

### 验收

通过纯内存 fake extension 验证：

~~~text
project discovered
extension registered
service resolved
command registered
resource registered
descriptor validated
~~~

### Merge Gate

Core 中不得出现：

- PhotoWaypoint
- BlogCTL
- IDFlow
- Dagger
- GitHub
- ADB

领域名称。

---

# Phase 1 — Self-hosting Foundation

## 目标

让 DevTool 自己成为第一个 Project Extension。

### 新增

~~~text
DevTool/
├── .devtool.toml
└── devcontrol/
    ├── project.go
    ├── build.go
    ├── verify.go
    ├── package.go
    └── install.go
~~~

### Stage 0

只保留最小 bootstrap：

~~~text
Go Toolchain
    ↓
build cmd/devtool
~~~

Bootstrap 不负责：

- verify
- package
- release
- dependency management
- UI
- Dagger orchestration

### Stage 1

~~~text
DevTool N
    ↓
load DevTool Project Extension
    ↓
portable-runtime
    ↓
runtime.dagger
    ↓
build N+1
    ↓
N+1 inspect itself
    ↓
N+1 continues through the same portable-runtime contract
~~~

### Windows

生成：

~~~text
devtool-next.exe
~~~

验证后由最小 installer/bootstrap 在旧进程退出后替换。

### 验收

必须真实执行：

~~~text
devtool project inspect
devtool build
devtool verify
devtool package
~~~

并验证新 binary 可以独立运行。

### Merge Gate

禁止 Host 内出现：

~~~text
if project == "DevTool"
~~~

式 self-host 特判。

---

# Phase 2 — Runtime Extension：Dagger

## 目标

接入 Dagger，但保持 Core 零 Dagger 依赖。

### 新增

~~~text
extensions/runtime/dagger/
~~~

### Contract

第一版保持极小：

~~~text
PortableRuntime.Invoke
~~~

不要提前实现：

- UniversalContainer
- UniversalServiceGraph
- UniversalFilesystem DSL

Project/Module 用 typed invocation 描述真实需要。

### DevEnvironment

Dagger build image 优先复用：

- DevEnvironment base
- DevEnvironment android

避免再维护第二份 Go/Node/Java/Android SDK 版本。

### Runtime dogfooding 顺序

DevTool 自身先通过普通 Project Extension 调用 `portable-runtime -> runtime.dagger` 完成 build / verify / package，证明控制面本身不享受 Runtime 特例。

随后 GoTiny 作为第一个外部真实验证项目。

GoTiny Project Extension 只定义：

~~~text
build
verify
package
~~~

Portable 实现全部走 runtime.dagger。

### 验收

- Go build
- Go test
- race
- vet/static validation
- artifact export
- cache hit
- cancellation
- Dagger failure propagation

### Merge Gate

GoTiny 项目中删除新的工程入口之外的 devtool 编排逻辑。

不增加：

~~~text
if dagger unavailable -> old go run path
~~~

---

# Phase 3 — PhotoWaypoint Portable Migration

## 目标

先迁 PhotoWaypoint 可移植部分，不碰 ADB。

### 迁入 Dagger Module

~~~text
build
verify
gomobile AAR
Android JVM test
APK debug
APK beta
Postgres integration
provider-gateway service
recheck service
package
~~~

### DevEnvironment

Android build 继续使用共享 Android image：

~~~text
Java 17
Gradle
Android SDK 35
NDK
Go
~~~

### 配置

Host 读取本地配置后，通过 typed config/secret 输入 Runtime。

不要把整个用户 .local 目录 mount 进 build container。

### Build Evidence

由 DevTool Host 提供：

- source commit
- source digest

Runtime 返回：

- artifact digest
- runtime/module version
- toolchain evidence

不要要求 build container 依赖工作区 .git 内部状态。

### 验收

新 Runtime 输出必须覆盖当前：

- AAR
- Debug APK
- Beta APK
- Postgres integration
- provider/recheck service readiness
- evidence

### Merge Gate

迁移完成后，从旧 PhotoWaypoint devtool 删除：

- portable Go build helper
- Gradle orchestration
- Postgres Docker lifecycle
- generic service build/start logic

不能留旧 build path。

---

# Phase 4 — Native Extension + PhotoWaypoint Cutover

## 目标

实现 Native 层并完成 PhotoWaypoint 整体切换。

### 首批 Native Extension

~~~text
native.process
native.filesystem
native.adb
~~~

### native.adb

迁入通用能力：

- devices
- server start/restart
- reverse
- install
- shell
- launch
- logcat

### PhotoWaypoint Project Extension

保留编排：

~~~text
device.full
device.runtime
device.doctor
gear
beta
deploy
~~~

例如：

~~~text
device.full
  ↓
portable build
  ↓
portable services
  ↓
native.adb require device
  ↓
adb reverse
  ↓
install
  ↓
launch
~~~

### 验收

使用真实 Android 设备完成：

- device discovery
- build
- service start
- reverse
- install
- launch
- runtime health
- logs

### Merge Gate

删除：

~~~text
PhotoWaypoint/go/cmd/devtool
~~~

以及项目内已经被 DevTool Extension 替代的 generic helpers。

PhotoWaypoint 主分支只保留：

- Project Extension
- Dagger Module
- 项目 Domain

---

# Phase 5 — Control Surface Contract

## 目标

在已有真实 Command/Resource 基础上实现 UI Contract。

### 实现

~~~text
ProjectDescriptor
Navigation
View
Resource
Action
Event
FeatureBinding
~~~

### Primitive

只做：

- Section
- Group
- Grid
- Text
- Status
- KeyValue
- List
- Table
- Progress
- Log
- Form
- Field
- Action

### Feature Registry

第一批：

~~~text
environment
settings
jobs
logs
~~~

### Fixture

必须使用真实 Project Descriptor：

- DevTool
- PhotoWaypoint
- GoTiny

### Merge Gate

UI package 中不得出现：

~~~text
photowaypoint/
gotiny/
devtool-page/
~~~

这样的项目专属页面目录。

DevTool 自身也必须通过 Descriptor 渲染。

---

# Phase 6 — DevTool Web UI

## 目标

实现完整通用控制面壳。

### 技术栈

~~~text
React + TypeScript
TanStack Query
JSON Forms
SSE
Feature Registry
~~~

### Host

~~~text
devtool ui
~~~

Go Host：

- bind 127.0.0.1
- random port
- serve embedded SPA
- session token
- Project API
- Resource API
- Command API
- SSE Event

### 第一版不引入

- Tauri
- Electron
- Backstage Runtime
- arbitrary JS plugin

### 验收

同一个 UI binary 可以切换：

- DevTool
- GoTiny
- PhotoWaypoint

无需前端项目专属代码。

---

# Phase 7 — BlogCTL Migration

## 目标

利用 BlogCTL 现有动态 Descriptor 验证 Settings/Jobs/Logs 的通用性。

### 迁入 DevTool Control Surface

- Environment
- Settings
- Jobs
- Logs
- Indexing control where generic model applies

### 保留 Blog Domain

- article
- publication binding
- platform publisher
- search/indexing domain logic
- browser session semantics

### Browser Extension 缩薄

保留：

- cookie/session
- login state
- platform browser handoff
- quick browser action

完整控制面进入 DevTool UI。

### Portable

Blog build/test/Astro/Node/Go 工程执行迁入 Runtime Extension。

### Merge Gate

删除 BlogCTL 中已经由 DevTool Control Surface 取代的：

- environment UI duplication
- jobs/logs generic renderer duplication
- duplicated generic tool health renderer

不保留旧 Environment 页面 fallback。

---

# Phase 8 — IDFlow Migration

## 目标

验证复杂 Feature 和 Browser Context 分界。

### DevTool UI

迁入：

- Workflow management
- Scheduler
- Jobs
- Logs
- Settings

### Feature Renderer

新增：

~~~text
ui.scheduler
ui.workflow
ui.chat
ui.browser-capture
~~~

只有确实通用的部分进入 Feature。

### IDFlow Extension

保留：

- active tab
- debugger
- capture
- browser session
- quick action

### Native Browser Extension

DevTool 通过 Browser Service Contract 与 IDFlow Extension Bridge 协作。

### Merge Gate

删除 IDFlow popup 中已迁入 DevTool 的管理型页面。

Browser Extension 不再维护第二套 Scheduler/Settings/Jobs 管理 UI。

---

# Phase 9 — DOWNKIT Migration

## 目标

验证另一个 Browser + Native 项目。

### Portable

迁：

- Go build
- extension build
- package
- test

### Browser Extension

保留：

- media detection
- page context

### Native

保留：

- Native Messaging
- FFmpeg/local tools
- whisper/llama sidecar
- installation hooks where required

### UI

通用：

- Jobs
- Logs
- Settings
- Tool health

进入 DevTool UI。

---

# Phase 10 — Infrastructure Extension

## 目标

在 Core/Project/Runtime/UI 已稳定后再接基础设施。

### 第一批

按真实需要实现：

~~~text
vcs.github
registry.ghcr
deploy.railway
~~~

### 未来企业

真实公司环境出现后再增加：

~~~text
vcs.gitlab
registry.harbor
deploy.kubernetes
packages.nexus
secrets.vault
~~~

不要为了“未来可能”提前维护空接口和 fake implementation。

---

## 4. 项目最终形态

### DevTool

~~~text
.devtool.toml
devcontrol/
core/
extensions/
ui/
~~~

### PhotoWaypoint

~~~text
.devtool.toml
go/devcontrol/
dagger module
domain/application code
~~~

删除旧 go/cmd/devtool。

### GoTiny

~~~text
.devtool.toml
devcontrol/
dagger module
library code
~~~

### BlogCTL

~~~text
.devtool.toml
devcontrol/
dagger module
blog domain
thin browser extension
~~~

### IDFlow

~~~text
.devtool.toml
devcontrol/
dagger module
browser domain
thin browser extension
~~~

### DOWNKIT

~~~text
.devtool.toml
devcontrol/
dagger module
native/browser domain
~~~

---

## 5. 删除清单

每个项目 merge 前执行 Legacy Audit。

搜索并删除：

- old devtool entrypoint
- old shell orchestration
- old npm workflow orchestration where replaced
- old config fallback
- deprecated env alias
- old UI page
- duplicate job/log renderer
- old Docker/Postgres lifecycle wrapper
- duplicate code intelligence bootstrap
- old CI workflow logic

CI 最终只调用：

~~~text
devtool verify
devtool build
devtool package
~~~

不复制内部工程步骤。

---

## 6. 验收策略

迁移期间可以比较：

~~~text
old result
vs
new result
~~~

但这种比较只存在迁移分支和测试过程。

最终 main：

~~~text
only new path
~~~

不允许：

~~~text
old || new
new fallback old
legacy mode
compat mode
~~~

---

## 7. 最终完成标准

迁移完成必须同时满足：

1. DevTool 自己完整 self-host。
2. DevTool Core 不依赖 Dagger SDK。
3. 所有 Project 只依赖 DevTool Contract。
4. Dagger 可以通过替换 Runtime Extension 被移除。
5. PhotoWaypoint 真机流程完整运行。
6. BlogCTL / IDFlow / DOWNKIT Browser Extension 已缩成 Browser Context Adapter。
7. DevTool UI 能渲染所有项目控制面。
8. UI 没有项目专属页面实现。
9. CI/Agent/Human 使用相同 Command Contract。
10. 主分支不存在历史兼容、旧入口、fallback 和双轨逻辑。

最终架构必须保持：

> **极简内核负责稳定；Extension 负责变化；配置负责组合；DevTool 持续用自己开发自己。**
