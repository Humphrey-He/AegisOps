package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	dockersvc "github.com/Humphrey-He/AegisOps/internal/docker"
)

type DockerHandler struct {
	service *dockersvc.Service
}

func NewDockerHandler(service *dockersvc.Service) *DockerHandler {
	return &DockerHandler{service: service}
}

func (h *DockerHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/docker/nodes", h.ListNodes)
	r.POST("/docker/nodes", h.CreateNode)
	r.GET("/docker/nodes/:id", h.GetNode)
	r.PATCH("/docker/nodes/:id", h.UpdateNode)
	r.DELETE("/docker/nodes/:id", h.DeleteNode)
	r.POST("/docker/nodes/:id/test", h.TestConnection)
	r.GET("/docker/nodes/:id/containers", h.ListContainers)
	r.GET("/docker/nodes/:id/containers/:containerId/logs", h.ContainerLogs)
	r.POST("/docker/nodes/:id/containers/:containerId/start", h.StartContainer)
	r.POST("/docker/nodes/:id/containers/:containerId/stop", h.StopContainer)
	r.POST("/docker/nodes/:id/containers/:containerId/restart", h.RestartContainer)
}

func (h *DockerHandler) ListNodes(c *gin.Context) {
	limit, offset := Pagination(c)
	items, total, err := h.service.ListNodes(c.Request.Context(), c.Query("keyword"), limit, offset)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, PageResult{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *DockerHandler) CreateNode(c *gin.Context) {
	var req dockersvc.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.CreateNode(c.Request.Context(), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	Created(c, item)
}

func (h *DockerHandler) GetNode(c *gin.Context) {
	item, err := h.service.GetNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		Error(c, http.StatusNotFound, err.Error())
		return
	}
	OK(c, item)
}

func (h *DockerHandler) UpdateNode(c *gin.Context) {
	var req dockersvc.UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.OperatorID = OperatorID(c)
	item, err := h.service.UpdateNode(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, item)
}

func (h *DockerHandler) DeleteNode(c *gin.Context) {
	if err := h.service.DeleteNode(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}

func (h *DockerHandler) TestConnection(c *gin.Context) {
	if err := h.service.TestConnection(c.Request.Context(), c.Param("id")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"connected": true})
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	items, err := h.service.ListContainers(c.Request.Context(), c.Param("id"), c.Query("all") == "true")
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, items)
}

func (h *DockerHandler) ContainerLogs(c *gin.Context) {
	logs, err := h.service.ContainerLogs(c.Request.Context(), c.Param("id"), c.Param("containerId"), c.DefaultQuery("tail", "200"))
	if err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"logs": logs})
}

func (h *DockerHandler) StartContainer(c *gin.Context) {
	if err := h.service.StartContainer(c.Request.Context(), c.Param("id"), c.Param("containerId")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"started": true})
}

func (h *DockerHandler) StopContainer(c *gin.Context) {
	if err := h.service.StopContainer(c.Request.Context(), c.Param("id"), c.Param("containerId")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"stopped": true})
}

func (h *DockerHandler) RestartContainer(c *gin.Context) {
	if err := h.service.RestartContainer(c.Request.Context(), c.Param("id"), c.Param("containerId")); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{"restarted": true})
}
