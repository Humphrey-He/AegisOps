# AegisOps v1.7.0 更新记录：自监控与迁移版本化

> 2026-10-25 · 基线：v1.6.0 → v1.7.0（5 个功能/CI 提交 + 发版收口提交，每项独立提交、独立测试）
> 本轮定位：**生产化增强第一轮**——把 v1.6.0 复盘里明确写下的两项工程欠账（无 /metrics 自监控、AutoMigrate 未版本化）还掉，不新增业务模块。

---

## 一、为什么做这两项

v1.6.0 更新记录 §四（诚实清单）的前两项边界，正是正式 Release 定级的已知短板：

| # | 边界 | 风险定性 | 本轮动作 |
|---|---|---|---|
| 1 | 无 /metrics 自监控 | 平台自身不可观测：API 耗时、任务积压、失败率全靠翻日志 | feat(metrics) 1f3737e + feat(task) 4211926 |
| 2 | AutoMigrate 未版本化 | 改列/回滚不支持；schema 变更无历史、无版本对齐手段，是正式 Release 明确的前置阻塞项 | feat(db) 6c810fc + ci(db) da059be |

---

## 二、逐项说明

### 1. feat(metrics)：独立端口的 Prometheus 自监控

**选型**：`prometheus/client_golang v1.21.1`（最新 v1.25.0 要求 Go 1.26，本机/CI 为 1.25，故取兼容版）。
**形态**：独立端口 `:9091`、无鉴权、靠网络层隔离——不进 JWT 中间件链，不污染主端口语义；`metrics.enabled: false` 可整体关闭。
**设计取舍**：
- 包级私有 registry（不用 prometheus 全局默认注册表）：防第三方库指标混入抓取输出，测试可独立 Gather
- HTTP 指标 route 用**路由模板**（`/api/tasks/:id`），未匹配路由归一 `unmatched`：带 ID 的真实路径不进标签，防基数爆炸
- 8 个 `aegisops_` 指标：HTTP 请求计数/耗时、派发抢占 won/lost、执行耗时（按任务类型）、终态计数、重试、调度入队、worker 错误
**验收**：registry/middleware 单测 + 单二进制冒烟实测（登录后 `aegisops_http_requests_total{method="POST",route="/api/auth/login",status="200"} 1`，未知路径归一 `unmatched/404`）。

### 2. feat(task)：任务派发与调度埋点

抢占计数落在 `claimDispatchTx` 的 RowsAffected 判定处——与 v1.6.0 的抢占并发修复（18dcc73）同点呼应：多实例部署后 `result="lost"` 的增长率就是抢占冲突率的直接观测。终态计数只在事务真正落库时打点（CANCELED no-op 分支不计数）；执行耗时包住 executor 调用（超时/重试路径都覆盖）。
**取舍**：指标断言测试不调 `t.Parallel()`——包级计数器在并行测试下无法做确定断言，用串行换取精确验证。

### 3. feat(db)：数据库迁移版本化（正式 Release 前置项）

**为什么自研 runner 而不用 golang-migrate/goose/gormigrate**：SQL 文件流派的 baseline 需按 SQLite/PostgreSQL 双方言维护两份 DDL，漂移风险高；gormigrate 的 InitSchema/空库语义多一层间接。本项目需要的语义很精确（前向迁移、每条事务化、双驱动经 GORM 抽象），约 130 行自研代码即可完全掌控——零新依赖，行为透明可测。

**核心语义（对应 Release 验收口径）**：
1. 干净库：顺序执行全部迁移，一键到最新
2. 存量 v1.6.0 库：无版本表 → 从头执行；`0001_baseline_v1_6_0` 是 AutoMigrate（GORM 只增不删），对存量库幂等无害，**baseline 幂等 = 免费的存量库打戳**，数据零丢失
3. 坏迁移：单事务内执行 DDL 与打戳 INSERT，失败整体回滚、不打戳、带迁移 ID 报错终止

**安全网**：降级保护（库里有二进制不认识的迁移 ID 时拒绝启动，防老版本写新 schema）；双实例并发迁移时打戳主键冲突天然互斥（为多实例预留）。

**冻结纪律**（写在 `migrations.go` 文件头）：改 model 必须配套编号迁移；0001 是全项目唯一保留的全量 AutoMigrate；新迁移只追加到列表末尾。`0002_dispatch_claim_index`（派发抢占复合索引）是第一例增量迁移，兼具真实性能收益与链路验证。`db.AutoMigrate` 从公共 API 移除，14 个调用点（main + 13 个测试文件）全部改走 `db.Migrate`，测试从此走真实迁移路径。

### 4. ci(db)：PostgreSQL 双驱动验证走 CI

backend job 增加 postgres:16 service，注入 `AEGISOPS_TEST_POSTGRES_DSN`。gated 用例在一次性建删的测试库上执行（不触碰业务库），干净迁移 + 存量升级两场景全覆盖；未设置 DSN 时自动跳过（本地无 Postgres 不阻塞）。**本地无 Docker/PostgreSQL，首次双驱动实测随本次发布 push 执行**（发布时网络不通，提交先落本地，网络恢复后推送）。

---

## 三、本轮验证汇总（2026-10-25 · Windows / Go 1.25 / Node 22）

- `go build ./...`、`go vet ./...` 通过；`go test ./... -count=1` **19 包全绿**（v1.6.0 为 17 包）
- `npm run build` 通过，dist 重建入库（内嵌产物与源码状态对齐），内嵌路由测试全绿
- 单二进制（SQLite 全新库）端到端：迁移 2 条自动应用、内嵌 UI 200、SPA 回退 200、登录 200、API 404 语义保持、`:9091/metrics` 全量指标 ✅
- 重启幂等：同一库第二次启动 0 条迁移重跑、登录正常 ✅
- 迁移专项单测：干净库/存量升级不丢数据/坏迁移回滚/幂等/降级保护/重复 ID 防护 全绿 ✅
- PostgreSQL 双驱动：CI service 已配置，首验随发布 push（结果回填 Release 验收清单 §0.2）

## 四、仍然开放的生产化边界（诚实清单）

通知通道仍为模板实现（Telegram/企业微信/SMTP 真实发送待做）；多实例部署未验证（本轮打戳互斥与抢占指标为它铺了路）；RBAC 每请求查库无缓存；redact 导出遮蔽为 fail-open（值得讨论）；主钥静态无 KMS；迁移 Down 仅用于开发/测试，生产只前向迁移；真实资源冒烟（主机 SSH/Docker/Service/Nginx 链路）仍未执行——正式 Release 定级仅剩这一项前置。
