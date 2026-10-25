# AegisOps OIDC SSO Integration Points Research

> 2026-10-26 · Source: code research subagent (Explore agent)
> Purpose: locate integration points for implementing OIDC SSO login, §1 of the enterprise enablement design
> Chinese version: [AegisOps OIDC SSO 集成点调研报告](./AegisOps%20OIDC%20SSO%20集成点调研报告.md)
> Related design: [AegisOps企业级增强设计：SSO登录、变更审批与资源范围权限](./AegisOps企业级增强设计：SSO登录、变更审批与资源范围权限.md) §1

All research complete. Here is the full report with file:line references.

---

## 1. internal/auth/service.go

**Config struct** (`internal/auth/service.go:15-23`):

```go
type Config struct {
    JWTSecret          string
    AccessTokenTTL     time.Duration
    RefreshTokenTTL    time.Duration
    InitialAdminUser   string
    InitialAdminPass   string
    InitialAdminEmail  string
    InitialDisplayName string
}
```

Defaults applied in `NewService` (51-62): AccessTokenTTL=2h, RefreshTokenTTL=7d, JWTSecret fallback `"aegisops-dev-secret"`.

**Service struct** (25-29): `{ db *gorm.DB; config Config; now func() time.Time }` — injectable fake clock.

**Claims** (31-36): `UserID uint` (`userId`), `Username string`, `TokenTyp string` (`tokenType`), embedded `jwt.RegisteredClaims` (Subject=username, IssuedAt, ExpiresAt). HS256 signing in `signToken` (370-382).

**Login** (243-269): signature `func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error)`. Flow: `Preload("Roles.Permissions")` lookup by username → `ErrInvalidCredentials` on not-found → status check `ErrUserDisabled` → `bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), ...)` at **line 255** → sets `user.LastLoginAt = &now` and updates column `last_login_at` directly (260-263) → `issueTokenPair(user)` → returns `LoginResult{User: user, Tokens: *tokens}` (46-49).

**Token issuance**: `issueTokenPair` (348-368) signs two tokens via `signToken(user, tokenType, issuedAt, expiresAt)`; the "access"/"refresh" distinction is purely the `TokenTyp` claim. `ParseToken(rawToken, expectedType)` (298-313): parses with HMAC-only check and then `if expectedType != "" && claims.TokenTyp != expectedType { return nil, appErrors.ErrUnauthorized }`. Passing `""` as expectedType accepts either type — useful if OIDC callback needs to parse arbitrary tokens.

**InitAdmin** (64-127): counts users; if 0, bcrypt-generates hash at **line 85** (`bcrypt.GenerateFromPassword` with `bcrypt.DefaultCost`) and sets `PasswordHash: string(hash)` at **line 92**, plus `MustChangePassword: true` (98). Creates admin + "Administrator"/"admin" role + all role_permissions + user_role in one transaction. If users exist it only calls `EnsurePermissions`.

**bcrypt hash set points**: only `service.go:92` (InitAdmin) and `handler/user.go:78` (user Create via `HashPassword`, service.go:315-321, min 8 chars).

**PasswordHash non-empty assumption**: login-time only. `Login` (255) and `ChangePassword` (330) call `bcrypt.CompareHashAndPassword` on the hash — an empty hash returns a non-ErrMismatchedHashAndPassword error ("hashedSecret too short"), which `Login` maps to `ErrInvalidCredentials`. The JWT middleware path (`GetCurrentUser`, 283-296) never touches PasswordHash, so an OIDC-provisioned user with empty PasswordHash can authenticate via tokens/refresh fine — but must not go through password `Login`/`ChangePassword` unless you pre-set a random hash or branch on empty hash. Note: `PasswordHash` has `gorm:"not null"` (model/auth.go:15) — SQLite won't enforce it, but Postgres will, so store `""`? No — it's `not null`, empty string is fine (not NULL). Also `userCreateRequest.Password` is `binding:"required"` (handler/user.go:26) if you provision users through that path.

---

## 2. internal/auth/middleware.go + guard.go

**Middleware** (`middleware.go:14-44`): reads `Authorization: Bearer` header; WebSocket upgrades may pass `?token=` query (20-22). `service.ParseToken(rawToken, "access")` → `service.GetCurrentUser(claims.UserID)` (DB load with Roles.Permissions preload, status check) → `c.Set(CurrentUserKey, user)` (41). `CurrentUserKey = "currentUser"` (12); the stored value is `*model.User`.

