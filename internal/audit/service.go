package audit

import (
	"context"

	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

type Entry struct {
	UserID       *uint
	Username     string
	Action       string
	ResourceType string
	ResourceID   string
	Result       model.AuditResult
	Message      string
	IPAddress    string
	UserAgent    string
	TraceID      string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, entry Entry) error {
	if entry.Result == "" {
		entry.Result = model.AuditResultSuccess
	}
	log := model.AuditLog{
		UserID:       entry.UserID,
		Username:     entry.Username,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Result:       entry.Result,
		Message:      entry.Message,
		IPAddress:    entry.IPAddress,
		UserAgent:    entry.UserAgent,
		TraceID:      entry.TraceID,
	}
	return s.db.WithContext(ctx).Create(&log).Error
}

func (s *Service) RecordGin(c *gin.Context, entry Entry) error {
	entry.IPAddress = firstNonEmpty(entry.IPAddress, c.ClientIP())
	entry.UserAgent = firstNonEmpty(entry.UserAgent, c.Request.UserAgent())
	entry.TraceID = firstNonEmpty(entry.TraceID, contextString(c, "traceId"), contextString(c, "requestId"), c.GetHeader("X-Request-Id"))
	if userValue, ok := c.Get(auth.CurrentUserKey); ok {
		if user, ok := userValue.(*model.User); ok {
			entry.UserID = &user.ID
			entry.Username = firstNonEmpty(entry.Username, user.Username)
		}
	}
	return s.Record(c.Request.Context(), entry)
}

func contextString(c *gin.Context, key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
