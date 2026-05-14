# AegisOps 前后端联调说明（前端侧）

## 1. 文档目的

本文档面向一期 MVP 前端与后端联调阶段，说明：

- 前端当前如何切换 mock / real API
- 前端现有代码期望的接口形态
- 后端当前已确认存在的接口
- 当前明确存在的联调阻塞项
- 前端侧建议的联调顺序与检查清单

本文档重点是“前端视角如何顺利接后端”，不是后端实现说明。

## 2. 前端本地运行方式

前端目录：

- `E:\awesomeProject\AegisOps\frontend`

启动方式：

```powershell
cd E:\awesomeProject\AegisOps\frontend
npm.cmd install
npm.cmd run dev
```

当前 `package.json` 中开发端口为：

- `4173`

默认访问地址通常为：

- `http://127.0.0.1:4173`

## 3. 前端当前的环境变量约定

前端配置定义在 `frontend/src/lib/config.ts`：

```ts
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api";
export const USE_MOCK = import.meta.env.VITE_USE_MOCK !== "false";
```

含义如下：

- `VITE_API_BASE_URL`
  - 未显式配置时默认为 `/api`
- `VITE_USE_MOCK`
  - 只要不是字符串 `"false"`，前端就会走 mock
  - 也就是说默认就是 mock 模式

联调时建议显式设置：

```powershell
$env:VITE_USE_MOCK="false"
$env:VITE_API_BASE_URL="http://127.0.0.1:8080/api"
npm.cmd run dev
```

如果不显式关闭 mock，前端看起来“能操作”，但其实请求并不会发到真实后端。

## 4. 后端本地运行方式

根据仓库 `README.md`，后端默认启动方式：

```powershell
cd E:\awesomeProject\AegisOps
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

默认地址：

- `http://127.0.0.1:8080`

默认管理员账号：

- 用户名：`admin`
- 密码：`admin123456`

前端联调目标 base URL 建议使用：

- `http://127.0.0.1:8080/api`

## 5. 前端当前接入层说明

### 5.1 请求封装

`frontend/src/lib/http.ts` 当前行为：

- 自动拼接 `API_BASE_URL + path`
- 自动发送 `Content-Type: application/json`
- 如果本地存在 token，则自动带上 `Authorization: Bearer <token>`
- 收到 401 时自动清理本地 session

### 5.2 会话存储

`frontend/src/store/sessionStore.ts` 当前保存：

- `token`
- `user`
- `permissions`

其中 token 被存进 localStorage，key 为：

- `aegisops-mvp-session`

### 5.3 启动 bootstrap

`frontend/src/app/App.tsx` 会在应用启动时调用 `useBootstrapSession()`。

当前问题是：`useBootstrapSession()` 仍然直接调用 `mockService`：

- `mockService.getSetupStatus()`
- `mockService.me(token)`

这意味着：

- 就算登录改成真实后端
- 刷新页面后的初始化与会话恢复仍可能使用 mock 逻辑

这是联调前端必须优先修复的一个阻塞点。

## 6. 前端已定义的 API 期望

前端在 `frontend/src/lib/api.ts` 中已经定义了以下 API 模块：

- `authApi`
- `dashboardApi`
- `usersApi`
- `rolesApi`
- `secretsApi`
- `hostsApi`
- `terminalApi`
- `dockerApi`
- `tasksApi`
- `auditsApi`

### 6.1 authApi 期望接口

- `GET /auth/setup-status`
- `POST /auth/setup`
- `POST /auth/login`
- `GET /auth/me`
- `POST /auth/logout`

### 6.2 dashboardApi 期望接口

- `GET /dashboard/summary`

### 6.3 usersApi 期望接口

- `GET /users?keyword=...`
- `POST /users`
- `PATCH /users/:id`

### 6.4 rolesApi 期望接口

- `GET /roles`
- `POST /roles`
- `PATCH /roles/:id`

### 6.5 secretsApi 期望接口

