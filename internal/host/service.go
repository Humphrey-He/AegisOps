package host

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
	timeout time.Duration
}

type CreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Address     string `json:"address" binding:"required"`
	SSHPort     int    `json:"sshPort"`
	SSHUser     string `json:"sshUser" binding:"required"`
	SSHSecretID string `json:"sshSecretId" binding:"required"`
	Group       string `json:"group"`
	Tags        string `json:"tags"`
	OperatorID  string `json:"-"`
}

type UpdateRequest struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	SSHPort     int    `json:"sshPort"`
	SSHUser     string `json:"sshUser"`
	SSHSecretID string `json:"sshSecretId"`
	Group       string `json:"group"`
	Tags        string `json:"tags"`
	OperatorID  string `json:"-"`
}

func NewService(db *gorm.DB, secrets *secret.Service) *Service {
	return &Service{db: db, secrets: secrets, timeout: 8 * time.Second}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Host, error) {
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	item := &model.Host{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Address:     req.Address,
		SSHPort:     req.SSHPort,
		SSHUser:     req.SSHUser,
		SSHSecretID: req.SSHSecretID,
		Group:       req.Group,
		Tags:        req.Tags,
		Status:      model.HostStatusUnknown,
		CreatedBy:   req.OperatorID,
		UpdatedBy:   req.OperatorID,
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) List(ctx context.Context, keyword string, limit, offset int) ([]model.Host, int64, error) {
	var items []model.Host
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Host{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR address LIKE ? OR tags LIKE ?", like, like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.Host, error) {
	var item model.Host
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*model.Host, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Address != "" {
		item.Address = req.Address
	}
	if req.SSHPort != 0 {
		item.SSHPort = req.SSHPort
	}
	if req.SSHUser != "" {
		item.SSHUser = req.SSHUser
	}
	if req.SSHSecretID != "" {
		item.SSHSecretID = req.SSHSecretID
	}
	item.Group = req.Group
	item.Tags = req.Tags
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.Host{}, "id = ?", id).Error
}

func (s *Service) TestSSH(ctx context.Context, id string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	value, err := s.secrets.DecryptValue(ctx, item.SSHSecretID)
	if err != nil {
		return err
	}
	secretItem, err := s.secrets.Get(ctx, item.SSHSecretID)
	if err != nil {
		return err
	}
	auth, err := sshAuthMethod(secretItem.Type, value)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User:            item.SSHUser,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         s.timeout,
	}
	address := net.JoinHostPort(item.Address, fmt.Sprintf("%d", item.SSHPort))
	client, err := ssh.Dial("tcp", address, config)
	now := time.Now().UTC()
	status := model.HostStatusOnline
	if err != nil {
		status = model.HostStatusOffline
	}
	_ = s.db.WithContext(ctx).Model(&model.Host{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"last_test_at": &now,
	}).Error
	if err != nil {
		return err
	}
	return client.Close()
}

func sshAuthMethod(secretType model.SecretType, value string) (ssh.AuthMethod, error) {
	switch secretType {
	case model.SecretTypeSSHPassword:
		return ssh.Password(value), nil
	case model.SecretTypeSSHPrivateKey:
		signer, err := ssh.ParsePrivateKey([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("secret type %s cannot be used for ssh", secretType)
	}
}
