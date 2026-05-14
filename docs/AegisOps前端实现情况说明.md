# AegisOps 前端实现情况说明

## 1. 文档目的

本文档用于说明当前仓库内 AegisOps 一期 MVP 前端的实际实现状态，区分：

- 已完成的前端基础设施与页面代码
- 当前运行时真正生效的前端入口
- 已预留但尚未完成真实后端接入的能力
- 下一步推进前端可交付版本时的重点事项

本文档基于仓库当前代码现状整理，而不是仅依据路线规划。

## 2. 当前总体状态

当前前端已经完成了较完整的控制台骨架与页面层开发，包含：

- React + Vite + TypeScript 的应用基础结构
- Ant Design 组件体系与统一主题
- React Query 数据请求层
- Zustand 会话状态管理
- mock 数据服务
- 多个业务页面与公共组件

但当前运行时入口仍停留在占位页。

也就是说，虽然前端代码中已经存在登录、工作台、资产、Docker、任务、审计、系统管理等页面实现，但 `frontend/src/app/router.tsx` 当前只注册了一个 `/` 路由，并渲染如下占位信息：

- 标题：`AegisOps console scaffold is ready`
- 副标题：`The frontend foundation is in place and can now be extended with real pages and API wiring.`
- 会话提示：`Cached session found` / `No active session`

因此，当前浏览器里看到的 `No active session` 不是后端错误，也不是已完成业务页面的最终表现，而是占位路由仍在生效的结果。

## 3. 已实现的前端目录与能力

### 3.1 应用基础层

已具备以下应用级能力：

- `frontend/src/app/providers.tsx`
  - 注入 Ant Design `ConfigProvider`
  - 注入 React Query `QueryClientProvider`
  - 配置中文 locale 与统一主题
- `frontend/src/app/queryClient.ts`
  - React Query 客户端
- `frontend/src/app/navigation.tsx`
  - 左侧导航定义
  - 权限过滤逻辑
  - 默认落点计算
- `frontend/src/layouts/AppShell.tsx`
  - 控制台壳层
  - 侧边导航
  - 顶栏、面包屑、用户菜单

### 3.2 会话与请求基础设施

已实现：

- `frontend/src/store/sessionStore.ts`
  - token、本地存储、用户信息、权限集合
  - `setSession` / `clearSession`
  - 应用启动时的 bootstrap 逻辑
- `frontend/src/lib/http.ts`
  - 统一 `fetch` 封装
  - 自动注入 `Authorization: Bearer <token>`
  - 401 自动清理本地登录态
- `frontend/src/lib/config.ts`
  - `VITE_API_BASE_URL`，默认 `/api`
  - `VITE_USE_MOCK`，默认不是 `"false"` 时即走 mock
- `frontend/src/lib/forms.ts`
  - 表单错误注入
  - 错误消息提取

### 3.3 mock 服务

`frontend/src/mocks/service.ts` 已经实现了一套较完整的本地演示逻辑，覆盖：

- 管理员初始化
- 登录 / 登出 / 当前用户
- 工作台摘要
- 用户 / 角色
- 凭证 / 主机
- SSH 测试任务
- Web 终端会话
- Docker 节点 / 容器 / 日志 / 动作
- 任务中心
- 审计日志

这意味着当前前端在 mock 模式下已经具备较强的可演示性。

## 4. 页面实现情况

### 4.1 已有页面代码

当前仓库中已经实现了以下页面：

- 认证
  - `frontend/src/pages/auth/LoginPage.tsx`
  - `frontend/src/pages/auth/SetupAdminPage.tsx`
- 工作台
  - `frontend/src/pages/dashboard/DashboardPage.tsx`
- 资产管理
  - `frontend/src/pages/assets/HostsPage.tsx`
  - `frontend/src/pages/secrets/SecretsPage.tsx`
- Docker
  - `frontend/src/pages/docker/DockerNodesPage.tsx`
  - `frontend/src/pages/docker/DockerNodeDetailPage.tsx`
- 任务
  - `frontend/src/pages/tasks/TasksPage.tsx`
  - `frontend/src/pages/tasks/TaskDetailPage.tsx`
- 审计
  - `frontend/src/pages/audits/AuditsPage.tsx`
- 系统管理
  - `frontend/src/pages/system/UsersPage.tsx`
  - `frontend/src/pages/system/RolesPage.tsx`
