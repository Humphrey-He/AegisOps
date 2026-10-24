package secret

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
)

func newTestService(t *testing.T) (*Service, func()) {
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
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	service, err := NewService(database, "test-master-key")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service, func() { _ = sqlDB.Close() }
}

func TestCreateAndDecryptRoundtrip(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	created, err := service.Create(ctx, CreateRequest{
		Name:    "prod-ssh",
		Type:    model.SecretTypeSSHPassword,
		Value:   "s3cret-password-123",
		Purpose: "host login",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 明文不允许出现在任何落库字段里。
	if strings.Contains(created.Ciphertext, "s3cret-password-123") {
		t.Fatal("ciphertext contains plaintext")
	}
	if created.MaskedValue == "s3cret-password-123" {
		t.Fatal("masked value leaks plaintext")
	}
	if created.Nonce == "" {
		t.Fatal("nonce is empty")
	}

	got, err := service.DecryptValue(ctx, created.ID)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "s3cret-password-123" {
		t.Fatalf("roundtrip got %q, want original value", got)
	}
}

func TestMaskBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"short all masked", "abc", "***"},
		{"six all masked", "abcdef", "******"},
		{"seven keeps head tail", "abcdefg", "abc*efg"},
		{"normal", "super-secret-value", "sup" + strings.Repeat("*", len("super-secret-value")-6) + "lue"},
		{"chinese runes by rune not byte", "一二三四五六七", "一二三*五六七"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Mask(tc.input); got != tc.want {
				t.Fatalf("Mask(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestDecryptForUseWritesAudit(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	created, err := service.Create(ctx, CreateRequest{
		Name:  "audit-me",
		Type:  model.SecretTypeSSHPrivateKey,
		Value: "-----BEGIN KEY-----xyz-----END KEY-----",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	use := UseContext{
		ResourceType: "host",
		ResourceID:   "host-1",
		Action:       "terminal.stream",
		OperatorID:   "user-1",
	}
	if _, err := service.DecryptValueForUse(ctx, created.ID, use); err != nil {
		t.Fatalf("decrypt for use: %v", err)
	}

	audits, total, err := service.ReadAudits(ctx, created.ID, 20, 0)
	if err != nil {
		t.Fatalf("read audits: %v", err)
	}
	if total != 1 || len(audits) != 1 {
		t.Fatalf("audits = %d/%d, want 1", len(audits), total)
	}
	audit := audits[0]
	if audit.Action != "terminal.stream" || audit.ResourceType != "host" ||
		audit.ResourceID != "host-1" || audit.OperatorID != "user-1" ||
		audit.Result != model.AuditResultSuccess {
		t.Fatalf("audit fields wrong: %+v", audit)
	}

	// 禁用后的解密必须失败，且失败也要留审计。
	if _, err := service.Update(ctx, created.ID, UpdateRequest{
		Status: strPtr(model.SecretStatusDisabled),
	}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := service.DecryptValueForUse(ctx, created.ID, use); err == nil {
		t.Fatal("decrypt disabled secret should fail")
	}
	_, total, err = service.ReadAudits(ctx, created.ID, 20, 0)
	if err != nil {
		t.Fatalf("read audits: %v", err)
	}
	if total != 2 {
		t.Fatalf("audits total = %d, want 2 (success + failure)", total)
	}
}

func TestDeleteBlockedWhileReferenced(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	created, err := service.Create(ctx, CreateRequest{
		Name:  "referenced",
		Type:  model.SecretTypeWebhook,
		Value: "https://hook.example.com/token",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete unreferenced secret should succeed: %v", err)
	}

	second, err := service.Create(ctx, CreateRequest{
		Name:  "referenced-2",
		Type:  model.SecretTypeWebhook,
		Value: "https://hook.example.com/token2",
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if err := service.UpsertReference(ctx, second.ID, "host", "host-9", "sshSecretId", "user-1"); err != nil {
		t.Fatalf("upsert reference: %v", err)
	}

	err = service.Delete(ctx, second.ID)
	if err == nil {
		t.Fatal("delete referenced secret should fail")
	}
	if !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("delete err = %v, want referenced-by message", err)
	}

	if err := service.DeleteReference(ctx, "host", "host-9", "sshSecretId"); err != nil {
		t.Fatalf("delete reference: %v", err)
	}
	if err := service.Delete(ctx, second.ID); err != nil {
		t.Fatalf("delete after reference removed: %v", err)
	}
}

func TestUpdateValueRotatesCiphertextAndKeyVersion(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	created, err := service.Create(ctx, CreateRequest{
		Name:  "rotate-me",
		Type:  model.SecretTypeAPIToken,
		Value: "first-value",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.KeyVersion != 1 {
		t.Fatalf("initial key version = %d, want 1", created.KeyVersion)
	}

	newValue := "second-value"
	updated, err := service.Update(ctx, created.ID, UpdateRequest{
		Value:      &newValue,
		OperatorID: "user-2",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.KeyVersion != 2 {
		t.Fatalf("key version = %d, want 2 after rotation", updated.KeyVersion)
	}
	if updated.LastRotatedAt == nil {
		t.Fatal("last rotated at is nil after rotation")
	}
	if updated.Ciphertext == created.Ciphertext || updated.Nonce == created.Nonce {
		t.Fatal("rotation should re-encrypt with a fresh nonce")
	}
	if updated.MaskedValue != Mask(newValue) {
		t.Fatalf("masked value = %q, want %q", updated.MaskedValue, Mask(newValue))
	}

	got, err := service.DecryptValue(ctx, created.ID)
	if err != nil {
		t.Fatalf("decrypt after rotation: %v", err)
	}
	if got != newValue {
		t.Fatalf("decrypted %q, want %q", got, newValue)
	}
}

func TestCreateRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	service, cleanup := newTestService(t)
	defer cleanup()

	if _, err := service.Create(context.Background(), CreateRequest{
		Name:  "bad",
		Type:  model.SecretType("does-not-exist"),
		Value: "x",
	}); err == nil {
		t.Fatal("create should reject unsupported secret type")
	}
}

func strPtr(v model.SecretStatus) *model.SecretStatus { return &v }
