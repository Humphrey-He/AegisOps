# AegisOps

AegisOps is a lightweight DevOps control plane MVP. The first backend version uses Go, Gin, SQLite, GORM, and zap.

## Backend Quick Start

Requirements:

- Go 1.24+
- Windows, Linux, or macOS

Run the API server:

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

Default configuration lives in [configs/config.yaml](</E:/awesomeProject/AegisOps/configs/config.yaml>).

The server listens on `:8080` by default and creates the SQLite database at `data/aegisops.db`.

Default administrator:

```text
username: admin
password: admin123456
```

Change these values before using the service outside local development.

## Smoke Checks

Health check:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

Login:

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

Run all backend tests:

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

The server smoke test covers:

- `/healthz`
- automatic administrator initialization
- `/api/auth/login`
- authenticated `/api/auth/me`

## Environment Overrides

Configuration values can be overridden with `AEGISOPS_` environment variables:

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DSN = "data/dev.db"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

## MVP Backend Modules

Implemented in the first backend pass:

- Auth and administrator initialization
- User, role, permission, and basic RBAC models
- Audit log storage
- Secret storage with AES-GCM encryption
- Host CRUD and SSH connection test
- Task, task step, and task log storage
- Docker node CRUD and basic container operations

