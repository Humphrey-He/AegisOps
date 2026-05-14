# NebulaOps 后端功能清单与项目排期

> 项目名：NebulaOps  
> 定位：面向中小研发团队和私有化环境的安全型轻量 DevOps 运维控制台  
> 后端技术路线：Go + PostgreSQL + Redis 可选 + Docker SDK + SSH + WebSocket  
> 目标：成熟、稳定、可落地，优先保障安全、审计、任务化、可恢复。

## 1. 后端建设原则

NebulaOps 后端不是普通 CRUD 后台，而是基础设施控制面。设计时必须优先考虑以下原则：

- 所有高危操作必须可审计。
- 所有长耗时操作必须任务化。
- 所有凭证、私钥、Token 必须加密存储。
- 所有远程操作必须有超时、取消、失败状态和错误原因。
- 所有发布、删除、升级必须考虑失败补偿。
- 所有 API 必须有统一响应、错误码、traceId。
- 第一阶段先做单体模块化，不急于微服务。
- MVP 可直连 SSH/Docker/Nginx/Jenkins，V1.5 再引入 Agent。

## 2. 推荐后端技术栈

### 2.1 基础栈

- Go 1.22+
- Gin 或 Echo：HTTP API
- PostgreSQL：正式数据存储
- SQLite：可作为单机轻量部署可选项
- GORM 或 sqlc：数据访问
- goose 或 golang-migrate：数据库迁移
- zap 或 zerolog：结构化日志
- Viper：配置管理
- Casbin：RBAC/资源权限
- bcrypt/argon2id：密码哈希
- OpenAPI：接口契约

### 2.2 运维能力依赖

- `golang.org/x/crypto/ssh`：SSH/WebSSH
- Docker Go SDK：Docker 节点和容器操作
- gorilla/websocket 或 nhooyr/websocket：WebSocket
- Registry HTTP API：镜像仓库
- Jenkins HTTP API：CI 集成
- Prometheus client：指标暴露

### 2.3 后续增强

- Redis：任务队列、缓存、分布式锁
- Asynq：异步任务队列
- OpenTelemetry：链路追踪
- Agent：远程节点本地执行器

## 3. 后端模块清单

## 3.1 账号与认证模块

功能清单：

- 用户注册或管理员创建用户
- 登录、退出
- Refresh Token 或服务端 Session
- 密码哈希加盐
- 密码重置
- 修改密码
- 登录失败次数限制
- 登录日志
- Token 失效管理

接口示例：

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/refresh`
- `GET /api/auth/me`
- `POST /api/auth/change-password`

验收标准：

- 密码不可明文入库。
- 登录失败达到阈值后临时锁定。
- 所有登录、退出、失败登录记录审计。
- Token 可主动失效。

## 3.2 权限与资源控制模块

功能清单：

- 用户管理
- 角色管理
- 权限点管理
- 用户绑定角色
- 角色绑定权限
- 资源级权限：主机、Docker 节点、服务、Nginx 节点、Registry
- 菜单权限
- API 权限中间件

权限模型：

```text
User -> UserRole -> Role -> RolePermission -> Permission
Role -> ResourceScope -> Resource
```

验收标准：

- 未登录返回 401。
- 无权限返回 403。
- 权限失败不可返回空响应。
- 高危接口必须单独权限点控制。
- 普通用户只能看到授权资源。

## 3.3 Secret 凭证模块

功能清单：

- 统一保存密码、SSH 私钥、Docker TLS、Registry 密码、Jenkins Token
- AES-GCM 或 KMS 风格加密
- 前端脱敏展示
- 凭证引用关系管理
- 凭证轮换
- 凭证访问审计

数据建议：

```text
secrets:
  id
  name
  type
  encrypted_value
  key_version
  owner_id
  created_at
  updated_at
