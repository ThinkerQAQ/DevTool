# DevTool 产品方案（PRD）

## 1. 一句话定义

> **DevTool 是一个极简、可扩展、配置驱动、可自举的工程控制面：Core 只负责发现、装配、路由、协议和呈现，所有项目能力通过 Extension 接入。**

DevTool 面向 Human、Local Agent、Cloud Agent、CI/CD 和统一 GUI。目标是让不同项目共享同一套工程控制面，而不再分别维护 CLI、UI、Agent Tool、CI Workflow 和本机控制逻辑。

核心原则固定为：

> **极简内核 + 可扩展 + 配置化 + 可自举。**

其中：

- **极简内核**：Core 只保留稳定且跨项目的机制，不承载项目领域逻辑。
- **可扩展**：Portable Runtime、Native Capability、Infrastructure、Project Control、Policy、UI Feature 都通过 Extension 接入。
- **配置化**：配置只负责发现、选择、组合和 wiring；行为与工程编排必须写正式代码，不把 TOML/YAML 变成新的脚本语言。
- **可自举**：DevTool 必须能够使用 DevTool 自己完成开发、验证、构建、打包和发布。

---

## 2. 产品目标

DevTool 需要同时支持：

- PhotoWaypoint
- BlogCTL
- IDFlow
- GoTiny
- DOWNKIT
- DevTool 自身
- 后续个人项目
- 后续企业内网项目

统一入口：

~~~text
Human
Agent
CI
GUI
   │
   ▼
DevTool
   │
   ├── Build / Test / Package
   ├── Code Intelligence
   ├── Services / Jobs / Logs
   ├── Device / Browser / Native OS
   ├── Artifact / Deploy
   └── Settings / Environment
~~~

同一工程能力只允许有一份正式实现。

---

## 3. 真实代码 Review 基线

本 PRD 基于现有项目代码重新收敛，而不是从抽象概念直接设计。

### 3.1 PhotoWaypoint

已重点 Review：

- go/cmd/devtool/main.go
- common.go
- build.go
- device.go
- services.go
- postgres.go
- code*.go
- Android Settings Model / Renderer
- Scene Plan / Result UI Registry
- DevEnvironment Base / Android 镜像

确认：

1. 当前 PhotoWaypoint DevTool 混合了通用 Runtime、工程 Capability 和项目编排，最需要拆层。
2. Go、gomobile、Gradle、PostgreSQL、provider-gateway、recheck 等大量逻辑属于 Portable Execution。
3. ADB、USB 真机、adb reverse、安装 APK、logcat 属于 Host Native。
4. Settings Model/Renderer 已验证“结构化描述 + 通用 Renderer”可行。
5. Scene UI Registry 已验证“稳定主干 + Registry + Extension”可行。

### 3.2 BlogCTL

已重点 Review：

- toolDescriptor
- toolConfigView
- toolHealth
- toolAction
- Environment 动态工具卡
- Tasks / Logs
- Publishing / Indexing
- Browser Extension / Native Bridge

确认：

1. BlogCTL 已经有成熟的 Descriptor-driven UI 雏形。
2. Settings / Environment / Jobs / Logs 具有明显通用性。
3. Publishing / Indexing 的复杂度说明不能把所有 UI 都硬塞进通用 Form DSL。
4. Browser cookie/session/native publishing 仍属于 Browser/Native Domain。

### 3.3 IDFlow

UI Review 基线为当前新 UI 分支/PR 中的：

- AI Chat
- Workflow
- Schedule
- Capture
- Settings
- Browser Bridge

确认：

1. 信息架构清晰，但当前 Tab、DOM、Bridge Endpoint 仍大量项目硬编码。
2. Scheduler / Settings / Jobs 适合通用化。
3. Chat / Browser Capture 需要 Feature Renderer。
4. Chrome activeTab/debugger/scripting/tabs 等能力只能留在 Browser Extension。

### 3.4 GoTiny / DOWNKIT

GoTiny 是纯 Go Runtime/Library 项目，适合验证最小 Portable Runtime 接入。

DOWNKIT 同时具备：

- Go build/package
- Browser Extension
- Native Messaging
- Windows/Linux/macOS 本机 sidecar

适合验证 Portable 与 Native 分界。

---

## 4. 市面方案调研后的产品决策

### 4.1 Dagger：作为可替换 Portable Runtime Extension

Dagger 已经提供：

