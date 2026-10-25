# AegisOps OIDC SSO 集成点调研报告

> 2026-10-26 · 来源：代码调研子代理（Explore agent）
> 调研目的：为《AegisOps企业级增强设计》§1 OIDC SSO 登录的实施定位集成点
> 英文原版：[AegisOps OIDC SSO Integration Points Research](./AegisOps%20OIDC%20SSO%20Integration%20Points%20Research.md)
> 关联设计：[AegisOps企业级增强设计：SSO登录、变更审批与资源范围权限](./AegisOps企业级增强设计：SSO登录、变更审批与资源范围权限.md) §1

调研完成。以下是完整报告，含文件行号引用。

---

## 1. internal/auth/service.go（认证服务）

**Config 结构体**（`internal/auth/service.go:15-23`）：

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

默认值在 `NewService`（51-62 行）设置：AccessTokenTTL=2h、RefreshTokenTTL=7d、JWTSecret 兜底值 `"aegisops-dev-secret"`。

**Service 结构体**（25-29 行）：`{ db *gorm.DB; config Config; now func() time.Time }`——`now` 可注入假时钟。

**JWT Claims**（31-36 行）：`UserID uint`（JSON 键 `userId`）、`Username string`、`TokenTyp string`（JSON 键 `tokenType`），内嵌 `jwt.RegisteredClaims`（Subject=username、IssuedAt、ExpiresAt）。HS256 签名在 `signToken`（370-382 行）。

**Login**（243-269 行）：签名 `func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error)`。流程：`Preload("Roles.Permissions")` 按用户名查询 → 查不到返回 `ErrInvalidCredentials` → 状态检查返回 `ErrUserDisabled` → **255 行**执行 `bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), ...)` → 设置 `user.LastLoginAt = &now` 并直接更新 `last_login_at` 列（260-263 行）→ `issueTokenPair(user)` → 返回 `LoginResult{User: user, Tokens: *tokens}`（46-49 行）。

**Token 签发**：`issueTokenPair`（348-368 行）通过 `signToken(user, tokenType, issuedAt, expiresAt)` 签出两个 token；"access"/"refresh" 的区分**纯粹是 `TokenTyp` claim**。`ParseToken(rawToken, expectedType)`（298-313 行）：先 HMAC 校验，再校验 `if expectedType != "" && claims.TokenTyp != expectedType { return nil, appErrors.ErrUnauthorized }`——传 `""` 表示接受任意类型，OIDC 回调如需解析任意 token 时有用。

**InitAdmin**（64-127 行）：统计用户数；为 0 时在 **85 行** `bcrypt.GenerateFromPassword`（`bcrypt.DefaultCost`）生成哈希，**92 行**写入 `PasswordHash: string(hash)`，**98 行**设 `MustChangePassword: true`。单事务内创建 admin 用户 + "Administrator"/"admin" 角色 + 全部 role_permissions + user_role。已有用户时只调 `EnsurePermissions`。

**bcrypt 哈希的写入点**：仅 `service.go:92`（InitAdmin）和 `handler/user.go:78`（用户创建走 `HashPassword`，service.go:315-321，最少 8 位）。

**PasswordHash 非空的假设只在登录路径**：`Login`（255 行）和 `ChangePassword`（330 行）会调 `bcrypt.CompareHashAndPassword`——空哈希返回"hashedSecret too short"错误，`Login` 将其映射为 `ErrInvalidCredentials`。JWT 中间件路径（`GetCurrentUser`，283-296 行）**完全不碰 PasswordHash**，所以 PasswordHash 为空的 OIDC 账号经 token/refresh 认证毫无问题——只是不能走密码 `Login`/`ChangePassword`，除非预设随机哈希或按空哈希分支。注意 `PasswordHash` 带 `gorm:"not null"`（model/auth.go:15）——SQLite 不强制，**PostgreSQL 会**；存空字符串合法（不是 NULL）。另外 `userCreateRequest.Password` 带 `binding:"required"`（handler/user.go:26），若复用该路径建号必须带密码。

---

## 2. internal/auth/middleware.go + guard.go

**认证中间件**（`middleware.go:14-44`）：读 `Authorization: Bearer` 头；WebSocket 升级请求可传 `?token=` 查询参数（20-22 行）。`service.ParseToken(rawToken, "access")` → `service.GetCurrentUser(claims.UserID)`（查库并 Preload Roles.Permissions、检查状态）→ `c.Set(CurrentUserKey, user)`（41 行）。`CurrentUserKey = "currentUser"`（12 行），存的值是 `*model.User`。

**RequirePasswordChangeCleared**（62-78 行）：在认证中间件之后执行。豁免名单是包级 map，键为 `c.FullPath()`（**路由模式**而非实际 URL），**52-57 行**：

