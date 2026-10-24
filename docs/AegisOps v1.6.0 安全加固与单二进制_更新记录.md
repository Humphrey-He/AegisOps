# AegisOps v1.6.0 更新记录：安全加固与单二进制分发

> 2026-10-24 · 基线：v1.5.12 → v1.6.0（8 个提交，每项独立提交、独立测试）
> 本轮定位：不新增业务模块，**把已知的诚实欠账还掉**——并发缺陷、安全缺口、测试空白、分发形态各一项。

---

## 一、为什么做这五项

v1.5.12 复盘（见验收清单 §0）确认了四个"知道自己没有"的边界，本轮逐项收口：

| # | 边界 | 风险定性 | 本轮动作 |
|---|---|---|---|
| 1 | 任务派发抢占不检查 RowsAffected | 多实例下同一任务双执行（单机被进程内 writeMu 掩盖） | fix(task) 18dcc73 |
| 2 | secret 包零测试 | 安全最重的模块（加密/审计/脱敏）无回归保障 | test(secret) 90c4aff |
| 3 | 登录无防爆破、初始口令不强制更换 | 初始口令写 在 README 里，等于平台裸奔 | feat(auth) 11abfa0 |
| 4 | SSH 拨号 InsecureIgnoreHostKey | 平台到托管主机之间可被中间人 | feat(ssh) 2e6c456 |
| 5 | 前端需要单独的静态服务/Node 环境 | 分发形态重，演示成本高 | feat(webui) c7d8f6f |

---

## 二、逐项说明

### 1. fix(task)：任务抢占的 RowsAffected 校验

**约束**：dispatch 抢占用条件更新 `UPDATE ... WHERE id=? AND status=pending`，跨实例并发时该更新可能影响 0 行（对手已抢先）。
**修复**：抽出 `claimDispatchTx` 返回是否真的抢到（`RowsAffected > 0`）；没抢到返回 `ErrClaimLost`，`RunOnce` 跳过该派发继续认领下一条，并限制连续输掉次数防自旋。
**边界**：单机进程内 writeMu 使竞态几乎不可能复现，因此测试直接覆盖条件更新的两种结果（抢到/没抢到）而非端到端竞态——测试注释里写明了这个取舍。
**验收**：`internal/task/claim_test.go` 两条用例 + 原有 worker 测试全绿。

### 2. test(secret)：安全核心模块补测试

覆盖六类行为：AES-GCM 加解密往返（明文不出现在任何落库字段）、Mask 边界（空/≤6 位/7 位/中文按 rune）、解密即审计（成功与禁用失败各留痕）、被引用密钥禁止删除（引用清空后可删）、轮换换密（KeyVersion++、新 nonce、旧密文失效）、不支持的类型拒绝。
**验收**：6 个用例全绿；`go test ./...` 从 14 个测试包增至 17 个（auth、hostkey 本轮新增）。

### 3. feat(auth)：登录限流 + 首启强制改密

- `LoginGuard`：用户名+IP 维度，窗口内 5 次失败锁 15 分钟；只计失败、成功清零；锁定期直接 429，不再消耗 bcrypt 比较（防时序放大）。单机内存实现，多实例应换共享存储（已在代码注释声明）。
- 首启强制改密：bootstrap 管理员带 `MustChangePassword` 标记；`RequirePasswordChangeCleared` 中间件把业务接口全部拦成 `403 MUST_CHANGE_PASSWORD`（豁免 me/change-password/logout/refresh）；新增 `POST /api/auth/change-password`（校验旧口令、≥8 位、新旧不同），双向审计。
- 前端：登录后命中标记自动跳改密页；任何 403 MUST_CHANGE_PASSWORD 响应全局兜底跳转。
**验收**：guard 4 条用例（阈值/清零/过期/key 隔离）+ 改密全流程用例；单二进制冒烟实测 403→200→200、6 次失败后 429。

### 4. feat(ssh)：主机公钥 TOFU 钉扎

**约束**：terminal 流式与 host TestSSH 两处拨号都 `InsecureIgnoreHostKey()`。
**选择**：`internal/hostkey`——首次连接信任并落库（authorized_keys 单行存 Host 记录，known_hosts 语义但不依赖文件系统，容器无状态友好），此后按公钥字节严格比对，不一致即 `ErrKeyMismatch` 拒绝；钉扎值损坏 fail-closed。
**已知边界（代码内声明）**：TOFU 防不了"首次连接就被中间人"，那需要带外指纹校验。
**验收**：3 条用例（TOFU 落库+同钥放行、换钥拒绝且不改写钉扎、损坏钉扎 fail-closed）。

### 5. feat(webui)：单二进制分发

`frontend/dist` 随仓库提交，经 `go:embed` 内嵌；Gin 兜底路由：静态资产直出、非 /api 未知路径回退 index.html（SPA）、/api 未知路径保持 JSON 404。
**形态**：`go build` 产出 **33MB 单 exe**，`git clone → go run → 打开 :8080` 即完整平台，无需 Node。
**代价与纪律**：前端源码变更后必须 `npm run build` 并提交 dist（README 已注明）。
**验收**：`TestEmbeddedServesFrontend`（/、/login 回退、真实资产 200、API 404 语义）+ 单 exe 冒烟实测。

---

## 三、本轮验证汇总（2026-10-24 · Windows / Go 1.25 / Node 22）

- `go build ./...`、`go vet ./...` 通过；`go test ./...` **17 包全绿**
- `npm run build` 通过
- 单二进制（SQLite 模式）端到端：内嵌 UI、SPA 回退、资产直出、API 404 语义 ✅
- 安全链路：强制改密 403→200→200 ✅；登录限流 429 ✅
- CI（GitHub Actions）随 push 自动验证

## 四、仍然开放的生产化边界（诚实清单）

AutoMigrate 未版本化（改列/回滚不支持）；无 /metrics 自监控；通知通道为模板实现；RBAC 每请求查库无缓存；redact 导出遮蔽为 fail-open（值得讨论）；主钥静态无 KMS；多实例部署未验证（本轮修复的是"抢到与否"的判定，部署形态仍单机）。
