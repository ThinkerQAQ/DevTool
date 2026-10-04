# DevTool 实现方案

## 1. 实现目标

DevTool 的实现目标是构建一个：

- 自举式
- 跨项目
- 跨平台
- 可插拔
- 可进入企业内网
- 对 Human / Agent / CI 提供统一入口

的工程控制面。

实现必须满足以下约束：

1. Project Provider 使用正式代码实现，不依赖脚本定义工程流程。
2. DevTool Core 不依赖具体项目。
3. Project Provider 不直接绑定 GitHub / GHCR / Railway 等具体基础设施。
4. Local / Agent / CI 调用相同 Project Provider。
5. DevTool 可以构建和验证 DevTool 自己。
6. Windows / Linux / Codespaces / CI 使用同一架构。
7. 不使用 Go 标准库 `plugin` 作为主要扩展机制。

---

## 2. 总体架构

```text
                           Bootstrap
                               |
                               v
                       +---------------+
                       | DevTool Host  |
                       +-------+-------+
                               |
             +-----------------+-----------------+
             |                 |                 |
             v                 v                 v
            SDK             Runtime        Capability Registry
             |                 |                 |
             v                 |                 |
      Project Provider         |                 |
             |                 |                 |
             +-----------------+-----------------+
                               |
                              Policy
                               |
                               v
                    Infrastructure Abstraction
                               |
                +--------------+--------------+
                |                             |
                v                             v
             Personal                    Enterprise
```

---

## 3. 仓库结构

推荐初始结构：

```text
DevTool/
├── cmd/
│   ├── devtool/
│   │   └── main.go
│   └── devtool-bootstrap/
│       └── main.go
│
├── protocol/
│   ├── version.go
│   ├── handshake.go
│   ├── command.go
│   ├── event.go
│   └── error.go
│
├── sdk/
│   ├── context.go
│   ├── project.go
│   ├── registry.go
│   ├── command.go
│   ├── provider.go
│   ├── result.go
│   └── artifact.go
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
│   ├── engine.go
│   └── rule.go
│
├── devcontrol/
│   ├── project.go
│   ├── build.go
│   ├── verify.go
│   ├── package.go
│   └── install.go
│
├── docs/
│   ├── product-design.md
│   └── implementation-design.md
│
├── go.mod
└── README.md
```

第一阶段不要一次性实现全部目录。目录代表最终边界，代码按迁移阶段逐步进入。

---

## 4. DevTool Host

Host 是用户看到的唯一通用入口：

```text
devtool <command>
```

Host 负责：

1. 查找项目根目录。
2. 读取最小项目元数据。
3. 解析 Infrastructure Profile。
4. 启动 Project Provider。
5. 完成协议 handshake。
6. 合并 Core Command 与 Project Command。
7. 执行 Policy。
8. 创建 Runtime Context。
9. 调用 Capability。
10. 输出结构化 Event。
11. 记录 Artifact / Evidence。

Host 不负责具体项目业务。

---

## 5. Project Discovery

项目根目录使用：

```text
.devtool.toml
```

该文件只负责发现和静态元数据，不允许演变成工作流 DSL。

示例：

```toml
version = 1

[project]
name = "PhotoWaypoint"

[provider]
type = "go"
module = "./go"
package = "./cmd/devprovider"
```

允许的内容：

- project name
- provider type
- provider source location
- protocol requirement
- optional default infrastructure profile

不允许的内容：

```toml
build = "go build ..."
verify = "./verify.ps1"
deploy = "npm run deploy"
```

工程逻辑必须写在 Provider 代码中。

---

## 6. Project Provider

### 6.1 定义

Project Provider 是项目自己的工程控制面实现。

依赖方向：

```text
Project
   |
   v
DevTool SDK
```

DevTool Core 不 import Project。

### 6.2 Provider 注册模型

建议 SDK：

```go
type Project struct {
    Name string
}

type CommandHandler func(Context, Request) error

type Registry interface {
    Project(Project)
    Command(name string, handler CommandHandler)
    Group(name string, register func(Registry))
}
```

PhotoWaypoint：

