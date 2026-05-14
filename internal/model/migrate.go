package model

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Role{},
		&Permission{},
		&UserRole{},
		&RolePermission{},
		&AuditLog{},
	)
}
