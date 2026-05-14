# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps 是一个面向轻量化运维控制面的 DevOps 平台 MVP。当前仓库包含：

- Go + Gin + SQLite + GORM + zap 的后端服务
- React + TypeScript + Vite 的前端控制台
- 一期阶段审查、二期路线与配套中文文档

GitHub 默认优先展示本中文文档；日文与英文说明请分别查看对应语言版本。

## 项目概览

当前项目目标是先完成一期 MVP 的安全底座与基础运维闭环，再进入二期“服务发布闭环”建设。

当前已具备的主干能力包括：

- 认证与默认管理员初始化
- 用户、角色、权限基础模型
- Secret 凭证存储与脱敏返回
- 审计日志基础能力
- 主机 CRUD 与 SSH 连通性测试
- 任务、任务步骤、任务日志模型
- Docker 节点 CRUD 与基础容器操作
- 前端真实业务路由与基础联调能力

## 目录说明

```text
cmd/         后端启动入口
configs/     配置文件
docs/        中文项目文档与路线文档
frontend/    前端控制台
internal/    后端业务实现
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

仅可用于本地开发环境，外部使用前请务必修改。

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

## 快速验证

后端健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
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

## 测试

运行后端测试：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

前端构建检查：

```powershell
Set-Location frontend
npm run build
```

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
- [AegisOps 阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps 二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps 当前联调阻塞清单](./docs/AegisOps当前联调阻塞清单.md)

## 当前阶段说明

当前项目状态更接近：

- 一期主干已完成
- 一期收口未完成
- 二期路线已明确，但建议先完成一期收口再全面进入二期

因此，当前仓库既不是纯概念项目，也还不能视为完整交付版。
