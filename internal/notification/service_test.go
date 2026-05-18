package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

func TestFormatTelegramMessageV2ChineseEnterpriseTemplate(t *testing.T) {
	req := SendRequest{
		EventID:      "alert-1",
		EventType:    "host_offline",
		Severity:     string(model.AlertEventSeverityCritical),
		Status:       string(model.AlertEventStatusOpen),
		Body:         "主机连续 3 次 SSH 检测失败",
		ResourceType: "host",
		ResourceID:   "host-1",
		ResourceName: "prod-web-01 / 10.0.1.12",
		TaskID:       "task-1",
		Suggestion:   "请检查主机网络、SSH 服务与绑定凭证状态。",
		TriggeredAt:  time.Date(2026, 5, 18, 15, 25, 12, 0, time.FixedZone("CST", 8*3600)),
	}

	message := formatTelegramMessageV2(req, LanguageChinese, "http://localhost:4173")

	for _, want := range []string{
		"[AegisOps] 严重告警：主机离线",
		"<b>级别：</b>严重",
		"<b>状态：</b>待处理",
		"<b>资源：</b>host / prod-web-01 / 10.0.1.12",
		"<b>影响：</b>主机连续 3 次 SSH 检测失败",
		"<b>建议：</b>",
		`<a href="http://localhost:4173/hosts/host-1">查看资源</a>`,
		`<a href="http://localhost:4173/tasks/task-1">查看任务</a>`,
		`<a href="http://localhost:4173/audits?resourceType=host&amp;resourceId=host-1">查看审计</a>`,
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
	if strings.Contains(message, "<pre>") {
		t.Fatalf("v2 message should not render detail as code block:\n%s", message)
	}
}

func TestFormatTelegramMessageVersionFallback(t *testing.T) {
	req := SendRequest{
		EventType:    "service_release_failed",
		Severity:     string(model.AlertEventSeverityWarning),
		ResourceType: "service",
		ResourceID:   "svc-1",
		Body:         "release failed",
	}

	v1 := formatTelegramMessage(req, LanguageEnglish, TemplateVersionV1)
	if !strings.Contains(v1, "AegisOps Alert Notification") || !strings.Contains(v1, "<pre>release failed</pre>") {
		t.Fatalf("v1 template did not keep legacy shape:\n%s", v1)
	}

	v2 := formatTelegramMessage(req, LanguageEnglish, "unknown")
	if !strings.Contains(v2, "[AegisOps] Warning alert: Service release failed") || strings.Contains(v2, "<pre>") {
		t.Fatalf("unknown template version should fall back to v2:\n%s", v2)
	}
}

func TestAbsoluteURLKeepsExternalLinks(t *testing.T) {
	if got := absoluteURL("/tasks/task-1", "http://localhost:4173/"); got != "http://localhost:4173/tasks/task-1" {
		t.Fatalf("absoluteURL relative = %q", got)
	}
	if got := absoluteURL("https://example.com/tasks/task-1", "http://localhost:4173"); got != "https://example.com/tasks/task-1" {
		t.Fatalf("absoluteURL external = %q", got)
	}
}
