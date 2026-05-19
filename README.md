# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

`AegisOps` 是一个面向轻量运维控制面、应用交付与稳定性治理的 DevOps 平台项目。

当前仓库最新正式版本为 `v1.0.0`，已经不是“只有脚手架”的状态，而是具备可本地运行、可演示、可联调、可继续扩展的一期正式基线。

## 项目定位

AegisOps 更适合以下场景：

- 个人运维平台
- 小团队内部运维控制台
- 后端/平台工程方向面试项目
- DevOps 控制面产品原型与二期演进基线

它当前更偏向“单机可运行、链路可验证、领域建模完整”的轻量平台，而不是大规模多租户 SaaS。

## 当前版本评价

从当前实现看，这个项目的优点比较明确：

- 领域边界清楚：围绕主机、Secret、Docker、Registry、服务发布、Nginx、任务、告警、通知、审计展开，主线完整
- 后端完成度高：不仅有 CRUD，还有 RBAC、审计、发布链路、健康检查、告警联动、导出/备份、调度等平台能力
- 前端不是纯展示壳：已经形成工作台式控制台，能承载真实业务对象和任务上下文
- 文档资产完整：产品定位、一期开发经验、阶段审查、二期规划、Release 验收口径都比较齐
- 本地可运行性好：默认 PostgreSQL 作为主阶段数据库，也保留 SQLite 作为本地演示与轻量开发选项

当前仍然存在的边界也需要如实说明：

- 默认部署形态仍以单机/本地环境为主，大规模生产治理能力还需要二期继续增强
- 导出、备份等能力以后端 API 为主，前端统一操作入口仍有继续补齐空间
- 权限细粒度、密钥生命周期、调度器高级能力、真实外部通知集成仍属于重点增强方向
- 若用于更严肃的生产环境，仍应继续做真实主机、Docker、Nginx、通知通道的长期回归验证

一句话评价：这是一个完成度较高、方向明确、很适合作为“后端主导的 DevOps 控制面项目”的 `v1.0.0` 基线。

## v1.0.0 已提供的能力

### 后端能力

- 身份与权限
  - 管理员初始化
  - 登录鉴权
  - 用户/角色管理
  - RBAC 权限控制
  - 审计日志
- 资产与凭证
  - 主机管理
  - SSH 连通性测试
  - Secret 加密存储与脱敏返回
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
  - 调度任务 API
  - Demo 数据注入
  - PostgreSQL 主阶段支持，SQLite 作为可选轻量模式

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

- `:8080`

默认数据库：

- PostgreSQL: `postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable`
- SQLite 可选示例配置：`configs/config.sqlite.example.yaml`

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
- [AegisOps阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps UIUX视觉与易用性专业优化方案](./docs/AegisOps%20UIUX视觉与易用性专业优化方案.md)
- [AegisOps二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps二期专项规划：通知告警与健康检查闭环](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps二期专项规划：导出、备份与故障排查包](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps二期专项规划：权限细粒度、密钥管理与任务调度](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)
- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## 当前阶段说明

当前仓库可以定义为：

- 已完成 `v1.0.0` 正式版本基线
- 已具备本地运行、功能演示、接口联调、继续二期开发的稳定基础
- 后续重点会从“一期主干搭建”转向“生产化增强、外部集成增强、二期专项建设”

如果你把它当作个人项目来看，它已经足够像一个真实运维平台；
如果你把它当作长期产品来看，二期的重点会更偏向“把已经搭起来的控制面，继续打磨成更稳、更细、更强的生产系统”。
