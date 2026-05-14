# AegisOps 一期 MVP 开发路线

> 基于现有前端、后端功能清单整理。  
> 后端一期技术栈：Go + Gin + SQLite + GORM + zap，Redis 前期可接入但不作为强依赖。  
> 目标：先做出可本地部署、可登录、可审计、可管理资产、可执行核心运维动作的最小闭环版本。

## 1. 范围判断

现有功能清单覆盖面比较完整，已经接近 V1.0/V1.5 规划，包含认证权限、Secret、审计、任务中心、主机、WebSSH、Docker、Registry、服务发布、Nginx、Jenkins、系统设置等模块。

一期 MVP 不建议一次性铺满所有模块。AegisOps 的核心价值不是普通后台 CRUD，而是“安全可追踪的运维控制面”。因此第一期应该优先建立安全底座和一个可运行的运维闭环。

一期建议保留：

- 账号登录与管理员初始化
- 基础用户、角色、权限
- Secret 凭证加密存储
- 审计日志
- 任务中心基础能力
- 主机资产管理
- SSH 连接测试
- WebSSH 基础会话
- Docker 节点管理
- 容器列表、日志、启动、停止、重启
- Dashboard 基础统计

一期建议延后：

- Registry 镜像仓库管理
- 服务发布、升级、回滚
- Nginx 配置管理
- Jenkins 集成
- 备份恢复
- 指标监控和告警
- Agent 架构
- 多租户、复杂资源范围权限

## 2. 一期技术基线

### 2.1 后端技术栈

| 类型 | 选型 | 一期说明 |
|---|---|---|
| HTTP 框架 | Gin | 提供 REST API、中间件、路由分组 |
| 数据库 | SQLite | 降低部署门槛，适合单机 MVP |
| ORM | GORM | 快速建模、迁移、查询 |
| 日志 | zap | 结构化日志，统一 traceId/requestId |
| 配置 | Viper 或 envconfig | 一期可用 YAML + 环境变量覆盖 |
| 认证 | JWT 或服务端 Session | 推荐一期先 JWT access token + refresh token |
| 密码哈希 | bcrypt | 管理员、用户密码不可明文存储 |
| 凭证加密 | AES-GCM | 主密钥来自配置或环境变量 |
| WebSocket | gorilla/websocket 或 nhooyr/websocket | WebSSH 与任务日志推送 |
| SSH | golang.org/x/crypto/ssh | 主机连接测试和 WebSSH |
| Docker | Docker Go SDK | Docker 节点和容器操作 |
| Redis | 可选 | 一期可用于登录限流、任务日志 pub/sub，不能成为启动强依赖 |

### 2.2 Redis 接入策略

Redis 前期可以接入，但建议做成可选能力：

- 未配置 Redis 时，任务状态、任务日志、登录失败计数全部落 SQLite。
- 配置 Redis 后，可用于登录失败计数、短期 token 黑名单、任务日志实时推送、轻量分布式锁。
- 一期不要引入 Asynq 作为强依赖，避免 MVP 部署复杂度上升。
- 所有 Redis 数据必须有 SQLite 或内存降级路径，保证单机离线可运行。

### 2.3 后端工程建议结构

```text
cmd/aegisops/
  main.go
internal/
  config/
  server/
  middleware/
  logger/
  db/
  model/
  repository/
  service/
  handler/
  auth/
  rbac/
  secret/
  audit/
  task/
  host/
  terminal/
  docker/
pkg/
  response/
  errors/
  crypto/
  pagination/
configs/
  config.yaml
migrations/
docs/
```

## 3. 一期模块拆分

### 3.1 工程底座

后端交付：

- Gin 服务启动、优雅关闭
- zap 日志初始化
- SQLite 连接和 GORM 自动迁移
- 配置加载
- 统一 API 响应
- 统一错误码
- requestId/traceId 中间件
- CORS 中间件
- `/healthz`、`/readyz`

前端交付：

- React + TypeScript + Vite 工程
- 路由、布局、登录页框架
- API client 封装
- 统一错误提示
- 403/404 页面
- 基础 DataTable、StatusBadge、DangerConfirm、EmptyState

验收：

- 后端可本地启动并自动创建 SQLite 数据库。
- 前端可连接后端 health API。
- 所有 API 返回统一结构并包含 traceId。

### 3.2 认证与权限

后端交付：

- 初始化管理员
- 登录、退出、刷新 token、获取当前用户
- 用户 CRUD
- 角色 CRUD
- 基础权限点
- 用户绑定角色
- 权限中间件
- 登录失败审计