**RequirePasswordChangeCleared** (62-78): checked after auth middleware. Allowlist is a package-level map keyed by `c.FullPath()` (route pattern, not URL) at **52-57**:

```go
var passwordChangeAllowlist = map[string]bool{
    "/api/auth/me": true,
    "/api/auth/change-password": true,
    "/api/auth/logout": true,
    "/api/auth/refresh": true,
}
```

A new OIDC route (e.g. `/api/auth/oidc/callback`) would need an entry here if `MustChangePassword` users should reach it. Blocked users get 403 `MUST_CHANGE_PASSWORD`.

**LoginGuard** (`guard.go:11-18`): in-memory, keyed by an opaque `key string` — the key format is entirely caller-defined (`failures map[string][]time.Time`). API:

- `NewLoginGuard() *LoginGuard` (20) — 5 attempts / 15min window / 15min lockout, injectable `now`
- `LockedRemaining(key string) time.Duration` (31) — 0 means unlocked
- `RecordFailure(key string)` (45)
- `Reset(key string)` (54)

Yes, it works with an IP-only key (e.g. `"oidc@" + c.ClientIP()`) — nothing parses the key.

---

## 3. internal/handler/auth.go + routes.go

**AuthHandler** (17-25): `{auth *auth.Service; audit *audit.Service; guard *auth.LoginGuard}`; guard created inside `NewAuthHandler` (24); `SetLoginGuard` (28-32) exists for tests.

**Login handler** (43-87): DTO `loginRequest{Username, Password}` with `binding:"required"` (34-37). Guard key: `guardKey := req.Username + "@" + c.ClientIP()` (**line 50**). Locked → 429 `TOO_MANY_ATTEMPTS` + audit entry "login blocked by rate limit". On `ErrInvalidCredentials` → `RecordFailure`. Success → `Reset`, audit, `response.OK(c, result)`. Response shape is `LoginResult` (service.go:46-49): `{user: {..., mustChangePassword, lastLoginAt, roles}, tokens: {accessToken, refreshToken, accessTokenExpiresAt, refreshTokenExpiresAt, tokenType:"Bearer"}}` — there is no separate top-level `mustChangePassword` field; it rides inside `user`.

**ChangePassword** (94-130): DTO `changePasswordRequest{OldPassword, NewPassword}` (89-92); user from `c.Get(auth.CurrentUserKey)`; success returns `gin.H{"changed": true}`; audit action `auth.change_password`.

**Other handlers**: `Logout` (132, audit `auth.logout`, 204), `Refresh` (141, takes `{refreshToken}`), `Me` (155, returns current user).

**Routes** (`handler/routes.go:18-26`): **outside JWT group**: `POST /auth/login`, `POST /auth/refresh`. Inside `authMiddleware` group (but before/without the change-password gate — the gate is only on the big `protected` group in router.go:143): `/auth/logout`, `/auth/me`, `/auth/change-password`. Router wiring: `internal/server/router.go:140-143` — `RegisterAuthRoutes(api, ...)` then `protected := api.Group("", authMiddleware, auth.RequirePasswordChangeCleared())` for all business routes. An OIDC login/callback route should be registered in `RegisterAuthRoutes` outside the protected group.

---

## 4. internal/model/auth.go

Full structs at `internal/model/auth.go`:

- **User** (12-24): `BaseModel` embedded; `Username` (`json:"username"`, size:64, uniqueIndex, not null), `PasswordHash` (**`json:"-"`**, size:255, not null), `DisplayName` (`json:"displayName"`), `Email` (`json:"email"`, index), `Status UserStatus` (`json:"status"`, default:active), `IsAdmin bool` (`json:"isAdmin"`), `MustChangePassword bool` (`json:"mustChangePassword"`), `LastLoginAt *time.Time` (`json:"lastLoginAt"`), `Roles []Role` (`json:"roles,omitempty"`, many2many:user_roles).
- **Role** (26-32): Name (uniqueIndex), Code (uniqueIndex), Description, `Permissions []Permission` many2many:role_permissions.
- **Permission** (34-41): Name, Code (uniqueIndex), Resource (index), Action (index), Description.
- **UserRole** (43-47): `UserID uint` + `RoleID uint` composite PK, `CreatedAt`.
- **RolePermission** (49-53): RoleID + PermissionID composite PK, CreatedAt.
- `UserStatus` consts (7-10): `"active"` / `"disabled"`.

`BaseModel` (`internal/model/base.go`): `ID uint` (`json:"id"`), `CreatedAt`/`UpdatedAt` (`json:"createdAt"/"updatedAt"`), `DeletedAt gorm.DeletedAt` (`json:"-"`). Preserve all tags exactly; any new column on User requires a migration.

