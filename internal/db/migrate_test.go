package db

import (
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return database
}

func sqliteIndexExists(t *testing.T, database *gorm.DB, name string) bool {
	t.Helper()
	var count int64
	if err := database.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&count).Error; err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	return count > 0
}

// 验收口径 1：干净库一键迁到最新。
func TestMigrateCleanDatabaseToLatest(t *testing.T) {
	database := openSQLiteTestDB(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate on clean database: %v", err)
	}

	ids, err := AppliedMigrations(database)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	want := []string{"0001_baseline_v1_6_0", "0002_dispatch_claim_index"}
	if len(ids) != len(want) {
		t.Fatalf("applied migrations = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("applied migrations = %v, want %v", ids, want)
		}
	}

	for _, table := range []string{"users", "tasks", "task_dispatches", "secrets", "scheduled_jobs"} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("table %s missing after migration", table)
		}
	}
	if !sqliteIndexExists(t, database, "idx_task_dispatches_claim") {
		t.Fatalf("idx_task_dispatches_claim missing after migration")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	database := openSQLiteTestDB(t)

	if err := Migrate(database); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	ids, err := AppliedMigrations(database)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(ids) != len(Migrations) {
		t.Fatalf("applied count = %d after rerun, want %d (no duplicate stamps)", len(ids), len(Migrations))
	}
}

// 验收口径 2：已有 v1.6.0 库（有表、无 schema_migrations）原地升级不丢数据。
func TestMigrateUpgradesLegacyDatabaseWithoutDataLoss(t *testing.T) {
	database := openSQLiteTestDB(t)

	// 模拟 v1.6.0 时代的库：启动时直接 AutoMigrate，没有版本表。
	if err := database.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("legacy AutoMigrate: %v", err)
	}
	legacyUser := model.User{Username: "legacy", PasswordHash: "x", DisplayName: "Legacy User", Status: model.UserStatusActive}
	if err := database.Create(&legacyUser).Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}
	legacyTask := model.Task{ID: "legacy-task-1", Type: "host.ssh.test", Title: "legacy task", Status: model.TaskStatusFailed, TargetType: "host", TargetID: "legacy-host-1"}
	if err := database.Create(&legacyTask).Error; err != nil {
		t.Fatalf("seed legacy task: %v", err)
	}

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate on legacy database: %v", err)
	}

	var userCount, taskCount int64
	if err := database.Model(&model.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users after upgrade: %v", err)
	}
	if err := database.Model(&model.Task{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks after upgrade: %v", err)
	}
	if userCount != 1 || taskCount != 1 {
		t.Fatalf("legacy data lost after upgrade: users=%d tasks=%d, want 1 each", userCount, taskCount)
	}
	var upgradedTask model.Task
	if err := database.First(&upgradedTask, "id = ?", legacyTask.ID).Error; err != nil {
		t.Fatalf("load legacy task after upgrade: %v", err)
	}
	if upgradedTask.Status != model.TaskStatusFailed {
		t.Fatalf("legacy task status = %s, want unchanged FAILED", upgradedTask.Status)
	}
	if !sqliteIndexExists(t, database, "idx_task_dispatches_claim") {
		t.Fatalf("incremental migration did not add idx_task_dispatches_claim to legacy database")
	}
	ids, err := AppliedMigrations(database)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(ids) != len(Migrations) {
		t.Fatalf("applied count = %d after legacy upgrade, want %d", len(ids), len(Migrations))
	}
}

// 验收口径 3：坏迁移报错停止，且事务回滚不留半成品。
func TestRunMigrationsStopsOnBrokenMigration(t *testing.T) {
	database := openSQLiteTestDB(t)

	broken := []*Migration{
		{
			ID: "9001_ok",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("CREATE TABLE IF NOT EXISTS mig_ok (id TEXT PRIMARY KEY)").Error
			},
		},
		{
			ID: "9002_broken",
			Up: func(tx *gorm.DB) error {
				if err := tx.Exec("CREATE TABLE IF NOT EXISTS mig_partial (id TEXT PRIMARY KEY)").Error; err != nil {
					return err
				}
				// 先建表再失败：验证事务把已执行的部分一起回滚。
				return fmt.Errorf("boom: intentional failure")
			},
		},
	}

	err := runMigrations(database, broken)
	if err == nil {
		t.Fatalf("broken migration did not fail")
	}
	if !strings.Contains(err.Error(), "9002_broken") {
		t.Fatalf("error does not name the broken migration: %v", err)
	}

	ids, err := AppliedMigrations(database)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(ids) != 1 || ids[0] != "9001_ok" {
		t.Fatalf("applied = %v, want only 9001_ok (broken one unstamped)", ids)
	}
	if !database.Migrator().HasTable("mig_ok") {
		t.Fatalf("migration before the broken one was not kept")
	}
	if database.Migrator().HasTable("mig_partial") {
		t.Fatalf("broken migration left partial changes behind; transaction did not roll back")
	}
}

// 降级保护：库里存在二进制不认识的迁移 ID 时拒绝迁移。
func TestMigrateRejectsUnknownNewerSchema(t *testing.T) {
	database := openSQLiteTestDB(t)
	if err := ensureMigrationTable(database); err != nil {
		t.Fatalf("ensure migration table: %v", err)
	}
	if err := database.Exec("INSERT INTO schema_migrations (id) VALUES ('9999_future')").Error; err != nil {
		t.Fatalf("stamp future migration: %v", err)
	}

	err := Migrate(database)
	if err == nil {
		t.Fatalf("Migrate accepted a schema newer than the binary")
	}
	if !strings.Contains(err.Error(), "9999_future") {
		t.Fatalf("error does not name the unknown migration: %v", err)
	}
}

