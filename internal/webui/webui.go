// Package webui 把内嵌的前端控制台产物挂到 Gin 的兜底路由：
// 静态资源直出，其余非 /api 路径回退 index.html（SPA 前端路由）。
package webui

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/Humphrey-He/AegisOps/frontend"
	"github.com/gin-gonic/gin"
)

// Register 在 engine 上注册 NoRoute 处理：/api 未知路径返回 JSON 404，
// 静态资源命中即直出，未命中的前端路由回退 index.html。
func Register(engine *gin.Engine) {
	dist, err := fs.Sub(frontend.Dist, "dist")
	if err != nil {
		return // go:embed 保证存在，防御性返回
	}
	fileServer := http.FileServer(http.FS(dist))
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "route not found"})
			return
		}
		trimmed := strings.TrimPrefix(path, "/")
		if trimmed != "" {
			if _, err := fs.Stat(dist, trimmed); err == nil {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "frontend assets missing")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}
