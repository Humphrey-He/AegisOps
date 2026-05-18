# AegisOps 二期专项规划：通知告警与健康检查闭环

## 1. 文档目的

本文档用于规划 AegisOps 下一期中最值得优先补齐的两块能力：

1. 通知告警  
2. 健康检查闭环

这两块能力直接决定 AegisOps 能否从“可发起操作的运维控制台”继续提升为“可感知风险、可闭环处置异常的个人运维平台”。

本文档重点回答以下问题：

1. 这一期具体要补什么，不补什么
2. 前端侧应该如何承接通知与健康状态
3. 后端侧应该如何编排发布后探活、失败标记、半自动回滚
4. 哪些接口、数据模型、任务链路需要新增
5. 这一期完成后，AegisOps 在个人博客和少量自建服务场景中能提升到什么程度

---

## 2. 本期定位

本期不是继续横向扩模块，而是在现有主机、Docker、Registry、Service、Nginx、Task、Audit 这些能力上，补齐“异常发现 -> 状态收敛 -> 消息通知 -> 人工确认/半自动回滚”的闭环。

本期的核心目标是：

- 发布失败时，系统能主动给出失败结果，而不是只停留在任务失败
- Nginx reload 失败时，系统能及时通知，而不是只留下一条审计记录
- 主机离线时，系统能形成统一异常事件，而不是只在某次 test-ssh 里失败
- 服务发布完成后，系统能自动做探活，失败后能标记为异常并支持半自动回滚

本期完成后，AegisOps 应至少具备如下体验：

1. 发起服务发布
2. 发布进入任务中心
3. 发布完成后自动执行健康检查
4. 健康检查失败时自动写入任务日志、标记发布失败、生成异常事件
5. 系统按通知策略推送到 Telegram / 企业微信 / 邮箱
6. 用户可在前端看到失败原因、通知发送情况和“回滚到上一版本”的入口
7. 用户确认后触发半自动回滚，并继续进入任务中心追踪

---

## 3. 本期范围

### 3.1 纳入本期

- 服务发布后的自动探活
- 发布失败后的异常状态标记
- 半自动回滚入口与后端编排
- Nginx reload 失败通知
- 主机离线通知
- Telegram / 企业微信 / 邮箱三类通知通道
- 告警规则与通知目标的基础配置
- 任务、审计、通知记录三者的关联

### 3.2 暂不纳入本期

- 复杂值班排班与告警升级策略
- 多级告警降噪与聚合编排
- 短信 / 电话等更重型通知通道
- 完整监控系统接入（Prometheus / Alertmanager）
- 自动灰度发布 / 蓝绿发布 / 金丝雀发布
- 完整自愈系统

本期聚焦的是“个人和小规模自建服务可用的轻量闭环”，而不是企业级监控告警平台。

---

## 4. 与现有能力的衔接

当前后端已经具备以下可复用基础：

- 服务发布 / 升级 / 回滚接口与发布记录
- Nginx test / publish / reload / rollback 任务化能力
- 主机 SSH test 任务化能力
- 统一任务中心（任务、步骤、日志）
- 审计日志能力
- RBAC 权限校验

因此本期不应另起一套“通知系统”或“健康检查系统”，而应直接挂在现有任务与审计骨架上。

建议复用原则如下：

1. 健康检查应作为发布任务的后续步骤，而不是单独散落在页面侧
2. 通知发送应绑定任务、资源、事件来源
3. 告警结果应进入审计与任务日志，而不是只写 stdout
4. 前端应优先消费统一事件/记录接口，而不是各页各自拼状态

---

## 5. 总体链路设计

### 5.1 服务发布后的健康检查闭环

建议链路：

