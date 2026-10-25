# AegisOps 企业级增强设计：SSO 登录、变更审批与资源范围权限

> 2026-10-26 · 基线：v1.7.0（含 9c504f8 会话语义修复、5d1fca5 探活修复）
> 定位：回答三个企业采用门槛级能力的"为什么做、做成什么样、改哪里、有什么坑"。
> 本文是设计文档，不是排期承诺；实施顺序建议见 §4。

---

## 0. 总览：为什么是这三件事

对比企业级平台的差距分析结论（2026-10-26 讨论）：AegisOps 缺的不是功能数量，而是"让组织放心把生产环境交给你"的那一层。三件事分别卡住三类采用者：

| 能力 | 卡住谁 | 一句话目标 |
|---|---|---|
| OIDC/OAuth2 SSO | 有统一身份的组织 | 用户用公司账号登录，管理员不为每个系统维护独立密码 |
| 变更审批（双人确认） | 有合规要求的团队 | 高危操作"有权限"不再等于"能执行"，需要第二人确认 |
| 资源范围权限 | 多人/多团队共用一套平台 | "有 hosts.view"不再等于"能看所有主机"，按范围隔离 |

共同原则（贯穿三个设计）：

1. **不破坏轻量自托管**：全部能力可关闭、可降级为现状（SSO 可选、审批可跳过模式、范围默认 ALL）
2. **向后兼容**：已有用户/角色/审计数据不迁移语义，老接口行为不变
3. **每一步都留审计**：三个能力本身就产生新的审计事件类型

---

## 1. OIDC/OAuth2 SSO 登录

### 1.1 背景

现状登录只有本地用户名密码（bcrypt）。组织里通常已有统一身份（Keycloak、Okta、Azure AD、企业微信/飞书扫码、Gitea/GitLab OAuth）。每个内部系统一套独立账号意味着：管理员要手动建号、改密、禁用离职人员；用户要记多套密码；密码策略与过期策略无法统一。这是企业采用的第一道门槛——很多团队在试点阶段就因为"不能接我们的 SSO"而放弃。

### 1.2 需求

- 管理员可配置一个或多个 OIDC Provider（issuer、client_id、client_secret、scopes），启用/停用
- 登录页出现"使用企业账号登录"入口，走 Authorization Code + PKCE 流程
- 首次 SSO 登录自动建立本地账号（provisioning），并可按 claim 映射初始角色
- SSO 用户不持有本地密码：改密/首启强制改密逻辑对其不生效
- 本地账号登录作为兜底保留（管理员账号、SSO 系统故障时可用）
- 本地管理员可随时禁用某个 SSO 同步来的账号

验收标准：

- [ ] 配置好 Provider 后，新用户点击 SSO 登录→授权→自动建号并进入控制台
- [ ] claim 中的 group/role 能映射为平台角色
- [ ] SSO 用户登录后 `mustChangePassword` 不触发
- [ ] Provider 故障时本地登录不受影响
- [ ] 全程审计：`auth.oidc_login` 成功/失败记录

### 1.3 为什么这么做（选型理由）

- **OIDC 而不是 LDAP**：LDAP 是目录协议，只解决"验证密码"，不解决"浏览器跳转授权/单点登出/token"；OIDC 覆盖登录全流程且是浏览器友好的事实标准。LDAP 可作为后续补充（密码验证源），不做首选
- **OIDC 而不是 SAML**：SAML 配置重、XML 签名坑多，互联网企业基本都支持 OIDC；传统企业 SAML 可以后补（库都现成）
- **Authorization Code + PKCE**：平台是机密客户端跑在服务端，Code 流最安全；PKCE 对反代场景多一层防护
- **保留本地账号兜底**：SSO 系统是外部依赖，用它登录管理平台不能把自己锁死在门外——管理员本地账号必须永远可用，这条要写进文档

### 1.4 要做成什么样子（目标形态）

