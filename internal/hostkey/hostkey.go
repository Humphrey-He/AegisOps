// Package hostkey 实现 SSH 主机公钥的 TOFU（首次信任）+ 钉扎校验。
// 语义等同 known_hosts，但钉扎值存在 Host 记录里：无状态容器友好，
// 也不依赖运维手工管理 known_hosts 文件。
package hostkey

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

// ErrKeyMismatch 表示主机公钥与已钉扎值不一致——同一台主机的密钥变了，
// 最常见的原因是中间人攻击（或主机重装）。必须拒绝连接。
var ErrKeyMismatch = errors.New("ssh host key mismatch: server key changed, possible MITM")

// Callback 返回可直接挂到 ssh.ClientConfig.HostKeyCallback 的校验函数：
// 未钉扎时信任一次并落库（TOFU）；已钉扎后严格比对公钥字节。
// 已知边界：TOFU 防不了"第一次连接就被中间人"，那是带外校验的职责。
func Callback(db *gorm.DB, host *model.Host) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		pinned := strings.TrimSpace(host.HostPublicKey)
		if pinned == "" {
			return trust(db, host, key)
		}
		parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pinned))
		if err != nil {
			return fmt.Errorf("parse pinned host key: %w", err)
		}
		if !bytes.Equal(parsed.Marshal(), key.Marshal()) {
			return ErrKeyMismatch
		}
		return nil
	}
}

func trust(db *gorm.DB, host *model.Host, key ssh.PublicKey) error {
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if err := db.Model(&model.Host{}).Where("id = ?", host.ID).Update("host_public_key", line).Error; err != nil {
		return fmt.Errorf("persist host key: %w", err)
	}
	host.HostPublicKey = line
	return nil
}