```go
func Register(r devtool.Registry) {
    r.Project(devtool.Project{
        Name: "PhotoWaypoint",
    })

    r.Command("build", Build)
    r.Command("verify", Verify)

    r.Group("device", func(r devtool.Registry) {
        r.Command("full", DeviceFull)
        r.Command("runtime", DeviceRuntime)
        r.Command("doctor", DeviceDoctor)
    })

    r.Group("gear", func(r devtool.Registry) {
        r.Command("sync", GearSync)
        r.Command("status", GearStatus)
    })
}
```

Provider main：

```go
func main() {
    devtool.Serve(project.Register)
}
```

---

## 7. Provider 运行方式

不采用：

```go
plugin.Open(...)
```

Project Provider 编译成独立 executable。

原因：

- Windows 支持
- Host / Provider 隔离
- 可独立升级
- 可独立崩溃恢复
- 协议可版本化
- 企业环境兼容
- 不受 Go `plugin` ABI / build tag 限制

运行模型：

```text
devtool
   |
   | stdin / stdout protocol
   v
project-provider
```

---

## 8. Provider 缓存

首次执行：

```text
find .devtool.toml
  |
  v
hash provider source
  |
  +-- cache hit  -> execute cached provider
  |
  +-- cache miss -> go build provider
                     |
                     v
                 cache binary
                     |
                     v
                   execute
```

建议缓存：

```text
.devtool/
└── cache/
    └── provider/
        ├── <source-hash>/
        │   └── provider.exe
        └── metadata.json
```

缓存目录属于 generated state，不进入 Git。

---

## 9. Host / Provider Protocol

第一阶段使用 line-delimited JSON over stdin/stdout，不一开始引入 gRPC。

最小协议包括 Handshake、Describe、Execute 和 Structured Event。

### 9.1 Handshake

Host：

```json
{
  "type": "handshake",
  "protocol": 1,
  "host_version": "0.1.0"
}
```

Provider：

```json
{
  "type": "handshake_result",
  "protocol": 1,
  "provider": "PhotoWaypoint",
  "provider_version": "0.1.0"
}
```

### 9.2 Describe

```json
{
  "project": "PhotoWaypoint",
  "commands": [
    "build",
    "verify",
    "device.full",
    "device.runtime",
    "gear.sync"
  ]
}
```

### 9.3 Execute

```json
{
  "type": "execute",
  "request_id": "req-123",
  "command": "device.full",
  "args": {
    "serial": "f9a1d1f2"
  }
}
```

### 9.4 Structured Event

```json
{
  "type": "event",
  "request_id": "req-123",
  "level": "info",
  "kind": "progress",
  "message": "Android build completed"
}
```

标准 kind：

```text
progress
log
warning
error
artifact
evidence
metric
result
```

Agent、CI、终端 UI 都消费同一套 Event。

---

## 10. SDK Context

Project Provider 不直接处理所有 OS 细节。

概念接口：

```go
type Context interface {
    Project() ProjectInfo

    Process() ProcessCapability
    Files() FilesystemCapability
    Network() NetworkCapability

    Git() GitCapability
    Go() GoCapability
    Node() NodeCapability
    Docker() DockerCapability
    Postgres() PostgresCapability
    Android() AndroidCapability
    ADB() ADBCapability
    CodeGraph() CodeGraphCapability
    LSP() LSPCapability

    Artifact() ArtifactCapability
    Deploy() DeploymentCapability

    Emit(Event)
}
```

实际实现应按 Go package 分拆，不建议做一个真正的巨型 interface；这里用于表达依赖模型。

---

## 11. Runtime

Runtime 负责所有跨项目基础执行语义。

### 11.1 Process

从 PhotoWaypoint 当前 `common.go` 抽：

- command execution
- stdout/stderr capture
- stdin forwarding
- environment merge
- timeout
- process lifecycle
- background process
- PID tracking
- Windows / Unix process handling

目标 API：

```go
result, err := ctx.Process.Run(process.Request{
    Program: "go",
    Args:    []string{"test", "./..."},
    Dir:     root,
})
```

### 11.2 Filesystem

统一：

- exists
- directory detection
- hashing
- JSON read/write
- atomic write
- temp dir
- artifact path
- cache path

### 11.3 Network

统一：

- HTTP client
- Proxy
- health check
- wait endpoint
- free port
- loopback
- download
- retry