- typed Module / Function
- Container / File / Directory
- Service
- Secret
- Cache
- Artifact
- Module dependency
- Local / CI 同构执行

它与 DevTool 的 Portable Execution 高度重叠。

产品决策：

> **不复制 Dagger 源码，不 fork Dagger，不让 Dagger API 泄漏进 DevTool Core。**

DevTool 只通过 Runtime Contract 和 Adapter 使用 Dagger。

未来 Dagger 不满足要求时，只替换 Runtime Extension，不修改：

- DevTool Core
- Project Extension
- Control Surface
- Native Extension
- Infrastructure Extension

### 4.2 Backstage：借鉴 Plugin / Extension Point，不直接作为本地 DevTool

Backstage 已证明：

- App 负责 wiring
- Plugin 提供功能
- Extension 组成 UI
- Utility API 解耦实现
- Route 可以通过间接引用解耦

DevTool 借鉴其 Extension 思想，但不直接引入整套 Backstage，因为 DevTool 的核心场景是：

- localhost
- 当前 workspace
- ADB
- Browser Extension
- Native Process
- Local Agent

### 4.3 JSON Forms：只负责 Schema Form

JSON Forms 的 Schema + Renderer Registry 模式适合：

- Command 参数
- Settings
- Config

但不承担：

- Jobs
- Chat
- Workflow
- Device Dashboard
- Browser Capture

复杂 UI 使用 DevTool Feature Renderer。

### 4.4 其他产品

Devbox、Score、Humanitec、Port 等在可复现环境、部署抽象、企业控制面方面有可借鉴点，但当前不作为 DevTool 第一阶段硬依赖。

原则：

> **已有成熟基础设施就通过 Extension 接入；DevTool 不重造已有且非差异化的执行引擎。**

---

## 5. 产品架构原则

### 5.1 Core 必须极简

Core 只负责：

~~~text
Project Discovery
Configuration
Extension Discovery
Extension Lifecycle
Registry
Service Routing
Command / Resource / Event Contract
Control Surface Contract
Policy Hook
UI Host
Self-host Bootstrap Boundary
~~~

Core 不包含：

- PhotoWaypoint Device
- Blog Publishing
- IDFlow Capture
- Dagger Container API
- GitHub API
- ADB 项目流程
- 企业内部平台逻辑
- 项目专属 React 页面

### 5.2 所有能力通过 Extension 接入

统一扩展模型：

~~~text
DevTool Extension
├── Project Extension
├── Runtime Extension
├── Native Extension
├── Infrastructure Extension
├── Policy Extension
└── UI Feature Extension
~~~

#### Project Extension

定义：

> 当前项目有哪些 Command、Resource、Control Surface，以及项目级编排。

#### Runtime Extension

定义：

> 可移植工程执行能力由谁实现。

第一实现：

~~~text
runtime.dagger
~~~

#### Native Extension

定义：

> 宿主机、物理设备、浏览器等无法自然容器化的能力。

例如：

~~~text
native.adb
native.browser
native.process
native.filesystem
~~~

#### Infrastructure Extension

例如：

~~~text
vcs.github
registry.ghcr
deploy.railway
~~~

未来可替换为：

~~~text
vcs.gitlab
registry.harbor
deploy.kubernetes
~~~

#### UI Feature Extension

定义复杂控制面 Renderer：

~~~text
ui.jobs
ui.logs
ui.scheduler
ui.workflow
ui.chat
ui.browser-capture
~~~

---

## 6. 配置化原则

配置只负责：

~~~text
发现
选择
组合
实例化
wiring
~~~

项目行为仍写正式代码。

示意：

~~~toml
version = 1

[project]
name = "PhotoWaypoint"

[extension.project]
type = "go"
module = "./go/devcontrol"
package = "./cmd/provider"

[extension.runtime]
type = "go"
module = "."
package = "./extensions/runtime/dagger/cmd/provider"

[service.portable-runtime]
provider = "runtime.dagger"

[service.adb]
provider = "native.adb"

[service.vcs]
provider = "vcs.github"

[ui]
features = ["environment", "jobs", "logs", "settings"]
~~~

禁止：

~~~toml
build = "go build ..."
verify = "./verify.ps1"
deploy = "npm run deploy"
~~~

DevTool 不创建新的脚本 DSL。

---

## 7. Portable 与 Native 的边界