```
登录页
├─ 用户名/密码（现状保留）
└─ [使用企业账号登录] → GET /api/auth/oidc/:provider/start
     ├─ 后端生成 state + PKCE verifier（短期存内存/DB），302 → Provider 授权页
     └─ GET /api/auth/oidc/:provider/callback?code&state
          ├─ 校验 state，用 code 换 token，取 ID Token / userinfo
          ├─ 按配置的 claim 取身份（sub/email/preferred_username）
          ├─ provisioning：存在则更新 last_login；不存在则建号 + 角色映射
          ├─ 签发平台自己的 JWT（与本地登录同一套 token 体系）
          └─ 302 回前端并携带 token（一次性 code 交换，防 referrer 泄漏）
```

配置面：`configs/config.yaml` 增加 `auth.oidc.providers[]`（issuer/client_id/client_secret/redirect/scopes/role_mapping/claim 映射），支持 `AEGISOPS_AUTH_OIDC_` 环境变量覆盖；管理员页面可看状态与启停。

### 1.5 现状盘点（有什么、在哪）

| 现有资产 | 位置 | 对 SSO 的意义 |
|---|---|---|
| 用户模型含 Email/IsAdmin/MustChangePassword/Status | `internal/model/auth.go:12`（User） | provisioning 可直接建号；`PasswordHash` 对 SSO 用户置空需处理 |
| JWT 签发/校验体系（access/refresh 双 token，`ParseToken` 带 tokenType） | `internal/auth/service.go` | SSO 回调后签发**平台自己的 JWT**，下游中间件零改动 |
| 登录限流 LoginGuard | `internal/auth`（5 次/15 分钟） | SSO 回调是公开端点，同样要防滥用 |
| 强制改密中间件 `RequirePasswordChangeCleared` | `internal/auth`（豁免名单机制已有） | SSO 用户需要豁免路径（见 1.7-4） |
| 前端登录页 + `useSessionStore` token 管理 | `frontend/src/pages/auth/LoginPage.tsx`、`store/sessionStore.ts` | 登录页加 SSO 按钮；回调页复用 setSession |
| 审计服务 `audit.RecordGin` | `internal/audit` | 新增 `auth.oidc_login` 事件 |

**目前不存在**：任何 OIDC/OAuth 代码（全仓 grep 为零）、外部身份概念、provisioning 逻辑。

### 1.6 要扩展哪里

1. `internal/auth/oidc.go`（新文件）：Provider 配置解析、发现文档拉取（`.well-known/openid-configuration`）、state/PKCE 管理、code 交换、claim 提取、provisioning
2. `internal/auth/service.go`：`Login` 旁路增加 `OIDCLogin(identity)`；`User` 增加字段 `AuthSource`（local/oidc:provider-id）、`ExternalSubject`（唯一索引）
3. `internal/handler/auth.go` + 路由：`/auth/oidc/:provider/start|callback` 两个**免鉴权**端点（走 LoginGuard 语义的防滥用）
4. `internal/config`：OIDC 配置段；`configs/config.yaml` 示例
5. 迁移：`0003_user_auth_source.go`（新列，走 1.7.0 建立的迁移纪律）
6. 前端：登录页 SSO 按钮（读后端下发的 provider 列表）、`/auth/oidc/callback` 前端承接页（换 token、写 sessionStore、跳转）

### 1.7 隐藏问题与对策

1. **回调如何把 token 交给前端**：SPA 场景不能把 JWT 放 URL hash 之外的地方（会进 referrer/历史记录）。→ 对策：回调端点签发**一次性短时 code**（60 秒，单次使用），前端用它换正式 token；code 存 DB 并在消费后删除
2. **state/PKCE 存哪**：多实例下不能存进程内存。→ 对策：与迁移同事务存 DB 表（或复用签名 cookie，HMAC 加时间戳）；首选 DB 表，为多实例铺路
3. **账号接管（account linking）**：若按 email 匹配本地已有账号，攻击者用同 email 的恶意 Provider 可接管。→ 对策：email 匹配默认**关闭**，仅 `ExternalSubject` 精确匹配；如需 email 合并，做成显式管理员操作并双人审计
4. **MustChangePassword 对 SSO 用户无意义**：本地密码概念不存在。→ 对策：`AuthSource != local` 的用户跳过强制改密判定（中间件里按 source 分支）
5. **PasswordHash 非空约束**：模型 `not null`。→ 对策：SSO 用户写入随机不可用哈希，或在 0003 迁移里放开为 nullable（推荐前者，少改约束）
6. **时钟偏移**：ID Token 校验默认允许 ±2 分钟偏移，可配置
7. **登出联动**：平台登出只清本地会话，Provider 侧 SSO 仍在（RP-initiated logout 可选实现，一期不做，文档注明）
8. **限流语义**：LoginGuard 按"用户名+IP"计数，SSO 流程没有用户名。→ 对策：回调端点按 IP 维度限流即可
9. **前端 state 传递丢刷新**：回调页刷新会把一次性 code 用掉。→ 对策：消费失败给可重试的明确报错页

