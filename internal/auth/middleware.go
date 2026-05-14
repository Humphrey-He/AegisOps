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
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
			c.Abort()
			return
		}

		claims, err := service.ParseToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), "access")
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
