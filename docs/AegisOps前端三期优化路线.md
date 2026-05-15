# AegisOps 前端三期优化路线

## 1. 文档目标

本文档用于整理 AegisOps 在二期基础上的前端三期优化路线，目标不是继续铺更多菜单，而是把当前已经成形的控制台页面打磨成更稳定、更顺手、更像企业级工作台的产品体验。

本路线按 `P0 / P1 / P2` 拆分，并进一步细化到页面级、功能级，便于直接进入排期、拆分任务、安排联调。

## 2. 三期定位

三期前端的核心定位是：

1. 把已经做出来的页面从“可用首版”推进到“可联调、可演示、可连续操作”
2. 把服务定义、发布、任务、审计之间的链路补成真正的工作台闭环
3. 把一些会影响真实体验的问题前移处理，包括字段映射、错误态、状态保持、性能与工程化

优先级定义如下：

- `P0`：影响真实联调、演示稳定性、核心操作闭环的问题，优先做
- `P1`：提升企业级工作台效率和跨页联动能力的问题，跟着核心链路做增强
- `P2`：性能、工程化、个性化和长期维护性优化

## 3. 当前前端基础判断

当前前端已经具备以下基础：

- 路由、权限、会话、数据请求骨架已成形
- `Dashboard / Hosts / Docker / Registries / Services / Tasks / Audits / Users / Roles / Terminal` 页面已具备首版结构
- `Hosts / Docker / Registries / Services` 已明显进入“资源工作台”形态
- 二期新增的 `ServicesPage` 已经具备服务定义、版本、实例、发布记录的前端工作台骨架

因此，三期前端不是从零开始，而是在现有页面上做闭环强化、跨页联动和企业级体验收口。

## 4. P0：联调稳定性与核心闭环优先

### 4.1 DashboardPage

- 页面文件：`frontend/src/pages/dashboard/DashboardPage.tsx`
- 目标：让首页从“摘要展示”升级为可靠的总览入口
- 开发项：
  1. 修复最近审计列表里 `undefined · undefined` 之类的展示问题，对 `actor / resourceName / createdAt` 做兜底映射
  2. 让顶部统计卡支持点击跳转，至少打通到 `Hosts / Docker Nodes / Tasks / Audits`
  3. 最近任务列表补上更明确的资源语义，避免只有 `target` 文本而缺少对象感
  4. 最近审计项支持直接跳转到审计详情或全量审计页
  5. 告警提示从单条文案优化为可读的异常摘要，例如区分主机异常、节点异常、服务异常
- 前端依赖：
  - `dashboardApi.summary`
  - `StatusBadge`
  - `TaskStatus`
- 后端依赖：
  - `dashboard summary` 返回的最近任务、最近审计字段命名稳定
  - 异常资源统计最好能按资源类型拆分
- 验收口径：
  - 首页不再出现 `undefined` 拼接展示
  - 从首页可稳定跳转到具体工作页面
  - 用户能从最近任务/审计快速判断“谁、对什么、做了什么”

### 4.2 ServicesPage

- 页面文件：`frontend/src/pages/services/ServicesPage.tsx`
- 目标：把服务定义页做成三期最核心的发布工作台
- 开发项：
  1. 发布抽屉补上提交前摘要区，明确展示 `当前版本 / 目标版本 / 镜像 Tag / 目标节点`
  2. 发布、升级、回滚三种模式补上差异化表单校验，避免共享表单导致误填
  3. 服务定义表单增加端口、环境变量、挂载项的重复校验与非法值提示
  4. 版本列表和发布记录增加跳转能力，能进入对应任务详情
  5. 实例列表增加跳转入口，能回到对应节点或后续实例详情页
  6. 当服务存在进行中的发布任务时，在详情面板顶部给出明显状态提示，避免重复操作
  7. 详情面板内的最近任务、最近审计优先改成按 `resourceType/resourceId` 精准过滤，而不是仅靠文本包含
