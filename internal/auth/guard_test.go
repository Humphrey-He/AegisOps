package auth

import (
	"testing"
	"time"
)

func newTestGuard(now func() time.Time) *LoginGuard {
	guard := NewLoginGuard()
	guard.now = now
	return guard
}

func TestLoginGuardLocksAfterThreshold(t *testing.T) {
	t.Parallel()

	current := time.Now()
	guard := newTestGuard(func() time.Time { return current })
	key := "admin@1.2.3.4"

	for i := 0; i < 4; i++ {
		guard.RecordFailure(key)
		if remaining := guard.LockedRemaining(key); remaining != 0 {
			t.Fatalf("locked after %d failures, want unlocked until threshold", i+1)
		}
	}

	guard.RecordFailure(key) // 第 5 次
	remaining := guard.LockedRemaining(key)
	if remaining <= 0 {
		t.Fatal("should be locked after 5th failure")
	}
	if remaining > 15*time.Minute {
		t.Fatalf("remaining = %v, want <= lockout duration", remaining)
	}
}

func TestLoginGuardResetClearsFailures(t *testing.T) {
	t.Parallel()

	current := time.Now()
	guard := newTestGuard(func() time.Time { return current })
	key := "admin@1.2.3.4"

	for i := 0; i < 5; i++ {
		guard.RecordFailure(key)
	}
	if guard.LockedRemaining(key) == 0 {
		t.Fatal("should be locked before reset")
	}

	guard.Reset(key)
	if remaining := guard.LockedRemaining(key); remaining != 0 {
		t.Fatalf("after reset remaining = %v, want 0", remaining)
	}
}

func TestLoginGuardLockExpires(t *testing.T) {
	t.Parallel()

	current := time.Now()
	guard := newTestGuard(func() time.Time { return current })
	key := "admin@1.2.3.4"

	for i := 0; i < 5; i++ {
		guard.RecordFailure(key)
	}
	if guard.LockedRemaining(key) == 0 {
		t.Fatal("should be locked right after failures")
	}

	// 推进到锁定期结束之后。
	current = current.Add(16 * time.Minute)
	if remaining := guard.LockedRemaining(key); remaining != 0 {
		t.Fatalf("lock should expire, remaining = %v", remaining)
	}
}

func TestLoginGuardKeysAreIndependent(t *testing.T) {
	t.Parallel()

	current := time.Now()
	guard := newTestGuard(func() time.Time { return current })

	for i := 0; i < 5; i++ {
		guard.RecordFailure("admin@1.1.1.1")
	}
	if remaining := guard.LockedRemaining("admin@2.2.2.2"); remaining != 0 {
		t.Fatalf("other key locked unexpectedly: %v", remaining)
	}
}
