package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	chatdomain "msg_server/server/chat/domain"
	"msg_server/server/dbman/domain"
	"msg_server/server/dbman/repository"
	dbservice "msg_server/server/dbman/service"
	sessiondomain "msg_server/server/session/domain"
)

type Handler struct {
	fileRepo   *repository.FileRepository
	chatSvc    *dbservice.ChatService
	userSvc    *dbservice.UserService
	sessionSvc *dbservice.SessionService
	tenantSvc  *dbservice.TenantService
	readyCheck func(context.Context) error
}

func NewHandler(fileRepo *repository.FileRepository, chatSvc *dbservice.ChatService, userSvc *dbservice.UserService, sessionSvc *dbservice.SessionService, tenantSvc *dbservice.TenantService, readyCheck func(context.Context) error) *Handler {
	return &Handler{fileRepo: fileRepo, chatSvc: chatSvc, userSvc: userSvc, sessionSvc: sessionSvc, tenantSvc: tenantSvc, readyCheck: readyCheck}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		if h.readyCheck != nil {
			if err := h.readyCheck(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/health/ready", func(c *gin.Context) {
		if h.readyCheck != nil {
			if err := h.readyCheck(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	h.registerDBRoutes(r.Group("/api/internal/v1/db"))
	h.registerDBRoutes(r.Group("/api/v1/db"))
}

func (h *Handler) registerDBRoutes(api *gin.RouterGroup) {
	api.POST("/org-units/create", h.createOrgUnit)
	api.POST("/org-units/list", h.listOrgUnits)
	api.POST("/users/create", h.createUser)
	api.POST("/users/status", h.updateUserStatus)
	api.POST("/users/search", h.searchUsers)
	api.POST("/users/authenticate", h.authenticateUser)
	api.POST("/users/aliases/list", h.listAliases)
	api.POST("/users/aliases/add", h.addAlias)
	api.POST("/users/aliases/delete", h.deleteAlias)
	api.POST("/users/aliases/audit", h.listAliasAudit)
	api.POST("/tenants/list", h.listTenants)
	api.POST("/tenants/get", h.getTenant)
	api.POST("/tenants/create", h.createTenant)
	api.POST("/tenants/update", h.updateTenant)

	api.POST("/files", h.createFile)
	api.GET("/files/search", h.searchFilesByRoom)
	api.POST("/rooms", h.createRoom)
	api.POST("/rooms/members", h.addMember)
	api.POST("/messages", h.createMessage)
	api.POST("/messages/read", h.markReadUpTo)
	api.POST("/messages/list", h.listMessages)
	api.POST("/messages/search", h.searchMessages)
	api.POST("/messages/readers", h.messageReaders)
	api.POST("/messages/last-read", h.lastReadMessage)
	api.POST("/messages/unread-count", h.unreadCount)
	api.POST("/messages/unread-counts", h.unreadCounts)
	api.POST("/rooms/list", h.listMyRooms)

	api.POST("/session/device/login", h.upsertDeviceSession)
	api.POST("/session/device/validate", h.validateSession)
	api.POST("/session/status/update", h.updateSessionUserStatus)
	api.POST("/session/notes/create", h.createSessionNote)
	api.POST("/session/notes/inbox", h.listSessionInbox)
	api.POST("/session/notes/read", h.markSessionNoteRead)
	api.POST("/session/chat/notify", h.saveSessionChatNotifications)
}

func (h *Handler) upsertDeviceSession(c *gin.Context) {
	var req sessiondomain.DeviceSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.sessionSvc.UpsertDeviceSession(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) validateSession(c *gin.Context) {
	var req struct {
		TenantID     string `json:"tenant_id" binding:"required"`
		UserID       string `json:"user_id" binding:"required"`
		SessionID    string `json:"session_id" binding:"required"`
		SessionToken string `json:"session_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	valid, err := h.sessionSvc.ValidateAndTouchSession(c.Request.Context(), req.TenantID, req.UserID, req.SessionID, req.SessionToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": valid})
}

func (h *Handler) updateSessionUserStatus(c *gin.Context) {
	var req sessiondomain.UserStatus
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.sessionSvc.UpdateUserStatus(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) createSessionNote(c *gin.Context) {
	var req sessiondomain.Note
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.sessionSvc.CreateNote(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) listSessionInbox(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
		Limit    int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	items, err := h.sessionSvc.ListInbox(c.Request.Context(), req.TenantID, req.UserID, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) markSessionNoteRead(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
		NoteID   string `json:"note_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.sessionSvc.MarkNoteRead(c.Request.Context(), req.TenantID, req.UserID, req.NoteID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) saveSessionChatNotifications(c *gin.Context) {
	var req struct {
		TenantID     string                        `json:"tenant_id" binding:"required"`
		SenderUserID string                        `json:"sender_user_id" binding:"required"`
		Input        sessiondomain.ChatNotifyInput `json:"input" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.sessionSvc.SaveChatNotifications(c.Request.Context(), req.TenantID, req.SenderUserID, req.Input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) createOrgUnit(c *gin.Context) {
	var req struct {
		TenantID string  `json:"tenant_id" binding:"required"`
		ParentID *string `json:"parent_id"`
		Name     string  `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := h.userSvc.CreateOrgUnit(c.Request.Context(), req.TenantID, req.ParentID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) listOrgUnits(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.userSvc.ListOrgUnits(c.Request.Context(), req.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) createUser(c *gin.Context) {
	var req struct {
		TenantID string          `json:"tenant_id" binding:"required"`
		User     chatdomain.User `json:"user" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := h.userSvc.CreateUser(c.Request.Context(), req.TenantID, req.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) updateUserStatus(c *gin.Context) {
	var req struct {
		TenantID string                `json:"tenant_id" binding:"required"`
		UserID   string                `json:"user_id" binding:"required"`
		Status   chatdomain.UserStatus `json:"status" binding:"required"`
		Note     string                `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.userSvc.UpdateStatus(c.Request.Context(), req.TenantID, req.UserID, req.Status, req.Note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) searchUsers(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		Q        string `json:"q" binding:"required"`
		Limit    int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	items, err := h.userSvc.SearchUsers(c.Request.Context(), req.TenantID, req.Q, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) authenticateUser(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.userSvc.Authenticate(c.Request.Context(), req.TenantID, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) listAliases(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	aliases, err := h.userSvc.ListAliases(c.Request.Context(), req.TenantID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, aliases)
}

func (h *Handler) addAlias(c *gin.Context) {
	var req struct {
		TenantID  string `json:"tenant_id" binding:"required"`
		UserID    string `json:"user_id" binding:"required"`
		Alias     string `json:"alias" binding:"required"`
		IP        string `json:"ip"`
		UserAgent string `json:"user_agent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.userSvc.AddAlias(c.Request.Context(), req.TenantID, req.UserID, req.Alias, req.IP, req.UserAgent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) deleteAlias(c *gin.Context) {
	var req struct {
		TenantID  string `json:"tenant_id" binding:"required"`
		UserID    string `json:"user_id" binding:"required"`
		Alias     string `json:"alias" binding:"required"`
		IP        string `json:"ip"`
		UserAgent string `json:"user_agent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.userSvc.DeleteAlias(c.Request.Context(), req.TenantID, req.UserID, req.Alias, req.IP, req.UserAgent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) listAliasAudit(c *gin.Context) {
	var req struct {
		TenantID        string     `json:"tenant_id" binding:"required"`
		UserID          string     `json:"user_id" binding:"required"`
		Limit           int        `json:"limit"`
		From            *time.Time `json:"from"`
		To              *time.Time `json:"to"`
		Action          string     `json:"action"`
		CursorCreatedAt *time.Time `json:"cursor_created_at"`
		CursorID        *string    `json:"cursor_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	items, err := h.userSvc.ListAliasAudit(c.Request.Context(), req.TenantID, req.UserID, req.Limit, req.From, req.To, req.Action, req.CursorCreatedAt, req.CursorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) listTenants(c *gin.Context) {
	items, err := h.tenantSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) getTenant(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.tenantSvc.GetByID(c.Request.Context(), req.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) createTenant(c *gin.Context) {
	var item chatdomain.Tenant
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.tenantSvc.Create(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) updateTenant(c *gin.Context) {
	var item chatdomain.Tenant
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.tenantSvc.UpdateConfig(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) createFile(c *gin.Context) {
	var item domain.FileObject
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if item.TenantID == "" || item.RoomID == "" || item.UploaderID == "" || item.ObjectKey == "" || item.ContentType == "" || item.OriginalName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id, room_id, uploader_id, object_key, content_type, original_name are required"})
		return
	}
	created, err := h.fileRepo.Create(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) searchFilesByRoom(c *gin.Context) {
	tenantID := c.Query("tenant_id")
	roomID := c.Query("room_id")
	if tenantID == "" || roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id and room_id are required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items, err := h.fileRepo.SearchByRoom(c.Request.Context(), tenantID, roomID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) searchMessages(c *gin.Context) {
	var req struct {
		TenantID string  `json:"tenant_id" binding:"required"`
		Q        string  `json:"q" binding:"required"`
		RoomID   *string `json:"room_id"`
		Limit    int     `json:"limit"`
		CursorID *string `json:"cursor_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 30
	}
	items, err := h.chatSvc.SearchMessages(c.Request.Context(), req.TenantID, req.Q, req.RoomID, req.Limit, req.CursorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) createRoom(c *gin.Context) {
	var req struct {
		TenantID  string              `json:"tenant_id" binding:"required"`
		Room      chatdomain.ChatRoom `json:"room" binding:"required"`
		MemberIDs []string            `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Room.Name == "" || req.Room.RoomType == "" || req.Room.CreatedBy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room.name, room.room_type, room.created_by are required"})
		return
	}
	roomID, err := h.chatSvc.CreateRoom(c.Request.Context(), req.TenantID, req.Room, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"room_id": roomID})
}

func (h *Handler) addMember(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		RoomID   string `json:"room_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.chatSvc.AddMember(c.Request.Context(), req.TenantID, req.RoomID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) createMessage(c *gin.Context) {
	var req chatdomain.Message
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TenantID == "" || req.RoomID == "" || req.SenderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id, room_id, sender_id are required"})
		return
	}
	created, err := h.chatSvc.CreateMessage(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) markReadUpTo(c *gin.Context) {
	var req struct {
		TenantID  string `json:"tenant_id" binding:"required"`
		RoomID    string `json:"room_id" binding:"required"`
		UserID    string `json:"user_id" binding:"required"`
		MessageID string `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.chatSvc.MarkReadUpTo(c.Request.Context(), req.TenantID, req.RoomID, req.UserID, req.MessageID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) listMessages(c *gin.Context) {
	var req struct {
		TenantID string  `json:"tenant_id" binding:"required"`
		RoomID   string  `json:"room_id" binding:"required"`
		Limit    int     `json:"limit"`
		CursorID *string `json:"cursor_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	items, err := h.chatSvc.ListMessages(c.Request.Context(), req.TenantID, req.RoomID, req.Limit, req.CursorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) messageReaders(c *gin.Context) {
	var req struct {
		TenantID  string `json:"tenant_id" binding:"required"`
		RoomID    string `json:"room_id" binding:"required"`
		MessageID string `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.chatSvc.GetMessageReaders(c.Request.Context(), req.TenantID, req.RoomID, req.MessageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) lastReadMessage(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		RoomID   string `json:"room_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	messageID, err := h.chatSvc.GetLastReadMessageID(c.Request.Context(), req.TenantID, req.RoomID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message_id": messageID})
}

func (h *Handler) unreadCount(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		RoomID   string `json:"room_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count, err := h.chatSvc.GetUnreadCount(c.Request.Context(), req.TenantID, req.RoomID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) unreadCounts(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.chatSvc.GetUnreadCounts(c.Request.Context(), req.TenantID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) listMyRooms(c *gin.Context) {
	var req struct {
		TenantID        string     `json:"tenant_id" binding:"required"`
		UserID          string     `json:"user_id" binding:"required"`
		Limit           int        `json:"limit"`
		CursorCreatedAt *time.Time `json:"cursor_created_at"`
		CursorRoomID    *string    `json:"cursor_room_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	items, err := h.chatSvc.ListMyRooms(c.Request.Context(), req.TenantID, req.UserID, req.Limit, req.CursorCreatedAt, req.CursorRoomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
