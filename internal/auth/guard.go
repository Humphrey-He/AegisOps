package auth

import (
	"sync"
	"time"
)

// LoginGuard 按「用户名 + 客户端 IP」对登录失败做窗口限流：
// 窗口内失败达到阈值后锁定一段时间，锁定期内直接拒绝，不再碰 bcrypt。
// 只统计失败；成功登录清零。单机内存实现，多实例部署时应换共享存储。
type LoginGuard struct {
	mu          sync.Mutex
	failures    map[string][]time.Time
	maxAttempts int
	window      time.Duration
	lockout     time.Duration
	now         func() time.Time
}

func NewLoginGuard() *LoginGuard {
	return &LoginGuard{
		failures:    map[string][]time.Time{},
		maxAttempts: 5,
		window:      15 * time.Minute,
		lockout:     15 * time.Minute,
		now:         time.Now,
	}
}

// LockedRemaining 返回 key 剩余锁定时长，0 表示当前未被锁定。
func (g *LoginGuard) LockedRemaining(key string) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	attempts := g.prune(key, now, g.lockout)
	if len(attempts) >= g.maxAttempts {
		if end := attempts[len(attempts)-1].Add(g.lockout); end.After(now) {
			return end.Sub(now)
		}
	}
	return 0
}

// RecordFailure 记录一次失败。达到阈值后即进入锁定（从最后一次失败起算）。
func (g *LoginGuard) RecordFailure(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	attempts := g.prune(key, now, g.window)
	g.failures[key] = append(attempts, now)
}

// Reset 在登录成功后清空该 key 的失败记录。
func (g *LoginGuard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.failures, key)
}

// prune 移除超过 maxAge 的记录并返回剩余记录；空则删除 map 项防止无限增长。
func (g *LoginGuard) prune(key string, now time.Time, maxAge time.Duration) []time.Time {
	attempts := g.failures[key]
	kept := make([]time.Time, 0, len(attempts))
	for _, at := range attempts {
		if now.Sub(at) < maxAge {
			kept = append(kept, at)
		}
	}
	if len(kept) > 0 {
		g.failures[key] = kept
	} else {
		delete(g.failures, key)
	}
	return kept
}