- 前端依赖：
  - `servicesApi.*`
  - `ResourceDetailPanel`
  - `ResourceActivityList`
  - `queryKeys.service*`
- 后端依赖：
  - 发布记录返回 `taskId`
  - 服务实例返回节点标识、实例状态、版本信息
  - 任务和审计支持按服务维度过滤
- 验收口径：
  - 能从一个服务详情页完成“查看定义 -> 发起发布 -> 看任务 -> 回看发布记录”
  - 用户在发起升级/回滚前能明确看到目标版本和影响对象

### 4.3 RegistriesPage

- 页面文件：`frontend/src/pages/registries/RegistriesPage.tsx`
- 目标：把 Registry 页从“浏览仓库”补强为“服务定义的上游入口”
- 开发项：
  1. 增加“基于当前 Registry 创建服务”的入口，并携带默认 `registryId`
  2. Repository / Tag / Manifest 区域补齐统一的 loading / empty / error 表达，减少错误信息散落
  3. Registry 测试结果保留在当前详情面板，不只弹消息，方便用户连续排障
  4. Manifest 区域补充更适合人读的摘要层，例如 `digest / contentType / layers 数量`
  5. 对认证失败、网络失败、空仓库这三类典型错误继续做前端分类收口
- 前端依赖：
  - `registriesApi.detail/repositories/tags/manifest/test`
  - `EmptyState`
  - `ResourceDetailPanel`
- 后端依赖：
  - 仓库、Tag、Manifest 接口的错误语义尽量稳定
  - 最好返回更结构化的 manifest 摘要字段
- 验收口径：
  - 用户能从当前 Registry 工作台直接走到服务定义创建
  - 失败信息能区分是认证、网络还是仓库内容问题

### 4.4 DockerNodesPage 与 DockerNodeDetailPage

- 页面文件：
  - `frontend/src/pages/docker/DockerNodesPage.tsx`
  - `frontend/src/pages/docker/DockerNodeDetailPage.tsx`
- 目标：把“节点 -> 容器 -> 动作 -> 结果”这条链路补稳
- 开发项：
  1. 节点详情页返回列表时保留 `selected` 状态，不丢失当前上下文
  2. 容器启停重启动作增加更明显的 pending 状态，避免重复点击
  3. 容器日志抽屉补充刷新入口和错误态
  4. 节点详情页增加与服务页的联动入口，例如查看关联服务实例
  5. 当节点离线时，弱化“查看容器”主操作，强化“测试连接”主操作
  6. 节点页最近任务、最近审计逐步改成精准过滤
- 前端依赖：
  - `dockerApi.listNodes/getNode/listContainers/runContainerAction`
  - `DangerConfirm`
  - `LogViewer`
- 后端依赖：
  - 容器动作接口最好返回 `taskId`
  - 容器日志接口需提供稳定错误返回
- 验收口径：
  - 用户能从节点页进入容器视图，再返回节点列表且不丢上下文
  - 容器动作后有明确反馈，不会出现重复操作的错觉

### 4.5 HostsPage 与 TerminalPage

- 页面文件：
  - `frontend/src/pages/assets/HostsPage.tsx`
  - `frontend/src/pages/terminal/TerminalPage.tsx`
- 目标：把主机接入、SSH 测试、终端会话这条链路做得更可靠
- 开发项：
  1. 主机详情面板补强测试任务反馈，不只提示“已提交”，而是明确显示最近一次测试状态
  2. 打开终端前增加会话创建中的状态反馈与失败兜底提示
  3. 终端页补强断线、重连、会话失效的状态表达
  4. 主机页上的主操作根据状态切换优先级，例如 `HEALTHY` 时主推打开终端，异常时主推 SSH 测试
  5. 最近任务、最近审计改为精确按主机资源过滤
