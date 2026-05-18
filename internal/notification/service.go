package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
)

const (
	LanguageChinese = "zh-CN"
	LanguageEnglish = "en-US"

	TemplateVersionV1 = "v1"
	TemplateVersionV2 = "v2"
)

type Service struct {
	db              *gorm.DB
	secrets         *secret.Service
	httpClient      *http.Client
	templateVersion string
	publicBaseURL   string
}

type Options struct {
	TemplateVersion string
	PublicBaseURL   string
}

type ChannelRequest struct {
	Name           string                        `json:"name" binding:"required"`
	Type           model.NotificationChannelType `json:"type" binding:"required"`
	Enabled        *bool                         `json:"enabled"`
	Language       string                        `json:"language"`
	Config         string                        `json:"config"`
	PublicConfig   string                        `json:"publicConfig"`
	ConfigSecretID string                        `json:"configSecretId"`
	DefaultTarget  string                        `json:"defaultTarget"`
	OperatorID     string                        `json:"-"`
}

type SendRequest struct {
	EventID      string
	EventType    string
	Severity     string
	Status       string
	Subject      string
	Body         string
	ResourceType string
	ResourceID   string
	ResourceName string
	TaskID       string
	ReleaseID    string
	Suggestion   string
	Language     string
	TriggeredAt  time.Time
	ResourceURL  string
	TaskURL      string
	AuditURL     string
}

