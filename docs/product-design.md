# DevTool 产品方案

## 1. 产品定义

DevTool 是一个**自举式、可插拔的工程控制面（Engineering Control Plane）**。

它为开发者、Local Agent、Cloud Agent、CI/CD 提供统一的工程入口，并把不同项目、不同工具链、不同基础设施环境下的开发、验证、构建、制品、部署流程收敛为同一套控制面。

一句话定义：

> 项目定义“怎么开发”，DevTool 定义“怎么执行工程能力”，基础设施 Provider 定义“在什么环境里执行”。

DevTool 不是某一个项目的脚本集合，也不是 PhotoWaypoint 专用 CLI。它需要同时服务：

- PhotoWaypoint
- BlogCTL
- GoTiny
- DOWNKIT
- DevTool 自身
- 未来个人项目
- 未来公司内网项目

---

## 2. 要解决的问题

### 2.1 工程入口碎片化

一个项目常见同时存在：

- PowerShell
- Bash
- Makefile
- npm scripts
- Go helper
- Gradle task
- Docker Compose
- CI YAML
- Agent Prompt

同一件事情会被重复描述多次，例如：

```text
Local
  -> go test ./...

Agent
  -> go test ./...

CI
  -> go test ./...

Release
  -> another script
```

这些入口长期会产生漂移。

DevTool 的目标是：

```text
Human
Agent
CI
  \
   -> devtool verify
   -> devtool build
   -> devtool deploy
```

工程编排只保留一份事实来源。

---

### 2.2 项目控制面与通用能力混在一起

PhotoWaypoint 现有 DevTool 同时包含：

- CodeGraph / LSP
- Git / Process / Network
- Docker / PostgreSQL
- Android / ADB / Gradle
- Device workflow
- Gear workflow
- Beta / License / Deploy

其中前半部分属于所有项目都可能复用的工程能力，后半部分属于 PhotoWaypoint 自己的项目语义。

DevTool 必须把这两层彻底分开。

---

### 2.3 本地、Agent 与 CI 行为不一致

理想状态：

```text
Developer -> devtool verify
Agent     -> devtool verify
CI        -> devtool verify
```

三者执行同一个 Project Provider 和同一套 Capability。

CI 只负责：

- Trigger
- Runner
- Secret Injection
- Cache
- Approval
- Artifact Persistence

CI 不重新定义工程流程。

---

### 2.4 个人环境与企业内网环境不一致

个人环境可能使用：

- GitHub
- GitHub Actions
- GHCR
- Railway
- 公网 Go Proxy
- npmjs

企业环境可能使用：

- GitLab / Gerrit
- Jenkins / GitLab CI / Tekton
- Harbor
- Kubernetes / Internal PaaS
- Nexus / Artifactory
- 内网 Go Proxy / npm Registry
- 企业 Secrets / SSO / Proxy

项目不应该感知这些差异。

因此 DevTool 需要额外抽象：

> Infrastructure Provider

---

## 3. 核心产品原则

### 3.1 单一工程入口

所有项目都通过：

```text
devtool <command>
```

执行工程操作。

不允许项目再维护一套独立脚本作为工程流程事实来源。

---

### 3.2 控制面代码化，不脚本化

项目控制面必须使用正式代码实现。

例如：

```go
func Verify(ctx devtool.Context) error {
    if err := ctx.Go.Test("./..."); err != nil {
        return err
    }
    return ctx.CodeGraph.Verify()
}
```

而不是：

```text
verify.ps1
verify.sh
Makefile
npm run verify
```

脚本可以存在于第三方工具内部或兼容层，但不能作为 DevTool 项目控制面的最终事实来源。

---

### 3.3 项目可插拔

每个项目拥有自己的 Project Provider：

```text
PhotoWaypoint Provider
Blog Provider
GoTiny Provider
DOWNKIT Provider
DevTool Provider
```

项目依赖 DevTool SDK。

DevTool Core 不依赖具体项目。

---

### 3.4 工具能力可插拔

DevTool 提供通用 Capability：

```text
Git
Go
Node
Java
Docker
PostgreSQL
Android
ADB
Gradle
LSP
CodeGraph
Serena
Network
Process
Filesystem
Artifact
Deploy
```

Project Provider 负责组合这些能力。

---

### 3.5 基础设施可插拔

DevTool 不直接绑定：

- GitHub
- GHCR
- Railway
- Docker Hub
- npmjs
- proxy.golang.org

而是通过 Infrastructure Provider：

```text
Personal Infrastructure
Enterprise Infrastructure A
Enterprise Infrastructure B
```

提供：

- VCS
- Registry
- Package Source
- Secrets
- Network
- Deployment
- Identity
- Observability

---

### 3.6 Policy 横切所有执行

企业环境除了基础设施变化，还存在策略约束：

- 禁止公网访问
- 只能使用内部 Registry
- Production 必须审批
- Secrets 不允许落盘
- Artifact 必须安全扫描
- 日志必须脱敏
- 某些命令禁止开发者执行

因此 DevTool 需要 Policy 层：

```text
Command
  -> Project Provider
  -> Policy
  -> Capability
  -> Infrastructure
```

---

### 3.7 Build Once, Promote Many

制品只构建一次。

例如：

```text
app:git-a83fd21
```

同一个 artifact 依次进入：

```text
DEV
 -> STAGING
 -> PROD
```

不能在三个环境分别重新构建。

---

### 3.8 Self-hosting

DevTool 必须能够开发、验证、构建和发布 DevTool 自己。

```text
DevTool N
  -> build DevTool N+1
  -> DevTool N+1 verify itself
  -> install / publish
```

如果某个通用能力只能服务业务项目、不能服务 DevTool 自身，就需要重新检查它所在的层次。