- 前端依赖：
  - `hostsApi.testSsh`
  - `terminalApi.create/detail/wsUrl`
  - `ConnectionStatus`
  - `TerminalFrame`
- 后端依赖：
  - 终端会话详情返回更明确的会话状态
  - WebSocket 关闭原因最好可映射到前端展示
- 验收口径：
  - 用户能稳定从主机页发起 SSH 测试并看到当前结果
  - 终端断开时前端能给出清晰提示，而不是只有空白终端

### 4.6 TasksPage 与 TaskDetailPage

- 页面文件：
  - `frontend/src/pages/tasks/TasksPage.tsx`
  - `frontend/src/pages/tasks/TaskDetailPage.tsx`
- 目标：把任务中心从“列表”升级为“全局追踪中枢”
- 开发项：
  1. 任务列表增加状态筛选、任务类型筛选、资源筛选
  2. 任务详情页增加来源资源入口，例如返回对应服务、主机或节点页面
  3. 步骤列表显示更直观的时间和失败步骤高亮
  4. 已完成任务自动停止轮询，减少无效刷新
  5. 任务列表支持从服务页、主机页、节点页带条件跳转进入
- 前端依赖：
  - `tasksApi.list/detail`
  - `queryKeys.task`
  - `LogViewer`
- 后端依赖：
  - 任务模型返回 `resourceType / resourceId / taskType / status`
  - 任务步骤返回稳定的时间字段
- 验收口径：
  - 用户能在任务页快速筛出某个资源相关任务
  - 从任务详情能回到对应业务对象，不再断链

### 4.7 AuditsPage

- 页面文件：`frontend/src/pages/audits/AuditsPage.tsx`
- 目标：先把审计页做成能稳定排查问题的页面
- 开发项：
  1. 修复 `actor / resourceName / summary` 的空值展示，彻底消除 `undefined` 外露
  2. 增加结果筛选、资源类型筛选、关键字筛选
  3. 审计详情抽屉补上更清晰的结构化信息区，区分 `动作 / 资源 / 操作人 / traceId / 结果`
  4. 当审计项可关联任务或资源时，提供跳转入口
  5. 优化首页最近审计与全量审计的字段口径，避免同一条记录显示不一致
- 前端依赖：
  - `auditsApi.list`
  - `AuditDrawer`
  - `StatusBadge`
- 后端依赖：
  - 审计模型字段命名统一
  - 最好补 `resourceId` 与可跳转线索字段
- 验收口径：
  - 审计页不再出现 `undefined` 展示
  - 用户能稳定从审计记录回到业务上下文

### 4.8 LoginPage 与 SetupAdminPage

- 页面文件：
  - `frontend/src/pages/auth/LoginPage.tsx`
  - `frontend/src/pages/auth/SetupAdminPage.tsx`
- 目标：把认证入口从 mock 适配态推进到真实联调可用态
- 开发项：
  1. 登录成功后的 session 映射适配真实 token 返回结构
  2. 初始化管理员流程适配真实接口和真实 setup 状态判断
  3. 刷新后的会话恢复从 mock bootstrap 切到真实 `me/setup-status`
  4. 会话失效、401 跳登录、初始化未完成跳 setup 这些流程做成统一跳转规则
- 前端依赖：
  - `authApi.login/initAdmin/me/setupStatus`
  - `sessionStore`
- 后端依赖：
  - 登录接口、`auth/me`、`setup-status` 契约冻结
- 验收口径：
  - 前端切到真实 API 后，登录、刷新恢复、失效重定向逻辑一致

### 4.9 Shared：数据层、状态层、公共组件

- 涉及文件：
  - `frontend/src/lib/api.ts`
  - `frontend/src/lib/queryKeys.ts`
  - `frontend/src/store/sessionStore.ts`
  - `frontend/src/styles/global.css`
  - 公共组件目录
