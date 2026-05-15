# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps is a DevOps platform focused on a lightweight operations control plane, delivery workflows, and stability management. The repository currently includes:

- a Go + Gin + SQLite + GORM + zap backend API and task execution core
- a React + TypeScript + Vite frontend console
- Chinese-first product, development, review, phase-two, and release documents

GitHub should display the Chinese `README.md` by default. This file is the English counterpart.

## Project Overview

Based on the current implementation, AegisOps already provides a runnable local operations platform baseline:

- identity and access: admin bootstrap, authentication, users, roles, RBAC, and audit logs
- assets and secrets: host management, SSH connectivity tests, encrypted secret storage, and masked responses
- operations execution: tasks, task steps, task logs, and a web terminal
- container and delivery workflows: Docker nodes, image registries, service definitions, release, upgrade, and rollback
- stability management: notification channels, alert rules, alert events, and host/service health checks
- release loop features: post-release probing, rollback suggestions, and Nginx node/config rollback support
- platform support: export, backup, scheduler APIs, and demo data seeding in development environments

The frontend console already includes major pages for dashboard, hosts, secrets, Docker, Nginx, registries, services, tasks, audits, alerts, notifications, users, roles, terminal, login, and admin setup.

## Directory Layout

```text
cmd/         backend entrypoint
configs/     configuration files
data/        SQLite data and local runtime artifacts
docs/        Chinese project documents and plans
frontend/    frontend console
internal/    backend business implementation
logs/        local runtime logs
pkg/         shared packages
```

## Backend Quick Start

Requirements:

- Go 1.24+
- Windows, Linux, or macOS

Run the backend:

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

Default configuration file:

- [configs/config.yaml](./configs/config.yaml)

Default listen address:

- `:8080`

Default SQLite database:

- `data/aegisops.db`

Default administrator:

```text
username: admin
password: admin123456
```

Change the default administrator password, JWT secret, and secret key before using the service beyond local development.

When `app.env` is `dev`, `development`, or `test`, the backend seeds demo registry, Docker node, service, and instance data for local walkthroughs of release, rollback, and health-check flows.

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

In development mode, the frontend uses the real backend by default and proxies `/api` to `http://127.0.0.1:8080`.

For a production-style preview:

```powershell
Set-Location frontend
npm run build
npm run preview
```

## Smoke Checks

Backend health checks:

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

Current user:

```powershell
$token = $login.data.tokens.accessToken
Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/api/auth/me `
  -Headers @{ Authorization = "Bearer $token" }
```

## Tests and Acceptance

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

Treat both checks as the minimum validation before each commit. For formal release acceptance, use [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md).

## Environment Overrides

Backend settings can be overridden with `AEGISOPS_` environment variables:

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DSN = "data/dev.db"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

## Documentation

Recommended core documents:

- [AegisOps Phase-One MVP Roadmap](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps Phase-One Development Handbook](./docs/AegisOps一期开发手册（真实经验与排查方法）.md)
- [AegisOps Stage Review (Phase-One Completion and Phase-Two Recommendations)](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps Formal Release Acceptance Checklist](./docs/AegisOps正式Release验收清单.md)
- [AegisOps Phase-Two Frontend and Backend Roadmap](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps Phase-Two Plan: Alerts and Health-Check Loop](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps Phase-Two Plan: Export, Backup, and Troubleshooting Pack](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps Phase-Two Plan: Fine-Grained Permissions, Secret Management, and Scheduling](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)

## Current Stage

This repository is no longer just a scaffold. Backend APIs, frontend business pages, demo data, and the basic integration loop are already in place, so local development, demos, and API integration can proceed directly.

Its current status is better described as:

- phase-one core capabilities mostly complete
- phase-one closure, production readiness checks, and several focused enhancements are still ongoing
- formal release readiness should be judged against [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)