1. 用户发起 `POST /api/services/:id/releases`
2. 后端创建发布任务与发布记录
3. 执行镜像校验、容器部署、实例状态写入
4. 发布完成后进入“健康检查”步骤
5. 按服务定义中的探活策略执行 HTTP / TCP / 命令级探活
6. 探活成功：发布任务成功结束，实例标记为健康
7. 探活失败：发布记录标记失败，实例标记异常，生成告警事件
8. 根据策略推送 Telegram / 企业微信 / 邮箱通知
9. 若服务允许半自动回滚，则前端展示“回滚到上一版本”入口
10. 用户确认后触发 rollback 任务

### 5.2 Nginx reload 失败通知链路

建议链路：

1. 用户发起 `reload` 或 `publish`
2. 后端执行 `nginx -t`
3. 后端执行 `nginx reload`
4. 任何步骤失败都写入任务日志与审计
5. 生成 `nginx_reload_failed` 异常事件
6. 命中通知规则后推送到配置通道

### 5.3 主机离线通知链路

建议链路：

1. 定时巡检或手动 test-ssh
2. SSH 失败超过阈值
3. 主机状态切换为 `UNREACHABLE`
4. 生成 `host_offline` 异常事件
5. 触发通知并在前端资产页、工作台、告警列表中同步体现

---

## 6. 前端侧开发规划

## 6.1 前端目标

前端本期不是简单“多一个通知设置页”，而是要把“异常可见、状态可读、动作可继续”这三件事做完整。

前端目标分为四类：

1. 告警配置可管理
2. 运行状态可感知
3. 发布失败可解释
4. 异常后续动作可继续

## 6.2 前端页面范围

建议新增或增强以下页面：

### A. 通知配置页

建议路径：

- `/settings/notifications`

页面内容建议包括：

- 通知通道列表
- 新建 Telegram 通道
- 新建企业微信通道
- 新建邮箱通道
- 通道启用/停用
- 发送测试消息
- 最近发送记录

关键字段建议：

- 通道名称
- 通道类型
- 启用状态
- 默认接收人 / 群组
- 最近一次发送结果
- 最近失败原因

### B. 告警规则页

建议路径：

- `/settings/alert-rules`

页面内容建议包括：

- 告警事件类型
- 触发条件
- 绑定资源范围
- 通知目标
- 是否允许重复告警抑制
- 是否需要手工确认

首期支持的事件类型建议固定为：

- `service_release_failed`
- `service_health_check_failed`
- `nginx_reload_failed`
- `host_offline`

### C. 服务详情页增强

现有服务页建议增加以下区块：

- 当前健康状态
- 最近一次健康检查结果
- 最近失败原因
- 最近一次通知发送结果
- 半自动回滚入口

发布记录表建议增加字段：

- 健康检查状态
- 通知状态
- 回滚建议状态
- 失败摘要

### D. Nginx 节点详情页增强

建议增加：

- 最近 reload 结果
- 最近 test 结果
- 最近失败原因
- 最近通知状态

### E. 主机详情页增强

建议增加：

- 最近离线时间
- 最近恢复时间
- 最近告警状态
- 连续失败次数

### F. 工作台 / 异常中心

建议新增“异常事件卡片”或“告警列表”：

- 未处理异常数量
- 最近 24 小时发布失败
- 最近 24 小时主机离线
- 最近 24 小时 Nginx reload 失败
- 点击可跳转对应资源或任务详情

---

## 6.3 前端交互设计重点

### 6.3.1 发布后的状态呈现

前端不能再只展示“发布成功 / 发布失败”。

建议拆成 4 层状态：

1. 发布任务状态
2. 健康检查状态
3. 通知状态
4. 回滚建议状态

示例：

- 发布成功，但健康检查失败
- 健康检查失败，通知已发送
- 通知发送失败，需要人工补发
- 建议回滚到上一稳定版本

### 6.3.2 半自动回滚交互

建议回滚入口采用二段式：

1. 系统提示建议回滚版本
2. 用户点击确认后再提交 rollback 任务

不建议本期默认自动回滚，原因是：