前端交付：

- 登录页
- 主框架和左侧菜单
- 用户菜单
- 用户管理页
- 角色管理页
- 权限不足提示

一期权限可以先使用简化 RBAC：

```text
User -> Role -> Permission
```

资源级权限先预留字段，不在一期做复杂规则。

验收：

- 未登录访问业务 API 返回 401。
- 无权限访问业务 API 返回 403。
- 管理员可以创建、禁用用户。
- 普通用户不能访问系统管理页面。

### 3.3 Secret 与审计

后端交付：

- Secret CRUD
- 支持 SSH 密码、SSH 私钥、Docker TLS/Token 类型预留
- AES-GCM 加密存储
- API 默认只返回脱敏信息
- 审计日志表
- 审计写入服务
- 审计查询接口

前端交付：

- 凭证列表
- 新增/编辑凭证抽屉
- SecretInput 脱敏输入
- 审计日志列表
- 审计详情抽屉

验收：

- 数据库中不出现明文密码、私钥、token。
- 新增、修改、删除凭证都会生成审计。
- 审计包含用户、动作、资源、结果、traceId、时间。

### 3.4 主机与 WebSSH

后端交付：

- 主机 CRUD
- 主机分组和标签可先简化为字符串字段
- 主机关联 SSH Secret
- SSH 连接测试
- WebSSH 会话创建
- WebSocket 转发终端输入输出
- 会话关闭和资源释放
- WebSSH 连接、断开审计

前端交付：

- 主机列表
- 主机新增/编辑
- SSH 测试按钮和状态展示
- 终端页面
- xterm.js 基础集成
- WebSSH 连接状态

验收：

- 可新增一台主机并测试 SSH 连接。
- 可从主机列表打开 WebSSH。
- 断开页面或关闭会话后，后端释放 SSH session/channel。
- 未授权用户不能打开终端。

### 3.5 任务中心基础版

后端交付：

- 任务表
- 任务步骤表
- 任务日志表
- 任务状态机：PENDING、RUNNING、SUCCESS、FAILED、CANCELED
- 简单本地 worker
- 任务详情和日志查询
- WebSocket 推送任务日志，可先用轮询替代实时推送

前端交付：

- 任务列表
- 任务详情
- 任务步骤展示
- 任务日志查看器
- 状态筛选

一期任务类型：

- SSH 连接测试，可选任务化
- Docker 节点连接测试
- Docker 容器操作

验收：

- 长耗时操作返回 taskId。
- 任务失败能看到失败步骤和错误原因。
- 刷新任务详情页后仍可恢复状态。

### 3.6 Docker 节点与容器

后端交付：

- Docker 节点 CRUD
- Docker 连接测试
- 节点信息查询
- 容器列表
- 容器详情
- 容器日志
- 容器启动、停止、重启
- 容器删除可以放入一期后段，必须二次确认和审计

前端交付：

- Docker 节点列表
- Docker 节点新增/编辑
- 连接测试
- 容器列表
- 容器详情
- 容器日志查看
- 容器启动、停止、重启操作

验收：

- 可新增 Docker 节点并测试连接。
- 可查看节点上的容器列表。
- 可查看容器日志。
- 启动、停止、重启容器有审计记录。
- 删除容器如进入一期，必须输入确认文本。

### 3.7 Dashboard

后端交付：

- 用户数、主机数、Docker 节点数、容器数
- 最近任务
- 最近审计
- 异常资源数量

前端交付：

- 工作台首页
- 统计卡片
- 最近任务列表
- 最近审计列表
- 快捷入口

验收：

- 登录后默认进入工作台。
- 数据为空时展示空状态。
- 最近任务和最近审计可跳转详情。

## 4. 一期开发路线

建议按 6 周推进，1 周一个小里程碑。若只有 1 名后端和 1 名前端，可以拉长到 8 周。

