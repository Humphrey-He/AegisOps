package rbac

import (
	"context"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) HasPermission(ctx context.Context, userID uint, permissionCode string) (bool, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, userID).Error; err != nil {
		return false, err
	}
	if user.IsAdmin {
		return true, nil
	}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			if permission.Code == permissionCode {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Service) EnsurePermission(ctx context.Context, userID uint, permissionCode string) error {
	ok, err := s.HasPermission(ctx, userID, permissionCode)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
