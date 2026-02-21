package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
	"msg_server/server/common/transport/httpresp"
	sessiondomain "msg_server/server/session/domain"
	sessionservice "msg_server/server/session/service"
)

type Handler struct {
	sessionSvc *sessionservice.SessionService
	noteSvc    *sessionservice.NoteService
	chatSvc    *sessionservice.ChatService
	auth       *commonauth.Service
	hub        *sessionservice.Hub
}

func NewHandler(sessionSvc *sessionservice.SessionService, noteSvc *sessionservice.NoteService, chatSvc *sessionservice.ChatService, auth *commonauth.Service, hub *sessionservice.Hub) *Handler {
	return &Handler{sessionSvc: sessionSvc, noteSvc: noteSvc, chatSvc: chatSvc, auth: auth, hub: hub}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ws/session", h.handleSessionWS)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		api.POST("/session/login", h.loginDevice)
		api.PATCH("/session/status", h.updateSessionStatus)
		api.POST("/notes", h.sendNote)
		api.GET("/notes/inbox", h.listInbox)
		api.POST("/notes/:id/read", h.markNoteRead)
		api.POST("/chat/notify", h.notifyChat)
	}
}

var sessionUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (h *Handler) handleSessionWS(c *gin.Context) {
	tenantID := strings.TrimSpace(c.Query("tenant_id"))
	userID := strings.TrimSpace(c.Query("user_id"))
	sessionID := strings.TrimSpace(c.Query("session_id"))
	sessionToken := strings.TrimSpace(c.Query("session_token"))

	ok, err := h.sessionSvc.ValidateSession(c.Request.Context(), tenantID, userID, sessionID, sessionToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}

	conn, err := sessionUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}

	client := &sessionservice.WSClient{TenantID: tenantID, UserID: userID, SessionID: sessionID, Conn: conn}
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	client.WriteJSON(map[string]any{
		"type":         "session.connected",
		"tenant_id":    tenantID,
		"user_id":      userID,
		"session_id":   sessionID,
		"connected_at": time.Now().UTC(),
	})

	for {
		if err := conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_, _ = h.sessionSvc.ValidateSession(c.Request.Context(), tenantID, userID, sessionID, sessionToken)
	}
}

func (h *Handler) loginDevice(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	authToken, err := authTokenFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		DeviceID       string   `json:"device_id" binding:"required"`
		DeviceName     string   `json:"device_name"`
		AllowedTenants []string `json:"allowed_tenants"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}

	session, err := h.sessionSvc.LoginDevice(c.Request.Context(), tenantID, userID, req.DeviceID, req.DeviceName, authToken, req.AllowedTenants)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *Handler) updateSessionStatus(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		Status     string `json:"status" binding:"required"`
		StatusNote string `json:"status_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if err := h.sessionSvc.UpdateStatus(c.Request.Context(), tenantID, userID, req.Status, req.StatusNote); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func (h *Handler) sendNote(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req sessiondomain.NoteCreateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	note, err := h.noteSvc.SendNote(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, note)
}

func (h *Handler) listInbox(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.noteSvc.ListInbox(c.Request.Context(), tenantID, userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) markNoteRead(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	noteID := strings.TrimSpace(c.Param("id"))
	if err := h.noteSvc.MarkNoteRead(c.Request.Context(), tenantID, userID, noteID); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func (h *Handler) notifyChat(c *gin.Context) {
	tenantID, userID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	authToken, err := authTokenFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req sessiondomain.ChatNotifyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if err := h.chatSvc.NotifyChat(c.Request.Context(), tenantID, userID, authToken, req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func actorFromContext(c *gin.Context) (string, string, error) {
	rawTenantID, ok := c.Get("auth_tenant_id")
	if !ok {
		return "", "", http.ErrNoCookie
	}
	rawUserID, ok := c.Get("auth_user_id")
	if !ok {
		return "", "", http.ErrNoCookie
	}
	tenantID, ok := rawTenantID.(string)
	if !ok || strings.TrimSpace(tenantID) == "" {
		return "", "", http.ErrNoCookie
	}
	userID, ok := rawUserID.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", "", http.ErrNoCookie
	}
	return tenantID, userID, nil
}

func authTokenFromContext(c *gin.Context) (string, error) {
	rawToken, ok := c.Get("auth_access_token")
	if !ok {
		return "", http.ErrNoCookie
	}
	token, ok := rawToken.(string)
	if !ok || strings.TrimSpace(token) == "" {
		return "", http.ErrNoCookie
	}
	return strings.TrimSpace(token), nil
}
