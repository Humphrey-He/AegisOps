package rbac

import (
	"errors"
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
)

var ErrForbidden = errors.New("forbidden")

func RequirePermission(service *Service, permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get(auth.CurrentUserKey)
		if !exists {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing current user")
			c.Abort()
			return
		}
		user, ok := userValue.(*model.User)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid current user")
			c.Abort()
			return
		}
		allowed, err := service.HasPermission(c.Request.Context(), user.ID, permissionCode)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			c.Abort()
			return
		}
		if !allowed {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}