- `GET /secrets?keyword=...`
- `POST /secrets`
- `PATCH /secrets/:id`

### 6.6 hostsApi 期望接口

- `GET /hosts?keyword=...`
- `POST /hosts`
- `PATCH /hosts/:id`
- `POST /hosts/:id/test-ssh`

### 6.7 terminalApi 期望接口

- `POST /hosts/:id/terminal/sessions`
- `GET /terminal/sessions/:id`

### 6.8 dockerApi 期望接口

- `GET /docker/nodes`
- `POST /docker/nodes`
- `PATCH /docker/nodes/:id`
- `POST /docker/nodes/:id/test`
- `GET /docker/nodes/:id`
- `GET /docker/nodes/:id/containers`
- `GET /docker/containers/:containerId/logs`
- `POST /docker/nodes/:id/containers/:containerId/start`
- `POST /docker/nodes/:id/containers/:containerId/stop`
- `POST /docker/nodes/:id/containers/:containerId/restart`

### 6.9 tasksApi 期望接口

- `GET /tasks`
- `GET /tasks/:id`

### 6.10 auditsApi 期望接口

- `GET /audits`

## 7. 已确认存在的后端接口

结合 `internal/server/router.go` 与各 handler 注册逻辑，当前后端已确认挂载在 `/api` 下的接口包括：

### 7.1 认证

- `POST /api/auth/login`
- `POST /api/auth/refresh`
- `POST /api/auth/logout`
- `GET /api/auth/me`

### 7.2 用户

- `GET /api/users`
- `POST /api/users`
- `GET /api/users/:id`
- `PATCH /api/users/:id`
- `DELETE /api/users/:id`

### 7.3 角色与权限

- `GET /api/roles`
- `POST /api/roles`
- `GET /api/roles/:id`
- `PATCH /api/roles/:id`
- `DELETE /api/roles/:id`
- `GET /api/permissions`
- `POST /api/permissions`

### 7.4 审计

- `GET /api/audits`
- `GET /api/audits/:id`

### 7.5 凭证

- `GET /api/secrets`
- `POST /api/secrets`
- `GET /api/secrets/:id`
- `PATCH /api/secrets/:id`
- `DELETE /api/secrets/:id`

### 7.6 主机

- `GET /api/hosts`
- `POST /api/hosts`
- `GET /api/hosts/:id`
- `PATCH /api/hosts/:id`
- `DELETE /api/hosts/:id`
- `POST /api/hosts/:id/test-ssh`

### 7.7 任务

- `GET /api/tasks`
- `POST /api/tasks`
- `GET /api/tasks/:id`
- `POST /api/tasks/:id/steps`
- `POST /api/tasks/:id/logs`

### 7.8 Docker

- `GET /api/docker/nodes`
- `POST /api/docker/nodes`
- `GET /api/docker/nodes/:id`
- `PATCH /api/docker/nodes/:id`
- `DELETE /api/docker/nodes/:id`
- `POST /api/docker/nodes/:id/test`
- `GET /api/docker/nodes/:id/containers`
- `GET /api/docker/nodes/:id/containers/:containerId/logs`
- `POST /api/docker/nodes/:id/containers/:containerId/start`
- `POST /api/docker/nodes/:id/containers/:containerId/stop`
- `POST /api/docker/nodes/:id/containers/:containerId/restart`

## 8. 当前前后端不一致点

这一部分是联调最关键的内容。

### 8.1 登录响应结构不一致

前端当前期望：

```ts
type AuthSession = {
  token: string;
  user: User;
  permissions: string[];
};
```