所有 Capability 不直接使用裸 `http.DefaultClient`。

这样企业内网可以统一替换网络策略。

### 11.4 Environment

负责：

- env resolution
- executable lookup
- OS / arch
- DevEnvironment metadata
- runtime path
- package source injection

### 11.5 Evidence

每个 verify/build/deploy 可以产出 Evidence：

```text
command
git SHA
artifact hash
tool versions
provider version
test result
deployment target
timestamp
```

CI 与 Agent 使用同一 Evidence 模型。

---

## 12. Capability

Capability 是跨项目复用的工程原语。

原则：

> Capability 回答“如何完成一种通用工程动作”；Project Provider 回答“当前项目为什么、何时、以什么顺序组合这些动作”。

### 12.1 Go Capability

```go
type Go interface {
    Build(BuildRequest) (Artifact, error)
    Test(TestRequest) error
    Vet(VetRequest) error
    Version() (string, error)
    InstallTool(ToolRequest) error
}
```

GoTiny Provider：

```go
func Verify(ctx devtool.Context, _ devtool.Request) error {
    if err := ctx.Go().Test(devtool.GoTest{Packages: []string{"./..."}}); err != nil {
        return err
    }
    if err := ctx.Go().Test(devtool.GoTest{
        Packages: []string{"./..."},
        Race: true,
    }); err != nil {
        return err
    }
    return ctx.Go().Vet(devtool.GoVet{Packages: []string{"./..."}})
}
```

### 12.2 Code Intelligence Capability

PhotoWaypoint 当前 `code.go`、`code_tools.go`、`code_workspace.go` 中的以下能力迁到 DevTool：

- gopls
- Kotlin LSP
- Serena
- CodeGraph
- bootstrap
- doctor
- sync
- MCP integration
- workspace management

最终：

```text
devtool code doctor
devtool code graph sync
devtool code graph query ...
devtool code lsp ...
```

对所有项目通用。

### 12.3 Docker / PostgreSQL / Android / ADB

这些能力都属于 Capability。

PhotoWaypoint 的 Device workflow 留在 PhotoWaypoint Provider。

---

## 13. Infrastructure Provider

Project Provider 不允许直接依赖 GitHub、GitLab、GHCR、Harbor、Railway、Kubernetes、npmjs、proxy.golang.org。

概念接口：

```go
type Infrastructure interface {
    VCS() VCS
    Registry() Registry
    Packages() Packages
    Secrets() Secrets
    Network() Network
    Deployment() Deployment
    Identity() Identity
    Observability() Observability
}
```

实际实现拆成小接口。

### 13.1 Personal Infrastructure

第一阶段：

```text
VCS
  GitHub

Registry
  GHCR

Packages
  public Go / npm / Maven / OCI sources

Deployment
  Railway / local container

Secrets
  local environment / CI injection

Network
  direct / configured proxy
```

### 13.2 Enterprise Infrastructure

未来：

```text
VCS
  GitLab / Gerrit

Registry
  Harbor

Packages
  Nexus / Artifactory

Deployment
  Kubernetes / Internal PaaS

Secrets
  Vault / Enterprise Secret Manager

Network
  corporate proxy / allowlist

Identity
  SSO / workload identity
```

增加 Enterprise Provider 不修改 Project Provider。

---

## 14. Policy

Policy 位于 Project Provider 与实际 Capability 执行之间。

```text
devtool deploy prod
   |
   v
Project Provider
   |
   v
Policy
   |
   +-- artifact scanned?
   +-- source branch allowed?
   +-- approval exists?
   +-- secrets policy valid?
   |
   v
Deployment Capability
```

最小接口：

```go
type Decision struct {
    Allowed          bool
    RequiresApproval bool
    Reason           string
}

type Policy interface {
    Evaluate(Context, Action) (Decision, error)
}
```

第一阶段 Personal Policy 可以默认允许大多数动作，但接口必须保留。

---

## 15. DevEnvironment Integration

DevEnvironment 不承载项目工作流。

它负责提供统一工具镜像：

```text
DevEnvironment
├── Go
├── Java
├── Node
├── Git
├── Android SDK
├── gopls
├── CodeGraph
├── Serena
└── common CLI
```

