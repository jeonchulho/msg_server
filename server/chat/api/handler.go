package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"msg_server/server/chat/domain"
	"msg_server/server/chat/service"
	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
)

type Handler struct {
	chat *service.ChatService
	ws   *service.RealtimeService
	auth *commonauth.Service
}

func NewHandler(chat *service.ChatService, ws *service.RealtimeService, jwtSecret string, jwtTTLMinutes int) *Handler {
	auth := commonauth.NewService(jwtSecret, jwtTTLMinutes)
	return &Handler{chat: chat, ws: ws, auth: auth}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, NewHealthResponse("ok")) })
	r.GET("/ws", h.handleWS)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		api.POST("/rooms", h.createRoom)
		api.GET("/rooms", h.listMyRooms)
		api.POST("/rooms/:id/members", h.addMember)
		api.POST("/rooms/:id/messages", h.createMessage)
		api.GET("/rooms/:id/messages", h.listMessages)
		api.GET("/rooms/:id/unread-count", h.getRoomUnreadCount)
		api.GET("/rooms/unread-counts", h.getMyUnreadCounts)
		api.POST("/rooms/:id/read", h.markRoomRead)
		api.GET("/rooms/:id/read", h.getMyReadState)
		api.GET("/rooms/:id/messages/:messageId/readers", h.getMessageReaders)
		api.GET("/messages/search", h.searchMessages)

	}
}

func (h *Handler) handleWS(c *gin.Context) {
	token, ok := wsAccessToken(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, NewErrorResponse("bearer token is required"))
		return
	}
	userID, tenantID, _, err := h.auth.ParseAuthContext(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse("invalid token"))
		return
	}
	roomID := strings.TrimSpace(c.Query("room_id"))
	if roomID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("room_id required"))
		return
	}
	isMember, err := h.chat.IsRoomMember(c.Request.Context(), tenantID, roomID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	if !isMember {
		c.JSON(http.StatusForbidden, NewErrorResponse("room access denied"))
		return
	}
	c.Set("auth_access_token", token)
	c.Set("auth_user_id", userID)
	c.Set("auth_tenant_id", tenantID)
	h.ws.HandleWS(c)
}

func wsAccessToken(c *gin.Context) (string, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token != "" {
			return token, true
		}
	}
	token := strings.TrimSpace(c.Query("access_token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	if token == "" {
		return "", false
	}
	return token, true
}

func (h *Handler) createRoom(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	var req struct {
		Name      string   `json:"name" binding:"required"`
		RoomType  string   `json:"room_type"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	if req.RoomType == "" {
		req.RoomType = "group"
	}
	id, err := h.chat.CreateRoom(c.Request.Context(), tenantID, domain.ChatRoom{Name: req.Name, RoomType: req.RoomType, CreatedBy: actorID}, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, NewIDResponse(id))
}

func (h *Handler) listMyRooms(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	cursor := c.Query("cursor")
	items, nextCursor, err := h.chat.ListMyRooms(c.Request.Context(), tenantID, actorID, limit, cursor)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewPaginatedResponse(items, nextCursor))
}

func (h *Handler) addMember(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	if err := h.chat.AddMember(c.Request.Context(), tenantID, roomID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewOKResponse())
}

func (h *Handler) createMessage(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	var req struct {
		Body   string   `json:"body" binding:"required"`
		FileID *string  `json:"file_id"`
		Emojis []string `json:"emojis"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	msg, err := h.chat.CreateMessage(c.Request.Context(), domain.Message{
		TenantID: tenantID,
		RoomID:   roomID,
		SenderID: actorID,
		Body:     req.Body,
		MetaJSON: service.BuildMessageMeta(req.FileID, req.Emojis),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func (h *Handler) listMessages(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	cursor := c.Query("cursor")
	items, nextCursor, err := h.chat.ListMessages(c.Request.Context(), tenantID, roomID, limit, cursor)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewPaginatedResponse(items, nextCursor))
}

func (h *Handler) getRoomUnreadCount(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	count, err := h.chat.GetUnreadCount(c.Request.Context(), tenantID, roomID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewRoomUnreadCountResponse(roomID, actorID, count))
}

func (h *Handler) getMyUnreadCounts(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	items, err := h.chat.GetUnreadCounts(c.Request.Context(), tenantID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) markRoomRead(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	var req struct {
		MessageID string `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	if err := h.chat.MarkReadUpTo(c.Request.Context(), tenantID, roomID, actorID, req.MessageID); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewOKResponse())
}

func (h *Handler) getMyReadState(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	actorID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	messageID, err := h.chat.GetLastReadMessageID(c.Request.Context(), tenantID, roomID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewReadStateResponse(roomID, actorID, messageID))
}

func (h *Handler) getMessageReaders(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	roomID := c.Param("id")
	messageID := c.Param("messageId")
	items, err := h.chat.GetMessageReaders(c.Request.Context(), tenantID, roomID, messageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) searchMessages(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewErrorResponse(err.Error()))
		return
	}
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	cursor := c.Query("cursor")
	var roomID *string
	if raw := c.Query("room_id"); raw != "" {
		parsed := raw
		roomID = &parsed
	}
	items, nextCursor, err := h.chat.SearchMessages(c.Request.Context(), tenantID, q, roomID, limit, cursor)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewPaginatedResponse(items, nextCursor))
}

func actorFromContext(c *gin.Context) (string, string, error) {
	rawID, ok := c.Get("auth_user_id")
	if !ok {
		return "", "", fmt.Errorf(ErrUnauthorized)
	}
	rawRole, ok := c.Get("auth_role")
	if !ok {
		return "", "", fmt.Errorf(ErrUnauthorized)
	}
	userID, ok := rawID.(string)
	if !ok {
		return "", "", fmt.Errorf(ErrUnauthorized)
	}
	role, ok := rawRole.(string)
	if !ok {
		return "", "", fmt.Errorf(ErrUnauthorized)
	}
	return userID, role, nil
}

func tenantFromContext(c *gin.Context) (string, error) {
	rawTenantID, ok := c.Get("auth_tenant_id")
	if !ok {
		return "", fmt.Errorf(ErrUnauthorized)
	}
	tenantID, ok := rawTenantID.(string)
	if !ok {
		return "", fmt.Errorf(ErrUnauthorized)
	}
	return tenantID, nil
}
