package model

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

// AuthSourceLocal 本地用户名密码账号。
const AuthSourceLocal = "local"

type User struct {
	BaseModel
	Username     string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"size:255;not null"`
	DisplayName  string     `json:"displayName" gorm:"size:128"`
	Email        string     `json:"email" gorm:"size:255;index"`
	Status       UserStatus `json:"status" gorm:"size:32;not null;default:active"`
	IsAdmin      bool       `json:"isAdmin" gorm:"not null;default:false"`
	// MustChangePassword 首次以初始口令登录后必须先改密才能使用平台。
	MustChangePassword bool       `json:"mustChangePassword" gorm:"not null;default:false"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
	Roles              []Role     `json:"roles,omitempty" gorm:"many2many:user_roles;"`
	// AuthSource 账号来源：local（默认）或 "oidc:<provider id>"。
	// 非 local 的账号不参与密码登录/改密（PasswordHash 为随机不可用值）。
	AuthSource string `json:"-" gorm:"size:64;not null;default:local"`
	// ExternalProvider/ExternalSubject 是 SSO 身份锚点（provider id + subject），
	// 联合唯一；本地账号保持 NULL（NULL 在两种方言的唯一索引中互不冲突）。
	ExternalProvider *string `json:"-" gorm:"size:64;uniqueIndex:idx_users_external_identity"`
	ExternalSubject  *string `json:"-" gorm:"size:128;uniqueIndex:idx_users_external_identity"`
}

type Role struct {
	BaseModel
	Name        string       `json:"name" gorm:"size:64;uniqueIndex;not null"`
	Code        string       `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Description string       `json:"description" gorm:"size:255"`
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
}

type Permission struct {
	BaseModel
	Name        string `json:"name" gorm:"size:128;not null"`
	Code        string `json:"code" gorm:"size:128;uniqueIndex;not null"`
	Resource    string `json:"resource" gorm:"size:64;index"`
	Action      string `json:"action" gorm:"size:64;index"`
	Description string `json:"description" gorm:"size:255"`
}

type UserRole struct {
	UserID    uint      `json:"userId" gorm:"primaryKey"`
	RoleID    uint      `json:"roleId" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
}

type RolePermission struct {
	RoleID       uint      `json:"roleId" gorm:"primaryKey"`
	PermissionID uint      `json:"permissionId" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"createdAt"`
}

// OIDCState 是 /auth/oidc/:id/start 签发的一次性授权状态（防 CSRF + 承载 PKCE verifier）。
// 存 DB 而非进程内存：多实例就绪；过期条目在签发新 state 时顺带清理。
type OIDCState struct {
	State      string    `gorm:"primaryKey;size:64" json:"-"`
	ProviderID string    `gorm:"size:64;not null;index" json:"-"`
	Verifier   string    `gorm:"size:128;not null" json:"-"`
	ExpiresAt  time.Time `gorm:"not null;index" json:"-"`
	CreatedAt  time.Time `json:"-"`
}

// OIDCExchangeCode 是回调签发给前端的一次性短时 code（60 秒），
// 前端用它换正式 token；避免 JWT 进入 URL（referrer/历史记录泄漏面）。
type OIDCExchangeCode struct {
	Code      string    `gorm:"primaryKey;size:64" json:"-"`
	UserID    uint      `gorm:"not null;index" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"-"`
	Used      bool      `gorm:"not null;default:false" json:"-"`
	CreatedAt time.Time `json:"-"`
}
