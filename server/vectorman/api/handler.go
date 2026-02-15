package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"msg_server/server/vectorman/service"
)

type Handler struct {
	svc *service.MilvusService
}

func NewHandler(svc *service.MilvusService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1/vectors/messages")
	{
		api.POST("/index", h.index)
		api.POST("/search", h.search)
	}
}

func (h *Handler) index(c *gin.Context) {
	var req struct {
		MessageID string `json:"message_id" binding:"required"`
		RoomID    string `json:"room_id" binding:"required"`
		Text      string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.IndexMessage(c.Request.Context(), req.MessageID, req.RoomID, req.Text); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) search(c *gin.Context) {
	var req struct {
		Query  string  `json:"query" binding:"required"`
		RoomID *string `json:"room_id"`
		Limit  int     `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 30
	}
	ids, err := h.svc.SemanticSearch(c.Request.Context(), req.Query, req.RoomID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ids": ids})
}
