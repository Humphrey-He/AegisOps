package db

import (
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/gorm"
)

// migrationOIDCSSO 为 SSO 登录补齐用户身份列与两张 OIDC 会话辅助表。
// 幂等：新库的列/表已由 0001（AllModels AutoMigrate）建出，这里逐项 Has 判断后跳过；
// 存量库（v1.7.x 升级）则在此真实补建。
// 外部身份联合唯一索引依赖 NULL 互不冲突的语义（本地账号两列均为 NULL），
// SQLite 与 PostgreSQL 的唯一索引默认都满足该语义。
var migrationOIDCSSO = &Migration{
	ID:          "0003_oidc_sso",
	Description: "user external identity columns and oidc state/exchange tables",
	Up: func(tx *gorm.DB) error {
		for _, column := range []string{"auth_source", "external_provider", "external_subject"} {
			if !tx.Migrator().HasColumn(&model.User{}, column) {
				if err := tx.Migrator().AddColumn(&model.User{}, column); err != nil {
					return err
				}
			}
		}
		if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external_identity
			ON users (external_provider, external_subject)`).Error; err != nil {
			return err
		}
		for _, table := range []any{&model.OIDCState{}, &model.OIDCExchangeCode{}} {
			if !tx.Migrator().HasTable(table) {
				if err := tx.Migrator().CreateTable(table); err != nil {
					return err
				}
			}
		}
		return nil
	},
	Down: func(tx *gorm.DB) error {
		if err := tx.Exec(`DROP INDEX IF EXISTS idx_users_external_identity`).Error; err != nil {
			return err
		}
		for _, column := range []string{"external_subject", "external_provider", "auth_source"} {
			if tx.Migrator().HasColumn(&model.User{}, column) {
				if err := tx.Migrator().DropColumn(&model.User{}, column); err != nil {
					return err
				}
			}
		}
		for _, table := range []any{&model.OIDCExchangeCode{}, &model.OIDCState{}} {
			if tx.Migrator().HasTable(table) {
				if err := tx.Migrator().DropTable(table); err != nil {
					return err
				}
			}
		}
		return nil
	},
}