- 终端
  - `frontend/src/pages/terminal/TerminalPage.tsx`
- 异常页
  - `frontend/src/pages/errors/ForbiddenPage.tsx`
  - `frontend/src/pages/errors/NotFoundPage.tsx`

### 4.2 页面侧公共组件

已经沉淀了可复用的前端组件：

- `AuditDrawer`
- `ConnectionStatus`
- `DangerConfirm`
- `DataTable`
- `EmptyState`
- `ErrorState`
- `FormDrawer`
- `LogViewer`
- `PageHeader`
- `PermissionGuard`
- `SecretInput`
- `StatusBadge`
- `TaskStatus`
- `TerminalFrame`

这些组件说明前端并不只是做了静态页面，而是已经进入“可交互业务控制台”的阶段。

## 5. 当前真实生效状态

这里需要特别说明当前运行时和代码实现之间的差异。

### 5.1 当前真正对外生效的只有占位路由

`frontend/src/app/router.tsx` 当前仅注册：

- `/` -> `HomePage`

且 `HomePage` 只是 scaffold 占位页。

因此：

- 登录页当前并不会自动对外可访问
- 工作台、主机、凭证、Docker、任务、审计、用户、角色等页面当前也没有被接入活动路由
- `AppShell`、权限导航、面包屑等能力虽然已经写好，但目前并未真正挂载到运行中的路由树

### 5.2 启动 bootstrap 逻辑当前仍依赖 mock

`frontend/src/app/App.tsx` 会调用 `useBootstrapSession()`。

但 `useBootstrapSession()` 当前在 `frontend/src/store/sessionStore.ts` 中直接调用的是：

- `mockService.getSetupStatus()`
- `mockService.me(token)`

这意味着即使部分 API 已经切换到真实后端：

- 应用首次启动时的初始化判断仍是 mock 逻辑
- 刷新页面后的登录态恢复仍是 mock 逻辑

这会造成一种“登录能调真接口，但刷新后行为不像真接口”的混合状态。它是下一步前端接入真实后端时必须优先处理的问题之一。

## 6. 已完成的前端 API 抽象

`frontend/src/lib/api.ts` 已经按业务模块整理了 API 调用入口，包含：

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

从模块划分看，一期 MVP 前端的交互范围已经明确，具备继续真实联调的结构基础。

## 7. 当前存在的主要问题与风险

### 7.1 路由未切换到真实业务页面

这是当前最明显的现状问题。

用户看到的仍是 scaffold 提示页，而不是业务控制台。这会让外部观感误以为“前端还没开始”，但实际上页面层代码已经具备一定完整度。

### 7.2 mock 模式默认开启

`frontend/src/lib/config.ts` 中：

- `API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api"`
- `USE_MOCK = import.meta.env.VITE_USE_MOCK !== "false"`

这意味着除非显式设置 `VITE_USE_MOCK=false`，否则默认走 mock。

如果没有明确说明运行方式，联调时很容易出现“前端看起来正常，但其实没有请求后端”的误判。

### 7.3 前端接口预期与后端当前响应结构存在差异

当前前端 API 设计明显更偏向“直接返回数组或业务对象”，例如：

- `usersApi.list(): Promise<User[]>`
- `rolesApi.list(): Promise<Role[]>`
- `secretsApi.list(): Promise<Secret[]>`
- `hostsApi.list(): Promise<Host[]>`
- `tasksApi.list(): Promise<Task[]>`
- `auditsApi.list(): Promise<AuditLog[]>`

但后端当前多个列表接口实际返回的是分页包裹结构，如：

- `{ items, total, page/pageSize }`
- 或 `{ items, total, limit, offset }`

这意味着在关闭 mock 后，前端列表页大概率不能直接工作，需要统一分页响应契约或在前端做转换适配。

### 7.4 登录态模型与后端认证返回结构存在差异

当前前端 `AuthSession` 期望结构是：

- `token`
- `user`
- `permissions`

但后端登录接口当前返回的是：

- `user`
- `tokens.accessToken`
- `tokens.refreshToken`
- `tokens.accessTokenExpiresAt`
- `tokens.refreshTokenExpiresAt`
- `tokens.tokenType`

同时，前端 `http.ts` 只会从 `sessionStore.token` 中取单一 token 注入请求头。

