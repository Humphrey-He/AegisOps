# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

`AegisOps` is a DevOps platform focused on a lightweight operations control plane, application delivery, and stability management.

The latest formal release in this repository is `v1.5.12`. At this stage, the project is no longer a scaffold. It already provides a runnable and stable baseline with real backend APIs, a usable frontend console, local demo data, and clear phase-two planning.

## Positioning

AegisOps is a strong fit for:

- a personal operations platform
- a small-team internal operations console
- a backend/platform engineering interview project
- a product prototype for a DevOps control plane

It currently fits best as a single-node or lightweight platform baseline rather than a large-scale multi-tenant SaaS product.

## Project Assessment

The project has several clear strengths:

- clear domain focus around hosts, secrets, Docker, registries, service delivery, Nginx, tasks, alerts, notifications, and audits
- strong backend completeness beyond CRUD, including RBAC, audit trails, release flows, health checks, export/backup, and scheduler APIs
- a frontend that already acts as a real console rather than a placeholder shell
- rich documentation covering product thinking, development experience, review checkpoints, phase-two planning, and release acceptance
- easy local adoption with PostgreSQL as the primary runtime database, while still keeping SQLite as an optional lightweight mode

Current boundaries should also be stated honestly:

- the default deployment model is still oriented toward local or single-node use
- export and backup are currently stronger on the backend/API side than on unified frontend workflows
- fine-grained permission evolution, secret lifecycle management, advanced scheduling, and real external notification integrations still belong to the next stage
- more long-running verification against real hosts, Docker, Nginx, and notification targets is still worthwhile for heavier production use

In short: this is a solid `v1.5.12` backend-driven DevOps control-plane project with good product direction and a credible extension path.

## What v1.5.12 Includes

### Backend capabilities

- identity and access
  - admin bootstrap
  - authentication
  - users and roles
  - RBAC
  - audit logs
- assets and secrets
  - host management
  - SSH connectivity tests
  - encrypted secret storage with masked responses
- operations execution
  - task center
  - task steps and logs
  - web terminal
- delivery workflows
  - Docker node management
  - image registry management
  - service definitions
  - release, upgrade, and rollback
  - post-release health probing and rollback suggestions
- stability management
  - notification channels
  - alert rules
  - alert events
  - host and service health checks
  - Nginx node and config publish/rollback flows
- platform support
  - export APIs
  - backup APIs
  - scheduler APIs
  - demo data seeding
  - PostgreSQL-first support with SQLite as an optional lightweight mode

### Frontend pages already implemented

- dashboard
- asset management
  - hosts
  - secrets
- runtime resources
  - Docker nodes
  - Nginx nodes
- application delivery
  - registries
  - services
- task list and task detail
- alert events
- notification channels
- alert rules
- audit log
- system management
  - users
  - roles
  - scheduled jobs
- login
- admin setup
- web terminal

Notes:

- the frontend already runs against the real backend by default in development
- export and backup are currently more complete as backend/API capabilities than as dedicated frontend management pages

## Directory Layout

```text
cmd/         backend entrypoint
configs/     configuration files
data/        local runtime artifacts and optional SQLite files
deploy/      deployment and smoke-test resources
docs/        Chinese project documents and plans
frontend/    frontend console
internal/    backend business implementation
logs/        local runtime logs
pkg/         shared packages
scripts/     local helper scripts
```

## Backend Quick Start

Requirements:

- Go 1.24+
- Windows / Linux / macOS

Run the backend:

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

Default configuration file:

- [configs/config.yaml](./configs/config.yaml)

Default listen address:

- `:8080`

Default database:

- PostgreSQL: `postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable`
- Optional SQLite example config: `configs/config.sqlite.example.yaml`

Default administrator:

```text
username: admin
password: admin123456
```

These defaults are only for local development and demos. Replace the administrator password, JWT secret, secret key, and PostgreSQL password before using the service beyond local use.

## Frontend Quick Start

Requirements:

- Node.js 18+
- npm 9+

Run the frontend:

```powershell
Set-Location frontend
npm install
npm run dev
```

Default URL:

- [http://localhost:4173](http://localhost:4173)

In development, the frontend uses the real backend by default and proxies `/api` to `http://127.0.0.1:8080`.

Production-style preview:

```powershell
Set-Location frontend
npm run build
npm run preview
```

## Smoke Checks

Health endpoints:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
Invoke-RestMethod http://127.0.0.1:8080/readyz
```

Login example:

```powershell
$body = @{ username = "admin"; password = "admin123456" } | ConvertTo-Json
$login = Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/api/auth/login `
  -ContentType "application/json" `
  -Body $body
$login.data.tokens.accessToken
```

Get the current user:

```powershell
$token = $login.data.tokens.accessToken
Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/api/auth/me `
  -Headers @{ Authorization = "Bearer $token" }
```

## Tests and Release Baseline

Run backend tests:

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

Run a frontend build check:

```powershell
Set-Location frontend
npm run build
```

Both checks should be treated as the minimum pre-commit validation.

For formal release acceptance, see:

- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## Environment Overrides

Backend settings can be overridden with `AEGISOPS_` environment variables, for example:

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DRIVER = "postgres"
$env:AEGISOPS_DATABASE_DSN = "postgres://aegisops:replace-me@127.0.0.1:5432/aegisops?sslmode=disable"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

Local PostgreSQL quick start:

```powershell
docker compose -f deploy/postgres/docker-compose.yaml up -d
```

## Documentation

Recommended core documents:

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

## Current Stage

The repository can now be described as:

- a completed `v1.5.12` formal release baseline
- ready for local execution, demos, API integration, and phase-two development
- moving from `v1.5.x` stabilization into production hardening and focused phase-two enhancements
