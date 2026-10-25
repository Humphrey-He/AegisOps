# AegisOps 下一轮开发交接与待办

> 2026-10-25 收工前整理 · 供下一轮开发直接接手
> 当日完整过程见 [AegisOps正式Release验收清单](./AegisOps正式Release验收清单.md) §0.2 与
> [AegisOps v1.7.0 自监控与迁移版本化_更新记录](./AegisOps%20v1.7.0%20自监控与迁移版本化_更新记录.md)

## 一、当前状态快照

- **v1.7.0 已发版（tag 已推）**，正式 Release 两个前置条件（迁移版本化 + 真实资源冒烟）全部满足，验收清单已按 §10 定级达标
- 真实资源冒烟（云主机 · 真实 Docker/sshd/nginx）四条链路全部通过；发现的升级端口冲突 bug 已修复（按服务标签清理旧版本容器）并云端复验
- 主干新增（随下个版本发布）：Go 1.27.3 工具链 + client_golang v1.25、worker 同轮认领修复（Go 1.27 暴露的时钟量化缺陷）
- CI：每次 push 自动跑 build/vet/test（Linux）+ Postgres 双驱动迁移用例 + 前端构建

## 二、下一轮待办（按优先级）

1. **修复 host.ssh.test 任务步骤状态不推进**（冒烟发现的非阻塞问题，约半小时）
   - 位置：`internal/host/service.go` `TestSSHTask`（约 :199）——任务建了 3 个步骤但从不更新状态，任务 SUCCESS 而 steps 全部停留 PENDING
   - 参考：`internal/nginx/service.go` 的 runner.Step 模式会随执行推进步骤状态；发布类任务（internal/service）同理
   - 测试：`internal/host/service_test.go` 已有 SSH 测试基建，补断言步骤终态即可
2. **真实通知通道**（v1.8.0 建议主目标）
   - v1.1.0 规划至今仍是模板实现，README 最后一个点名的已知边界
   - 顺序建议：企业微信 Webhook（国内直连，一个群机器人即可测）→ SMTP → Telegram（需代理）
   - 通道抽象已就位（notification service 有 channel 类型与 secret 化配置），补真实 sender + 发送结果记录
3. **其余已知边界**（择机）：redact 导出遮蔽 fail-open、RBAC 每请求查库无缓存、主钥静态无 KMS、多实例部署未验证（打戳互斥与抢占指标已为它铺路）、迁移 Down 仅开发/测试用

## 三、环境与操作备忘

- **本机 Go**：`GOTOOLCHAIN=auto`，go.mod 锁 1.27.3（自动下载工具链，无需手动装版本）；`GOTOOLCHAIN=local` 的旧机器跑不了本项目，注意新贡献者文档口径
- **GOPROXY**：默认 goproxy.io 偶发断连（拉 go.yaml.in 时实测），失败时 `GOPROXY=https://goproxy.cn,direct` 重试即可
- **云主机冒烟**（真实资源复跑指引）：交叉编译 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o aegisops-linux ./cmd/aegisops` → scp（注意带目标文件名，默认名不会覆盖旧二进制）→ 先 `pkill -x aegisops` 再替换（Text file busy）→ 后台启动加 `< /dev/null`（防 ssh 挂起）→ SQLite 模式绑定 127.0.0.1 → 一次性 ed25519 冒烟密钥（不录主密钥，测后删 authorized_keys 行）。详见验收清单 §0.2
- **CI Postgres**：`AEGISOPS_TEST_POSTGRES_DSN` 未设置时双驱动用例自动跳过；测试建删一次性数据库，不碰业务库

## 四、发版节奏提醒

主干在 v1.7.0 之后已有 4 个待发版提交（docker 修复、冒烟记录、工具链、worker 修复）；下一版收口时记得：版本号 bump（frontend/package.json + lockfile 两处）、重建 dist、更新记录文档、验收清单记录、打 tag。