### 1.8 分期建议

- 一期：单 Provider、Code+PKCE、自动 provisioning、角色静态映射、本地兜底
- 二期：多 Provider、按 claim 动态映射角色、SSO 会话管理面板（看哪些人是 SSO 来的）、SCIM/定时同步（禁用离职账号）

---

## 2. 变更审批（双人确认）

### 2.1 背景

现状高危操作的唯一门槛是 RBAC 权限点：有 `services.release` 就能直接发布，有 `secrets.rotate` 就能直接换密钥。在单人场景这完全够用；在多人场景里，"有权限的人失误"与"有权限的人误用"都没有第二道闸。企业合规（无论 ISO27001 还是等保）普遍要求生产变更留有第二人确认的痕迹。

### 2.2 需求

- 可配置的"高危操作清单"启用双人确认：建议首批覆盖——服务发布/升级/回滚、Nginx 配置发布与 reload、Secret 轮换、非脱敏导出/备份
- 发起人提交后操作进入"待审批"，由**另一名**具备对应权限的用户批准后才执行
- 审批人不能是发起人本人（自批自审是这类系统的第一大坑）
- 待审批有 TTL（默认 30 分钟），过期自动作废
- 全链审计：submitted / approved / rejected / expired 四类事件，含双方身份
- 功能可全局关闭（单人模式默认关），关闭时行为与现状完全一致

验收标准：

- [ ] 开启后，发布操作返回"待审批"而不是直接执行；第二人批准后任务才产生
- [ ] 发起人自己批不了自己
- [ ] 过期未批的操作作废并有审计记录
- [ ] 关闭开关后行为与现状逐字节一致

### 2.3 为什么"双人确认"而不是完整审批流

完整审批流（多级、条件路由、委托代理、SLA 提醒）是一个独立产品的体量，且需要通知、组织架构等前置能力。双人确认是 80/20：实现面小（一个模型 + 一层拦截），合规价值立竿见影（审计里有第二人），并且是审批流的合法子集——将来升级审批流时"单级审批"就是它的退化形态，不会推翻重来。

### 2.4 要做成什么样子（目标形态）

```
发起人 POST /api/services/:id/releases（带 X-Confirm-Intent 或普通请求）
  └─ 命中高危清单且审批开启：
       1. 校验请求合法（参数验证先做，失败直接 400，不进审批）
       2. 创建 ApprovalRequest{operation, payload_snapshot, requester_id, ttl}
       3. 返回 202 {approvalId, status:"pending"}（不是 201）
审批人 GET /api/approvals?status=pending → POST /api/approvals/:id/approve|reject
  └─ approve：按 payload_snapshot 原样重放原操作（服务层函数级重放，非 HTTP 重放）
       产生任务后 ApprovalRequest 关联 task_id，状态 → executed
前端：发起人看到"待审批"状态卡；审批人在工作台/告警中心看到待办入口
```

### 2.5 现状盘点（有什么、在哪）

