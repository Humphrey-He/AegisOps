package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Humphrey-He/AegisOps/internal/healthcheck"
	"github.com/Humphrey-He/AegisOps/internal/rbac"
)

type HealthCheckHandler struct {
	service *healthcheck.Service
}

func NewHealthCheckHandler(service *healthcheck.Service) *HealthCheckHandler {
	return &HealthCheckHandler{service: service}
}

func (h *HealthCheckHandler) RegisterRoutes(r gin.IRouter, rbacService *rbac.Service) {
	r.GET("/services/:id/health-checks", rbac.RequirePermission(rbacService, "healthchecks.view"), h.ServiceChecks)
	r.GET("/releases/:id/health-checks", rbac.RequirePermission(rbacService, "healthchecks.view"), h.ReleaseChecks)
	r.GET("/services/:id/rollback-suggestion", rbac.RequirePermission(rbacService, "services.release"), h.RollbackSuggestion)
	r.GET("/hosts/:id/availability", rbac.RequirePermission(rbacService, "healthchecks.view"), h.HostAvailability)
}

func (h *HealthCheckHandler) ServiceChecks(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListServiceChecks(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *HealthCheckHandler) ReleaseChecks(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListReleaseChecks(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *HealthCheckHandler) HostAvailability(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListHostAvailability(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *HealthCheckHandler) RollbackSuggestion(c *gin.Context) {
	result, err := h.service.RollbackSuggestion(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, result)
}