~~~text
                    Project Extension
                          │
                    DevTool Contract
                          │
             ┌────────────┴────────────┐
             ▼                         ▼
      Portable Runtime            Native Runtime
             │                         │
      Runtime Extension           Native Extension
             │                         │
           Dagger                ADB / Browser / OS
~~~

### Portable

优先进入 Runtime Extension：

- Go build/test
- Node/TypeScript build/test
- Gradle build
- gomobile build
- PostgreSQL integration
- Service lifecycle
- package
- artifact
- cache
- secret injection
- CI execution

### Native

保留在 Native Extension：

- USB / ADB
- adb reverse/install/logcat
- real Chrome/Edge session
- Browser debugger/cookies/tabs
- Native Messaging
- Windows Registry
- 系统编辑器
- 本机进程/设备集成

---

## 8. 统一 Control Surface

DevTool UI 是所有项目的完整工程控制中心。

核心模型只保留：

~~~text
Navigation
View
Resource
Action
Event
~~~

Project Extension 返回 Project Descriptor：

~~~text
ProjectDescriptor
├── Commands
├── Resources
├── Navigation
├── Views
└── FeatureBindings
~~~

DevTool UI 不允许出现：

~~~text
PhotoWaypointPage
BlogCTLPage
IDFlowPage
~~~

项目通过 Descriptor 和 Feature Binding 获得 UI。

---

## 9. UI 产品边界

### 9.1 DevTool UI

负责完整控制中心：

- Environment
- Settings
- Jobs
- Logs
- Scheduler
- Build / Verify
- Services
- Artifact
- Deploy
- Code Intelligence
- Workflow / Chat 等可选 Feature

### 9.2 Browser Extension

Browser Extension 只保留浏览器上下文能力和必要 Quick UI。

IDFlow 示例：

~~~text
Extension
├── Current Tab
├── Capture
├── Browser Session
└── Quick Action

DevTool UI
├── Workflow
├── Scheduler
├── Jobs
├── Logs
└── Settings
~~~

BlogCTL / DOWNKIT 同理。

原则：

> **必须发生在浏览器里的能力留 Extension；完整控制面统一进入 DevTool UI。**

---

## 10. UI 技术方向

第一版：

~~~text
Go Host
  +
React + TypeScript SPA
  +
TanStack Query
  +
JSON Forms
  +
DevTool Feature Registry
  +
SSE Events
~~~

Browser Extension 继续保持轻量 TypeScript/JavaScript，不要求为了统一技术栈迁 React。

第一阶段不引入：

- Backstage Runtime
- Tauri
- Electron
- 任意项目 JS Plugin
- 万能 UI DSL

---

## 11. Self-hosting 是一级能力

DevTool 必须把自己视为一个普通项目。

~~~text
DevTool N
   │
   ▼
DevTool Project Extension
   │
   ▼
Runtime / Native / Infrastructure Extension
   │
   ▼
Build DevTool N+1
   │
   ▼
N+1 Verify N+1
   │
   ▼
Package / Install / Release
~~~

Core 不允许出现大量：

~~~text
if project == DevTool
~~~

式特殊逻辑。

Self-hosting 的意义是持续证明：

1. Project Extension 契约足够通用。
2. Runtime Extension 能服务控制面自身。
3. Build / Verify / Package / Release 没有依赖外部隐藏脚本。
4. DevTool 的开发者、Agent 和 CI 真正使用同一个 DevTool。

如果 DevTool 自己无法通过 DevTool 开发，当前架构视为未完成。

---

## 12. 非目标

当前不做：

- 通用 IDE
- 第二个 Dagger
- 第二个 Kubernetes
- 第二个 Backstage
- YAML/TOML Workflow Engine
- 项目专属 UI 框架
- 任意前端 Bundle 注入
- 历史兼容层
- Legacy Config 迁移器
- 旧 CLI Alias
- 新旧控制面长期双轨运行

---

## 13. 第一阶段成功标准

1. Core 不依赖任何项目和 Dagger API。
2. Extension 可以注册 Service / Command / Resource / Feature。
3. Project Extension 可以调用抽象 Service，而不知道具体实现。
4. runtime.dagger 可以被替换，不影响 Core Contract。
5. PhotoWaypoint Portable Build 可由 Dagger 执行，ADB 由 Native Extension 执行。
6. GoTiny 可以使用最小 Project Extension。
7. BlogCTL Settings/Jobs/Logs 可以由通用 Control Surface 表达。
8. IDFlow Scheduler/Settings 可以通用化，Capture 保留 Browser Extension。
9. DevTool UI 不包含项目命名空间和项目专属页面代码。
10. DevTool 可以完整 self-host：build / verify / package / install / release。
11. 最终主分支不存在旧 DevTool 兼容逻辑。
12. 所有迁移后的项目只保留新架构入口。