| 周期 | 目标 | 后端重点 | 前端重点 | 验收结果 |
|---|---|---|---|---|
| 第 1 周 | 工程骨架 | Gin、zap、SQLite、GORM、配置、健康检查、统一响应 | Vite、路由、布局、API client、基础组件 | 前后端可本地联调 |
| 第 2 周 | 登录与权限 | 管理员初始化、登录、JWT、用户、角色、权限中间件 | 登录页、主框架、菜单、用户/角色页 | 可登录并按角色访问页面 |
| 第 3 周 | Secret 与审计 | Secret 加密、审计写入、审计查询 | 凭证管理、审计列表、危险确认 | 凭证不明文入库，关键操作可追踪 |
| 第 4 周 | 主机与 WebSSH | 主机 CRUD、SSH 测试、WebSSH WebSocket | 主机页、SSH 测试、xterm 终端页 | 可打开授权主机终端 |
| 第 5 周 | 任务中心与 Docker | 任务表、worker、Docker 节点、容器查询和操作 | 任务中心、Docker 节点、容器列表和日志 | 可管理 Docker 节点和基础容器操作 |
| 第 6 周 | MVP 收口 | 权限补齐、审计补齐、错误码、部署脚本、数据迁移检查 | Dashboard、空状态、错误状态、E2E 冒烟 | 可演示一条完整运维闭环 |

## 5. 一期接口优先级

P0 接口：

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/refresh`
- `GET /api/auth/me`
- `GET /api/dashboard/summary`
- `GET /api/users`
- `POST /api/users`
- `PATCH /api/users/:id`
- `GET /api/roles`
- `POST /api/roles`
- `GET /api/secrets`
- `POST /api/secrets`
- `GET /api/audits`
- `GET /api/hosts`
- `POST /api/hosts`
- `POST /api/hosts/:id/test-ssh`
- `POST /api/hosts/:id/terminal/sessions`
- `GET /api/tasks`
- `GET /api/tasks/:id`
- `GET /api/docker/nodes`
- `POST /api/docker/nodes`
- `POST /api/docker/nodes/:id/test`
- `GET /api/docker/nodes/:id/containers`
- `GET /api/docker/nodes/:id/containers/:containerId/logs`
- `POST /api/docker/nodes/:id/containers/:containerId/start`
- `POST /api/docker/nodes/:id/containers/:containerId/stop`
- `POST /api/docker/nodes/:id/containers/:containerId/restart`

P1 接口：

- 凭证引用检查
- 任务日志 WebSocket
- 容器删除
- Docker 镜像列表
- 审计导出
- 主机批量刷新

## 6. 数据模型优先级

一期 P0 数据表：

- `users`
- `roles`
- `permissions`
- `user_roles`
- `role_permissions`
- `secrets`
- `audit_logs`
- `hosts`
- `tasks`
- `task_steps`
- `task_logs`
- `docker_nodes`

一期 P1 数据表：

- `login_sessions`
- `token_blacklist`
- `host_groups`
- `host_tags`
- `terminal_sessions`
- `system_settings`

SQLite 设计注意事项：

- ID 可以先使用 UUID 字符串，后续迁移 PostgreSQL 更平滑。
- 时间字段统一使用 UTC 存储。
- JSON 配置字段可用 `text` 存储，GORM 层封装序列化。
- 外键可以先由应用层保证，但关键引用删除必须做显式检查。

## 7. MVP 验收口径

一期完成后，至少能演示以下闭环：

1. 管理员初始化并登录。
2. 创建普通用户和角色，配置基础权限。
3. 新增 SSH 凭证，数据库中凭证明文不可见。
4. 新增主机并测试 SSH 连接。
5. 打开 WebSSH，与主机进行基础交互。
6. 新增 Docker 节点并测试连接。
7. 查看容器列表和容器日志。
8. 启动、停止、重启容器，操作进入任务或审计。
9. 在任务中心查看任务状态、步骤、日志。
10. 在审计日志中追踪登录、凭证、主机、Docker 操作。

## 8. 风险与取舍

### 8.1 需要控制的风险

- WebSSH 资源泄漏：必须保证 WebSocket 断开后释放 SSH channel/session。
- 凭证泄漏：Secret API 不能返回明文，日志不能打印敏感字段。
- SQLite 并发写：任务日志高频写入需要控制批量和频率。
- Docker 危险操作：删除容器、停止关键容器必须有权限和确认。
- 权限绕过：前端隐藏菜单不等于授权，后端必须校验。

### 8.2 明确不做的内容

- 一期不做微服务。
- 一期不做 Kubernetes。
- 一期不做 Agent。
- 一期不做服务发布完整流水线。
- 一期不做 Nginx/Jenkins。
- 一期不做复杂多租户和资源范围权限。

## 9. 二期衔接建议

一期稳定后，二期优先补“服务发布闭环”：

- Registry 管理
- 服务定义
- 服务实例
- 发布任务
- 发布失败补偿
- 服务版本
- 简单回滚

三期再进入 Nginx、Jenkins、备份恢复、指标监控。这样路线会更稳：先有安全底座和运维资产控制，再扩展发布与配置管理能力。