因此登录成功后的 session 结构也需要前后端统一，或者在前端接入层做一次映射。

### 7.5 权限字段结构也未完全统一

当前前端权限系统使用字符串数组，例如：

- `dashboard.view`
- `users.manage`
- `docker.view`

而后端当前 `auth/me` 返回的是用户模型，用户角色里再 preload 了权限对象，并不是前端当前直接消费的 `string[] permissions` 结构。

权限模型是否由后端直接扁平化返回，仍需明确。

### 7.6 部分前端预留能力尚未找到后端同名接口

前端已预留但当前后端代码中未直接确认到完整同名接口的能力包括：

- `GET /auth/setup-status`
- `POST /auth/setup`
- `GET /dashboard/summary`
- `POST /hosts/:id/terminal/sessions`
- `GET /terminal/sessions/:id`

这部分仍需后端补齐或前端改为对接新的实际接口。

## 8. 可以认定为“已完成”的前端成果

如果按“前端侧代码资产是否已形成”来判断，一期 MVP 前端目前可以认定已经完成了以下部分：

- 信息架构与导航结构
- 管理控制台基础壳层
- 多数核心页面的首版交互实现
- mock 数据流与演示闭环
- 权限感知的导航与页面控制
- 通用表格、抽屉、状态展示、日志展示等可复用组件

也就是说，前端当前不是从零开始，而是已经具备“切换真实路由并逐步联调”的条件。

## 9. 下一步建议

建议按下面顺序推进：

1. 先切换真实路由
2. 统一登录态模型与 `auth/me` 返回结构
3. 统一列表接口的分页响应格式
4. 优先打通登录、当前用户、用户、角色、审计这几类后端已确认存在的接口
5. 再推进主机、凭证、任务、Docker、终端等模块的逐项联调
6. 最后再处理工作台聚合接口与体验收口

## 10. UI 亮点与优化方向

### 10.1 当前前端 UI 与功能设计的亮点

从企业级控制台的角度看，当前前端已经有几个不错的基础亮点：

1. 页面结构比较克制，接近运维控制台而不是营销页
   - `PageHeader + Card + DataTable + FormDrawer` 的组合比较统一
   - 页面信息密度适中，便于继续往“高频操作后台”方向演进

2. 关键操作按钮离数据对象很近
   - 主机页已经把 `SSH 测试`、`打开终端`、`编辑` 放在同一行
   - Docker 节点页已经把 `查看容器`、`测试连接`、`编辑` 放在同一行
   - 这种“就地操作”方式符合企业级应用对效率的要求

3. 已经有明显的“动作 -> 任务 -> 追踪”闭环
   - 主机页和 Docker 页的检测动作会跳转到任务详情
   - 任务详情页又提供步骤、状态、日志三层反馈
   - 这比单纯弹一个成功提示更像真实运维场景

4. 权限驱动导航已经具备企业后台雏形
   - 导航菜单可按权限裁剪
   - 页面内按钮也通过 `PermissionGuard` 做了收口
   - 这为后续做角色差异化操作界面打下了基础

5. 运维感较强的专用界面已经出现
   - `TaskDetailPage` 的步骤与日志面板
   - `TerminalPage` 的连接状态与终端框体
   - 这些页面已经有“控制台产品”而不是“普通 CRUD 后台”的味道

### 10.2 建议采用的一个企业级优化方向

建议把 `主机页` 和 `Docker 节点页` 优先升级为：

- `主列表 + 右侧资源详情工作台` 的企业级控制台模式

这类模式在云控制台、SRE 平台、资产运维平台中很常见。它的核心价值是让用户在不频繁跳页的情况下完成：

- 看状态
- 看上下文
- 发起动作
- 跟踪结果

#### 为什么这个方向最值得先做

当前页面虽然已经有按钮和数据表，但资源上下文仍然比较分散：

- 想看主机信息，要在列表里找
- 想测 SSH，要点按钮后跳任务详情
- 想开终端，又要进入另一页
- 想确认刚才动作的影响，还要再切去任务中心

这会导致操作链路被切碎。

对于企业级控制台来说，更理想的体验是：

- 左侧保持资源列表
- 右侧固定展示当前选中资源的详情与动作区
- 动作执行后，结果仍在当前上下文里反馈

#### 建议的首批落地页面

建议先做两页试点：

