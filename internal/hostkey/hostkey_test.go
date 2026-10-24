package hostkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"path/filepath"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func newHostWithDB(t *testing.T) (*gorm.DB, *model.Host, func()) {
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
	host := &model.Host{ID: "host-1", Name: "web-1", Address: "127.0.0.1", SSHUser: "root"}
	if err := database.Create(host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	return database, host, func() { _ = sqlDB.Close() }
}

func newSSHPublicKey(t *testing.T) (ssh.PublicKey, ssh.PublicKey) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	first, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate second key: %v", err)
	}
	second, err := ssh.NewPublicKey(otherPub)
	if err != nil {
		t.Fatalf("ssh second public key: %v", err)
	}
	return first, second
}

func TestCallbackTrustsOnFirstUseAndPins(t *testing.T) {
	t.Parallel()

	database, host, cleanup := newHostWithDB(t)
	defer cleanup()
	key, _ := newSSHPublicKey(t)

	// 未钉扎：首次信任并落库（TOFU）。
	if err := Callback(database, host)("", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("first connect should trust (TOFU): %v", err)
	}
	if host.HostPublicKey == "" {
		t.Fatal("host key should be pinned in memory")
	}
	var stored model.Host
	if err := database.First(&stored, "id = ?", host.ID).Error; err != nil {
		t.Fatalf("load host: %v", err)
	}
	if stored.HostPublicKey == "" {
		t.Fatal("host key should be persisted")
	}

	// 已钉扎：同一把钥匙放行。
	if err := Callback(database, host)("", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("second connect with same key should pass: %v", err)
	}
}

func TestCallbackRejectsChangedKey(t *testing.T) {
	t.Parallel()

	database, host, cleanup := newHostWithDB(t)
	defer cleanup()
	key, attackerKey := newSSHPublicKey(t)

	if err := Callback(database, host)("", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	pinned := host.HostPublicKey

	err := Callback(database, host)("", &net.TCPAddr{}, attackerKey)
	if !errors.Is(err, ErrKeyMismatch) {
		t.Fatalf("changed key err = %v, want ErrKeyMismatch", err)
	}
	if host.HostPublicKey != pinned {
		t.Fatal("mismatch must not rewrite the pinned key")
	}
}

func TestCallbackRejectsCorruptPinnedKey(t *testing.T) {
	t.Parallel()

	database, host, cleanup := newHostWithDB(t)
	defer cleanup()
	key, _ := newSSHPublicKey(t)

	host.HostPublicKey = "not-a-valid-key"
	if err := Callback(database, host)("", &net.TCPAddr{}, key); err == nil {
		t.Fatal("corrupt pinned key should fail closed")
	}
}