| 现有资产 | 位置 | 对审批的意义 |
|---|---|---|
| 高危操作全部收敛为"创建任务"（发布/回滚/Nginx 发布都走 `taskService`） | `internal/service`、`internal/nginx`（enqueueChange 等） | **拦截点天然存在**：在"创建任务"前插一层即可，审批通过=放行建任务 |
| `TaskDispatch.Source` 枚举已有 `MANUAL/SYSTEM/SCHEDULED`（SYSTEM 未用） | `internal/model/scheduler.go:12` | 审批执行的任务可标注来源，前端能区分 |
| 审计服务与 AuditLog（action/resourceType/result/message） | `internal/audit`、`internal/model/audit.go` | 四类审批事件直接落现有表 |
| 用户/权限体系（判断"谁能批"= 具有同一权限点的其他用户） | `internal/rbac` | 审批人资格判定复用 `HasPermission` |
| 真实通知通道 | 尚未实现（v1.8.0 计划） | "通知审批人"依赖它——审批先做轮询待办，通知后补 |

**目前不存在**：任何审批/确认模型；`202` 语义；待办入口。

### 2.6 要扩展哪里

1. `internal/model/approval.go`（新）：`ApprovalRequest{ID, Operation, ResourceType, ResourceID, PayloadJSON, RequesterID, ApproverID, Status(pending/approved/rejected/expired/executed), TaskID, ExpiresAt, ...}`——迁移 `0004_approvals.go`
2. `internal/approval/`（新包）：创建/批准/拒绝/过期扫描（复用 scheduler 的定时机制或 worker tick）、重放分发器（operation → 原服务函数的注册表）
3. 服务层拦截：高危操作入口处插 `approval.Gate(ctx, operation, payload)`，命中则返回 `ErrApprovalRequired`；handler 把它映射为 202
4. 配置：`approval.enabled` + 高危清单（默认全关）
5. 前端：`http.ts` 识别 202 语义 → 发起人视角"待审批"卡片；新增待审批列表页/工作台入口；审批操作（批准/拒绝带备注）
6. 审计：四种事件的 action 命名 `approval.submitted/approved/rejected/expired`

### 2.7 隐藏问题与对策

1. **自批自审**：必须服务端判定 `approver_id != requester_id`，前端隐藏只是体验。→ 对策：approve 端点硬校验，违例记审计（这是攻击信号）
2. **payload 篡改与重放一致性**：批准的是"快照"，批准后原请求参数必须按快照重放，不能让批准后实际执行的内容和审批时看到的不同。→ 对策：审批时存 payload 快照，执行用快照；服务层函数级重放而非拼接原 HTTP 请求
3. **审批人权限与发起人不同**：A 有发布权限、B 只有查看权限，B 能不能批？→ 对策：B 必须也持有同一操作权限点才能批准（在 approve 时再验一次 `HasPermission`，因为权限可能在等待期间被收回）
4. **TTL 过期的竞态**：过期扫描与批准并发。→ 对策：approve 在事务内带条件更新 `WHERE status='pending' AND expires_at > now()`，RowsAffected=0 即返回"已过期"（与任务抢占同一模式，v1.6.0 已有成熟先例）
5. **执行失败的归属**：批准了但发布失败，审批记录不能显示"成功"。→ 对策：`executed` 状态只表示"已放行"，最终结果看关联 task 的状态；任务详情页反向链到审批记录
6. **通知审批人缺前置**：没有真实通知通道时审批人不知道有待办。→ 对策：一期做工作台待办卡片 + 页面轮询；通知通道上线后接"审批请求"事件类型
7. **绕过路径**：非脱敏导出、Secret rotate 这些不走任务中心的操作也要覆盖，否则"审批高危清单"有名无实。→ 对策：拦截点放在服务层函数（所有入口必经），而不是 HTTP 中间件（会漏内部调用）
8. **与首启改密门禁叠加**：待审批人自己还没改密（403 门禁）时他的批准请求也会被拦。→ 对策：正常语义，无需特判，文档写明
9. **流量放大**：payload 快照可能很大（如 Nginx 整份配置）。→ 对策：快照存引用（config_version_id）而非全量内容，重放时取当前版本并记录版本号

### 2.8 分期建议

- 一期：全局开关 + 默认高危清单（发布/回滚/Nginx 发布/Secret 轮换）+ 双人确认 + TTL + 待办页
- 二期：自定义清单、审批备注模板、与通知通道联动、导出类操作纳入、审批委托

---

## 3. 资源范围权限

### 3.1 背景

