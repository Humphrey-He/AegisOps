package model

func AllModels() []any {
	return []any{
		&User{},
		&Role{},
		&Permission{},
		&UserRole{},
		&RolePermission{},
		&AuditLog{},
		&Secret{},
		&Host{},
		&Task{},
		&TaskStep{},
		&TaskLog{},
		&DockerNode{},
	}
}