- 个人服务场景里误回滚的代价仍然不低
- 当前系统还不具备足够成熟的自动判定和多环境保护

### 6.3.3 通知测试入口

每类通知通道都应该支持前端直接触发“发送测试消息”，否则配置完成后很难确认链路是否可用。

---

## 6.4 前端接口需求

建议前端需要消费的新增接口包括：

- `GET /api/notifications/channels`
- `POST /api/notifications/channels`
- `PATCH /api/notifications/channels/:id`
- `POST /api/notifications/channels/:id/test`
- `GET /api/notifications/events`
- `GET /api/notifications/records`
- `GET /api/alert-rules`
- `POST /api/alert-rules`
- `PATCH /api/alert-rules/:id`
- `GET /api/services/:id/health-checks`
- `POST /api/services/:id/rollbacks/suggested`
- `GET /api/hosts/:id/availability`
- `GET /api/nginx/nodes/:id/operations`

---

## 6.5 前端验收标准

前端本期验收建议以以下标准为准：

1. 可配置至少一种 Telegram / 企业微信 / 邮箱通知通道
2. 可在前端测试通知发送
3. 服务发布失败后，服务详情页能明确显示失败步骤与健康检查结果
4. 发布失败时，前端可展示是否已通知
5. 健康检查失败时，前端能展示推荐回滚入口
6. 主机离线与 Nginx reload 失败都能在工作台或异常列表中被看到

---

## 7. 后端侧开发规划

## 7.1 后端目标

后端本期目标不是只“多发一条消息”，而是建立一套稳定的事件驱动闭环：

1. 识别异常
2. 记录异常
3. 触发通知
4. 更新任务与资源状态
5. 提供后续动作入口

## 7.2 后端模块拆分建议

建议新增或增强以下模块：

### A. notification 模块

职责：

- 管理通知通道
- 管理发送器
- 发送测试消息
- 发送正式告警消息
- 记录发送结果

建议支持的发送器：

- Telegram Bot
- 企业微信机器人 Webhook
- SMTP 邮箱

建议目录：

- `internal/notification`

### B. alert 模块

职责：

- 统一管理异常事件
- 规则匹配
- 去重与抑制
- 关联通知目标
- 关联资源和任务

建议目录：

- `internal/alert`

### C. healthcheck 模块

职责：

- 定义服务探活策略
- 执行发布后探活
- 记录探活结果
- 输出统一结果给服务发布流程

建议目录：

- `internal/healthcheck`

### D. scheduler / inspector 模块

职责：

- 定时巡检主机、Nginx、服务
- 驱动主机离线事件生成
- 驱动健康检查重试

如果本期不想独立 worker，可先在进程内实现轻量调度器。

---

## 7.3 后端数据模型建议

建议新增以下表：

### 7.3.1 通知通道表

- `notification_channels`

建议字段：

- `id`
- `name`
- `type` (`telegram` / `wecom` / `email`)
- `enabled`
- `config_encrypted`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

### 7.3.2 告警规则表

- `alert_rules`

建议字段：

- `id`
- `name`
- `event_type`
- `resource_type`
- `resource_scope`
- `channel_ids`
- `enabled`
- `dedupe_window_seconds`
- `require_ack`
- `created_by`
- `updated_by`

### 7.3.3 异常事件表

- `alert_events`

建议字段：

- `id`
- `event_type`
- `resource_type`
- `resource_id`
- `task_id`
- `release_id`
- `severity`
- `status` (`OPEN` / `ACKED` / `RESOLVED`)
- `summary`
- `detail`
- `dedupe_key`
- `first_triggered_at`
- `last_triggered_at`
- `resolved_at`

### 7.3.4 通知发送记录表

- `notification_records`

建议字段：

- `id`
- `event_id`
- `channel_id`
- `status` (`PENDING` / `SUCCESS` / `FAILED`)
- `provider_message_id`
- `response_excerpt`
- `error_message`
- `created_at`
- `finished_at`