现状 RBAC 是**接口级**：`hosts.view` 意味着"能看全部主机"。一旦平台被一个部门共用，A 团队能看到 B 团队的主机、Secret 引用关系、失败任务——信息边界立刻失守。二期专项规划已设计过资源范围（ALL/GROUP/OWNER/RESOURCE_SET），本文把它收敛为可落地的形态。

### 3.2 需求

- 用户/角色可绑定"范围"：ALL（现状）、或按 Environment、或 OWNER（自己是资源的创建人）、或显式资源集合
- 所有资源查询（列表/详情/操作）按范围过滤；越权访问单个资源 ID 返回 404（不泄露存在性）或 403（现有 403 语义统一后二选一）
- 连带可见性：能看到某主机的人，能看到它的任务/告警/审计（与现有 dashboard 聚合一致）
- 范围变更实时生效（不要求重启）
- 管理员（IsAdmin）始终 ALL

验收标准：

- [ ] 同一权限点 `hosts.view`，scope=env:prod 的用户列表只见 prod 主机，直接访问 dev 主机 ID 得到 404
- [ ] 主机可见者能看到其任务/告警/审计；不可见者对应的聚合数字为 0
- [ ] Secret 被范围外资源引用时不泄露引用关系内容

### 3.3 为什么按"Environment 起步 + OWNER 兜底"收敛

- 现有资源已经普遍携带维度：Host/DockerNode/Registry/ServiceDefinition 都有 `Environment` 字段，Host 还有 `Group/Tags`——**环境是唯一已经全域存在的分组键**，用它起步零迁移成本
- OWNER（`CreatedBy` 字段各模型已有）适合"个人工作区"场景，实现最简单
- RESOURCE_SET（显式清单）管理成本最高、最容易配错，放最后
- GROUP 语义与 Environment 高度重叠，合并进 Environment 即可，不单独做

### 3.4 要做成什么样子（目标形态）

```
ScopeRule{ID, SubjectType(user|role), SubjectID, Scope(ALL|ENVIRONMENT|OWNER|SET),
          RefValue(environment_code / 资源清单JSON), CreatedBy...}
判定顺序（短路）：
  IsAdmin → ALL
  任一 ScopeRule=ALL → ALL
  请求的资源 ∈ 任一规则的集合 → allow
  否则 deny
```

- 角色级绑定优先（配一次全组生效），用户级覆盖（特例）
- 前端：角色编辑页加"资源范围"配置器；用户视角无感知（列表自然变少）

### 3.5 现状盘点（有什么、在哪）

| 现有资产 | 位置 | 对范围权限的意义 |
|---|---|---|
| RBAC：`RequirePermission` → `HasPermission` 每请求 `Preload("Roles.Permissions").First(&user)` 三表联查 | `internal/rbac/middleware.go:15`、`service.go:18` | 范围判定要挂在这条链上，**先做缓存否则性能放大**（见 3.7-1） |
| Environment 模型与维度字段 | `internal/model/environment.go:12`；Host/DockerNode/Registry/Service 均有 `Environment` 列 | 范围键现成 |
| `CreatedBy/UpdatedBy` 字段全域存在 | 各资源模型 | OWNER 范围现成 |
| 资源查询统一走 GORM 列表接口（handler → service） | `internal/handler` 各资源 | 过滤器的注入点集中 |
| dashboard 聚合查询（summary/context） | `internal/handler/dashboard.go` | 范围过滤必须同步覆盖，否则数字与列表不一致 |
| 前端 `permissionsFromUser`（isAdmin → `["*"]`） | `frontend/src/lib/api.ts:681` | 前端权限模型需扩展 scope 概念 |

**目前不存在**：任何资源级判定；ScopeRule 概念；跨资源可见性联动。

### 3.6 要扩展哪里

1. `internal/model/scope.go`（新）+ 迁移 `0005_scope_rules.go`
2. `internal/rbac` 扩展：`ResourceScope(ctx, userID) → ScopeDecision`（缓存判定结果）；`ScopeFilter(query, decision, resourceTable)` GORM 作用域
3. 各资源 service 的 List/Get 注入过滤器；Get 按 ID 访问时做判定（404 语义）
4. 连带资源（task/alert/audit 按 ResourceType+ResourceID 关联）继承主资源可见性
5. dashboard 聚合查询同步过滤
6. 缓存：`HasPermission` 与 `ScopeDecision` 引入带失效的进程内缓存（角色变更/范围变更时失效；多实例用 DB 版本号或短 TTL，见 3.7-6）
7. 前端：角色页范围配置器；列表页无需改（数据已被过滤）