---

## 14. 最终产品形态

~~~text
                       Human / Agent / CI / GUI
                                 │
                                 ▼
                          DevTool Core
                极简 Discovery / Registry / Router
                                 │
                  ┌──────────────┼──────────────┐
                  ▼              ▼              ▼
             Project Ext    Runtime Ext     Native Ext
                  │              │              │
                  │           Dagger        ADB/Browser
                  │
                  ├──────── Infrastructure Ext
                  │
                  └──────── UI Feature Ext
                                 │
                                 ▼
                         Control Surface
~~~

长期约束：

> **Core 越小越稳定；能力全部通过清晰 Contract 扩展；配置只做 wiring；项目不依赖具体实现；DevTool 必须持续用自己开发自己。**


---

## 13.1 Agent Capability 不做代理层

Agent-facing Capability 只表达稳定工程意图，不机械镜像底层 Provider API。

禁止：

~~~text
Provider 新增 findReferences
        ↓
DevTool 立刻新增 code_references

Provider 新增 diagnostics
        ↓
DevTool 立刻新增 code_diagnostics
~~~

这种设计会让 DevTool 退化成 MCP/API Proxy，并让 Agent Tool 数量随底层实现无限增长。

正确分层：

~~~text
Cloud / Local Agent
        │
        ▼
Intent-level Capability
        │
        └── code_context
                │
                ├── indexed search
                ├── realtime symbols
                ├── references
                └── diagnostics
                        │
                        ▼
              replaceable Providers
~~~

约束：

1. 新增 Provider 方法，不自动产生新的 Agent Tool。
2. Provider Contract 可以细粒度，因为它只服务 Extension 之间的内部适配。
3. Agent Tool 只有出现新的稳定工程意图时才新增。
4. 一个 Capability 可以组合多个 Service/Provider，并附带 project/worktree 上下文。
5. 对外结果应表达工程上下文或工程结果，而不是简单转发底层工具响应。

当前 Code Intelligence 的第一版 Agent Capability 只暴露：

~~~text
code_context
~~~

底层仍可以扩展 `search / symbols / references / diagnostics / impact / callers / callees` 等语义，但这些不会按 1:1 关系暴露到 Agent Tool surface。

---

## 14. Agent Runtime 边界与项目价值

DevTool 已经从“统一 CLI 壳”演进为 **project-scoped capability runtime for agents**，但产品边界仍然保持克制。

~~~text
Human / Local Agent / Cloud Agent / CI
                 │
                 ▼
              DevTool
     config / capability / policy
       provider routing / lifecycle
                 │
                 ▼
        Environment / Runtime
                 │
       ┌─────────┼─────────┐
       ▼         ▼         ▼
     Docker    Podman    Kubernetes
~~~

DevTool 负责的是：

- 项目声明一次能力和 Provider；
- Local Agent / Cloud Agent 使用同一套 Capability Contract；
- CodeGraph、LSP、SCM、Runtime 等能力统一发现和路由；
- Provider 生命周期、side effect/policy 和 self-hosting；
- 避免 Agent 临时拼 shell command 或自行决定 fallback。

Docker / Kubernetes 等负责的是执行基础设施。Kubernetes 可以成为 Environment Provider，但不会替代 DevTool 的 Agent/Project capability 语义。

明确不进入 DevTool Core 的职责：

- Pod/Node 调度；
- 集群网络和 Service Discovery；
- Autoscaling；
- 容器恢复策略；
- 多节点资源编排。

当前演进依据真实问题驱动：

~~~text
thin wrapper
  -> Project Contract
  -> Extension / Provider
  -> config wiring
  -> Agent Gateway
  -> CodeGraph + LSP
  -> SCM
  -> self-hosting
  -> process/MCP/container lifecycle reuse
  -> Environment abstraction
  -> generic process extension + external loader
~~~

详细性能问题、Kubernetes 边界和 Lifecycle 决策见 docs/runtime-lifecycle.md。
