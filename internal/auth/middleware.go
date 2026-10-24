package auth

import (
	"net/http"
	"strings"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
)

const CurrentUserKey = "currentUser"

func Middleware(service *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		rawToken := ""
		if strings.HasPrefix(header, "Bearer ") {
			rawToken = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		} else if isWebSocketUpgrade(c) {
			rawToken = strings.TrimSpace(c.Query("token"))
		}
		if rawToken == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
			c.Abort()
			return
		}

		claims, err := service.ParseToken(rawToken, "access")
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			c.Abort()
			return
		}
		user, err := service.GetCurrentUser(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user")
			c.Abort()
			return
		}
		c.Set(CurrentUserKey, user)
		c.Next()
	}
}

func isWebSocketUpgrade(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade")
}

// passwordChangeAllowlist 是 MustChangePassword 用户仍可访问的路由（路由模式）。
var passwordChangeAllowlist = map[string]bool{
	"/api/auth/me":              true,
	"/api/auth/change-password": true,
	"/api/auth/logout":          true,
	"/api/auth/refresh":         true,
}

// RequirePasswordChangeCleared 拦截「必须先改密」的用户访问业务接口：
// 初始管理员口令是公开信息，不改密就放行等于平台裸奔。
// 必须挂在 auth.Middleware 之后（依赖 CurrentUserKey）。
func RequirePasswordChangeCleared() gin.HandlerFunc {
	return func(c *gin.Context) {
		if passwordChangeAllowlist[c.FullPath()] {
			c.Next()
			return
		}
		if userValue, ok := c.Get(CurrentUserKey); ok {
			if user, ok := userValue.(*model.User); ok && user.MustChangePassword {
				response.Error(c, http.StatusForbidden, "MUST_CHANGE_PASSWORD",
					"initial password must be changed before using the platform")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
