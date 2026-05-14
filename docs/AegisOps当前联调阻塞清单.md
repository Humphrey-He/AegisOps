# AegisOps 当前联调阻塞清单

## 1. 文档目的

本文档用于整理 AegisOps 当前前后端联调阶段的主要阻塞项，帮助前端、后端和产品在同一份清单上协同推进。

本文档重点回答四件事：

1. 哪些模块已经初步打通
2. 哪些模块仍处于半联调状态
3. 当前最影响页面可用性的阻塞点是什么
4. 每个阻塞点更适合由前端处理、后端处理，还是双方共同处理

## 2. 当前联调总体判断

当前联调状态可以概括为：

- 真实后端链路已经接入
- 认证链路基本打通
- 用户、角色模块已进入真实接口适配阶段
- 资源管理、任务、Docker、终端等模块大多仍处于“接口连上但数据模型未对齐”的状态

因此目前不是“完全未联调”，也不是“可以完整验收”，而是：

- `基础通路已建立`
- `业务页面仍有多项关键阻塞待清理`

## 3. 已确认打通的部分

以下内容已经确认具备基础联调条件：

### 3.1 前端默认走真实后端

当前前端配置：

- `frontend/src/lib/config.ts`

现状：

- `VITE_USE_MOCK === "true"` 才启用 mock
- 默认状态下前端走真实 API

### 3.2 前端本地代理已配置

当前 Vite 配置：

- `frontend/vite.config.ts`

现状：

- `/api` 已代理到 `http://127.0.0.1:8080`

### 3.3 后端基础服务在线

已确认以下接口可访问：

