// 有序迁移列表：列表顺序即执行顺序，新迁移只允许追加到末尾。
//
// 冻结纪律（自 0002 起）：
//   - 修改 model（加列/改列/加索引）必须配套一条编号迁移，
//     在迁移里对存量库执行等效变更——只有 0001 baseline 允许全量 AutoMigrate
//   - 只改 model 不加迁移会导致：新库（0001 建出最新 schema）与存量库
//     （走增量迁移）结构漂移，这是本机制要防住的唯一错误
//   - Up 必须同时兼容 SQLite 与 PostgreSQL；方言差异在迁移内部分支处理
package db

// Migrations 是按序执行的全部迁移，ID 前缀 4 位数字保证可读的先后关系。
var Migrations = []*Migration{
	migrationBaselineV1_6_0,
	migrationDispatchClaimIndex,
	migrationOIDCSSO,
}