func TestValidateMigrationsRejectsDuplicateIDs(t *testing.T) {
	duplicate := []*Migration{
		{ID: "9001_dup", Up: func(tx *gorm.DB) error { return nil }},
		{ID: "9001_dup", Up: func(tx *gorm.DB) error { return nil }},
	}
	if err := validateMigrations(duplicate); err == nil {
		t.Fatalf("validateMigrations accepted duplicate IDs")
	}
	if err := validateMigrations([]*Migration{{ID: "", Up: func(tx *gorm.DB) error { return nil }}}); err == nil {
		t.Fatalf("validateMigrations accepted empty ID")
	}
}

// withDatabaseName 替换 DSN 里的库名。必须走 URL 解析只改 Path：
// DSN 形如 postgres://aegisops:aegisops@host/aegisops，朴素字符串替换
// 会先命中用户名部分（"//aegisops:"），把用户名改坏导致连接失败。
func withDatabaseName(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + strings.TrimPrefix(name, "/")
	return u.String(), nil
}

func TestWithDatabaseNameRewritesOnlyPath(t *testing.T) {
	t.Parallel()

	got, err := withDatabaseName("postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable", "aegisops_migrate_test_42")
	if err != nil {
		t.Fatalf("withDatabaseName: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse rewritten DSN: %v", err)
	}
	if u.User.Username() != "aegisops" {
		t.Fatalf("username was rewritten: %q", u.User.Username())
	}
	if u.Path != "/aegisops_migrate_test_42" {
		t.Fatalf("path = %q, want the test database", u.Path)
	}
	if u.RawQuery != "sslmode=disable" {
		t.Fatalf("query lost: %q", u.RawQuery)
	}
}

// PostgreSQL 双驱动验证：需要显式提供 DSN（CI 无 Postgres 服务，自动跳过）。
// 测试在一次性数据库上运行，结束后删除，不触碰 DSN 指向的库。
func TestMigratePostgresCleanAndLegacyUpgrade(t *testing.T) {
	dsn := os.Getenv("AEGISOPS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AEGISOPS_TEST_POSTGRES_DSN not set; skipping postgres migration verification")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect postgres admin: %v", err)
	}
	sqlAdmin, err := admin.DB()
	if err != nil {
		t.Fatalf("get admin sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlAdmin.Close() })

	testDBName := fmt.Sprintf("aegisops_migrate_test_%d", rand.Int63())
	if err := admin.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName)).Error; err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", testDBName)).Error
	})

	testDSN, err := withDatabaseName(dsn, testDBName)
	if err != nil {
		t.Fatalf("rewrite test DSN: %v", err)
	}
	database, err := Open(config.DatabaseConfig{
		Driver: "postgres",
		DSN:    testDSN,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get test sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// 场景 1：干净库一键到最新。
	if err := Migrate(database); err != nil {
		t.Fatalf("postgres Migrate on clean database: %v", err)
	}
	ids, err := AppliedMigrations(database)
	if err != nil {
		t.Fatalf("postgres AppliedMigrations: %v", err)
	}
	if len(ids) != len(Migrations) {
		t.Fatalf("postgres applied = %v, want %d entries", ids, len(Migrations))
	}
	var indexCount int64
	if err := database.Raw("SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_task_dispatches_claim'").Scan(&indexCount).Error; err != nil {
		t.Fatalf("query pg_indexes: %v", err)
	}
	if indexCount != 1 {
		t.Fatalf("postgres idx_task_dispatches_claim count = %d, want 1", indexCount)
	}

	// 场景 2：存量库原地升级不丢数据。清掉重建 legacy 状态。
	if err := migrationBaselineV1_6_0.Down(database); err != nil {
		t.Fatalf("postgres baseline Down for legacy rebuild: %v", err)
	}
	if err := database.Exec("DROP TABLE IF EXISTS schema_migrations").Error; err != nil {
		t.Fatalf("drop schema_migrations for legacy rebuild: %v", err)
	}
	if err := database.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("postgres legacy AutoMigrate: %v", err)
	}
	legacyTask := model.Task{ID: "pg-legacy-task-1", Type: "noop", Title: "pg legacy", Status: model.TaskStatusPending, TargetType: "host", TargetID: "h1"}
	if err := database.Create(&legacyTask).Error; err != nil {
		t.Fatalf("seed postgres legacy task: %v", err)
	}

	if err := Migrate(database); err != nil {
		t.Fatalf("postgres Migrate on legacy database: %v", err)
	}
	var taskCount int64
	if err := database.Model(&model.Task{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count postgres tasks after upgrade: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("postgres legacy data lost: tasks=%d, want 1", taskCount)
	}
	indexCount = 0
	if err := database.Raw("SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_task_dispatches_claim'").Scan(&indexCount).Error; err != nil {
		t.Fatalf("query pg_indexes after upgrade: %v", err)
	}
	if indexCount != 1 {
		t.Fatalf("postgres upgraded legacy DB missing claim index")
	}
}
