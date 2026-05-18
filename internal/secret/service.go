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
	"time"

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
	Purpose     string           `json:"purpose"`
	Value       string           `json:"value" binding:"required"`
	ExpiresAt   *time.Time       `json:"expiresAt"`
	OperatorID  string           `json:"-"`
}

type UpdateRequest struct {
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	Purpose     *string             `json:"purpose"`
	Status      *model.SecretStatus `json:"status"`
	Value       *string             `json:"value"`
	ExpiresAt   *time.Time          `json:"expiresAt"`
	OperatorID  string              `json:"-"`
}

type RotateRequest struct {
	Value      string `json:"value" binding:"required"`
	OperatorID string `json:"-"`
}

type UseContext struct {
	ResourceType string
	ResourceID   string
	Action       string
	OperatorID   string
	TaskID       string
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
		Purpose:     strings.TrimSpace(req.Purpose),
		Status:      model.SecretStatusActive,
		Ciphertext:  ciphertext,
		Nonce:       nonce,
		MaskedValue: Mask(req.Value),
		KeyVersion:  1,
		ExpiresAt:   req.ExpiresAt,
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
	return s.DecryptValueForUse(ctx, id, UseContext{Action: "secret.decrypt"})
}

func (s *Service) DecryptValueForUse(ctx context.Context, id string, use UseContext) (string, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		_ = s.recordReadAudit(ctx, id, use, model.AuditResultFailure, err.Error())
		return "", err
	}
	if item.Status == model.SecretStatusDisabled {
		err := fmt.Errorf("secret is disabled")
		_ = s.recordReadAudit(ctx, id, use, model.AuditResultFailure, err.Error())
		return "", err
	}
	value, err := s.decrypt(item.Nonce, item.Ciphertext)
	if err != nil {
		_ = s.recordReadAudit(ctx, id, use, model.AuditResultFailure, err.Error())
		return "", err
	}
	_ = s.recordReadAudit(ctx, id, use, model.AuditResultSuccess, "")
	return value, nil
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
	if req.Purpose != nil {
		item.Purpose = strings.TrimSpace(*req.Purpose)
	}
	if req.Status != nil {
		item.Status = *req.Status
	}
	if req.Value != nil {
		nonce, ciphertext, err := s.encrypt(*req.Value)
		if err != nil {
			return nil, err
		}
		item.Nonce = nonce
		item.Ciphertext = ciphertext
		item.MaskedValue = Mask(*req.Value)
		item.KeyVersion++
		now := time.Now().UTC()
		item.LastRotatedAt = &now
	}
	item.ExpiresAt = req.ExpiresAt
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Rotate(ctx context.Context, id string, req RotateRequest) (*model.Secret, error) {
	value := req.Value
	return s.Update(ctx, id, UpdateRequest{
		Value:      &value,
		OperatorID: req.OperatorID,
	})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.SecretReference{}).Where("secret_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("secret is referenced by %d resource(s)", count)
	}
	return s.db.WithContext(ctx).Delete(&model.Secret{}, "id = ?", id).Error
}

func (s *Service) References(ctx context.Context, secretID string) ([]model.SecretReference, error) {
	var items []model.SecretReference
	err := s.db.WithContext(ctx).Where("secret_id = ?", secretID).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (s *Service) ReadAudits(ctx context.Context, secretID string, limit, offset int) ([]model.SecretReadAudit, int64, error) {
	var items []model.SecretReadAudit
	var total int64
	query := s.db.WithContext(ctx).Model(&model.SecretReadAudit{}).Where("secret_id = ?", secretID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) UpsertReference(ctx context.Context, secretID, resourceType, resourceID, fieldName, operatorID string) error {
	if strings.TrimSpace(secretID) == "" || strings.TrimSpace(resourceType) == "" || strings.TrimSpace(resourceID) == "" {
		return nil
	}
	var existing model.SecretReference
	err := s.db.WithContext(ctx).Where("resource_type = ? AND resource_id = ? AND field_name = ?", resourceType, resourceID, fieldName).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&model.SecretReference{
			ID:           uuid.NewString(),
			SecretID:     secretID,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			FieldName:    fieldName,
			CreatedBy:    operatorID,
		}).Error
	}
	if err != nil {
		return err
	}
	existing.SecretID = secretID
	existing.CreatedBy = operatorID
	return s.db.WithContext(ctx).Save(&existing).Error
}

func (s *Service) DeleteReference(ctx context.Context, resourceType, resourceID, fieldName string) error {
	return s.db.WithContext(ctx).Where("resource_type = ? AND resource_id = ? AND field_name = ?", resourceType, resourceID, fieldName).Delete(&model.SecretReference{}).Error
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
	case model.SecretTypeSSHPassword, model.SecretTypeSSHPrivateKey, model.SecretTypeDockerTLS, model.SecretTypeDockerToken, model.SecretTypeWebhook, model.SecretTypeAPIToken, model.SecretTypeSMTP:
		return true
	default:
		return false
	}
}

func (s *Service) recordReadAudit(ctx context.Context, secretID string, use UseContext, result model.AuditResult, errMessage string) error {
	action := strings.TrimSpace(use.Action)
	if action == "" {
		action = "secret.decrypt"
	}
	return s.db.WithContext(ctx).Create(&model.SecretReadAudit{
		ID:           uuid.NewString(),
		SecretID:     secretID,
		ResourceType: use.ResourceType,
		ResourceID:   use.ResourceID,
		Action:       action,
		OperatorID:   use.OperatorID,
		TaskID:       use.TaskID,
		Result:       result,
		ErrorMessage: errMessage,
	}).Error
}