DevTool Capability 负责调用和管理这些工具。

```text
DevEnvironment
       |
       v
Executable / Toolchain
       |
       v
DevTool Capability
       |
       v
Project Provider
```

---

## 16. Self-hosting / Bootstrap

### 16.1 Stage 0

第一次 clone DevTool 时没有 `devtool` binary。

Stage 0 只负责：

```text
detect Go
  |
  v
go build ./cmd/devtool
  |
  v
obtain DevTool
```

Stage 0 不负责 verify、deploy、CodeGraph、LSP、release、project workflow。

### 16.2 Stage 1

```text
DevTool N
  |
  v
devtool build
  |
  v
DevTool N+1
  |
  v
DevTool N+1 verify
  |
  v
DevTool N+1 smoke
  |
  v
install / release
```

### 16.3 Windows 安全替换

```text
devtool.exe
  |
  v
build -> devtool-next.exe
  |
  v
verify devtool-next.exe
  |
  v
exit current process
  |
  v
atomic install / replace
```

必要时使用极小 installer/bootstrap helper 完成替换。

---

## 17. DevTool 自己的 Project Provider

DevTool 也实现 Project Provider：

```text
DevTool/
└── devcontrol/
    ├── project.go
    ├── build.go
    ├── verify.go
    ├── package.go
    └── install.go
```

例如：

```go
func Register(r devtool.Registry) {
    r.Project(devtool.Project{Name: "DevTool"})
    r.Command("build", Build)
    r.Command("verify", Verify)
    r.Command("package", Package)
    r.Command("install", Install)
}
```

禁止 Host 中出现大量针对 DevTool 自身的 special case。

---

## 18. PhotoWaypoint 迁移

PhotoWaypoint 是第一迁移项目。

### 18.1 迁入 DevTool Runtime / Capability

从 `common.go`：

- process execution
- executable lookup
- env merge
- file operations
- hashing
- HTTP
- port allocation
- Docker detection
- generic process lifecycle

从 `code*.go`：

- gopls
- Kotlin LSP
- Serena
- CodeGraph
- code workspace
- bootstrap / doctor / sync

以及：

- Git generic operations
- PostgreSQL generic lifecycle
- Android SDK resolution
- ADB generic operations
- Gradle generic operations

### 18.2 留在 PhotoWaypoint Provider

- gomobile / AAR build
- device full/runtime
- gear catalog
- beta build
- beta config
- Firebase project semantics
- license
- deployment semantics
- PhotoWaypoint contracts
- project service definitions
- provider/recheck orchestration
- PhotoWaypoint-specific acceptance flow

### 18.3 迁移目标

最终删除：

```text
PhotoWaypoint/go/cmd/devtool
```

替换为：

```text
PhotoWaypoint/
├── .devtool.toml
└── go/
    ├── cmd/
    │   └── devprovider/
    └── devcontrol/
        ├── project.go
        ├── build.go
        ├── verify.go
        ├── device.go
        ├── gear.go
        ├── beta.go
        ├── services.go
        └── deploy.go
```

用户入口保持：

```text
devtool build
devtool verify
devtool device full
```

---

## 19. BlogCTL 迁移

BlogCTL 现有能力可拆为：

### 迁入 DevTool

- generic workspace primitives
- process runner
- doctor framework
- Node executable resolution
- Git
- Java
- Network
- toolchain discovery

### 留在 Blog Domain

- site
- publishing
- sync
- search
- AI Search
- browser bridge
- content/engine repository relationship
- platform adapters

BlogCTL 领域 API 可以同时被 `blogctl CLI` 和 DevTool Project Provider 调用，不复制领域实现。

---

## 20. GoTiny 迁移

GoTiny 用于验证最小 Provider。

目标结构：

```text
.devtool.toml
devcontrol/
```

Provider 只定义：

```text
build
verify
```

验证标准：

> 普通 Go Library 项目接入 DevTool 不需要复制 Runtime 逻辑。

---

## 21. DOWNKIT 迁移

DOWNKIT 用于验证：

- Go bridge
- Browser Extension
- Packaging
- Cross-platform artifact
- Release workflow

Provider 负责组合能力，底层 Process / Node / Go / Artifact 全部复用 DevTool。

---

