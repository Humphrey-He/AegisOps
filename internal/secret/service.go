package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

var ErrUnsupportedSecretType = errors.New("unsupported secret type")

type Service struct {
	db  *gorm.DB
	aes cipher.AEAD
}

type CreateRequest struct {
	Name        string           `json:"name" binding:"required"`
	Type        model.SecretType `json:"type" binding:"required"`
	Description string           `json:"description"`
	Value       string           `json:"value" binding:"required"`
	OperatorID  string           `json:"-"`
}

type UpdateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Value       *string `json:"value"`
	OperatorID  string  `json:"-"`
}

func NewService(db *gorm.DB, masterKey string) (*Service, error) {
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm: %w", err)
	}
	return &Service{db: db, aes: aesgcm}, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Secret, error) {
	if !isSupportedType(req.Type) {
		return nil, ErrUnsupportedSecretType
	}
	nonce, ciphertext, err := s.encrypt(req.Value)
	if err != nil {
		return nil, err
	}
	secret := &model.Secret{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Ciphertext:  ciphertext,
		Nonce:       nonce,
		MaskedValue: Mask(req.Value),
		CreatedBy:   req.OperatorID,
		UpdatedBy:   req.OperatorID,
	}
	return secret, s.db.WithContext(ctx).Create(secret).Error
}

func (s *Service) List(ctx context.Context, keyword string, limit, offset int) ([]model.Secret, int64, error) {
	var items []model.Secret
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Secret{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
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

func (s *Service) Get(ctx context.Context, id string) (*model.Secret, error) {
	var item model.Secret
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DecryptValue(ctx context.Context, id string) (string, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return s.decrypt(item.Nonce, item.Ciphertext)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*model.Secret, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Value != nil {
		nonce, ciphertext, err := s.encrypt(*req.Value)
		if err != nil {
			return nil, err
		}
		item.Nonce = nonce
		item.Ciphertext = ciphertext
		item.MaskedValue = Mask(*req.Value)
	}
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.Secret{}, "id = ?", id).Error
}

func (s *Service) encrypt(plaintext string) (string, string, error) {
	nonce := make([]byte, s.aes.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := s.aes.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Service) decrypt(encodedNonce, encodedCiphertext string) (string, error) {
	nonce, err := base64.StdEncoding.DecodeString(encodedNonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	plaintext, err := s.aes.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func Mask(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 6 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:3]) + strings.Repeat("*", len(runes)-6) + string(runes[len(runes)-3:])
}

func isSupportedType(secretType model.SecretType) bool {
	switch secretType {
	case model.SecretTypeSSHPassword, model.SecretTypeSSHPrivateKey, model.SecretTypeDockerTLS, model.SecretTypeDockerToken:
		return true
	default:
		return false
	}
}