后端当前登录响应实际更接近：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "user": {},
    "tokens": {
      "accessToken": "...",
      "refreshToken": "...",
      "accessTokenExpiresAt": "...",
      "refreshTokenExpiresAt": "...",
      "tokenType": "Bearer"
    }
  },
  "traceId": "..."
}
```

需要统一的点：

- 前端到底保存 `accessToken` 还是一整个 `tokens`
- 前端需要不要支持 refresh token
- `permissions` 是由后端直接返回，还是前端从 `user.roles.permissions` 推导

### 8.2 `auth/me` 响应结构不一致

前端当前期望：

```ts
type CurrentUserPayload = {
  user: User;
  permissions: string[];
};
```

后端当前 `auth/me` 返回的是当前用户对象本身，而不是 `{ user, permissions }` 包裹结构。

因此刷新页面时，前端不能直接按当前模型消费后端返回。

### 8.3 列表接口返回结构不一致

前端多数列表接口当前期望直接得到数组，例如：

- `User[]`
- `Role[]`
- `Secret[]`
- `Host[]`
- `Task[]`
- `AuditLog[]`

但后端多个列表接口当前实际返回分页对象，例如：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "pageSize": 20
  },
  "traceId": "..."
}
```

或部分 handler 返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [],
    "total": 0,
    "limit": 20,
    "offset": 0
  }
}
```

必须统一至少两个问题：

- 前端列表页是否统一按分页对象消费
- 后端是否统一 `page/pageSize` 或 `limit/offset`

### 8.4 响应包装格式未完全统一

当前仓库里同时存在两套响应风格：

1. `pkg/response/response.go`
   - `code: "OK"`
   - `message`
   - `data`
   - `traceId`

2. `internal/handler/response.go`
   - `code: 0` 或状态码
   - `message`
   - `data`
   - 无 `traceId`

前端 `frontend/src/types/api.ts` 当前统一按以下格式解析：

```ts
type ApiResponse<T> = {
  code: string;
  message: string;
  data: T;
  traceId: string;
};
```

因此如果命中仍使用旧 handler 响应格式的接口，前端的错误处理和 traceId 展示会不稳定。

### 8.5 部分前端接口当前后端未确认存在

前端已预留但当前未确认存在的接口：

- `GET /api/auth/setup-status`
- `POST /api/auth/setup`
- `GET /api/dashboard/summary`
- `POST /api/hosts/:id/terminal/sessions`
- `GET /api/terminal/sessions/:id`

这些接口如果暂时没有后端实现，前端需要：

- 要么临时保留 mock
- 要么在页面上标记“待后端接入”
- 要么改由现有后端能力组合实现

### 8.6 Docker 日志接口路径不一致

前端当前写的是：

- `GET /docker/containers/:containerId/logs`

后端当前真实路由是：

- `GET /docker/nodes/:id/containers/:containerId/logs`

这个是确定性的路径不一致，联调前必须改一边。

### 8.7 Docker 日志响应体不一致

前端当前期望：

- `string[]`

后端当前返回：

```json
{
  "logs": [...]
}
```

需要统一为：

- 前端改为读取 `data.logs`
  或
- 后端直接返回字符串数组

### 8.8 SSH 测试与容器动作返回体不一致

前端当前把以下操作当作“返回 Task”处理：

- `POST /hosts/:id/test-ssh`
- `POST /docker/nodes/:id/test`
- `POST /docker/nodes/:id/containers/:containerId/start`
- `POST /docker/nodes/:id/containers/:containerId/stop`
- `POST /docker/nodes/:id/containers/:containerId/restart`

但后端当前多数返回的是：

- `{ connected: true }`
- `{ started: true }`
- `{ stopped: true }`
- `{ restarted: true }`

而不是 `Task` 对象。

这会直接影响任务中心跳转、按钮成功提示和详情页链路。

### 8.9 用户与角色字段模型不完全一致

前端当前模型使用：

- 用户 `roleIds: string[]`
- 角色 `permissions: string[]`

后端当前更偏向：

- 用户 `roleIds: []uint`
- 角色 `permissionIds: []uint`
- 返回实体中常带完整 `Roles` / `Permissions` 对象

因此：

- 表单提交字段类型要统一
- 列表页与详情页显示也要统一映射方式

## 9. 前端联调前建议先处理的事项

建议优先完成以下前端改造，再进入大规模联调：

1. 将 `useBootstrapSession()` 改为支持真实 `authApi`
2. 统一 session 模型，至少明确 `accessToken` 如何落库
3. 统一列表接口消费方式，明确是否分页
4. 统一 `auth/me` 的消费结构
5. 修正 Docker 日志接口路径
6. 把动作类接口返回从“直接当 Task”改为“按真实返回值处理”或要求后端返回 Task
7. 将真实业务路由接入 `router.tsx`

## 10. 建议联调顺序

建议按风险由低到高推进：

### 第一阶段：认证与基础会话

- 登录
- 当前用户
- 登出
- 401 清理登录态
- 刷新页面恢复登录态

### 第二阶段：系统管理

- 用户列表
- 新建用户
- 编辑用户
- 角色列表
- 新建角色
- 编辑角色
- 权限列表

### 第三阶段：审计与任务

- 审计列表
- 审计详情
- 任务列表
- 任务详情

### 第四阶段：资产与 Docker

- 凭证列表 / 新建 / 编辑
- 主机列表 / 新建 / 编辑
- SSH 测试
- Docker 节点列表 / 新建 / 编辑
- 容器列表
- 容器日志
- 启停重启动作

### 第五阶段：工作台与终端

- 工作台聚合摘要
- Web 终端会话

## 11. 联调检查清单

每轮联调建议至少检查以下项目：

- 前端是否真的关闭了 mock
- 登录后请求头是否带上 Bearer token
- 401 时是否自动清除本地 session
- 页面刷新后是否还能恢复登录态
- 列表页是否能正确解析后端分页结构
- 表单校验错误是否能回填到字段
- 403 是否能进入无权限页或展示明确提示
- 后端错误是否能透出 `traceId`
- 审计与任务的详情页链路是否闭环

## 12. 支持 UI 优化方向的联调补充建议

如果按《AegisOps 前端实现情况说明》中提出的“主列表 + 右侧资源详情工作台”方向继续演进，联调阶段建议额外补充以下接口能力：

1. 资源关联任务过滤
   - 建议支持：
   - `GET /api/tasks?resourceType=host&resourceId=...`
   - `GET /api/tasks?resourceType=dockerNode&resourceId=...`

2. 资源关联审计过滤
   - 建议支持：
   - `GET /api/audits?resourceType=host&resourceId=...`
   - `GET /api/audits?resourceType=dockerNode&resourceId=...`

3. 动作接口统一返回任务对象或任务 ID
   - 适用接口：
   - `POST /api/hosts/:id/test-ssh`
   - `POST /api/docker/nodes/:id/test`
   - `POST /api/docker/nodes/:id/containers/:containerId/start`
   - `POST /api/docker/nodes/:id/containers/:containerId/stop`
   - `POST /api/docker/nodes/:id/containers/:containerId/restart`

4. 资源详情接口保持稳定
   - 主机页依赖 `GET /api/hosts/:id`
   - Docker 节点页依赖 `GET /api/docker/nodes/:id`
   - 如果后续详情面板需要展示健康摘要，建议直接在详情接口里返回最近检测结果字段

这几项不是必须一次做完，但如果准备把前端从“列表页 + 跳转页”升级成“资源工作台模式”，它们会直接决定体验上限。

## 13. 当前结论

从前端侧看，一期 MVP 联调已经具备启动条件，但还没有进入“直接切真接口就能跑通”的状态。

目前最大的阻塞不是页面数量不够，而是接口契约仍未统一，主要集中在：

- session 结构
- `auth/me` 返回形态
- 列表分页结构
- 动作接口返回值
- Docker 日志路径
- mock bootstrap 仍未切换

建议把这些差异先收口，再开始逐页联调。这样后面的推进会顺很多，也更容易判断问题到底在前端适配、接口定义还是后端实现。
