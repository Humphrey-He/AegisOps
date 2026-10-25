package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

func TestRegistryExposesRuntimeAndAppMetrics(t *testing.T) {
	// Vec 指标是惰性的：没有打过点的标签组合不会出现在 Gather 输出里，
	// 所以先给每个 Vec 一个样本点，再断言指标族存在。
	HTTPRequests.WithLabelValues("GET", "/healthz", "200").Inc()
	HTTPRequestDuration.WithLabelValues("GET", "/healthz").Observe(0.001)
	DispatchClaims.WithLabelValues("won").Inc()
	DispatchDuration.WithLabelValues("noop").Observe(0.001)
	DispatchCompletions.WithLabelValues(string(model.TaskDispatchStatusSuccess)).Inc()

	families, err := Registry().Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	// 运行时自监控：Go runtime 与进程指标。
	for _, want := range []string{"go_goroutines", "process_cpu_seconds_total"} {
		if !names[want] {
			t.Errorf("registry missing runtime metric %q; families=%v", want, keysOf(names))
		}
	}
	// 业务指标：全部 aegisops_ 前缀指标族都已注册。
	for _, want := range []string{
		"aegisops_http_requests_total",
		"aegisops_http_request_duration_seconds",
		"aegisops_dispatch_claims_total",
		"aegisops_dispatch_duration_seconds",
		"aegisops_dispatch_completions_total",
		"aegisops_dispatch_retries_total",
		"aegisops_scheduler_dispatches_total",
		"aegisops_worker_errors_total",
	} {
		if !names[want] {
			t.Errorf("registry missing app metric %q; families=%v", want, keysOf(names))
		}
	}
}

func TestHandlerServesPrometheusTextFormat(t *testing.T) {
	HTTPRequests.WithLabelValues("GET", "/healthz", "200").Inc()

	server := httptest.NewServer(Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /metrics body: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "# HELP") {
		t.Fatalf("/metrics output missing # HELP exposition markers")
	}
	for _, want := range []string{"go_goroutines", "aegisops_http_requests_total"} {
		if !strings.Contains(text, want) {
			t.Fatalf("/metrics output missing %q", want)
		}
	}
}

func keysOf(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
