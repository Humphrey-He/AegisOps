# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

[![CI](https://github.com/Humphrey-He/AegisOps/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Humphrey-He/AegisOps/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

`AegisOps` 是一个面向轻量运维控制面、应用交付与稳定性治理的 DevOps 平台项目。

当前仓库最新版本为 `v1.6.0`：在 v1.5.12 基线上完成一轮安全加固（SSH 主机密钥钉扎、登录限流、首启强制改密、任务抢占并发修复、secret 包测试补齐）并支持**单二进制分发**（前端经 go:embed 内嵌）。`go test ./...` 18 包全绿，CI 全绿。主干在此之上已并入 **Prometheus `/metrics` 自监控**（独立端口 `:9091`）与**数据库迁移版本化**（schema_migrations + 有序增量迁移），将随 v1.7.0 发版收口。

## 快速启动（单二进制，2 条命令）

```powershell
# ① 克隆并启动（前端已内嵌进二进制，无需 Node）
git clone https://github.com/Humphrey-He/AegisOps; cd AegisOps
$env:GOCACHE="$PWD\.gocache"; $env:AEGISOPS_DATABASE_DRIVER="sqlite"; $env:AEGISOPS_DATABASE_DSN="data/aegisops.db"; go run ./cmd/aegisops

# ② 浏览器打开 http://localhost:8080
#    初始账号 admin / admin123456，首次登录强制改密后才能使用
```

> 需要 PostgreSQL 作为主数据库时：`docker compose -f deploy/postgres/docker-compose.yaml up -d`。前端二次开发走 Vite 开发模式，见下文「前端启动」；改完执行 `npm run build` 并提交 `frontend/dist`（内嵌产物随仓库走）。

## 项目定位

AegisOps 更适合以下场景：

- 个人运维平台
- 小团队内部运维控制台
- 后端/平台工程方向面试项目
- DevOps 控制面产品原型与二期演进基线

它当前更偏向“单机可运行、链路可验证、领域建模完整”的轻量平台，而不是大规模多租户 SaaS。

## 当前已提供的能力（v1.6.0 基线 + 主干增量）

### 后端能力

- 身份与权限
  - 管理员初始化
  - 登录鉴权
  - 用户/角色管理
  - RBAC 权限控制
  - 审计日志
  - 登录失败限流（用户名 + IP，5 次失败锁 15 分钟，成功即清零）
  - 首启强制改密（初始口令公开，未改密前业务接口一律 403）
- 资产与凭证
  - 主机管理
  - SSH 连通性测试
  - Secret 加密存储与脱敏返回
  - SSH 主机公钥 TOFU 钉扎（known_hosts 语义存于 Host 记录，密钥变化即拒绝）
- 运维执行
  - 任务中心
  - 任务步骤与任务日志
  - Web 终端
- 应用交付
  - Docker 节点管理
  - 镜像仓库管理
  - 服务定义
  - 服务发布、升级、回滚
  - 发布后健康检查与回滚建议
- 稳定性治理
  - 通知通道
  - 告警规则
  - 告警事件
  - 主机与服务健康检查
  - Nginx 节点与配置发布/回滚
- 平台支撑
  - 导出 API
  - 备份 API
  - 调度任务 API（含后台到期派发）
  - 任务派发多实例抢占安全（条件更新 + RowsAffected 校验 + 租约）
  - Demo 数据注入
  - PostgreSQL 主阶段支持，SQLite 作为可选轻量模式
  - **数据库迁移版本化**：`schema_migrations` 版本表 + 有序增量迁移，启动时自动执行；SQLite/PostgreSQL 双驱动（CI 每次跑真实 Postgres 用例）；存量 v1.6.x 库原地升级不丢数据，坏迁移事务回滚并报错停止
  - **单二进制分发**：前端产物经 go:embed 内嵌，`go build` 产出即全量
  - **Prometheus /metrics 自监控**（独立端口 `:9091`，`metrics.enabled` 可关）：Go 运行时/进程指标、HTTP 请求计数与耗时（按路由模板）、任务派发抢占/执行/重试/终态、调度入队与 worker 错误计数

### 前端已落地页面

- 工作台
- 资产管理
  - 主机
  - 凭证
- 运行资源
  - Docker 节点
  - Nginx 节点
- 应用交付
  - 镜像仓库
  - 服务定义
- 任务中心与任务详情
- 告警事件
- 通知通道
- 告警规则
- 操作审计
- 系统管理
  - 用户
  - 角色
  - 调度任务
- 登录页
- 初始化管理员页
- Web 终端页

说明：

- 当前前端控制台已经能直接对接真实后端运行
- 导出、备份等能力目前以后端接口与数据能力为主，前端统一管理入口仍可继续增强

## 架构总览

```mermaid
flowchart LR
    subgraph FE["前端 frontend/ · React 18 + TypeScript + Ant Design 5 + Vite"]
        UI["控制台：工作台 / 资产 / Docker / 服务发布 / 任务 / 告警 / 审计 / Web 终端（xterm）"]
    end
    UI -- "HTTP /api · JWT" --> MW
    UI -- "WebSocket /terminal/sessions/:id/ws" --> TM
    subgraph BE["后端 cmd/aegisops · Gin · :8080"]
        MW["中间件链：Logger → Recovery → JWT 认证 → RBAC（RequirePermission）→ 审计"]
        TM["terminal 会话服务（SSH）"]
        MW --> HD["23 个 Handler · 约 145 条路由"]
        HD --> MOD
        subgraph MOD["领域模块 internal/"]
            GOV["治理：auth · rbac · audit · secret（加密存储 + 脱敏返回）"]
            RES["资源：host · docker · registry · nginx"]
            DELI["交付：service 发布/升级/回滚 · healthcheck"]
            RUN["运行：task executor+worker · scheduler · environment"]
            STAB["稳定：alert · notification"]
            EXP["支撑：exporter 导出/备份 · demo 数据"]
        end
    end
    MOD -- "GORM" --> DB[("PostgreSQL（主）/ SQLite（轻量）")]
```

## 目录结构

```text
cmd/         后端启动入口
configs/     配置文件
data/        本地运行产物与可选 SQLite 数据文件
deploy/      部署/烟雾测试相关资源
docs/        中文项目文档与规划
frontend/    前端控制台
internal/    后端业务实现
logs/        本地运行日志
pkg/         公共包
scripts/     本地辅助脚本
```

## 后端启动

环境要求：

- Go 1.24+
- Windows / Linux / macOS

启动命令：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

默认配置文件：

- [configs/config.yaml](./configs/config.yaml)

默认监听地址：

- HTTP API 与控制台：`:8080`
- Prometheus 指标：`:9091`（独立端口、无鉴权，`metrics.enabled: false` 可关闭）

默认数据库：

- PostgreSQL: `postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable`
- SQLite 可选示例配置：`configs/config.sqlite.example.yaml`

数据库迁移说明：

- 启动时自动执行版本化迁移：`schema_migrations` 版本表记录已应用迁移，干净库一键迁到最新，存量 v1.6.x 库原地升级不丢数据
- 迁移实现在 `internal/db/`（`migrate.go` runner + 按编号命名的迁移文件），每条迁移在单事务内执行，失败即回滚并停止
- **改 model 必须配套编号迁移**：`0001_baseline_v1_6_0` 是唯一保留的全量 AutoMigrate（冻结点），之后的 schema 变更一律走增量迁移并追加到 `migrations.go` 列表末尾

默认管理员账号：

```text
username: admin
password: admin123456
```

以上默认凭据与 DSN 仅用于本地开发与演示。用于长期环境前，请务必修改：

- 管理员密码
- `JWT Secret`
- `Secret Key`

## 前端启动

环境要求：

- Node.js 18+
- npm 9+

启动命令：

```powershell
Set-Location frontend
npm install
npm run dev
```

默认访问地址：

- [http://localhost:4173](http://localhost:4173)

当前前端开发模式默认直连真实后端，并通过本地代理转发 `/api` 到 `http://127.0.0.1:8080`。

生产构建预览：

```powershell
Set-Location frontend
npm run build
npm run preview
```

## 快速验证

健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
Invoke-RestMethod http://127.0.0.1:8080/readyz
```

指标端点（Prometheus 格式，独立端口、无鉴权，只应暴露给内网/监控网络）：

```powershell
(Invoke-WebRequest http://127.0.0.1:9091/metrics).Content | Select-String "aegisops_http_requests_total"
```

登录示例：

```powershell
$body = @{ username = "admin"; password = "admin123456" } | ConvertTo-Json
$login = Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/api/auth/login `
  -ContentType "application/json" `
  -Body $body
$login.data.tokens.accessToken
```

查询当前用户：

```powershell
$token = $login.data.tokens.accessToken
Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/api/auth/me `
  -Headers @{ Authorization = "Bearer $token" }
```

## 测试与发布基线

后端测试：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

前端构建检查：

```powershell
Set-Location frontend
npm run build
```

建议把上面两项作为最基础的提交前检查。

正式发版前，请对照：

- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## 环境变量覆盖

可以通过 `AEGISOPS_` 前缀环境变量覆盖后端配置，例如：

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_METRICS_ENABLED = "true"
$env:AEGISOPS_METRICS_ADDR = ":19091"
$env:AEGISOPS_DATABASE_DRIVER = "postgres"
$env:AEGISOPS_DATABASE_DSN = "postgres://aegisops:replace-me@127.0.0.1:5432/aegisops?sslmode=disable"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

本地 PostgreSQL 快速启动：

```powershell
docker compose -f deploy/postgres/docker-compose.yaml up -d
```

## 文档导航

建议优先阅读以下中文文档：

- [AegisOps产品定位与目标用户分析](./docs/AegisOps产品定位与目标用户分析.md)
- [AegisOps产品未来演进方向分析](./docs/AegisOps产品未来演进方向分析.md)
- [AegisOps一期MVP开发路线](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps一期开发手册（真实经验与排查方法）](./docs/AegisOps一期开发手册（真实经验与排查方法）.md)
- [AegisOps阶段审查报告（一期完成度与二期建议）](./docs/archive/AegisOps阶段审查报告（一期完成度与二期建议）.md)（历史快照，v0.x 阶段）
- [AegisOps UIUX视觉与易用性专业优化方案](./docs/AegisOps%20UIUX视觉与易用性专业优化方案.md)
- [AegisOps二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps二期专项规划：通知告警与健康检查闭环](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps二期专项规划：导出、备份与故障排查包](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps二期专项规划：权限细粒度、密钥管理与任务调度](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)
- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## 当前阶段说明

当前仓库可以定义为：

- 已完成 `v1.6.0` 版本基线：v1.5.12 的稳定演示基线 + 一轮安全加固 + 单二进制分发
- 主干已并入 Prometheus `/metrics` 自监控与数据库迁移版本化（详见上文能力清单）
- 已具备本地运行、功能演示、接口联调、继续二期开发的稳定基础
- 后续重点会从 `v1.6.x` 收口转向"生产化增强、外部集成增强、二期专项建设"（真实通知通道等仍是已知边界，见 Release 验收清单）

如果你把它当作个人项目来看，它已经足够像一个真实运维平台；
如果你把它当作长期产品来看，二期的重点会更偏向“把已经搭起来的控制面，继续打磨成更稳、更细、更强的生产系统”。