### 3.7 隐藏问题与对策

1. **每请求查库的性能放大**：范围判定比 `HasPermission` 更重（规则匹配 + 集合运算），现状已经每请求三表联查，叠加后不可接受。→ 对策：**缓存是前置项不是优化项**——进程内 TTL 缓存（如 30s）+ 变更主动失效；实施顺序上缓存先于范围
2. **间接资源可见性**：NginxNode 挂在 HostID 下、ServiceDefinition 指向 DockerNode、Secret 被 Host 引用——"能看 Nginx 节点"是否隐含"能看它宿主主机"？→ 对策：一期明确规则"间接资源独立判定"（能看到 Nginx 节点≠能看到宿主主机），文档写死；后续按需加"级联授权"
3. **404 vs 403 语义**：403 泄露"资源存在"，404 不泄露但让用户困惑。→ 对策：按 ID 直访统一返回 404（不泄露存在性），列表自然过滤；文档写明这是隐私语义
4. **聚合查询的一致性**：dashboard summary 若不过滤，限权用户看到全局失败任务数但点进去是空的。→ 对策：聚合与列表必须同一过滤器，做成共享 GORM scope 防漂移
5. **写操作的连带**：A 对 env:dev 有 manage 权限，发布服务引用了 env:prod 的 DockerNode——写操作的目标资源校验必须也过 scope，否则"读隔离写穿透"。→ 对策：`enqueueChange` 已有 `ensureTargetEnvironment` 钩子，在同一位置叠加 scope 校验
6. **多实例缓存失效**：进程内缓存在多实例下会短暂不一致。→ 对策：一期接受短 TTL（30s）最终一致；二期用 DB 里的 `rbac_version` 计数器做主动失效
7. **demo 数据与存量数据**：存量资源 `Environment` 可能为空串。→ 对策：范围匹配把"无环境"视为独立桶（仅 ALL 或显式 SET 可见），迁移文档写明
8. **告警与任务的资源引用可能悬空**（资源被删）：过滤时悬空引用按"跟随资源删除语义"处理（GORM 软删下跟随 DeletedAt）
9. **测试矩阵爆炸**：权限点 × 范围 × 资源类型组合巨大。→ 对策：用表驱动测试覆盖判定核心（纯函数化），接口层只测每资源一条代表性链路

### 3.8 分期建议

- 前置：RBAC 缓存（独立提交，性能收益独立成立）
- 一期：ScopeRule 模型 + Environment 范围 + Host/DockerNode/Service 三个资源 + dashboard 联动
- 二期：OWNER 范围、Registry/Nginx/Secret 补全、RESOURCE_SET、级联授权讨论

---

## 4. 三者的耦合与实施顺序

依赖关系（→ 表示"被依赖"）：

```
RBAC 缓存 → 资源范围权限
真实通知通道 → 审批体验（待办提醒）
OIDC provisioning → 资源范围（SSO 用户进角色，角色带范围）
```

推荐顺序及理由：

1. **RBAC 缓存**——独立价值 + 是范围权限的前置，半天级工作量
2. **OIDC SSO**——最硬的企业门槛，不依赖另外两项
3. **双人确认审批**——依赖通知通道才有体验，但可先落功能后补通知
4. **资源范围权限**——最复杂、影响面最大，放最后且在缓存之上

## 5. 统一验收口径

每个特性合入时，除功能验收外必须满足：

- [ ] 关闭开关后行为与现状一致（可降级）
- [ ] 新增迁移遵循 0002 之后的纪律（改 model 必配迁移）
- [ ] 新端点有审计事件
- [ ] `go test ./...` 全绿 + 表驱动用例覆盖判定核心
- [ ] 真实浏览器端到端走一遍（本机冒烟流程已有成熟脚本）
