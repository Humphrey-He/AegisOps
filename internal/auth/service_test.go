package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	appErrors "github.com/Humphrey-He/AegisOps/pkg/errors"
)

func newAuthService(t *testing.T) (*Service, func()) {
	t.Helper()
	database, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	service := NewService(database, Config{})
	return service, func() { _ = sqlDB.Close() }
}

func TestInitAdminRequiresPasswordChange(t *testing.T) {
	t.Parallel()

	service, cleanup := newAuthService(t)
	defer cleanup()
	ctx := context.Background()

	admin, err := service.InitAdmin(ctx)
	if err != nil {
		t.Fatalf("init admin: %v", err)
	}
	if !admin.MustChangePassword {
		t.Fatal("bootstrapped admin must carry MustChangePassword=true")
	}
}

func TestChangePasswordFlow(t *testing.T) {
	t.Parallel()

	service, cleanup := newAuthService(t)
	defer cleanup()
	ctx := context.Background()

	admin, err := service.InitAdmin(ctx)
	if err != nil {
		t.Fatalf("init admin: %v", err)
	}

	// 错误旧口令 → 拒绝且标记不清。
	if err := service.ChangePassword(ctx, admin.ID, "wrong-old", "new-password-1"); !errors.Is(err, appErrors.ErrInvalidCredentials) {
		t.Fatalf("wrong old password err = %v, want ErrInvalidCredentials", err)
	}

	// 新旧相同 / 过短 → 输入非法。
	if err := service.ChangePassword(ctx, admin.ID, "admin123456", "admin123456"); !errors.Is(err, appErrors.ErrInvalidInput) {
		t.Fatalf("same password err = %v, want ErrInvalidInput", err)
	}
	if err := service.ChangePassword(ctx, admin.ID, "admin123456", "short"); !errors.Is(err, appErrors.ErrInvalidInput) {
		t.Fatalf("short password err = %v, want ErrInvalidInput", err)
	}

	// 正确改密 → 标记清除，新口令可登录，旧口令失效。
	if err := service.ChangePassword(ctx, admin.ID, "admin123456", "aegisops-new-pass"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := service.Login(ctx, "admin", "admin123456"); !errors.Is(err, appErrors.ErrInvalidCredentials) {
		t.Fatalf("old password should stop working, err = %v", err)
	}
	result, err := service.Login(ctx, "admin", "aegisops-new-pass")
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if result.User.MustChangePassword {
		t.Fatal("flag should be cleared after password change")
	}
}