- `GET /healthz`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/users`
- `GET /api/roles`
- `GET /api/audits`
- `GET /api/hosts`
- `GET /api/secrets`
- `GET /api/tasks`
- `GET /api/docker/nodes`

说明：

- 后端服务本身可用
- 认证、鉴权、基础路由注册没有整体性中断

## 4. 阻塞项分级

建议按以下优先级理解当前问题：

### P0

会直接导致页面打不开、关键路径不通、用户无法完成基本操作。

### P1

页面可打开，但功能主链路会失败或结果错误。

### P2

页面能运行，但数据展示、字段语义、交互细节存在明显偏差。

## 5. 当前联调阻塞清单

## 5.1 P0 - 资源列表接口返回结构未统一

### 现象

前端多个资源页当前仍把列表接口当作数组消费，但后端返回的是分页对象。

涉及前端接口：

- `hostsApi.list`
- `secretsApi.list`
- `tasksApi.list`
- `dockerApi.listNodes`
- `auditsApi.list`

涉及前端文件：

- `frontend/src/lib/api.ts`

涉及后端文件：

- `internal/handler/hosts.go`
- `internal/handler/secrets.go`
- `internal/handler/tasks.go`
- `internal/handler/docker.go`
- `internal/handler/audit.go`

### 现状差异

前端期望：

- `Host[]`
- `Secret[]`
- `Task[]`
- `DockerNode[]`
- `AuditLog[]`

后端实际多数返回：

```json
{
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "pageSize": 20
  }
}
```

或：

```json
{
  "data": {
    "items": [],
    "total": 0,
    "limit": 20,
    "offset": 0
  }
}
```

### 影响

- 列表页大概率拿不到正确数据
- 表格 `dataSource` 可能收到对象而不是数组
- 页面虽然能发请求，但无法稳定显示真实列表

### 责任归属

- 前端主责
- 后端协助

### 建议处理

前端统一增加分页解包逻辑，至少先兼容：

- `data.items`

同时建议双方统一分页返回结构，避免同一项目里同时存在：

- `page/pageSize`
- `limit/offset`

## 5.2 P0 - 动作类接口返回模型不一致

### 现象

前端当前把以下接口都当成“返回任务对象”处理：

- `POST /hosts/:id/test-ssh`
- `POST /docker/nodes/:id/test`
- `POST /docker/nodes/:id/containers/:containerId/start`
- `POST /docker/nodes/:id/containers/:containerId/stop`
- `POST /docker/nodes/:id/containers/:containerId/restart`

涉及前端文件：

- `frontend/src/lib/api.ts`
- `frontend/src/pages/assets/HostsPage.tsx`
- `frontend/src/pages/docker/DockerNodesPage.tsx`
- `frontend/src/pages/docker/DockerNodeDetailPage.tsx`

涉及后端文件：

- `internal/handler/hosts.go`
- `internal/handler/docker.go`

### 现状差异

前端期望：

- 返回 `Task`
- 然后前端跳转 `/tasks/${task.id}`

后端实际返回：

- `{"connected": true}`
- `{"started": true}`
- `{"stopped": true}`
- `{"restarted": true}`

### 影响

- 前端会把返回结果错误地当成任务对象
- 页面跳转任务详情时可能拿到空 ID 或错误路径
- 任务中心链路无法闭环

### 责任归属

- 前后端共同处理

### 建议处理

二选一：

1. 后端统一返回任务对象或至少返回任务 ID
2. 前端改为不依赖任务对象，动作成功后只提示结果，不跳任务详情

如果产品目标是“动作可追踪”，建议优先采用第 1 种。

## 5.3 P0 - Docker 日志接口路径与返回结构不一致

### 现象

前端与后端对 Docker 日志接口定义不一致。

涉及前端文件：

- `frontend/src/lib/api.ts`
- `frontend/src/pages/docker/DockerNodeDetailPage.tsx`

涉及后端文件：

- `internal/handler/docker.go`

### 现状差异

前端当前请求：

- `GET /docker/containers/:containerId/logs`

后端当前提供：

- `GET /docker/nodes/:id/containers/:containerId/logs`

同时：

前端期望：

- `string[]`

后端返回：

```json
{
  "data": {
    "logs": "..."
  }
}
```

更进一步，后端 `ContainerLogs` 当前返回的本体还是单个字符串，而不是字符串数组。

涉及后端实现：

- `internal/docker/service.go`

### 影响

- 容器日志页无法直接显示真实日志
- 即使接口能通，前端拿到的数据结构也不匹配

### 责任归属

- 前后端共同处理

### 建议处理

统一三件事：

1. 路由路径
2. `data` 内部结构
3. 前端日志组件的消费模型

## 5.4 P0 - Web 终端接口尚未存在

### 现象

前端已预留终端会话接口，但后端当前没有对应路由。

涉及前端文件：

- `frontend/src/lib/api.ts`
- `frontend/src/pages/assets/HostsPage.tsx`

前端当前调用：

- `POST /hosts/:id/terminal/sessions`
- `GET /terminal/sessions/:id`

后端当前在 `internal/server/router.go` 和 `internal/handler` 中没有对应注册。

### 影响

- “打开终端”链路在真实联调模式下无法工作
- 终端页即使存在，也缺少真实会话来源

### 责任归属

- 后端主责
- 前端配合

### 建议处理

如果终端是一期必须交付能力，需要尽快补：

1. 创建终端会话接口
2. 查询终端会话接口
3. 后续 WebSocket 通道

如果终端不是本期必交项，则前端应明确标记为“待后端接入”。

## 5.5 P0 - 前端路由中缺少终端页路由

### 现象

前端主机页点击“打开终端”后会跳：

- `/terminal/${sessionId}`

但当前路由表中没有注册对应终端路由。

涉及文件：

- `frontend/src/app/router.tsx`

### 影响

- 即便后端接口补齐，前端也会因缺少路由而无法正常进入终端页

### 责任归属

- 前端主责

### 建议处理

补齐终端页路由注册，并接入权限守卫。

## 5.6 P1 - 审计模型字段未对齐

### 现象

审计页当前页面模型与后端返回字段不一致。

涉及前端文件：

- `frontend/src/types/models.ts`
- `frontend/src/pages/audits/AuditsPage.tsx`
- `frontend/src/lib/api.ts`

涉及后端文件：

- `internal/model/audit.go`
- `internal/handler/audit.go`

### 现状差异

前端模型：

- `actor`
- `resourceName`
- `summary`
- `result: "SUCCESS" | "FAILED"`

后端返回：

- `username`
- `resourceId`
- `message`
- `result: "success" | "failure"`

### 影响

- 审计页字段会显示为空或语义错位
- 搜索条件与详情展示都会偏差

### 责任归属

- 前端主责
- 后端可选配合

### 建议处理

前端增加 `AuditLog` 的后端映射层，把字段统一成页面需要的模型。

## 5.7 P1 - 主机模型字段未对齐

### 现象

主机页前端模型与后端主机模型存在明显差异。

前端模型见：

- `frontend/src/types/models.ts`

后端模型见：

- `internal/model/host.go`

### 现状差异

前端使用：

- `port`
- `secretId`
- `lastCheckedAt`
- `tags: string[]`
- `status: HEALTHY / UNREACHABLE / UNKNOWN / TESTING`

后端使用：

- `sshPort`
- `sshSecretId`
- `lastTestAt`
- `tags: string`
- `status: ONLINE / OFFLINE / UNKNOWN`
- 另外还要求 `sshUser`

### 影响

- 主机创建和编辑表单字段无法直接对接
- 列表状态与最近检测字段显示会错位
- 标签结构需要转换

### 责任归属

- 前后端共同处理

### 建议处理

优先做前端映射层，短期兼容后端字段。

同时建议确认最终统一模型，避免长期维护双语义字段。

## 5.8 P1 - 凭证模型字段未对齐

### 现象

前端凭证页模型与后端 Secret 模型不一致。

前端模型见：

- `frontend/src/types/models.ts`

后端模型见：

- `internal/model/secret.go`
- `internal/secret/service.go`

### 现状差异

前端使用：

- `username`
- `valueMasked`
- `usedBy`
- `secretValue`

后端使用：

- `maskedValue`
- 不直接返回 `username`
- 不直接返回 `usedBy`
- 创建/更新时字段名是 `value`

### 影响

- 列表页展示字段无法直接复用
- 新建/编辑表单提交字段名不匹配

### 责任归属

- 前后端共同处理

### 建议处理

建议先由前端做请求/响应映射，并由产品确认：

- 一期是否真的需要 `username`
- 一期是否需要 `usedBy`

## 5.9 P1 - Docker 节点模型字段未对齐

### 现象

前端 Docker 节点模型与后端模型不一致。

前端模型见：

- `frontend/src/types/models.ts`

后端模型见：

- `internal/model/docker.go`
- `internal/docker/service.go`

### 现状差异

前端使用：

- `tlsEnabled: boolean`
- `containerCount`
- `lastCheckedAt`

后端使用：

- `authType`
- `secretId`
- `lastTestAt`
- 未直接返回 `containerCount`

### 影响

- 节点表单无法直接提交到后端模型
- 列表页统计字段需要额外计算或补接口

### 责任归属

- 前后端共同处理

### 建议处理

短期建议前端先映射：

- `tlsEnabled=true` -> `authType=TLS`

但更根本的是要确定一期到底以：

- `tlsEnabled`
还是
- `authType + secretId`

作为最终页面模型。

## 5.10 P1 - 任务模型字段未对齐

### 现象

前端任务页面模型与后端 Task 模型差异较大。

前端模型见：

- `frontend/src/types/models.ts`

后端模型见：

- `internal/model/task.go`

### 现状差异

前端使用：

- `target`
- `initiatedBy`
- `progress`
- `summary`
- `logs[].timestamp`
- `steps[].title`
- `steps[].detail`

后端使用：

- `targetType`
- `targetId`
- `createdBy`
- `title`
- 无 `progress`
- `logs[].createdAt`
- `steps[].name`
- `steps[].result / error`

### 影响

- 任务中心和任务详情页无法直接渲染真实任务数据
- 即使请求成功，页面字段也会大量错位

### 责任归属

- 前后端共同处理

### 建议处理

短期建议前端先做一层任务 DTO 映射，把后端字段转成页面展示模型。

## 5.11 P1 - 角色权限保存未真正联调

### 现象

前端角色页虽然已经接入真实 `roles` 列表和保存，但保存时并没有把权限勾选真实映射到后端。

涉及前端文件：

- `frontend/src/lib/api.ts`
- `frontend/src/pages/system/RolesPage.tsx`

### 现状差异

前端当前提交：

- `permissionIds: []`

也就是权限配置在真实模式下仍未真正写入后端。

### 影响

- 角色页看起来能编辑，但权限配置是假的或不完整的

### 责任归属

- 前端主责
- 后端协助

### 建议处理

先补 `GET /permissions` 的消费与权限勾选到 `permissionIds` 的映射。

## 5.12 P2 - 仪表盘与管理员初始化仍未完成真实联调

### 现象

前端已预留以下接口，但当前后端未确认存在：

- `GET /auth/setup-status`
- `POST /auth/setup`
- `GET /dashboard/summary`

### 影响

- 初始化管理员页无法走真实后端
- 仪表盘页暂时无法基于真实数据工作

### 责任归属

- 前后端共同处理

### 建议处理

如果一期验收不依赖这两块，可暂时降级优先级。

## 6. 按责任方整理

### 6.1 前端主责

- 补列表接口分页解包
- 补审计模型映射
- 补主机 / 凭证 / Docker / 任务 DTO 映射
- 补终端页路由
- 补角色权限保存映射

### 6.2 后端主责

- 补 Web 终端接口
- 明确动作接口是否返回任务对象
- 如需统一，补 dashboard / setup-status / setup 接口

### 6.3 双方共同确认

- 列表分页结构统一
- 动作类接口返回模型统一
- Docker 日志接口路径与数据结构统一
- Host / Secret / DockerNode / Task / Audit 的最终 DTO 契约统一

## 7. 建议排期顺序

建议按以下顺序推进：

1. 列表分页解包与 DTO 映射
2. 动作接口返回模型统一
3. Docker 日志接口对齐
4. 角色权限真实保存
5. 终端路由与终端接口补齐
6. 仪表盘与管理员初始化补齐

## 8. 最终结论

当前 AegisOps 的联调阻塞并不集中在“网络不通”或“后端没起来”，而是集中在：

- 前后端业务模型未统一
- 列表与动作接口返回结构不一致
- 个别关键接口仍缺失

因此下一阶段最有效的推进方式不是继续盲目加页面，而是先把 DTO、分页、动作返回、终端接口这几类契约问题收口。

只要这些问题清理完成，当前前端已有页面就能较快进入可用状态。