### 7.3.5 服务健康检查记录表

- `service_health_checks`

建议字段：

- `id`
- `service_id`
- `release_id`
- `task_id`
- `strategy_type` (`HTTP` / `TCP` / `COMMAND`)
- `target`
- `status`
- `http_status`
- `latency_ms`
- `output`
- `error_message`
- `started_at`
- `finished_at`

### 7.3.6 主机可用性记录表

- `host_availability_checks`

建议字段：

- `id`
- `host_id`
- `task_id`
- `status`
- `failure_reason`
- `started_at`
- `finished_at`

---

## 7.4 后端核心流程设计

## 7.4.1 服务发布后的健康检查

建议在现有 `service.Release()` 流程后追加步骤：

1. `validate release request`
2. `resolve release version`
3. `prepare docker execution`
4. `deploy container and persist state`
5. `run post-release health check`
6. `evaluate rollback suggestion`
7. `dispatch notifications if needed`

核心要求：

- 健康检查必须进入任务步骤与日志
- 健康检查失败不能只写日志，必须影响 release record 状态
- 健康检查失败必须生成异常事件
- 健康检查失败后应返回推荐回滚版本

### 探活策略建议

首期建议支持 3 类：

- HTTP 探活：URL、期望状态码、超时、重试次数
- TCP 探活：host:port 是否连通
- 命令探活：在目标主机或容器内执行简短校验命令

### 回滚策略建议

本期建议只支持“半自动回滚”：

- 系统识别最近稳定版本
- 将推荐版本写入事件和接口返回
- 用户在前端确认后，后端触发 rollback 任务

---

## 7.4.2 Nginx reload 失败告警

现有 Nginx 任务链路已经具备 test / publish / reload / rollback 能力，本期建议增强为：

1. reload 失败写任务日志
2. reload 失败写审计
3. reload 失败生成异常事件
4. 命中规则后触发通知
5. 在节点维度记录最近失败状态

建议事件类型：

- `nginx_reload_failed`
- `nginx_publish_failed`

---

## 7.4.3 主机离线告警

建议新增两类来源：

1. 手动 test-ssh 失败
2. 定时巡检失败

主机离线判定建议：

- 单次失败只记检查失败，不立刻告警
- 连续 N 次失败后才切换为 `UNREACHABLE`
- 恢复成功后生成 `host_recovered` 事件并清理打开态异常

建议首期默认策略：

- 连续 3 次失败判定离线
- 连续 1 次成功判定恢复

---

## 7.5 后端接口建议

### 通知通道

- `GET /api/notifications/channels`
- `POST /api/notifications/channels`
- `GET /api/notifications/channels/:id`
- `PATCH /api/notifications/channels/:id`
- `POST /api/notifications/channels/:id/test`

### 告警规则

- `GET /api/alert-rules`
- `POST /api/alert-rules`
- `PATCH /api/alert-rules/:id`
- `DELETE /api/alert-rules/:id`

### 异常事件 / 通知记录

- `GET /api/alerts/events`
- `GET /api/alerts/events/:id`
- `POST /api/alerts/events/:id/ack`
- `POST /api/alerts/events/:id/resolve`
- `GET /api/alerts/records`

### 健康检查

- `GET /api/services/:id/health-checks`
- `POST /api/services/:id/health-checks/run`
- `GET /api/releases/:id/health-checks`

### 半自动回滚辅助

- `GET /api/services/:id/rollback-suggestion`
- `POST /api/services/:id/rollbacks`

### 主机可用性

- `GET /api/hosts/:id/availability`
- `POST /api/hosts/:id/checks/run`

---

## 7.6 后端任务与审计要求

本期所有关键链路都应继续沿用任务中心和审计体系。

### 任务化要求

以下动作必须进入任务中心：

- 发布后的健康检查
- Nginx reload / publish 失败后的补充检查
- 主机巡检
- 半自动回滚
- 批量通知发送

