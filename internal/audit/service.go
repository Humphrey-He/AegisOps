package audit

import (
	"context"
	"log"
	"sync"

	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	queue   chan model.AuditLog
	once    sync.Once
	writeMu sync.Mutex
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
	service := &Service{db: db, queue: make(chan model.AuditLog, 256)}
	service.startWorker()
	return service
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
	select {
	case s.queue <- log:
	default:
		go s.insert(context.Background(), log)
	}
	return nil
}

func (s *Service) startWorker() {
	s.once.Do(func() {
		go func() {
			for logItem := range s.queue {
				s.insert(context.Background(), logItem)
			}
		}()
	})
}

func (s *Service) insert(ctx context.Context, logItem model.AuditLog) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.db.WithContext(ctx).Create(&logItem).Error; err != nil {
		log.Printf("record audit log failed: %v", err)
	}
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