## 22. CI/CD

CI YAML 只做平台集成：

```yaml
steps:
  - checkout
  - setup devtool
  - run: devtool verify
  - run: devtool build
  - run: devtool package
```

Production：

```text
devtool deploy staging
devtool deploy production
```

Deployment Capability 通过 Infrastructure Provider 决定实际使用 Railway、Kubernetes 或 Internal PaaS。

CI 不直接复制部署细节。

---

## 23. Artifact Promotion

```text
git SHA
  |
  v
devtool build
  |
  v
artifact
  |
  +-- SHA256
  +-- provenance
  +-- tool versions
  +-- source SHA
```

之后同一个 artifact：

```text
DEV
 |
 v
STAGING
 |
 v
PROD
```

---

## 24. Agent Integration

Agent 约定：

```text
1. devtool project context
2. devtool code ...
3. modify source
4. devtool verify
5. read structured evidence
```

Agent 不直接假设 Go project layout、Gradle task、Docker command、package registry 或 deployment platform。

---

## 25. 第一阶段实现顺序

### Phase A — Foundation

实现：

- `cmd/devtool`
- project discovery
- provider protocol
- provider build/cache
- SDK registry
- process runtime
- filesystem runtime
- environment runtime
- structured event
- self-hosting skeleton

完成标准：

```text
DevTool can discover itself
DevTool can build DevTool provider
DevTool can execute one project command
```

### Phase B — Code Intelligence

从 PhotoWaypoint 抽：

- CodeGraph
- gopls
- Kotlin LSP
- Serena
- code doctor
- code sync

完成标准：

```text
devtool code doctor
devtool code graph sync
```

不依赖 PhotoWaypoint。

### Phase C — PhotoWaypoint Provider

机械迁移现有流程，先不重写业务逻辑。

先迁：

- build
- verify
- device
- gear

完成标准：

```text
devtool build
devtool verify
devtool device full
devtool gear sync
```

行为与旧 DevTool 等价。

### Phase D — Infrastructure Capability

抽：

- Docker
- PostgreSQL
- Android
- ADB
- Gradle
- Git
- Network

PhotoWaypoint Provider 只负责组合。

### Phase E — GoTiny

用第二个项目验证通用抽象。

如果某个抽象只能被 PhotoWaypoint 使用，重新评估是否应进入 Core。

### Phase F — BlogCTL

迁移通用 Runtime：

- workspace
- process
- doctor
- node toolchain

保留 Blog Domain。

逐步取消 npm scripts 作为控制面事实来源，直接通过 Node Capability 调用真正 executable。

### Phase G — DOWNKIT

验证 Browser Extension、package、release、cross-platform。

### Phase H — Personal Infrastructure

正式实现：

- GitHub VCS
- GHCR
- package sources
- Railway / container deployment
- local/CI secrets

### Phase I — Enterprise Readiness

定义稳定企业接口：

- Enterprise VCS
- Registry
- Package Source
- Secrets
- Network
- Identity
- Policy
- Deployment
- Observability

当前不要求实现具体公司系统，但接口和依赖方向必须稳定。

---

## 26. 架构约束

以下约束作为 Code Review Rule：

1. DevTool Core 不 import 项目 package。
2. Project Provider 不直接调用 GitHub/Railway/GHCR SDK。
3. Project Provider 不重新实现通用 Process/Network/File helpers。
4. Capability 不包含 PhotoWaypoint/BlogCTL 等项目语义。
5. `.devtool.toml` 不允许演变成工作流 DSL。
6. CI 不复制 Project Provider 工程流程。
7. PowerShell/Bash/Make/npm scripts 不作为项目控制面的最终事实来源。
8. 所有长期执行流程提供结构化 Event。
9. Artifact 必须可追溯到 source SHA 和 build evidence。
10. DevTool 必须持续 self-hosting。

---

## 27. 最终调用链

```text
Human / Agent / CI
        |
        v
    devtool
        |
        v
Project Discovery
        |
        v
Project Provider
        |
        v
      Policy
        |
        v
   Capability
        |
        v
Infrastructure Provider
        |
        v
OS / DevEnvironment / Enterprise Platform
```

项目语义、工程能力、基础设施、企业策略各自只有一个清晰责任。