1. `frontend/src/pages/assets/HostsPage.tsx`
2. `frontend/src/pages/docker/DockerNodesPage.tsx`

因为这两页最符合“查看资源 -> 发起动作 -> 观察结果”的企业级运维路径。

#### 建议的界面形态

在不推翻现有页面结构的前提下，可演进为：

1. 左侧仍是 `DataTable` 主列表
2. 点击某一行后，在右侧打开固定宽度详情面板，或桌面端双栏布局
3. 详情面板顶部显示：
   - 资源名称
   - 当前状态
   - 最近检测时间
   - 关键属性摘要
4. 详情面板中部显示：
   - 标签 / 说明 / 绑定关系
   - 最近任务
   - 最近审计
5. 详情面板底部或顶部操作条显示：
   - 主机页：`SSH 测试`、`打开终端`、`编辑`
   - Docker 页：`测试连接`、`查看容器`、`编辑`

#### 建议优先补充的页面功能

这个方向不是只改视觉，而是同步增强页面功能：

1. 资源详情就地查看
   - 主机：地址、端口、绑定凭证、标签、最近检测结果
   - Docker 节点：Endpoint、TLS、容器数、最近检测结果

2. 动作结果就地反馈
   - 当前点击 `SSH 测试` 或 `测试连接` 后会直接跳任务详情
   - 优化后建议先留在当前页，在详情面板内显示“任务已提交 + 当前状态 + 进入详情”入口

3. 最近活动回看
   - 在资源详情面板内展示最近 3~5 条相关任务或审计
   - 用户不用离开当前资源页就能判断“这个资源刚刚发生了什么”

4. 状态驱动按钮可用性
   - 例如主机状态为 `TESTING` 时，临时禁用重复测试按钮
   - Docker 节点状态为 `OFFLINE` 时，弱化 `查看容器` 的主操作优先级

#### 前端落地方式

这项优化可以按两期拆分，风险不高。

第一期只做前端即可启动：

1. 在主机页和 Docker 页新增 `selectedResource` 状态
2. 抽一个公共组件，例如 `ResourceDetailPanel`
3. 复用现有组件：
   - `StatusBadge`
   - `ConnectionStatus`
   - `TaskStatus`
   - `LogViewer`
   - `FormDrawer`
4. 列表行点击后打开详情面板，不改变现有新增/编辑抽屉逻辑
5. 动作提交后先在详情面板显示任务卡片，再保留“查看详情”跳转

第二期再做体验增强：

1. 列表行高亮当前选中资源
2. 面板中加入最近任务与最近审计
3. 对不同状态显示不同主按钮
4. 为高风险动作补充统一确认组件

#### 对后端配合的最小诉求

为了让这个方向更完整，后端最好逐步补充以下能力：

1. 资源关联任务过滤
   - 例如 `GET /tasks?target=...` 或 `GET /tasks?resourceType=host&resourceId=...`

2. 资源关联审计过滤
   - 例如 `GET /audits?resourceType=host&resourceId=...`

3. 动作返回统一任务对象或任务 ID
   - 这样前端可以在详情面板里稳定展示执行进度

#### 验收标准

如果这个优化方向落地成功，至少应满足：

1. 用户在主机页可以不离开页面完成：
   - 查看资源信息
   - 发起 SSH 测试
   - 查看最新任务状态
   - 决定是否进入终端

2. 用户在 Docker 节点页可以不离开页面完成：
   - 查看节点状态
   - 发起连通性检测
   - 查看最近容器上下文
   - 决定是否进入容器视图

3. 任务中心仍保留为全局追踪页，但不再承担所有即时反馈职责

这个方向的价值在于，它不会推翻现有前端结构，却能明显提升：

- 页面专业感
- 操作效率
- 状态可见性
- 企业级控制台的一致性

## 11. 结论

当前 AegisOps 一期 MVP 前端的真实状态可以概括为：

- 页面和组件层并不空白，已经有较完整的控制台实现
- 当前对外运行的仍是占位路由，不代表前端页面层未完成
- mock 演示链路已经成形
- 真实后端接入仍处于“结构已准备好，但接口契约尚未完全对齐”的阶段

因此，下一阶段的重点不再是从零搭 UI，而是：

- 打开真实路由
- 消除 mock / api 混用
- 对齐前后端接口契约
- 逐模块完成联调收口
