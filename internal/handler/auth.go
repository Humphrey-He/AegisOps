package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	appErrors "github.com/Humphrey-He/AegisOps/pkg/errors"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth  *auth.Service
	audit *audit.Service
	guard *auth.LoginGuard
}

func NewAuthHandler(authService *auth.Service, auditService *audit.Service) *AuthHandler {
	return &AuthHandler{auth: authService, audit: auditService, guard: auth.NewLoginGuard()}
}

// SetLoginGuard 供测试注入带假时钟的限流器。
func (h *AuthHandler) SetLoginGuard(guard *auth.LoginGuard) {
	if guard != nil {
		h.guard = guard
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	// 用户名 + 客户端 IP 维度的失败限流：锁定期间不再消耗 bcrypt 比较。
	guardKey := req.Username + "@" + c.ClientIP()
	if remaining := h.guard.LockedRemaining(guardKey); remaining > 0 {
		_ = h.audit.RecordGin(c, audit.Entry{
			Username:     req.Username,
			Action:       "auth.login",
			ResourceType: "auth",
			Result:       model.AuditResultFailure,
			Message:      "login blocked by rate limit",
		})
		response.Error(c, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS",
			fmt.Sprintf("too many failed attempts, retry in %s", remaining.Round(time.Second)))
		return
	}
	result, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, appErrors.ErrInvalidCredentials) {
			h.guard.RecordFailure(guardKey)
		}
		_ = h.audit.RecordGin(c, audit.Entry{
			Username:     req.Username,
			Action:       "auth.login",
			ResourceType: "auth",
			Result:       model.AuditResultFailure,
			Message:      err.Error(),
		})
		writeError(c, err)
		return
	}
	h.guard.Reset(guardKey)
	_ = h.audit.RecordGin(c, audit.Entry{
		UserID:       &result.User.ID,
		Username:     result.User.Username,
		Action:       "auth.login",
		ResourceType: "auth",
		Result:       model.AuditResultSuccess,
	})
	response.OK(c, result)
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userValue, ok := c.Get(auth.CurrentUserKey)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing current user")
		return
	}
	user, ok := userValue.(*model.User)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid current user")
		return
	}
	if err := h.auth.ChangePassword(c.Request.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		_ = h.audit.RecordGin(c, audit.Entry{
			UserID:       &user.ID,
			Username:     user.Username,
			Action:       "auth.change_password",
			ResourceType: "auth",
			Result:       model.AuditResultFailure,
			Message:      err.Error(),
		})
		writeError(c, err)
		return
	}
	_ = h.audit.RecordGin(c, audit.Entry{
		UserID:       &user.ID,
		Username:     user.Username,
		Action:       "auth.change_password",
		ResourceType: "auth",
		Result:       model.AuditResultSuccess,
	})
	response.OK(c, gin.H{"changed": true})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	_ = h.audit.RecordGin(c, audit.Entry{
		Action:       "auth.logout",
		ResourceType: "auth",
		Result:       model.AuditResultSuccess,
	})
	response.NoContent(c)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	tokens, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, tokens)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userValue, ok := c.Get(auth.CurrentUserKey)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing current user")
		return
	}
	response.OK(c, userValue)
}
