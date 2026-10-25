// 迁移版本化 runner：schema_migrations 版本表 + 有序前向迁移，每条迁移事务化。
//
// 语义（对应正式 Release 验收口径）：
//   1. 干净库：顺序执行全部迁移，一键到最新
//   2. 存量库（有表、无 schema_migrations）：从头执行——0001 baseline 是
//      AutoMigrate，GORM 只增不删，对存量 schema 幂等无害，等于"打戳"，
//      之后的增量迁移真实执行，数据零丢失
//   3. 坏迁移：单事务内执行 DDL 与打戳 INSERT，失败即整体回滚、不打戳、
//      带迁移 ID 报错终止，下次启动从同一条重试
//
// 附带安全网：
//   - 降级保护：库里存在本二进制不认识的迁移 ID 时拒绝迁移，防老版本写新 schema
//   - 双实例并发：同时打戳触发主键冲突，后来者报错退出，天然互斥
package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Migration 是一条版本化迁移。Up 必须可在两种驱动（SQLite/PostgreSQL）上执行；
// Down 仅用于开发/测试回滚，生产只做前向迁移。
type Migration struct {
	ID          string
	Description string
	Up          func(*gorm.DB) error
	Down        func(*gorm.DB) error
}

// Migrate 按顺序应用全局迁移列表（见 migrations.go）到最新版本。
func Migrate(database *gorm.DB) error {
	return runMigrations(database, Migrations)
}

// runMigrations 是可测试的核心：给定迁移列表，应用到 database。
func runMigrations(database *gorm.DB, migrations []*Migration) error {
	if err := validateMigrations(migrations); err != nil {
		return err
	}
	if err := ensureMigrationTable(database); err != nil {
		return err
	}
	applied, err := appliedMigrationIDs(database)
	if err != nil {
		return err
	}

	known := make(map[string]bool, len(migrations))
	for _, migration := range migrations {
		known[migration.ID] = true
	}
	for _, id := range applied {
		if !known[id] {
			return fmt.Errorf("database schema has migration %q which this binary does not know; upgrade the binary instead of running an older version against a newer schema", id)
		}
	}

	appliedSet := make(map[string]bool, len(applied))
	for _, id := range applied {
		appliedSet[id] = true
	}
	for _, migration := range migrations {
		if appliedSet[migration.ID] {
			continue
		}
		if err := applyMigration(database, migration); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.ID, err)
		}
	}
	return nil
}

// applyMigration 在单事务内执行迁移并打戳：DDL 与版本记录同生共死。
func applyMigration(database *gorm.DB, migration *Migration) error {
	return database.Transaction(func(tx *gorm.DB) error {
		if migration.Up != nil {
			if err := migration.Up(tx); err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO schema_migrations (id) VALUES (?)", migration.ID).Error
	})
}

// AppliedMigrations 返回已应用的迁移 ID（按应用顺序），供启动日志与排查。
func AppliedMigrations(database *gorm.DB) ([]string, error) {
	if err := ensureMigrationTable(database); err != nil {
		return nil, err
	}
	return appliedMigrationIDs(database)
}

func ensureMigrationTable(database *gorm.DB) error {
	// 该 DDL 在 SQLite 与 PostgreSQL 上同构。
	return database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		id VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error
}

func appliedMigrationIDs(database *gorm.DB) ([]string, error) {
	var ids []string
	if err := database.Raw("SELECT id FROM schema_migrations ORDER BY applied_at ASC, id ASC").Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func validateMigrations(migrations []*Migration) error {
	seen := make(map[string]bool, len(migrations))
	for _, migration := range migrations {
		if migration.ID == "" {
			return errors.New("migration with empty ID")
		}
		if seen[migration.ID] {
			return fmt.Errorf("duplicate migration ID %q", migration.ID)
		}
		seen[migration.ID] = true
	}
	// 列表顺序即执行顺序：新迁移只允许追加到末尾，不做隐式排序。
	return nil
}