```

验收标准：

- 业务表不直接存明文密码。
- API 不返回完整密文或明文。
- 凭证读取必须记录用途和调用方。
- 删除凭证前检查是否被资源引用。

## 3.4 审计模块

功能清单：

- 操作审计
- 登录审计
- 远程命令审计
- 高危操作审计
- 审计查询
- 审计导出

必须审计的动作：

- 登录、退出、登录失败
- 新增/删除/修改用户和角色
- 新增/删除/修改主机和凭证
- WebSSH 连接、断开、命令摘要
- 容器创建、删除、停止、重启、升级
- 镜像删除
- Nginx 配置变更、test、reload、rollback
- Jenkins 构建触发

验收标准：

- 审计日志不可被普通用户删除。
- 每条审计包含 userId、clientIp、resourceType、resourceId、action、result、traceId、createdAt。
- 高危操作失败也必须记录。

## 3.5 任务中心模块

功能清单：

- 创建任务
- 任务步骤
- 任务状态机
- 任务日志
- 任务取消
- 任务重试
- 任务失败原因
- WebSocket 推送任务日志

任务状态：

```text
PENDING -> RUNNING -> SUCCESS
                 └-> FAILED
                 └-> CANCELED
```

任务类型：

- Docker 节点探测
- 服务发布
- 服务升级
- 服务回滚
- 容器删除
- Nginx 配置测试
- Nginx 配置发布
- Jenkins Job 同步
- Jenkins 构建

验收标准：

- 长耗时操作必须返回 taskId。
- 任务步骤有开始时间、结束时间、状态、错误原因。
- 服务发布失败可定位到具体步骤。
- 任务日志可实时查看。

## 3.6 主机资产模块

功能清单：

- 主机 CRUD
- 主机分组
- 主机标签
- SSH 连接测试
- 主机基础信息采集
- 主机关联 SSH 凭证
- 主机状态刷新

验收标准：

- SSH 连接测试必须有超时。
- 主机凭证通过 Secret 引用。
- 主机删除前检查是否存在服务、任务或权限引用。
- 批量刷新不能无限开 goroutine，必须受 worker 限制。

## 3.7 WebSSH 模块

功能清单：

- 创建终端会话
- WebSocket 连接
- SSH shell 通道
- 终端输入输出转发
- 心跳检测
- 空闲超时
- 会话强制关闭
- 终端尺寸调整
- 命令审计

验收标准：

- WebSocket 断开必须释放 SSH session/channel。
- 单用户终端数可配置。
- 全局终端数可配置。
- 命令审计可以按安全策略开启或脱敏。
- 不允许未授权用户连接主机。

## 3.8 Docker 节点模块

功能清单：

- Docker 节点 CRUD
- TLS 配置
- 连接测试
- 节点信息采集：CPU、内存、容器数、镜像数
- 容器列表
- 容器详情
- 容器日志
- 容器启动、停止、重启、删除
- 镜像列表

验收标准：

- Docker API 调用必须有 context timeout。
- TLS 证书通过 Secret 管理。
- 删除容器是高危操作，必须审计。
- 容器操作必须任务化或至少记录操作状态。

## 3.9 Registry 镜像仓库模块

功能清单：

- Registry CRUD
- Registry 连接测试
- 镜像列表
- Tag 列表
- Manifest 查询
- Digest 删除

验收标准：

- Registry 密码通过 Secret 管理。
- 删除镜像必须有独立权限点。
- 删除操作必须生成审计。
- 镜像查询失败要区分认证失败、网络失败、仓库不存在。

## 3.10 服务发布模块

功能清单：

- 服务定义
- 服务分组
- 服务版本
- 环境变量
- 端口映射
- 挂载配置
- CPU/内存限制
- 手动发布
- 自动选择 Docker 节点
- 服务实例列表
- 实例日志
- 实例启动、停止、重启、删除

发布流程：

```text
校验参数
  -> 校验权限
  -> 锁定服务发布
  -> 拉取镜像
  -> 创建容器
  -> 启动容器
  -> 健康检查
  -> 写入服务实例
  -> 记录审计
  -> 释放锁
```

验收标准：

- 容器创建成功但 DB 写入失败时必须删除容器。
- DB 写入成功但容器启动失败时必须记录失败实例状态。
- 同一服务同一环境不能并发发布。
- 发布任务必须可查步骤日志。

## 3.11 服务升级与回滚模块

功能清单：

- 选择目标镜像 Tag
- 创建新版本
- 灰度比例预留
- 先新后旧升级
- 健康检查
- 失败回滚
- 手动回滚到历史版本

推荐策略：

```text
创建新容器
  -> 健康检查
  -> 更新流量入口或标记当前版本
  -> 停止旧容器
  -> 保留回滚窗口
