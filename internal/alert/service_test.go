package alert

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/notification"
)

func TestCreateRuleAppliesDefaultsAndNormalizesLanguage(t *testing.T) {
	database := openTestDB(t)
	service := NewService(database, notification.NewService(database))

	rule, err := service.CreateRule(context.Background(), RuleRequest{
		Name:       "Service release failed",
		EventType:  "service_release_failed",
		Language:   "english",
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if !rule.Enabled {
		t.Fatal("rule should default to enabled")
	}
	if rule.DedupeWindowSeconds != 300 {
		t.Fatalf("dedupe window = %d, want 300", rule.DedupeWindowSeconds)
	}
	if rule.Language != notification.LanguageEnglish {
		t.Fatalf("language = %q, want %q", rule.Language, notification.LanguageEnglish)
	}
	if rule.CreatedBy != "user-1" || rule.UpdatedBy != "user-1" {
		t.Fatalf("unexpected operator metadata: createdBy=%q updatedBy=%q", rule.CreatedBy, rule.UpdatedBy)
	}
}

func TestCreateEventDeduplicatesOpenEventsAndKeepsSingleRecord(t *testing.T) {
	database := openTestDB(t)
	service := NewService(database, notification.NewService(database))

	first, err := service.CreateEvent(context.Background(), EventRequest{
		EventType:    "service_release_failed",
		ResourceType: "service",
		ResourceID:   "svc-1",
		Detail:       "first failure",
	})
	if err != nil {
		t.Fatalf("create first event: %v", err)
	}

	if first.Severity != model.AlertEventSeverityWarning {
		t.Fatalf("severity = %s, want %s", first.Severity, model.AlertEventSeverityWarning)
	}
	if first.DedupeKey != "service_release_failed:service:svc-1" {
		t.Fatalf("dedupe key = %q, want %q", first.DedupeKey, "service_release_failed:service:svc-1")
	}
	if first.Summary != "service_release_failed" {
		t.Fatalf("summary = %q, want %q", first.Summary, "service_release_failed")
	}

	second, err := service.CreateEvent(context.Background(), EventRequest{
		EventType:    "service_release_failed",
		ResourceType: "service",
		ResourceID:   "svc-1",
		Summary:      "release failed again",
		Detail:       "second failure",
	})
	if err != nil {
		t.Fatalf("create duplicate event: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("duplicate event id = %q, want %q", second.ID, first.ID)
	}
	if second.Detail != "second failure" {
		t.Fatalf("detail = %q, want %q", second.Detail, "second failure")
	}
	if second.Summary != "release failed again" {
		t.Fatalf("summary = %q, want %q", second.Summary, "release failed again")
	}

	var count int64
	if err := database.Model(&model.AlertEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count alert events: %v", err)
	}
	if count != 1 {
		t.Fatalf("alert event count = %d, want 1", count)
	}
}

func TestAckAndResolveEventUpdateState(t *testing.T) {
	database := openTestDB(t)
	service := NewService(database, notification.NewService(database))

	event, err := service.CreateEvent(context.Background(), EventRequest{
		EventType: "host_offline",
		Summary:   "host unreachable",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	acked, err := service.AckEvent(context.Background(), event.ID, "operator-1")
	if err != nil {
		t.Fatalf("ack event: %v", err)
	}
	if acked.Status != model.AlertEventStatusAcked {
		t.Fatalf("acked status = %s, want %s", acked.Status, model.AlertEventStatusAcked)
	}
	if acked.AckedAt == nil || acked.AckedBy != "operator-1" {
		t.Fatalf("unexpected ack fields: ackedAt=%v ackedBy=%q", acked.AckedAt, acked.AckedBy)
	}

	resolved, err := service.ResolveEvent(context.Background(), event.ID, "operator-2")
	if err != nil {
		t.Fatalf("resolve event: %v", err)
	}
	if resolved.Status != model.AlertEventStatusResolved {
		t.Fatalf("resolved status = %s, want %s", resolved.Status, model.AlertEventStatusResolved)
	}
	if resolved.ResolvedAt == nil || resolved.ResolvedBy != "operator-2" {
		t.Fatalf("unexpected resolve fields: resolvedAt=%v resolvedBy=%q", resolved.ResolvedAt, resolved.ResolvedBy)
	}
}

func TestCreateEventDispatchesNotificationRecordForMatchingRule(t *testing.T) {
	database := openTestDB(t)
	notifications := notification.NewService(database)
	service := NewService(database, notifications)

	channel, err := notifications.CreateChannel(context.Background(), notification.ChannelRequest{
		Name:          "Ops Telegram",
		Type:          model.NotificationChannelTypeTelegram,
		Language:      "en",
		Config:        "{}",
		DefaultTarget: "ops",
		OperatorID:    "user-1",
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	_, err = service.CreateRule(context.Background(), RuleRequest{
		Name:       "Host offline",
		EventType:  "host_offline",
		ChannelIDs: channel.ID,
		OperatorID: "user-1",
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	first, err := service.CreateEvent(context.Background(), EventRequest{
		EventType:    "host_offline",
		ResourceType: "host",
		ResourceID:   "host-1",
		Severity:     model.AlertEventSeverityCritical,
		Summary:      "host unreachable",
		Detail:       "dial tcp timeout",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if first == nil || first.ID == "" {
		t.Fatal("expected created alert event")
	}

	var records []model.NotificationRecord
	if err := database.Order("created_at ASC").Find(&records).Error; err != nil {
		t.Fatalf("list notification records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("notification record count = %d, want 1", len(records))
	}
	if records[0].Status != model.NotificationRecordStatusSuccess {
		t.Fatalf("notification record status = %s, want %s", records[0].Status, model.NotificationRecordStatusSuccess)
	}

	if _, err := service.CreateEvent(context.Background(), EventRequest{
		EventType:    "host_offline",
		ResourceType: "host",
		ResourceID:   "host-1",
		Severity:     model.AlertEventSeverityCritical,
		Detail:       "duplicate host unreachable event",
	}); err != nil {
		t.Fatalf("create duplicate event: %v", err)
	}

	if err := database.Order("created_at ASC").Find(&records).Error; err != nil {
		t.Fatalf("reload notification records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("notification record count after dedupe = %d, want 1", len(records))
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("open sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}
