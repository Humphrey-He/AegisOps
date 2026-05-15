# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps 是一个面向轻量运维控制面、交付发布与稳定性治理的 DevOps 平台。当前仓库包含：

- Go + Gin + SQLite + GORM + zap 的后端 API 与任务执行主干
- React + TypeScript + Vite 的前端控制台
- 中文优先的产品、开发、审查、二期规划与 Release 文档

GitHub 默认优先展示本中文文档；日文与英文说明请查看对应语言版本。

## 项目概览

按当前仓库实现，AegisOps 已具备一套可运行的本地运维平台主干：

- 身份与权限：管理员初始化、登录鉴权、用户/角色管理、RBAC 权限控制、审计日志
- 资产与凭证：主机管理、SSH 连通性测试、Secret 加密存储与脱敏返回
- 运维执行：任务中心、任务步骤与任务日志、Web 终端
- 容器与交付：Docker 节点、镜像 Registry、服务定义、发布、升级、回滚
- 稳定性治理：通知通道、告警规则、告警事件、主机与服务健康检查
- 发布闭环：发布后自动探活、失败后的回滚建议、Nginx 节点管理与配置回滚
- 平台支撑：导出、备份、调度器 API，以及开发环境 Demo 数据注入

当前前端控制台已经提供的主要业务页面包括：

- 工作台
- 资产管理：主机、凭证
- 运维执行：Docker、Nginx
- 交付发布：Registry、服务定义
- 任务中心与任务详情
- 稳定性与告警配置
- 审计日志
- 系统管理：用户、角色
- 登录、初始化管理员、终端等辅助页面

## 目录说明

```text
cmd/         后端启动入口
configs/     配置文件
data/        SQLite 数据与本地运行产物
docs/        中文项目文档与专项规划
frontend/    前端控制台
internal/    后端业务实现
logs/        本地运行日志
pkg/         公共包
```

## 后端启动

环境要求：

- Go 1.24+
- Windows、Linux 或 macOS

启动命令：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

默认配置文件位于 [configs/config.yaml](./configs/config.yaml)。

默认监听地址：

- `:8080`

默认 SQLite 数据库：

- `data/aegisops.db`

默认管理员账号：

```text
username: admin
password: admin123456
```

仅可用于本地开发环境，外部使用前请务必修改默认管理员密码、JWT Secret 与 Secret Key。

当前 `app.env` 为 `dev`、`development` 或 `test` 时，后端会自动注入 Demo Registry、Demo Docker 节点、示例服务与实例数据，便于本地演示发布、回滚与健康检查链路。

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

当前前端开发模式已默认对接真实后端，并通过本地代理转发 `/api` 到 `http://127.0.0.1:8080`。

如果需要做生产构建预览，可执行：

```powershell
Set-Location frontend
npm run build
npm run preview
```

## 快速验证

后端健康检查：

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

## 测试与验收

运行后端测试：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

执行前端构建检查：

```powershell
Set-Location frontend
npm run build
```

建议把以上两项作为每次提交前的基础检查；如需做正式版本验收，请进一步对照 [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md) 逐项核对。

## 环境变量覆盖

可通过 `AEGISOPS_` 前缀环境变量覆盖后端配置，例如：

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DSN = "data/dev.db"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

## 文档导航

建议优先阅读以下中文文档：

- [AegisOps 一期 MVP 开发路线](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps 一期开发手册（真实经验与排查方法）](./docs/AegisOps一期开发手册（真实经验与排查方法）.md)
- [AegisOps 阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps 正式 Release 验收清单](./docs/AegisOps正式Release验收清单.md)
- [AegisOps 二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps 二期专项规划：通知告警与健康检查闭环](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps 二期专项规划：导出、备份与故障排查包](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps 二期专项规划：权限细粒度、密钥管理与任务调度](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)

## 当前阶段说明

当前项目已经不是“只有脚手架”的状态，后端接口、前端业务页面、Demo 数据与基础联调链路都已落地，本地开发、功能演示与接口联调可以直接开展。

但它目前更适合定义为：

- 一期主干能力基本完成
- 一期收口、生产化验收与若干专项增强仍在继续
- 是否进入正式 Release，建议以 [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md) 为准