---

## 5. internal/db migrations

- `internal/db/migrations.go:12-15` — ordered list, append-only:

```go
var Migrations = []*Migration{
    migrationBaselineV1_6_0,
    migrationDispatchClaimIndex,
}
```

- `Migration` struct (`migrate.go:25-30`): `{ID, Description string; Up, Down func(*gorm.DB) error}`.
- Pattern for 0003 (mirror `migration_0002_dispatch_claim_index.go:12-23`): declare `var migrationXxx = &Migration{ID: "0003_oidc_sso", Description: ..., Up: func(tx *gorm.DB) error {...}, Down: ...}` in a new file `migration_0003_oidc_sso.go`, then append to the list. Rules from the migrations.go header (lines 1-8): model changes must ship a matching numbered migration; Up must work on both SQLite and PostgreSQL (branch on dialect inside the migration if needed); only 0001 may AutoMigrate (`migration_0001_baseline.go:18` uses `tx.AutoMigrate(model.AllModels()...)`).

---

## 6. Rate limiting

LoginGuard is invoked in the **handler**, not middleware — `internal/handler/auth.go:50-78` (keyed `username@ClientIP`, checked before bcrypt, `RecordFailure` only on `ErrInvalidCredentials`, `Reset` on success). Guard instance lives on `AuthHandler` (created at handler/auth.go:24), injectable via `SetLoginGuard`. For OIDC you'd construct/reuse a guard call in the new OIDC handler with your own key format.

---

## 7. Config

`SecurityConfig` (41-46): `{JWTSecret, AccessTokenTTL (time.Duration), RefreshTokenTTL, SecretKey string}` with `mapstructure:"jwt_secret"` etc. Viper pattern (75-98): `v.SetDefault("security.access_token_ttl", "2h")` etc.; env override via `AEGISOPS_` prefix with `.`→`_` replacer. To add a nested OIDC section: (a) add field `OIDC OIDCConfig \`mapstructure:"oidc"\`` to `Config` (10-18), (b) define `type OIDCConfig struct{...}` with mapstructure tags, (c) add `v.SetDefault("oidc.xxx", ...)` in `newViper`. Sample YAML at `configs/config.yaml` (security section lines ~24-28). Note `time.Duration` fields parse from strings like `"2h"`.

---

## 8. Tests

**Service-with-sqlite pattern** (`internal/auth/service_test.go:14-32`): `newAuthService(t)` → `db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "aegisops.db")})` → `db.Migrate(database)` → `NewService(database, Config{})` (zero config = dev defaults) → returns cleanup closure. Tests use `t.Parallel()`, plain `testing` (no testify). The `now` field is unexported, so in-package tests can inject a fake clock (as `guard_test.go:8-12` does for LoginGuard via `guard.now = now`).

**httptest.NewServer pattern** — used in 6 packages: `internal/registry/service_test.go:24`, `internal/healthcheck/service_test.go:24,64`, `internal/scheduler/service_test.go:157`, `internal/service/service_test.go:215`, `internal/metrics/registry_test.go:56`. Canonical example (registry/service_test.go:23-32):

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ... w.WriteHeader(http.StatusOK) }))
defer server.Close()
```

then point the service at `server.URL`. This is the established pattern for mocking external HTTP (an OIDC provider's discovery/tokeninfo endpoints). `internal/server/router_test.go` also exists for full-router integration tests.

---

## Key takeaways for the OIDC feature

1. Add login route outside JWT group in `RegisterAuthRoutes` (routes.go:18-21); add any OIDC paths used post-login to `passwordChangeAllowlist` (middleware.go:52) if MustChangePassword users must reach them.
2. Provisioning OIDC users: `PasswordHash` is `not null` but `bcrypt.CompareHashAndPassword("")` fails — set a random unguessable hash, or branch in `Login` (empty-hash users are SSO-only). `userCreateRequest.Password` is `binding:"required"` if you reuse that path.
3. Reuse `issueTokenPair`/`signToken` for OIDC sessions — they take a `model.User`, so no changes needed; claims carry only userID/username/tokenType.
4. Model/migration discipline: any new User columns (e.g. `oidc_subject`) need `User` struct field + a `0003_*` migration handling SQLite+Postgres.
5. Config: follow the `mapstructure` + `SetDefault("oidc....")` viper pattern; wire through `internal/server/router.go:65-72` where `auth.Config` is constructed.