- 目标：先把多页面都会踩到的基础问题收掉
- 开发项：
  1. 统一列表接口适配层，收口分页结构差异
  2. 统一 loading / empty / error / permission 视图表达
  3. 统一页面 URL 状态保持策略，至少覆盖 `selected / keyword / filter`
  4. 统一动作提交后的任务反馈模式，减少有的跳页、有的弹消息、有的只改列表的割裂感
  5. 逐步移除页面里的临时文本过滤逻辑，改成结构化查询参数
- 后端依赖：
  - 列表分页 DTO 尽量统一
  - 常见动作接口尽量返回任务对象或 `taskId`
- 验收口径：
  - 多个页面的交互和反馈方式明显更一致
  - 前后端联调时不会因为响应结构不一致频繁返工

## 5. P1：企业级工作台体验增强

### 5.1 DashboardPage：驾驶舱化

- 页面文件：`frontend/src/pages/dashboard/DashboardPage.tsx`
- 开发项：
  1. 拆成“资源健康 / 发布动态 / 待处理异常 / 最近审计”四块
  2. 增加按资源类型分组的异常面板
  3. 增加到服务发布、任务中心、审计中心的快捷入口
  4. 将首页变成真实导航页，而不是仅展示数字
- 后端依赖：
  - dashboard summary 提供更细的异常分类与最近发布信息

### 5.2 ServicesPage：完整发布工作台

- 页面文件：`frontend/src/pages/services/ServicesPage.tsx`
- 开发项：
  1. 增加“当前版本 vs 目标版本”的差异摘要
  2. 增加发布前检查区，例如 Registry 连通性、节点状态、端口冲突风险
  3. 发布结果面板展示更完整的业务结果，而不只是任务号
  4. 历史版本支持对比与一键回滚入口
  5. 服务详情结构逐步收敛成 `概览 / 版本 / 实例 / 活动` 的稳定工作台布局
- 后端依赖：
  - 版本差异、预检查、发布记录元数据

### 5.3 RegistriesPage：从浏览到投递

- 页面文件：`frontend/src/pages/registries/RegistriesPage.tsx`
- 开发项：
  1. 从 Tag 直接进入“创建服务”或“发起升级”
  2. 为常用仓库/Tag 提供最近使用提示
  3. Manifest 摘要与服务页镜像字段联动
- 后端依赖：
  - 仓库、Tag 查询性能与稳定性

### 5.4 DockerNodesPage / DockerNodeDetailPage：资源到服务的 drilldown

- 页面文件：
  - `frontend/src/pages/docker/DockerNodesPage.tsx`
  - `frontend/src/pages/docker/DockerNodeDetailPage.tsx`
- 开发项：
  1. 节点详情里增加关联服务实例入口
  2. 容器列表与服务实例建立关系展示
  3. 容器动作与任务中心、审计中心贯通
- 后端依赖：
  - 容器到服务实例的关系标识

### 5.5 HostsPage / TerminalPage：排障工作台增强

- 页面文件：
  - `frontend/src/pages/assets/HostsPage.tsx`
  - `frontend/src/pages/terminal/TerminalPage.tsx`
- 开发项：
  1. 主机页补最近会话状态与最近登录信息
  2. 终端页增加右侧上下文信息，例如主机地址、凭证、会话创建时间
  3. 终端页支持更清晰的断线恢复提示
- 后端依赖：
  - 终端会话元数据与状态流转字段

### 5.6 TasksPage / AuditsPage：全链路追踪联动

- 页面文件：
  - `frontend/src/pages/tasks/TasksPage.tsx`
  - `frontend/src/pages/tasks/TaskDetailPage.tsx`
  - `frontend/src/pages/audits/AuditsPage.tsx`
- 开发项：
  1. 任务详情与审计详情互相跳转
  2. 从资源页带条件进入任务/审计页时保留筛选上下文
  3. 增加时间线式展示，强化“动作发生顺序”理解
- 后端依赖：
  - 任务、审计共享 trace 维度或业务关联字段

