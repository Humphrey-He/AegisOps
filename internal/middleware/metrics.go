package middleware

import (
	"strconv"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/metrics"
	"github.com/gin-gonic/gin"
)

// Metrics 记录每个 HTTP 请求的计数与耗时。
// route 用路由模板而非真实路径；未匹配路由归一为 "unmatched"，
// 避免带 ID 的 URL 把标签基数打爆。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())
		metrics.HTTPRequests.WithLabelValues(method, route, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
	}
}
