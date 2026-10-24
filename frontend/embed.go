// Package frontend 通过 go:embed 内嵌构建好的控制台静态产物（dist/），
// 使后端单二进制即可完整分发，无需单独部署前端静态服务。
// 产物由 `npm run build` 生成；修改前端源码后需重新构建并提交 dist。
package frontend

import "embed"

// Dist 内嵌 frontend/dist 目录。
//
//go:embed all:dist
var Dist embed.FS
