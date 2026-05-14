package handler

import (
	"net/http"

	"github.com/Humphrey-He/AegisOps/internal/audit"
	"github.com/Humphrey-He/AegisOps/internal/auth"
	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth  *auth.Service
	audit *audit.Service
}

func NewAuthHandler(authService *auth.Service, auditService *audit.Service) *AuthHandler {
	return &AuthHandler{auth: authService, audit: auditService}
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
	result, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
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
	_ = h.audit.RecordGin(c, audit.Entry{
		UserID:       &result.User.ID,
		Username:     result.User.Username,
		Action:       "auth.login",
		ResourceType: "auth",
		Result:       model.AuditResultSuccess,
	})
	response.OK(c, result)
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
