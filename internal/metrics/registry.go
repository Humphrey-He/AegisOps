// Package metrics 持有 AegisOps 的 Prometheus 指标注册表与业务指标定义。
//
// 指标注册在包级私有 registry 上（不使用 prometheus 全局默认注册表），
// 避免第三方库的指标混入抓取输出，也让测试可以独立 Gather。
// 指标实例是包级变量：任务 worker、调度器等深层调用点直接打点，
// 不通过依赖注入层层传递。
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

func init() {
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

var (
	// HTTPRequests 按 method/route/status 统计 HTTP 请求总数。
	// route 用路由模板（如 /api/tasks/:id）；未匹配路由归一为 "unmatched"，
	// 真实路径不进标签，防止标签基数爆炸。
	HTTPRequests = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name: "aegisops_http_requests_total",
		Help: "HTTP 请求总数（method/route/status）",
	}, []string{"method", "route", "status"})

	// HTTPRequestDuration 按 method/route 统计 HTTP 请求耗时分布。
	HTTPRequestDuration = promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
		Name: "aegisops_http_request_duration_seconds",
		Help: "HTTP 请求耗时分布（秒）",
	}, []string{"method", "route"})

	// DispatchClaims 统计派发抢占结果；result 为 won（抢到）或 lost（被其他实例抢先）。
	DispatchClaims = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name: "aegisops_dispatch_claims_total",
		Help: "任务派发抢占结果计数（result=won|lost）",
	}, []string{"result"})

	// DispatchDuration 按任务类型统计派发执行耗时。
	DispatchDuration = promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
		Name: "aegisops_dispatch_duration_seconds",
		Help: "任务派发执行耗时分布（秒，按任务类型）",
	}, []string{"type"})

	// DispatchCompletions 按终态统计派发完成数（status=SUCCESS|FAILED|TIMEOUT）。
	DispatchCompletions = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name: "aegisops_dispatch_completions_total",
		Help: "任务派发终态计数（status=SUCCESS|FAILED|TIMEOUT）",
	}, []string{"status"})

	// DispatchRetries 统计派发进入重试的次数。
	DispatchRetries = promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "aegisops_dispatch_retries_total",
		Help: "任务派发重试次数",
	})

	// SchedulerDispatches 统计调度任务成功入队的派发数。
	SchedulerDispatches = promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "aegisops_scheduler_dispatches_total",
		Help: "调度任务成功入队的派发数",
	})

	// WorkerErrors 统计派发 worker 单次轮询的错误数。
	WorkerErrors = promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "aegisops_worker_errors_total",
		Help: "任务派发 worker 轮询错误次数",
	})
)

// Handler 返回 /metrics 抓取端点，供独立 metrics 端口使用。
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// Registry 暴露 Gatherer 给测试断言指标族。
func Registry() prometheus.Gatherer {
	return registry
}