---

## 4. 产品角色

### 4.1 Human Developer

开发者只需要知道：

```text
devtool doctor
devtool code ...
devtool build
devtool verify
devtool run
devtool package
devtool deploy
```

不需要记住项目底层所有工具链命令。

---

### 4.2 Local Agent

Agent 的默认规则：

1. 先读取 DevTool Project Context。
2. 优先通过 DevTool 查询代码、依赖和工程状态。
3. 不直接绕过 DevTool 调用底层工程命令。
4. 修改完成后执行统一 verify。
5. 通过结构化事件读取 artifact / evidence / warning / result。

---

### 4.3 Cloud Agent

Cloud Agent 和 Local Agent 使用相同控制面。

区别只在 Infrastructure Provider 和执行环境。

---

### 4.4 CI/CD

CI 调用：

```text
devtool verify
devtool build
devtool package
devtool deploy
```

流水线平台负责调度，不负责重新实现业务工程逻辑。

---

## 5. 产品能力模型

### 5.1 Core

```text
Project Discovery
Provider Loading
Command Registry
Task Execution
Structured Events
Cache
Evidence
Artifact
Configuration
Environment
```

### 5.2 Capability

```text
Git
Go
Node
Java
Docker
PostgreSQL
Android
ADB
Gradle
LSP
CodeGraph
Serena
Filesystem
Process
Network
Artifact
Deployment
```

### 5.3 Project Provider

```text
PhotoWaypoint
  build
  verify
  device
  gear
  beta
  deploy

Blog
  build
  dev
  verify
  site
  sync
  search
  ai-search

GoTiny
  build
  verify

DOWNKIT
  build
  verify
  package

DevTool
  build
  verify
  package
  install
  release
```

### 5.4 Infrastructure Provider

```text
Personal
  GitHub
  GHCR
  Railway
  public package registries

Enterprise
  GitLab / Gerrit
  Harbor
  Kubernetes / PaaS
  Nexus / Artifactory
  enterprise proxy
  enterprise secrets
```

---

## 6. 与 DevEnvironment 的关系

DevEnvironment 和 DevTool 解决两个不同问题。

### DevEnvironment

回答：

> 当前执行环境里有什么？

例如：

- Go
- Java
- Node
- Android SDK
- gopls
- CodeGraph
- Serena
- Docker CLI

### DevTool

回答：

> 这些能力应该如何组合来完成当前项目的工程动作？

关系：

```text
DevEnvironment
    |
    | provides executables
    v
DevTool Capability
    |
    v
Project Provider
    |
    v
Human / Agent / CI
```

---

## 7. 项目模型

### PhotoWaypoint

PhotoWaypoint 是第一验证项目，也是最复杂的项目。

它验证：

- Go + Android
- gomobile / AAR
- PostgreSQL
- Docker
- ADB
- 多服务启动
- Device workflow
- Beta
- Gear
- Deployment
- LSP / CodeGraph

PhotoWaypoint 可以驱动 DevTool 抽象，但不能成为 DevTool 的领域模型。

---

### BlogCTL

BlogCTL 现有 CLI 已经是博客领域控制面。

应拆成：

- 通用 workspace / process / doctor / node 能力 -> DevTool
- site / sync / search / AI Search / browser bridge -> Blog 领域

BlogCTL 的领域能力可以继续作为独立 CLI 存在，同时被 Project Provider 复用。

---

### GoTiny

GoTiny 是验证简单 Go Library Project Provider 的理想项目。

它应该证明：

> 普通 Go 项目只需要极少量 Project Provider 代码。

---

### DOWNKIT

DOWNKIT 验证：

- Go bridge
- Browser Extension
- Packaging
- 跨平台发布

---

### DevTool

DevTool 自己是第一方 self-hosting 项目。

它必须持续 dogfooding DevTool SDK 和 Capability。

---

## 8. 非目标

当前阶段不做：

- 通用 GUI IDE
- 替代 Git
- 替代 Docker / Kubernetes
- 替代 CI 平台
- 替代 package manager
- 在 DevTool Core 中实现具体业务领域
- 用配置语言重新发明另一种脚本系统
- 使用 Go 原生 `plugin` 动态 `.so` 作为扩展核心机制

---

## 9. 成功标准

第一阶段成功标准：

1. DevTool 独立仓库可构建。
2. DevTool 可以开发 DevTool 自己。
3. PhotoWaypoint 现有 DevTool 可以拆为 Core/Capability/Project Provider。
4. Local、Agent、CI 调用同一个 `devtool verify`。
5. GoTiny 可以用最小 Provider 接入。
6. BlogCTL 可以复用 DevTool Runtime，而不复制 workspace/process/doctor。
7. 项目工程流程不再以 PowerShell/Bash/npm scripts 为唯一事实来源。
8. DevTool 接口没有硬编码 GitHub / Railway / GHCR。
9. Personal Infrastructure 可运行。
10. 架构允许未来增加 Enterprise Infrastructure Provider，而无需修改 Project Provider。

---

## 10. 最终产品形态

```text
                         Human / Agent / CI
                                |
                                v
                         +--------------+
                         |   DevTool    |
                         |     Host     |
                         +------+-------+
                                |
              +-----------------+-----------------+
              |                                   |
              v                                   v
       Project Provider                         Policy
       project semantics                   execution constraints
              |                                   |
              +-----------------+-----------------+
                                |
                                v
                          Capabilities
                  Git / Go / Node / Docker
                  LSP / CodeGraph / Deploy
                                |
                                v
                    Infrastructure Provider
                                |
            +-------------------+-------------------+
            |                   |                   |
            v                   v                   v
         Personal          Enterprise A        Enterprise B
```
