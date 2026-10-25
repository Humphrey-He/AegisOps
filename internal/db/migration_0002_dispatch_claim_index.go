package db

import "gorm.io/gorm"

// migrationDispatchClaimIndex 给派发 worker 的抢占热查询加复合索引：
//
//	WHERE status = ? AND queued_at <= ? (AND lease 过期判断)
//	ORDER BY queued_at ASC, created_at ASC   （worker.go claimNext）
//
// 该索引不来自 model tag（AutoMigrate 不会生成），是第一条例行的增量迁移，
// 同时验证 0002+ 增量链路在两种驱动上的行为。
var migrationDispatchClaimIndex = &Migration{
	ID:          "0002_dispatch_claim_index",
	Description: "composite index for task dispatch claim query",
	Up: func(tx *gorm.DB) error {
		// CREATE INDEX IF NOT EXISTS 在 SQLite 与 PostgreSQL 上同构。
		return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_task_dispatches_claim
			ON task_dispatches (status, queued_at, created_at)`).Error
	},
	Down: func(tx *gorm.DB) error {
		return tx.Exec(`DROP INDEX IF EXISTS idx_task_dispatches_claim`).Error
	},
}