```go
var passwordChangeAllowlist = map[string]bool{
    "/api/auth/me": true,
    "/api/auth/change-password": true,
    "/api/auth/logout": true,
    "/api/auth/refresh": true,
}
```

若 `MustChangePassword` 用户需要触达新的 OIDC 路由（如 `/api/auth/oidc/callback`），需在此加条目。被拦用户得到 403 `MUST_CHANGE_PASSWORD`。

**LoginGuard**（`guard.go:11-18`）：进程内存实现，键为不透明 `key string`——键格式完全由调用方定义（`failures map[string][]time.Time`）。API：

- `NewLoginGuard() *LoginGuard`（20 行）——5 次尝试 / 15 分钟窗口 / 15 分钟锁定，`now` 可注入
- `LockedRemaining(key string) time.Duration`（31 行）——返回 0 表示未锁定
- `RecordFailure(key string)`（45 行）
- `Reset(key string)`（54 行）

**可以用纯 IP 键**（如 `"oidc@" + c.ClientIP()`）——没有任何地方解析键的内容。

---

## 3. internal/handler/auth.go + routes.go

**AuthHandler**（17-25 行）：`{auth *auth.Service; audit *audit.Service; guard *auth.LoginGuard}`；guard 在 `NewAuthHandler`（24 行）内部创建；`SetLoginGuard`（28-32 行）供测试注入。

**Login handler**（43-87 行）：DTO `loginRequest{Username, Password}` 带 `binding:"required"`（34-37 行）。Guard 键：`guardKey := req.Username + "@" + c.ClientIP()`（**50 行**）。锁定 → 429 `TOO_MANY_ATTEMPTS` + 审计"login blocked by rate limit"。`ErrInvalidCredentials` → `RecordFailure`。成功 → `Reset`、审计、`response.OK(c, result)`。响应结构是 `LoginResult`（service.go:46-49）：`{user: {..., mustChangePassword, lastLoginAt, roles}, tokens: {accessToken, refreshToken, accessTokenExpiresAt, refreshTokenExpiresAt, tokenType:"Bearer"}}`——**没有独立的顶层 mustChangePassword 字段**，它在 user 对象内部。

**ChangePassword**（94-130 行）：DTO `changePasswordRequest{OldPassword, NewPassword}`（89-92 行）；user 取自 `c.Get(auth.CurrentUserKey)`；成功返回 `gin.H{"changed": true}`；审计动作 `auth.change_password`。

**其他 handler**：`Logout`（132 行，审计 `auth.logout`，204）、`Refresh`（141 行，入参 `{refreshToken}`）、`Me`（155 行，返回当前用户）。

**路由**（`handler/routes.go:18-26`）：**JWT 组之外**：`POST /auth/login`、`POST /auth/refresh`。`authMiddleware` 组内（但在改密门禁之前——门禁只加在 router.go:143 的大 `protected` 组上）：`/auth/logout`、`/auth/me`、`/auth/change-password`。路由装配在 `internal/server/router.go:140-143`：先 `RegisterAuthRoutes(api, ...)`，然后 `protected := api.Group("", authMiddleware, auth.RequirePasswordChangeCleared())` 挂全部业务路由。**OIDC 的登录/回调路由应注册在 `RegisterAuthRoutes` 里、protected 组之外**。

---

## 4. internal/model/auth.go（模型）

完整结构体在 `internal/model/auth.go`：

- **User**（12-24 行）：`BaseModel` 内嵌；`Username`（`json:"username"`，size:64，uniqueIndex，not null）、`PasswordHash`（**`json:"-"`**，size:255，not null）、`DisplayName`（`json:"displayName"`）、`Email`（`json:"email"`，index）、`Status UserStatus`（`json:"status"`，default:active）、`IsAdmin bool`（`json:"isAdmin"`）、`MustChangePassword bool`（`json:"mustChangePassword"`）、`LastLoginAt *time.Time`（`json:"lastLoginAt"`）、`Roles []Role`（`json:"roles,omitempty"`，many2many:user_roles）。
- **Role**（26-32 行）：Name（uniqueIndex）、Code（uniqueIndex）、Description、`Permissions []Permission` many2many:role_permissions。
- **Permission**（34-41 行）：Name、Code（uniqueIndex）、Resource（index）、Action（index）、Description。
- **UserRole**（43-47 行）：`UserID uint` + `RoleID uint` 联合主键、`CreatedAt`。
- **RolePermission**（49-53 行）：RoleID + PermissionID 联合主键、CreatedAt。
- `UserStatus` 常量（7-10 行）：`"active"` / `"disabled"`。

