# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps is a lightweight DevOps control plane MVP. The current repository includes:

- a Go + Gin + SQLite + GORM + zap backend
- a React + TypeScript + Vite frontend console
- phase-one review notes, phase-two roadmap, and supporting Chinese documentation

GitHub should display the Chinese `README.md` by default. This file is the English counterpart.

## Project Overview

The current project strategy is to finish the phase-one MVP security foundation and basic operations loop first, then move into phase two with a complete service delivery loop.

Main capabilities already present in the repository:

- authentication and default administrator bootstrap
- user, role, and permission base models
- secret storage with masked responses
- audit log foundations
- host CRUD and SSH connectivity test
- task, task step, and task log models
- Docker node CRUD and basic container operations
- real frontend routes and basic live API integration

## Directory Layout

```text
cmd/         backend entrypoint
configs/     configuration files
docs/        Chinese project documents
frontend/    frontend console
internal/    backend business implementation
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

Change these values before using the service outside local development.

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

In the current development setup, the frontend uses the real backend by default and proxies `/api` to `http://127.0.0.1:8080`.

## Smoke Checks

Backend health check:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
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

## Tests

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
- [AegisOps Stage Review (Phase-One Completion and Phase-Two Recommendations)](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps Phase-Two Frontend and Backend Roadmap](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps Current Integration Blockers](./docs/AegisOps当前联调阻塞清单.md)

## Current Stage

The project is currently best described as:

- phase-one core implementation mostly in place
- phase-one closure still incomplete
- phase-two scope is clear, but phase-one closure should be finished before fully expanding into phase two
