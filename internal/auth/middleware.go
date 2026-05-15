package auth

import (
	"net/http"
	"strings"

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