### 5.7 SecretsPage / RolesPage / UsersPage：系统管理补强

- 页面文件：
  - `frontend/src/pages/secrets/SecretsPage.tsx`
  - `frontend/src/pages/system/RolesPage.tsx`
  - `frontend/src/pages/system/UsersPage.tsx`
- 开发项：
  1. 凭证页增加引用影响面视图，明确“被哪些主机 / Registry / 节点使用”
  2. 角色页增加权限模板或权限分组优化，降低配置成本
  3. 用户页补角色视图与状态反馈一致性
- 后端依赖：
  - 凭证引用关系、用户角色关系接口更完整

## 6. P2：性能、工程化与长期维护性

### 6.1 路由级拆包与 chunk 优化

- 涉及页面：全站，优先 `Terminal / DockerNodeDetail / Services / Registries`
- 开发项：
  1. 按页面做 `lazy + Suspense` 拆包
  2. 将 `xterm.js`、日志查看等重资源模块延迟加载
  3. 分析 `vendor / antd / xterm / 页面业务` 的 chunk 边界
  4. 针对 Vite 的大包告警做页面级和依赖级拆分复盘
- 验收口径：
  - 首屏加载更轻
  - 重页面不再拖累登录后首个渲染

### 6.2 表单与工作台组件标准化

- 涉及页面：`Services / Registries / Hosts / Docker / Secrets / Roles`
- 开发项：
  1. 抽取统一的资源详情面板 schema
  2. 抽取动态表单段能力，例如端口、环境变量、挂载
  3. 沉淀统一的危险操作确认组件用法
- 验收口径：
  - 新页面开发不再重复造大量表单和详情块

### 6.3 数据层与契约层工程化

- 涉及文件：`frontend/src/lib/api.ts` 及相关 hooks
- 开发项：
  1. 统一 query key 组织方式
  2. 统一 DTO 映射层，减少页面直接适配后端原始字段
  3. 逐步引入前端契约测试或 mock 样例校验
- 验收口径：
  - 后端字段轻微变化不会把页面改动扩散到很多地方

### 6.4 工作台状态持久化与深链接

- 涉及页面：`Hosts / Docker / Registries / Services / Tasks / Audits`
- 开发项：
  1. 持久化选中项、筛选条件、排序方式
  2. 支持复制当前工作台链接给其他成员
  3. 保留从首页或其他页面 drilldown 进入后的上下文
- 验收口径：
  - 刷新、返回、分享链接后都能回到接近原来的工作位置

### 6.5 角色化首页与个性化工作台

- 涉及页面：`Dashboard` 及全站导航
- 开发项：
  1. 不同角色展示不同快捷入口与重点卡片
  2. 支持更偏运维或更偏交付的首页视图
- 验收口径：
  - 首页更像工作入口，而不是统一模板

## 7. 推荐实施顺序

建议三期前端按以下顺序推进：

1. `P0-认证与数据层收口`
   - 登录态、分页适配、字段兜底、任务/审计结构统一
2. `P0-核心工作台收口`
   - Dashboard、Services、Tasks、Audits
3. `P0-资源页收口`
   - Hosts、Docker、Registries、Terminal
4. `P1-发布闭环增强`
   - 服务发布工作台、跨页 drilldown、任务审计联动
5. `P2-性能与工程化`
   - 按页拆包、组件标准化、状态持久化

## 8. 最终建议

如果三期只抓一条主线，建议优先围绕 `ServicesPage -> TasksPage -> AuditsPage -> DashboardPage` 这条链路推进，因为它最接近 AegisOps 当前产品价值的核心：

- 服务定义是发布入口
- 任务中心是执行追踪入口
- 审计日志是回溯入口
- Dashboard 是全局总览入口

把这四块做扎实，再去做更大范围的页面增强，前端三期会更稳，也更容易和后端形成真正的联调闭环。
