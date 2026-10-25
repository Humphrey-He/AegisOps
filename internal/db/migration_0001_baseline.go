package db

import (
	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/gorm"
)

// migrationBaselineV1_6_0 把 v1.6.0 及之前由启动时 AutoMigrate 产出的全量 schema
// 冻结为 baseline。这是全项目唯一保留的 AutoMigrate 调用点：
//   - 干净库：由它建出全部表
//   - 存量库（v1.6.x 升级）：AutoMigrate 只增不删、幂等无害，执行后打戳，
//     相当于把存量库登记进版本管理，数据不动
//
// Down 仅用于开发/测试重建，生产只前向迁移。
var migrationBaselineV1_6_0 = &Migration{
	ID:          "0001_baseline_v1_6_0",
	Description: "freeze v1.6.0 AutoMigrate schema as baseline",
	Up: func(tx *gorm.DB) error {
		return tx.AutoMigrate(model.AllModels()...)
	},
	Down: func(tx *gorm.DB) error {
		// AllModels 父表在前子表在后，逆序 DROP 避开外键依赖。
		all := model.AllModels()
		for i := len(all) - 1; i >= 0; i-- {
			if err := tx.Migrator().DropTable(all[i]); err != nil {
				return err
			}
		}
		return nil
	},
}