type channelConfig struct {
	WebhookURL string `json:"webhookUrl"`
	BotToken   string `json:"botToken"`
	ChatID     string `json:"chatId"`
	SMTPHost   string `json:"smtpHost"`
	SMTPPort   string `json:"smtpPort"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	To         string `json:"to"`
}

func NewService(db *gorm.DB, secrets ...*secret.Service) *Service {
	var secretService *secret.Service
	if len(secrets) > 0 {
		secretService = secrets[0]
	}
	return &Service{db: db, secrets: secretService, httpClient: &http.Client{Timeout: 10 * time.Second}, templateVersion: TemplateVersionV2}
}

func (s *Service) SetOptions(opts Options) {
	s.templateVersion = normalizeTemplateVersion(opts.TemplateVersion)
	s.publicBaseURL = strings.TrimRight(strings.TrimSpace(opts.PublicBaseURL), "/")
}

func (s *Service) ListChannels(ctx context.Context, limit, offset int) ([]model.NotificationChannel, int64, error) {
	var items []model.NotificationChannel
	var total int64
	query := s.db.WithContext(ctx).Model(&model.NotificationChannel{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Config = ""
		items[i].Language = normalizeLanguage(items[i].Language)
	}
	return items, total, nil
}

func (s *Service) GetChannel(ctx context.Context, id string) (*model.NotificationChannel, error) {
	var item model.NotificationChannel
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	item.Config = ""
	item.Language = normalizeLanguage(item.Language)
	return &item, nil
}

func (s *Service) CreateChannel(ctx context.Context, req ChannelRequest) (*model.NotificationChannel, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	configSecretID, err := s.resolveConfigSecret(ctx, "", req)
	if err != nil {
		return nil, err
	}
	item := &model.NotificationChannel{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(req.Name),
		Type:            req.Type,
		Enabled:         enabled,
		Language:        normalizeLanguage(req.Language),
		ConfigEncrypted: legacyConfig("", configSecretID),
		PublicConfig:    strings.TrimSpace(req.PublicConfig),
		ConfigSecretID:  configSecretID,
		DefaultTarget:   strings.TrimSpace(req.DefaultTarget),
		CreatedBy:       req.OperatorID,
		UpdatedBy:       req.OperatorID,
	}
	if item.Name == "" {
		return nil, fmt.Errorf("notification channel name is required")
	}
	if err := validateType(item.Type); err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	if item.ConfigSecretID != "" && s.secrets != nil {
		_ = s.secrets.UpsertReference(ctx, item.ConfigSecretID, "notification_channel", item.ID, "config", req.OperatorID)
	}
	item.Config = ""
	return item, nil
}

func (s *Service) UpdateChannel(ctx context.Context, id string, req ChannelRequest) (*model.NotificationChannel, error) {
	item, err := s.GetChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.Type != "" {
		if err := validateType(req.Type); err != nil {
			return nil, err
		}
		item.Type = req.Type
	}
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if req.Language != "" {
		item.Language = normalizeLanguage(req.Language)
	}
	configSecretID, err := s.resolveConfigSecret(ctx, item.ID, req)
	if err != nil {
		return nil, err
	}
	if req.PublicConfig != "" {
		item.PublicConfig = strings.TrimSpace(req.PublicConfig)
	}
	if configSecretID != "" {
		item.ConfigSecretID = configSecretID
		item.ConfigEncrypted = ""
	}
	item.DefaultTarget = strings.TrimSpace(req.DefaultTarget)
	item.UpdatedBy = req.OperatorID
	if err := s.db.WithContext(ctx).Save(item).Error; err != nil {
		return nil, err
	}
	if item.ConfigSecretID != "" && s.secrets != nil {
		_ = s.secrets.UpsertReference(ctx, item.ConfigSecretID, "notification_channel", item.ID, "config", req.OperatorID)
	}
	if item.ConfigSecretID == "" && s.secrets != nil {
		_ = s.secrets.DeleteReference(ctx, "notification_channel", item.ID, "config")
	}
	item.Config = ""
	item.Language = normalizeLanguage(item.Language)
	return item, nil
}

func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	if s.secrets != nil {
		_ = s.secrets.DeleteReference(ctx, "notification_channel", id, "config")
	}
	return s.db.WithContext(ctx).Delete(&model.NotificationChannel{}, "id = ?", id).Error
}

func (s *Service) TestChannel(ctx context.Context, id string) (*model.NotificationRecord, error) {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	language := normalizeLanguage(channel.Language)
	return s.Send(ctx, *channel, SendRequest{
		Severity:     string(model.AlertEventSeverityInfo),
		Status:       string(model.AlertEventStatusOpen),
		Subject:      text(language, "AegisOps 通知通道测试", "AegisOps notification channel test"),
		Body:         text(language, "Telegram 通知通道已连通，可以接收 AegisOps 告警消息。", "Telegram channel is connected and ready to receive AegisOps alerts."),
		ResourceType: "notification_channel",
		ResourceID:   id,
		ResourceName: channel.Name,
		Language:     language,
		TriggeredAt:  time.Now().UTC(),
	})
}

func (s *Service) Send(ctx context.Context, channel model.NotificationChannel, req SendRequest) (*model.NotificationRecord, error) {
	record := &model.NotificationRecord{
		ID:          uuid.NewString(),
		EventID:     req.EventID,
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		ChannelType: channel.Type,
		Status:      model.NotificationRecordStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	status := model.NotificationRecordStatusSuccess
	responseText := "notification delivered"
	errMessage := ""
	if channel.Enabled {
		if err := s.dispatch(ctx, channel, req); err != nil {
			status = model.NotificationRecordStatusFailed
			errMessage = err.Error()
			responseText = ""
		}
	} else {
		status = model.NotificationRecordStatusFailed
		errMessage = "notification channel is disabled"
		responseText = ""
	}
	updates := map[string]interface{}{
		"status":           status,
		"response_excerpt": responseText,
		"error_message":    errMessage,
		"finished_at":      &now,
	}
	if err := s.db.WithContext(ctx).Model(&model.NotificationRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	channelUpdates := map[string]interface{}{
		"last_status":  status,
		"last_error":   errMessage,
		"last_sent_at": &now,
	}
	_ = s.db.WithContext(ctx).Model(&model.NotificationChannel{}).Where("id = ?", channel.ID).Updates(channelUpdates).Error
	record.Status = status
	record.ResponseExcerpt = responseText
	record.ErrorMessage = errMessage
	record.FinishedAt = &now
	if status == model.NotificationRecordStatusFailed {
		return record, errors.New(errMessage)
	}
	return record, nil
}

func (s *Service) Records(ctx context.Context, limit, offset int) ([]model.NotificationRecord, int64, error) {
	var items []model.NotificationRecord
	var total int64
	query := s.db.WithContext(ctx).Model(&model.NotificationRecord{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) dispatch(ctx context.Context, channel model.NotificationChannel, req SendRequest) error {
	var cfg channelConfig
	configJSON, err := s.channelConfigJSON(ctx, channel, req)
	if err != nil {
		return err
	}
	if strings.TrimSpace(configJSON) != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return fmt.Errorf("parse channel config: %w", err)
		}
	}
	language := firstNonEmpty(req.Language, channel.Language, LanguageChinese)
	switch channel.Type {
	case model.NotificationChannelTypeTelegram:
		return s.dispatchWebhook(ctx, firstNonEmpty(cfg.WebhookURL, telegramURL(cfg.BotToken)), map[string]interface{}{
			"chat_id":                  firstNonEmpty(cfg.ChatID, channel.DefaultTarget),
			"text":                     s.formatTelegramMessage(req, language),
			"parse_mode":               "HTML",
			"disable_web_page_preview": true,
		})
	case model.NotificationChannelTypeWecom:
		return s.dispatchWebhook(ctx, cfg.WebhookURL, map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": s.formatPlainMessage(req, language),
			},
		})
	case model.NotificationChannelTypeEmail:
		return s.dispatchEmail(cfg, req, language)
	default:
		return validateType(channel.Type)
	}
}

func (s *Service) dispatchWebhook(ctx context.Context, url string, payload interface{}) error {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) dispatchEmail(cfg channelConfig, req SendRequest, language string) error {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return nil
	}
	port := firstNonEmpty(cfg.SMTPPort, "25")
	to := firstNonEmpty(cfg.To, "")
	if to == "" {
		return fmt.Errorf("email recipient is required")
	}
	addr := cfg.SMTPHost + ":" + port
	message := "Subject: " + notificationSubject(req, language) + "\r\n\r\n" + s.formatPlainMessage(req, language)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, firstNonEmpty(cfg.From, cfg.Username), strings.Split(to, ","), []byte(message))
}

func validateType(value model.NotificationChannelType) error {
	switch value {
	case model.NotificationChannelTypeTelegram, model.NotificationChannelTypeWecom, model.NotificationChannelTypeEmail:
		return nil
	default:
		return fmt.Errorf("unsupported notification channel type: %s", value)
	}
}

func telegramURL(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	return "https://api.telegram.org/bot" + token + "/sendMessage"
}

func (s *Service) channelConfigJSON(ctx context.Context, channel model.NotificationChannel, req SendRequest) (string, error) {
	if strings.TrimSpace(channel.ConfigSecretID) != "" {
		if s.secrets == nil {
			return "", fmt.Errorf("notification channel config secret service is unavailable")
		}
		return s.secrets.DecryptValueForUse(ctx, channel.ConfigSecretID, secret.UseContext{
			ResourceType: "notification_channel",
			ResourceID:   channel.ID,
			Action:       "notification.dispatch",
			TaskID:       req.TaskID,
		})
	}
	return channel.ConfigEncrypted, nil
}

func legacyConfig(config, secretID string) string {
	if strings.TrimSpace(secretID) != "" {
		return ""
	}
	return strings.TrimSpace(config)
}

func (s *Service) resolveConfigSecret(ctx context.Context, channelID string, req ChannelRequest) (string, error) {
	if strings.TrimSpace(req.ConfigSecretID) != "" {
		return strings.TrimSpace(req.ConfigSecretID), nil
	}
	if strings.TrimSpace(req.Config) == "" {
		return "", nil
	}
	if s.secrets == nil {
		return "", nil
	}
	secretType := model.SecretTypeAPIToken
	switch req.Type {
	case model.NotificationChannelTypeWecom:
		secretType = model.SecretTypeWebhook
	case model.NotificationChannelTypeEmail:
		secretType = model.SecretTypeSMTP
	}
	nameParts := []string{"notification", string(req.Type), strings.TrimSpace(req.Name)}
	if channelID != "" {
		nameParts = append(nameParts, channelID)
	}
	created, err := s.secrets.Create(ctx, secret.CreateRequest{
		Name:        strings.Join(nonEmpty(nameParts...), "-"),
		Type:        secretType,
		Purpose:     "notification_channel",
		Description: "Notification channel sensitive config",
		Value:       strings.TrimSpace(req.Config),
		OperatorID:  req.OperatorID,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func nonEmpty(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, strings.TrimSpace(value))
		}
	}
	return items
}

func (s *Service) formatTelegramMessage(req SendRequest, language string) string {
	if normalizeTemplateVersion(s.templateVersion) == TemplateVersionV1 {
		return formatTelegramMessageV1(req, language)
	}
	return formatTelegramMessageV2(req, language, s.publicBaseURL)
}

func formatTelegramMessage(req SendRequest, language, templateVersion string) string {
	if normalizeTemplateVersion(templateVersion) == TemplateVersionV1 {
		return formatTelegramMessageV1(req, language)
	}
	return formatTelegramMessageV2(req, language, "")
}

func formatTelegramMessageV1(req SendRequest, language string) string {
	language = normalizeLanguage(language)
	triggeredAt := req.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now().UTC()
	}
	lines := []string{
		"<b>" + escapeHTML(text(language, "AegisOps 告警通知", "AegisOps Alert Notification")) + "</b>",
		"",
		"<b>" + label(language, "标题", "Subject") + "：</b>" + escapeHTML(notificationSubject(req, language)),
	}
	if req.Severity != "" {
		lines = append(lines, "<b>"+label(language, "级别", "Severity")+"：</b>"+escapeHTML(localizedSeverity(req.Severity, language)))
	}
	if req.EventType != "" {
		lines = append(lines, "<b>"+label(language, "事件", "Event")+"：</b>"+escapeHTML(localizedEventType(req.EventType, language)))
	}
	if req.ResourceType != "" || req.ResourceID != "" {
		lines = append(lines, "<b>"+label(language, "资源", "Resource")+"：</b><code>"+escapeHTML(joinResource(req.ResourceType, req.ResourceID))+"</code>")
	}
	if req.TaskID != "" {
		lines = append(lines, "<b>"+label(language, "任务", "Task")+"：</b><code>"+escapeHTML(req.TaskID)+"</code>")
	}
	if req.ReleaseID != "" {
		lines = append(lines, "<b>"+label(language, "发布", "Release")+"：</b><code>"+escapeHTML(req.ReleaseID)+"</code>")
	}
	if strings.TrimSpace(req.Body) != "" {
		lines = append(lines, "", "<b>"+label(language, "详情", "Detail")+"：</b>", "<pre>"+escapeHTML(req.Body)+"</pre>")
	}
	if strings.TrimSpace(req.Suggestion) != "" {
		lines = append(lines, "", "<b>"+label(language, "建议", "Suggestion")+"：</b>", "<pre>"+escapeHTML(req.Suggestion)+"</pre>")
	}
	if req.EventID != "" {
		lines = append(lines, "", "<b>"+label(language, "事件ID", "Event ID")+"：</b><code>"+escapeHTML(req.EventID)+"</code>")
	}
	lines = append(lines, "<b>"+label(language, "时间", "Time")+"：</b>"+escapeHTML(triggeredAt.Local().Format("2006-01-02 15:04:05 MST")))
	return strings.Join(lines, "\n")
}

func formatTelegramMessageV2(req SendRequest, language, publicBaseURL string) string {
	language = normalizeLanguage(language)
	triggeredAt := req.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now().UTC()
	}
	severity := localizedSeverity(req.Severity, language)
	if severity == "" {
		severity = text(language, "信息", "Info")
	}
	summary := notificationSubject(req, language)
	status := localizedStatus(req.Status, language)
	if status == "" {
		status = localizedStatus(string(model.AlertEventStatusOpen), language)
	}
	lines := []string{
		"<b>" + escapeHTML(text(language, "[AegisOps] "+severity+"告警："+summary, "[AegisOps] "+severity+" alert: "+summary)) + "</b>",
		"",
		"<b>" + label(language, "级别", "Severity") + "：</b>" + escapeHTML(severity),
		"<b>" + label(language, "状态", "Status") + "：</b>" + escapeHTML(status),
	}
	if resource := displayResource(req); resource != "" {
		lines = append(lines, "<b>"+label(language, "资源", "Resource")+"：</b>"+escapeHTML(resource))
	}
	if req.EventType != "" {
		lines = append(lines, "<b>"+label(language, "事件", "Event")+"：</b>"+escapeHTML(localizedEventType(req.EventType, language)))
	}
	if body := sanitizeMessage(req.Body); body != "" {
		lines = append(lines, "<b>"+label(language, "影响", "Impact")+"：</b>"+escapeHTML(body))
	}
	lines = append(lines, "<b>"+label(language, "时间", "Time")+"：</b>"+escapeHTML(triggeredAt.Local().Format("2006-01-02 15:04:05 MST")))
	if req.Suggestion != "" {
		lines = append(lines, "", "<b>"+label(language, "建议", "Suggestion")+"：</b>", escapeHTML(sanitizeMessage(req.Suggestion)))
	}
	if actions := actionLinks(req, language, publicBaseURL); len(actions) > 0 {
		lines = append(lines, "", "<b>"+label(language, "操作", "Actions")+"：</b>", strings.Join(actions, " | "))
	}
	if req.EventID != "" {
		lines = append(lines, "", "<b>"+label(language, "事件ID", "Event ID")+"：</b><code>"+escapeHTML(req.EventID)+"</code>")
	}
	return strings.Join(lines, "\n")
}

func (s *Service) formatPlainMessage(req SendRequest, language string) string {
	if normalizeTemplateVersion(s.templateVersion) == TemplateVersionV1 {
		return formatPlainMessageV1(req, language)
	}
	return formatPlainMessageV2(req, language, s.publicBaseURL)
}

func formatPlainMessage(req SendRequest, language, templateVersion string) string {
	if normalizeTemplateVersion(templateVersion) == TemplateVersionV1 {
		return formatPlainMessageV1(req, language)
	}
	return formatPlainMessageV2(req, language, "")
}

func formatPlainMessageV1(req SendRequest, language string) string {
	language = normalizeLanguage(language)
	triggeredAt := req.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now().UTC()
	}
	parts := []string{
		text(language, "AegisOps 告警通知", "AegisOps Alert Notification"),
		label(language, "标题", "Subject") + ": " + notificationSubject(req, language),
	}
	if req.Severity != "" {
		parts = append(parts, label(language, "级别", "Severity")+": "+localizedSeverity(req.Severity, language))
	}
	if req.EventType != "" {
		parts = append(parts, label(language, "事件", "Event")+": "+localizedEventType(req.EventType, language))
	}
	if req.ResourceType != "" || req.ResourceID != "" {
		parts = append(parts, label(language, "资源", "Resource")+": "+joinResource(req.ResourceType, req.ResourceID))
	}
	if req.TaskID != "" {
		parts = append(parts, label(language, "任务", "Task")+": "+req.TaskID)
	}
	if req.ReleaseID != "" {
		parts = append(parts, label(language, "发布", "Release")+": "+req.ReleaseID)
	}
	if req.Body != "" {
		parts = append(parts, label(language, "详情", "Detail")+": "+req.Body)
	}
	if req.Suggestion != "" {
		parts = append(parts, label(language, "建议", "Suggestion")+": "+req.Suggestion)
	}
	if req.EventID != "" {
		parts = append(parts, label(language, "事件ID", "Event ID")+": "+req.EventID)
	}
	parts = append(parts, label(language, "时间", "Time")+": "+triggeredAt.Local().Format("2006-01-02 15:04:05 MST"))
	return strings.Join(parts, "\n")
}

func formatPlainMessageV2(req SendRequest, language, publicBaseURL string) string {
	language = normalizeLanguage(language)
	triggeredAt := req.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = time.Now().UTC()
	}
	severity := localizedSeverity(req.Severity, language)
	if severity == "" {
		severity = text(language, "信息", "Info")
	}
	summary := notificationSubject(req, language)
	status := localizedStatus(req.Status, language)
	if status == "" {
		status = localizedStatus(string(model.AlertEventStatusOpen), language)
	}
	parts := []string{
		text(language, "[AegisOps] "+severity+"告警："+summary, "[AegisOps] "+severity+" alert: "+summary),
		"",
		label(language, "级别", "Severity") + ": " + severity,
		label(language, "状态", "Status") + ": " + status,
	}
	if resource := displayResource(req); resource != "" {
		parts = append(parts, label(language, "资源", "Resource")+": "+resource)
	}
	if req.EventType != "" {
		parts = append(parts, label(language, "事件", "Event")+": "+localizedEventType(req.EventType, language))
	}
	if body := sanitizeMessage(req.Body); body != "" {
		parts = append(parts, label(language, "影响", "Impact")+": "+body)
	}
	parts = append(parts, label(language, "时间", "Time")+": "+triggeredAt.Local().Format("2006-01-02 15:04:05 MST"))
	if req.Suggestion != "" {
		parts = append(parts, "", label(language, "建议", "Suggestion")+":", sanitizeMessage(req.Suggestion))
	}
	if actions := plainActionLinks(req, language, publicBaseURL); len(actions) > 0 {
		parts = append(parts, "", label(language, "操作", "Actions")+":", strings.Join(actions, " | "))
	}
	if req.EventID != "" {
		parts = append(parts, "", label(language, "事件ID", "Event ID")+": "+req.EventID)
	}
	return strings.Join(parts, "\n")
}

func notificationSubject(req SendRequest, language string) string {
	if req.EventType != "" {
		return localizedEventType(req.EventType, language)
	}
	return req.Subject
}

func joinResource(resourceType, resourceID string) string {
	if resourceType == "" {
		return resourceID
	}
	if resourceID == "" {
		return resourceType
	}
	return resourceType + "/" + resourceID
}

func displayResource(req SendRequest) string {
	name := strings.TrimSpace(req.ResourceName)
	if name == "" {
		name = strings.TrimSpace(req.ResourceID)
	}
	if req.ResourceType == "" {
		return name
	}
	if name == "" {
		return req.ResourceType
	}
	return req.ResourceType + " / " + name
}

func sanitizeMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	if len(lines) > 4 {
		lines = append(lines[:4], "...")
	}
	return strings.Join(lines, "\n")
}

func actionLinks(req SendRequest, language, publicBaseURL string) []string {
	actions := make([]string, 0, 3)
	if req.ResourceURL != "" {
		actions = append(actions, htmlLink(absoluteURL(req.ResourceURL, publicBaseURL), text(language, "查看资源", "View resource")))
	} else if req.ResourceType != "" && req.ResourceID != "" {
		actions = append(actions, htmlLink(absoluteURL(defaultResourceURL(req.ResourceType, req.ResourceID), publicBaseURL), text(language, "查看资源", "View resource")))
	}
	if req.TaskURL != "" {
		actions = append(actions, htmlLink(absoluteURL(req.TaskURL, publicBaseURL), text(language, "查看任务", "View task")))
	} else if req.TaskID != "" {
		actions = append(actions, htmlLink(absoluteURL("/tasks/"+req.TaskID, publicBaseURL), text(language, "查看任务", "View task")))
	}
	if req.AuditURL != "" {
		actions = append(actions, htmlLink(absoluteURL(req.AuditURL, publicBaseURL), text(language, "查看审计", "View audit")))
	} else if req.ResourceType != "" && req.ResourceID != "" {
		actions = append(actions, htmlLink(absoluteURL("/audits?resourceType="+req.ResourceType+"&resourceId="+req.ResourceID, publicBaseURL), text(language, "查看审计", "View audit")))
	}
	return actions
}

func plainActionLinks(req SendRequest, language, publicBaseURL string) []string {
	actions := make([]string, 0, 3)
	if req.ResourceURL != "" {
		actions = append(actions, text(language, "查看资源", "View resource")+" "+absoluteURL(req.ResourceURL, publicBaseURL))
	} else if req.ResourceType != "" && req.ResourceID != "" {
		actions = append(actions, text(language, "查看资源", "View resource")+" "+absoluteURL(defaultResourceURL(req.ResourceType, req.ResourceID), publicBaseURL))
	}
	if req.TaskURL != "" {
		actions = append(actions, text(language, "查看任务", "View task")+" "+absoluteURL(req.TaskURL, publicBaseURL))
	} else if req.TaskID != "" {
		actions = append(actions, text(language, "查看任务", "View task")+" "+absoluteURL("/tasks/"+req.TaskID, publicBaseURL))
	}
	if req.AuditURL != "" {
		actions = append(actions, text(language, "查看审计", "View audit")+" "+absoluteURL(req.AuditURL, publicBaseURL))
	} else if req.ResourceType != "" && req.ResourceID != "" {
		actions = append(actions, text(language, "查看审计", "View audit")+" "+absoluteURL("/audits?resourceType="+req.ResourceType+"&resourceId="+req.ResourceID, publicBaseURL))
	}
	return actions
}

func htmlLink(url, label string) string {
	return `<a href="` + escapeHTML(url) + `">` + escapeHTML(label) + `</a>`
}

func absoluteURL(value, publicBaseURL string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base == "" {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return base + value
}

func defaultResourceURL(resourceType, resourceID string) string {
	switch strings.TrimSpace(resourceType) {
	case "host":
		return "/hosts/" + resourceID
	case "service":
		return "/services/" + resourceID
	case "docker_node":
		return "/docker/nodes/" + resourceID
	case "nginx_node":
		return "/nginx/nodes/" + resourceID
	case "notification_channel":
		return "/alerts/channels"
	default:
		return "/resources/context?resourceType=" + resourceType + "&resourceId=" + resourceID
	}
}

func localizedSeverity(value, language string) string {
	if normalizeLanguage(language) == LanguageEnglish {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "CRITICAL":
			return "Critical"
		case "WARNING":
			return "Warning"
		case "INFO":
			return "Info"
		default:
			return value
		}
	}
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "CRITICAL":
		return "严重"
	case "WARNING":
		return "警告"
	case "INFO":
		return "提示"
	default:
		return value
	}
}

func localizedStatus(value, language string) string {
	if normalizeLanguage(language) == LanguageEnglish {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "OPEN":
			return "Open"
		case "ACKED":
			return "Acknowledged"
		case "RESOLVED":
			return "Resolved"
		default:
			return value
		}
	}
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "OPEN":
		return "待处理"
	case "ACKED":
		return "已确认"
	case "RESOLVED":
		return "已恢复"
	default:
		return value
	}
}

func localizedEventType(value, language string) string {
	if normalizeLanguage(language) == LanguageEnglish {
		switch strings.TrimSpace(value) {
		case "service_release_failed":
			return "Service release failed"
		case "service_health_check_failed":
			return "Service health check failed"
		case "nginx_reload_failed":
			return "Nginx reload failed"
		case "nginx_publish_failed":
			return "Nginx config publish failed"
		case "host_offline":
			return "Host offline"
		case "host_recovered":
			return "Host recovered"
		default:
			return value
		}
	}
	switch strings.TrimSpace(value) {
	case "service_release_failed":
		return "服务发布失败"
	case "service_health_check_failed":
		return "服务健康检查失败"
	case "nginx_reload_failed":
		return "Nginx 重载失败"
	case "nginx_publish_failed":
		return "Nginx 配置发布失败"
	case "host_offline":
		return "主机离线"
	case "host_recovered":
		return "主机恢复"
	default:
		return value
	}
}

func normalizeLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "en-us", "english":
		return LanguageEnglish
	case "zh", "zh-cn", "cn", "chinese", "":
		return LanguageChinese
	default:
		return LanguageChinese
	}
}

func normalizeTemplateVersion(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", TemplateVersionV2, "2":
		return TemplateVersionV2
	case TemplateVersionV1, "1":
		return TemplateVersionV1
	default:
		return TemplateVersionV2
	}
}

func NormalizeLanguage(value string) string {
	return normalizeLanguage(value)
}

func label(language, zh, en string) string {
	return text(language, zh, en)
}

func text(language, zh, en string) string {
	if normalizeLanguage(language) == LanguageEnglish {
		return en
	}
	return zh
}

func escapeHTML(value string) string {
	return html.EscapeString(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