### 审计要求

以下动作必须写审计：

- 新增 / 修改 / 删除通知通道
- 发送测试通知
- 修改告警规则
- 用户确认回滚
- 用户确认告警

---

## 7.7 后端配置建议

建议新增配置项：

```yaml
notifications:
  telegram:
    enabled: true
  wecom:
    enabled: true
  email:
    enabled: true

healthcheck:
  default_timeout_seconds: 10
  default_retry_times: 3
  default_retry_interval_seconds: 5

alerts:
  host_offline_fail_threshold: 3
  host_recover_success_threshold: 1
  dedupe_window_seconds: 300
```

---

## 7.8 后端测试要求

本期建议至少补以下测试：

1. 发布成功但探活失败，release record 正确标记失败
2. 探活失败时任务步骤、日志、异常事件、通知记录都能落库
3. Nginx reload 失败时能触发异常事件和通知
4. 主机连续失败达到阈值后生成离线事件
5. Telegram / 企业微信 / 邮箱发送器在 mock 下可测试
6. 告警去重窗口生效，短时间内不重复轰炸

---

## 8. 推荐开发顺序

建议按以下顺序推进：

### 阶段一：后端底座

- 建立 `notification / alert / healthcheck` 基础模型与表
- 打通最小 Telegram 通道
- 打通服务发布后的健康检查写库能力

### 阶段二：服务发布闭环

- 将健康检查接入 release 流程
- 失败后生成异常事件
- 失败后返回推荐回滚版本

### 阶段三：前端首轮接入

- 服务详情页展示健康检查结果
- 发布记录展示通知状态
- 前端展示半自动回滚入口

### 阶段四：Nginx 与主机告警

- 接入 Nginx reload 失败事件
- 接入主机离线巡检与通知
- 工作台增加异常中心

### 阶段五：多通道完善

- 企业微信通道
- 邮箱通道
- 告警规则和通知配置页

---

## 9. 本期验收标准

本期建议以以下闭环作为总验收：

### 场景一：服务发布失败

1. 发起服务发布
2. 部署完成后自动执行探活
3. 探活失败
4. 发布记录状态变为失败
5. 任务中心可看到失败步骤与日志
6. 告警事件生成
7. Telegram / 企业微信 / 邮箱至少一种通知成功送达
8. 前端展示推荐回滚入口
9. 用户确认后可发起 rollback

### 场景二：Nginx reload 失败

1. 发起 reload
2. 任务失败
3. 审计记录存在
4. 告警事件存在
5. 通知可送达

### 场景三：主机离线

1. 巡检连续失败达到阈值
2. 主机状态切为异常
3. 工作台出现离线事件
4. 通知发出
5. 主机恢复后事件可自动收敛或标记恢复

---

## 10. 风险点与注意事项

### 10.1 不要把通知写死在业务代码里

应该通过事件 -> 规则 -> 通道 的方式解耦，否则后面一加企业微信和邮箱就会迅速散掉。

### 10.2 不要默认全自动回滚

本期只建议“半自动回滚”。自动回滚需要更成熟的探活判定、窗口控制和幂等保护。

### 10.3 不要把健康检查只做成前端轮询

健康检查必须由后端落任务、落记录、落结果，否则无法形成真正闭环。

### 10.4 主机离线要做阈值判定

不能一次失败就推送，否则误报会非常多。

### 10.5 通知失败也要可见

告警最怕“原问题发生了，通知又失败了，但系统没人知道”。通知记录本身必须可查询。

---

## 11. 一句话结论

这一期的本质，不是“多做两个功能页”，而是把 AegisOps 从“能发起运维动作”推进到“能发现异常、能通知、能给出回滚路径”的轻量闭环平台。

对个人博客和少量自建服务场景来说，这一轮能力补齐之后，AegisOps 才会真正开始具备“日常自用运维平台”的价值。
