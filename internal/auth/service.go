package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/model"
	appErrors "github.com/Humphrey-He/AegisOps/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Config struct {
	JWTSecret          string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	InitialAdminUser   string
	InitialAdminPass   string
	InitialAdminEmail  string
	InitialDisplayName string
}

type Service struct {
	db     *gorm.DB
	config Config
	now    func() time.Time
}

type Claims struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	TokenTyp string `json:"tokenType"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken           string    `json:"accessToken"`
	RefreshToken          string    `json:"refreshToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	TokenType             string    `json:"tokenType"`
}

type LoginResult struct {
	User   model.User `json:"user"`
	Tokens TokenPair  `json:"tokens"`
}

func NewService(db *gorm.DB, config Config) *Service {
	if config.AccessTokenTTL == 0 {
		config.AccessTokenTTL = 2 * time.Hour
	}
	if config.RefreshTokenTTL == 0 {
		config.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if config.JWTSecret == "" {
		config.JWTSecret = "aegisops-dev-secret"
	}
	return &Service{db: db, config: config, now: time.Now}
}

func (s *Service) InitAdmin(ctx context.Context) (*model.User, error) {
	username := s.config.InitialAdminUser
	if username == "" {
		username = "admin"
	}
	password := s.config.InitialAdminPass
	if password == "" {
		password = "admin123456"
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  firstNonEmpty(s.config.InitialDisplayName, "Administrator"),
		Email:        s.config.InitialAdminEmail,
		Status:       model.UserStatusActive,
		IsAdmin:      true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permissions, err := seedPermissions(ctx, tx)
		if err != nil {
			return err
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		role := model.Role{Name: "Administrator", Code: "admin", Description: "Built-in administrator role"}
		if err := tx.FirstOrCreate(&role, model.Role{Code: role.Code}).Error; err != nil {
			return err
		}
		for _, permission := range permissions {
			if err := tx.FirstOrCreate(&model.RolePermission{}, model.RolePermission{
				RoleID:       role.ID,
				PermissionID: permission.ID,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID}).Error
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func seedPermissions(ctx context.Context, tx *gorm.DB) ([]model.Permission, error) {
	definitions := []model.Permission{
		{Name: "Dashboard View", Code: "dashboard.view", Resource: "dashboard", Action: "view"},
		{Name: "Users View", Code: "users.view", Resource: "users", Action: "view"},
		{Name: "Users Manage", Code: "users.manage", Resource: "users", Action: "manage"},
		{Name: "Roles View", Code: "roles.view", Resource: "roles", Action: "view"},
		{Name: "Roles Manage", Code: "roles.manage", Resource: "roles", Action: "manage"},
		{Name: "Secrets View", Code: "secrets.view", Resource: "secrets", Action: "view"},
		{Name: "Secrets Manage", Code: "secrets.manage", Resource: "secrets", Action: "manage"},
		{Name: "Registries View", Code: "registries.view", Resource: "registries", Action: "view"},
		{Name: "Registries Manage", Code: "registries.manage", Resource: "registries", Action: "manage"},
		{Name: "Services View", Code: "services.view", Resource: "services", Action: "view"},
		{Name: "Services Manage", Code: "services.manage", Resource: "services", Action: "manage"},
		{Name: "Services Release", Code: "services.release", Resource: "services", Action: "release"},
		{Name: "Hosts View", Code: "hosts.view", Resource: "hosts", Action: "view"},
		{Name: "Hosts Manage", Code: "hosts.manage", Resource: "hosts", Action: "manage"},
		{Name: "Terminal Open", Code: "terminal.open", Resource: "terminal", Action: "open"},
		{Name: "Docker View", Code: "docker.view", Resource: "docker", Action: "view"},
		{Name: "Docker Manage", Code: "docker.manage", Resource: "docker", Action: "manage"},
		{Name: "Tasks View", Code: "tasks.view", Resource: "tasks", Action: "view"},
		{Name: "Audits View", Code: "audits.view", Resource: "audits", Action: "view"},
	}
	permissions := make([]model.Permission, 0, len(definitions))
	for _, definition := range definitions {
		permission := definition
		if err := tx.WithContext(ctx).Where("code = ?", definition.Code).FirstOrCreate(&permission, definition).Error; err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	var user model.User
	err := s.db.WithContext(ctx).Preload("Roles.Permissions").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, appErrors.ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, appErrors.ErrInvalidCredentials
	}

	now := s.now().UTC()
	user.LastLoginAt = &now
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("last_login_at", now).Error; err != nil {
		return nil, err
	}
	tokens, err := s.issueTokenPair(user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: user, Tokens: *tokens}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}
	user, err := s.GetCurrentUser(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	return s.issueTokenPair(*user)
}

func (s *Service) GetCurrentUser(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, appErrors.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, appErrors.ErrUserDisabled
	}
	return &user, nil
}

func (s *Service) ParseToken(rawToken, expectedType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, appErrors.ErrUnauthorized
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, appErrors.ErrUnauthorized
	}
	if expectedType != "" && claims.TokenTyp != expectedType {
		return nil, appErrors.ErrUnauthorized
	}
	return claims, nil
}

func (s *Service) HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", appErrors.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (s *Service) issueTokenPair(user model.User) (*TokenPair, error) {
	now := s.now().UTC()
	accessExpiresAt := now.Add(s.config.AccessTokenTTL)
	refreshExpiresAt := now.Add(s.config.RefreshTokenTTL)

	access, err := s.signToken(user, "access", now, accessExpiresAt)
	if err != nil {
		return nil, err
	}
	refresh, err := s.signToken(user, "refresh", now, refreshExpiresAt)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:           access,
		RefreshToken:          refresh,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshTokenExpiresAt: refreshExpiresAt,
		TokenType:             "Bearer",
	}, nil
}

func (s *Service) signToken(user model.User, tokenType string, issuedAt, expiresAt time.Time) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		TokenTyp: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Username,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.JWTSecret))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