`BaseModel`（`internal/model/base.go`）：`ID uint`（`json:"id"`）、`CreatedAt`/`UpdatedAt`（`json:"createdAt"/"updatedAt"`）、`DeletedAt gorm.DeletedAt`（`json:"-"`）。**所有 json tag 必须原样保留**；User 加任何新列都需要配套迁移。

---

## 5. internal/db 迁移

- `internal/db/migrations.go:12-15`——有序列表，**只允许追加**：

```go
var Migrations = []*Migration{
    migrationBaselineV1_6_0,
    migrationDispatchClaimIndex,
}
```

- `Migration` 结构体（`migrate.go:25-30`）：`{ID, Description string; Up, Down func(*gorm.DB) error}`。
- 0003 的写法（镜像 `migration_0002_dispatch_claim_index.go:12-23`）：在新文件 `migration_0003_oidc_sso.go` 里声明 `var migrationXxx = &Migration{ID: "0003_oidc_sso", ...}`，然后追加进列表。规则见 migrations.go 文件头（1-8 行）：**改 model 必须配套编号迁移；Up 必须同时兼容 SQLite 和 PostgreSQL**（方言差异在迁移内部分支）；**只有 0001 允许 AutoMigrate**。

---

## 6. 限流

LoginGuard 在 **handler 里调用，不在中间件**——`internal/handler/auth.go:50-78`（键 `username@ClientIP`，在 bcrypt 之前检查；仅 `ErrInvalidCredentials` 时 `RecordFailure`；成功 `Reset`）。Guard 实例挂在 `AuthHandler` 上（auth.go:24 创建），`SetLoginGuard` 供测试注入。OIDC 侧复用同一 guard、自定义键格式即可。

---

## 7. 配置

`SecurityConfig`（41-46 行）：`{JWTSecret, AccessTokenTTL (time.Duration), RefreshTokenTTL, SecretKey string}`，带 `mapstructure:"jwt_secret"` 等 tag。Viper 模式（75-98 行）：`v.SetDefault("security.access_token_ttl", "2h")` 等；env 覆盖经 `AEGISOPS_` 前缀 + `.`→`_` 替换器。新增嵌套 OIDC 配置段的做法：(a) 在 `Config`（10-18 行）加字段 `OIDC OIDCConfig \`mapstructure:"oidc"\``；(b) 定义带 mapstructure tag 的 `type OIDCConfig struct{...}`；(c) 在 `newViper` 加 `v.SetDefault("oidc.xxx", ...)`。注意 `time.Duration` 字段从 `"2h"` 这类字符串解析。YAML 示例在 `configs/config.yaml`（security 段约 24-28 行）。

---

## 8. 测试

**sqlite 服务测试模式**（`internal/auth/service_test.go:14-32`）：`newAuthService(t)` → `db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "aegisops.db")})` → `db.Migrate(database)` → `NewService(database, Config{})`（零值配置 = 开发默认值）→ 返回 cleanup 闭包。测试用 `t.Parallel()`、纯 `testing`（**无 testify**）。`now` 字段未导出，包内测试可注入假时钟（`guard_test.go:8-12` 对 LoginGuard 就是这么做的）。

**httptest.NewServer 模式**——全仓 6 个包在用：`internal/registry/service_test.go:24`、`internal/healthcheck/service_test.go:24,64`、`internal/scheduler/service_test.go:157`、`internal/service/service_test.go:215`、`internal/metrics/registry_test.go:56`。标准范例（registry/service_test.go:23-32）：

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ... w.WriteHeader(http.StatusOK) }))
defer server.Close()
```

然后把服务指向 `server.URL`。这就是 mock 外部 HTTP（OIDC Provider 的 discovery/token 端点）的既有模式。`internal/server/router_test.go` 提供全路由集成测试先例。

---

## 关键结论（对 OIDC 实施的直接指导）

1. **登录路由**：加在 `RegisterAuthRoutes` 里 JWT 组之外（routes.go:18-21）；若 `MustChangePassword` 用户需要触达 OIDC 相关路径，在 `passwordChangeAllowlist`（middleware.go:52）加条目
2. **Provisioning**：`PasswordHash` not null 但 `bcrypt("")` 会失败——预置随机不可猜哈希，或在 `Login` 里按空哈希分支（空哈希用户即 SSO-only 用户）。`userCreateRequest.Password` 带 `binding:"required"`
3. **会话签发复用**：OIDC 会话直接复用 `issueTokenPair`/`signToken`——入参是 `model.User`，零改动；claims 只带 userID/username/tokenType
4. **模型/迁移纪律**：User 新列（如 `oidc_subject`）= User 结构体字段 + `0003_*` 迁移，双驱动兼容
5. **配置**：`mapstructure` + `v.SetDefault("oidc....")` 的 viper 模式；在 `internal/server/router.go:65-72`（构造 `auth.Config` 处）接线
