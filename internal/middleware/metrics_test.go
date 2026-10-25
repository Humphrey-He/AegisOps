package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Humphrey-He/AegisOps/internal/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsMiddlewareCountsRouteRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Metrics())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	before := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/ping", "200"))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	after := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/ping", "200"))
	if after-before != 1 {
		t.Fatalf("GET /ping counter delta = %v, want 1", after-before)
	}

	beforeBoom := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/boom", "500"))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	afterBoom := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "/boom", "500"))
	if afterBoom-beforeBoom != 1 {
		t.Fatalf("GET /boom counter delta = %v, want 1", afterBoom-beforeBoom)
	}
}

// 未匹配路由必须归一为 "unmatched"：真实路径不进标签，否则带 ID 的 URL 会打爆基数。
func TestMetricsMiddlewareNormalizesUnmatchedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Metrics())

	before := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "unmatched", "404"))
	for _, path := range []string{"/login", "/assets/some-unique-id-12345", "/tasks/abc-xyz/whatever"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	after := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET", "unmatched", "404"))
	if after-before != 3 {
		t.Fatalf("unmatched counter delta = %v, want 3", after-before)
	}
}