```

验收标准：

- 不允许先删除旧实例再创建新实例。
- 升级失败时旧版本仍可用。
- 每次升级都有版本快照。
- 回滚也是任务，也需要审计。

## 3.12 Nginx 管理模块

功能清单：

- Nginx 节点 CRUD
- 节点连接测试
- 配置文件列表
- 配置读取
- 配置版本保存
- 配置 diff
- 配置 test
- reload
- rollback
- 配置模板

验收标准：

- 配置 reload 前必须 test 通过。
- 每次保存必须生成版本。
- 每次 reload 必须记录审计。
- rollback 必须生成新任务。
- 远端 API 需要 HMAC 签名或 Agent 通道。

## 3.13 Jenkins/CI 模块

功能清单：

- Jenkins 配置
- 连接测试
- Job 同步
- Job 列表
- 触发构建
- 构建任务记录

验收标准：

- Jenkins Token 通过 Secret 管理。
- 构建触发必须审计。
- Job 同步不直接硬删除，使用状态标记。
- 构建失败要展示远端错误原因。

## 3.14 系统配置与运维模块

功能清单：

- 系统参数
- 文件上传
- 备份恢复
- 健康检查
- 指标暴露
- 版本信息
- 初始化管理员

验收标准：

- 上传文件限制类型和大小。
- 备份包含数据库和必要配置，不包含明文密钥。
- `/healthz` 和 `/readyz` 可用于部署探针。

## 4. 后端里程碑排期

按 2 周一个 Sprint。默认 2 名后端研发，可 18 周交付 V1.0 试运行版。

| Sprint | 周期 | 目标 | 后端交付 |
|---|---:|---|---|
| Sprint 0 | 第 1 周 | 工程骨架 | Go 项目结构、配置、日志、数据库迁移、OpenAPI、CI、健康检查 |
| Sprint 1 | 第 2-3 周 | 认证权限 | 用户、角色、权限、登录、密码哈希、权限中间件、登录审计 |
| Sprint 2 | 第 4-5 周 | 安全底座 | Secret 加密、审计模块、统一响应、错误码、traceId |
| Sprint 3 | 第 6-7 周 | 主机与 WebSSH | 主机资产、SSH 凭证引用、连接测试、WebSSH、会话清理、命令审计 |
| Sprint 4 | 第 8-9 周 | Docker 与 Registry | Docker 节点、TLS、容器操作、Registry、镜像和 Tag 查询 |
| Sprint 5 | 第 10-11 周 | 任务中心与服务发布 | 任务表、worker、任务日志、服务定义、手动发布、失败补偿 |
| Sprint 6 | 第 12 周 | MVP 稳定 | 权限补齐、审计补齐、接口压测、部署脚本、MVP 修复 |
| Sprint 7 | 第 13-14 周 | 服务升级回滚 | 服务版本、升级任务、健康检查、回滚、发布锁 |
| Sprint 8 | 第 15-16 周 | Nginx 管理 | Nginx 节点、配置版本、diff、test、reload、rollback |
| Sprint 9 | 第 17 周 | Jenkins 集成 | Jenkins 配置、Job 同步、构建触发、构建审计 |
| Sprint 10 | 第 18 周 | V1.0 收口 | 备份恢复、指标、文档、性能优化、安全检查 |

## 5. 后端验收口径

### MVP 验收

- 可登录并按权限访问 API。
- 可新增主机并测试 SSH。
- 可通过 WebSSH 登录授权主机。
- 可新增 Docker 节点并测试连接。
- 可新增 Registry 并查询镜像 Tag。
- 可定义服务并发布容器。
- 发布过程有 taskId、步骤、日志和失败原因。
- 高危操作有审计日志。
- 密码和凭证不明文存储。

### V1.0 验收

- 支持服务升级和回滚。
- 支持 Nginx 配置版本、diff、test、reload、rollback。
- 支持 Jenkins Job 同步和触发构建。
- 所有远程操作有超时。
- WebSSH 无明显连接泄漏。
- 后端接口错误码稳定。
- 有部署文档和备份恢复方案。

## 6. 成熟落地建议

第一版不要做微服务，也不要过早上 Kubernetes。后端必须先把认证、权限、Secret、审计、任务中心这 5 个底座打牢。后续所有功能都挂在这 5 个底座上，才能让 NebulaOps 从“能操作机器的后台”变成“可控、可信、可追踪的企业级运维平台”。
