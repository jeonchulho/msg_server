package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	chatdomain "msg_server/server/chat/domain"
	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
	"msg_server/server/common/transport/httpresp"
	orgHub "msg_server/server/orgHub"
	sessiondomain "msg_server/server/session/domain"
	sessionservice "msg_server/server/session/service"
	tenantHub "msg_server/server/tenantHub"
)

type Handler struct {
	users  *orgHub.Service
	tenant *tenantHub.Service
	svc    *sessionservice.Service
	auth   *commonauth.Service
	hub    *sessionservice.Hub
}

func NewHandler(users *orgHub.Service, tenant *tenantHub.Service, svc *sessionservice.Service, auth *commonauth.Service, hub *sessionservice.Hub) *Handler {
	return &Handler{users: users, tenant: tenant, svc: svc, auth: auth, hub: hub}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ws/session", h.handleSessionWS)
	r.POST("/api/v1/auth/login", h.loginAuth)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		adminOrManager := api.Group("")
		adminOrManager.Use(middleware.RequireRoles(string(chatdomain.UserRoleAdmin), string(chatdomain.UserRoleManager)))
		adminOrManager.POST("/org-units", h.createOrgUnit)
		adminOrManager.GET("/org-units", h.listOrgUnits)
		adminOrManager.POST("/users", h.createUser)
		adminOrManager.GET("/tenants", h.listTenants)
		adminOrManager.POST("/tenants", h.createTenant)
		adminOrManager.PATCH("/tenants/:id", h.updateTenant)

		api.PATCH("/users/:id/status", h.updateUserStatus)
		api.GET("/users/search", h.searchUsers)
		api.GET("/users/me/aliases", h.listMyAliases)
		api.POST("/users/me/aliases", h.addMyAlias)
		api.DELETE("/users/me/aliases", h.deleteMyAlias)
		api.GET("/users/me/aliases/audit", h.listMyAliasAudit)

		api.POST("/session/login", h.loginDevice)
		api.PATCH("/session/status", h.updateSessionStatus)
		api.POST("/notes", h.sendNote)
		api.GET("/notes/inbox", h.listInbox)
		api.POST("/notes/:id/read", h.markNoteRead)
		api.POST("/chat/notify", h.notifyChat)
	}
}

func (h *Handler) createOrgUnit(c *gin.Context) {
	tenantID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		ParentID *string `json:"parent_id"`
		Name     string  `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	id, err := h.users.CreateOrgUnit(c.Request.Context(), tenantID, req.ParentID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, httpresp.NewIDResponse(id))
}

func (h *Handler) listOrgUnits(c *gin.Context) {
	tenantID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	items, err := h.users.ListOrgUnits(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) createUser(c *gin.Context) {
	tenantID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		OrgID      string                `json:"org_id" binding:"required"`
		Email      string                `json:"email" binding:"required"`
		Password   string                `json:"password" binding:"required"`
		Name       string                `json:"name" binding:"required"`
		Title      string                `json:"title"`
		Role       chatdomain.UserRole   `json:"role"`
		Status     chatdomain.UserStatus `json:"status"`
		StatusNote string                `json:"status_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	id, err := h.users.CreateUser(c.Request.Context(), tenantID, chatdomain.User{
		OrgID:        req.OrgID,
		Email:        req.Email,
		PasswordHash: req.Password,
		Name:         req.Name,
		Title:        req.Title,
		Role:         req.Role,
		Status:       req.Status,
		StatusNote:   req.StatusNote,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, httpresp.NewIDResponse(id))
}

func (h *Handler) updateUserStatus(c *gin.Context) {
	targetUserID := c.Param("id")
	tenantID, actorID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	actorRole, err := actorRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	if actorID != targetUserID && actorRole != string(chatdomain.UserRoleAdmin) && actorRole != string(chatdomain.UserRoleManager) {
		c.JSON(http.StatusForbidden, httpresp.NewErrorResponse(httpresp.ErrCannotUpdateOtherUserState))
		return
	}
	var req struct {
		Status chatdomain.UserStatus `json:"status" binding:"required"`
		Note   string                `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if err := h.users.UpdateStatus(c.Request.Context(), tenantID, targetUserID, req.Status, req.Note); err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func (h *Handler) searchUsers(c *gin.Context) {
	tenantID, _, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.users.SearchUsers(c.Request.Context(), tenantID, q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) listMyAliases(c *gin.Context) {
	tenantID, actorID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	aliases, err := h.users.ListAliases(c.Request.Context(), tenantID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, AliasesResponse{Aliases: aliases})
}

func (h *Handler) addMyAlias(c *gin.Context) {
	tenantID, actorID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		Alias string `json:"alias" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if err := h.users.AddAlias(c.Request.Context(), tenantID, actorID, req.Alias, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func (h *Handler) deleteMyAlias(c *gin.Context) {
	tenantID, actorID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		Alias string `json:"alias" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if err := h.users.DeleteAlias(c.Request.Context(), tenantID, actorID, req.Alias, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewOKResponse())
}

func (h *Handler) listMyAliasAudit(c *gin.Context) {
	tenantID, actorID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	action := c.Query("action")
	cursor := c.Query("cursor")

	var fromPtr *time.Time
	if raw := c.Query("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(httpresp.ErrFromMustBeRFC3339))
			return
		}
		fromPtr = &parsed
	}

	var toPtr *time.Time
	if raw := c.Query("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(httpresp.ErrToMustBeRFC3339))
			return
		}
		toPtr = &parsed
	}

	items, nextCursor, err := h.users.ListAliasAudit(c.Request.Context(), tenantID, actorID, limit, fromPtr, toPtr, action, cursor)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse[chatdomain.AliasAudit]{Items: items, NextCursor: nextCursor})
}

func (h *Handler) listTenants(c *gin.Context) {
	items, err := h.tenant.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) createTenant(c *gin.Context) {
	var req struct {
		TenantID                string `json:"tenant_id" binding:"required"`
		Name                    string `json:"name" binding:"required"`
		DeploymentMode          string `json:"deployment_mode" binding:"required"`
		DedicatedDSN            string `json:"dedicated_dsn"`
		DedicatedRedisAddr      string `json:"dedicated_redis_addr"`
		DedicatedLavinMQURL     string `json:"dedicated_lavinmq_url"`
		DedicatedMinIOEndpoint  string `json:"dedicated_minio_endpoint"`
		DedicatedMinIOAccessKey string `json:"dedicated_minio_access_key"`
		DedicatedMinIOSecretKey string `json:"dedicated_minio_secret_key"`
		DedicatedMinIOBucket    string `json:"dedicated_minio_bucket"`
		DedicatedMinIOUseSSL    bool   `json:"dedicated_minio_use_ssl"`
		UserCountThreshold      int    `json:"user_count_threshold"`
		IsActive                *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	if req.UserCountThreshold == 0 {
		req.UserCountThreshold = 200
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	item, err := h.tenant.Create(c.Request.Context(), chatdomain.Tenant{
		TenantID:                req.TenantID,
		Name:                    req.Name,
		DeploymentMode:          req.DeploymentMode,
		DedicatedDSN:            req.DedicatedDSN,
		DedicatedRedisAddr:      req.DedicatedRedisAddr,
		DedicatedLavinMQURL:     req.DedicatedLavinMQURL,
		DedicatedMinIOEndpoint:  req.DedicatedMinIOEndpoint,
		DedicatedMinIOAccessKey: req.DedicatedMinIOAccessKey,
		DedicatedMinIOSecretKey: req.DedicatedMinIOSecretKey,
		DedicatedMinIOBucket:    req.DedicatedMinIOBucket,
		DedicatedMinIOUseSSL:    req.DedicatedMinIOUseSSL,
		UserCountThreshold:      req.UserCountThreshold,
		IsActive:                isActive,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) updateTenant(c *gin.Context) {
	tenantID := c.Param("id")
	var req struct {
		Name                    string `json:"name" binding:"required"`
		DeploymentMode          string `json:"deployment_mode" binding:"required"`
		DedicatedDSN            string `json:"dedicated_dsn"`
		DedicatedRedisAddr      string `json:"dedicated_redis_addr"`
		DedicatedLavinMQURL     string `json:"dedicated_lavinmq_url"`
		DedicatedMinIOEndpoint  string `json:"dedicated_minio_endpoint"`
		DedicatedMinIOAccessKey string `json:"dedicated_minio_access_key"`
		DedicatedMinIOSecretKey string `json:"dedicated_minio_secret_key"`
		DedicatedMinIOBucket    string `json:"dedicated_minio_bucket"`
		DedicatedMinIOUseSSL    bool   `json:"dedicated_minio_use_ssl"`
		UserCountThreshold      int    `json:"user_count_threshold" binding:"required"`
		IsActive                bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	item, err := h.tenant.UpdateConfig(c.Request.Context(), chatdomain.Tenant{
		TenantID:                tenantID,
		Name:                    req.Name,
		DeploymentMode:          req.DeploymentMode,
		DedicatedDSN:            req.DedicatedDSN,
		DedicatedRedisAddr:      req.DedicatedRedisAddr,
		DedicatedLavinMQURL:     req.DedicatedLavinMQURL,
		DedicatedMinIOEndpoint:  req.DedicatedMinIOEndpoint,
		DedicatedMinIOAccessKey: req.DedicatedMinIOAccessKey,
		DedicatedMinIOSecretKey: req.DedicatedMinIOSecretKey,
		DedicatedMinIOBucket:    req.DedicatedMinIOBucket,
		DedicatedMinIOUseSSL:    req.DedicatedMinIOUseSSL,
		UserCountThreshold:      req.UserCountThreshold,
		IsActive:                req.IsActive,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) loginAuth(c *gin.Context) {
	var req struct {
		TenantID string `json:"tenant_id" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	user, err := h.users.Authenticate(c.Request.Context(), req.TenantID, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrInvalidCredentials))
		return
	}
	token, err := h.auth.GenerateToken(user.ID, req.TenantID, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewTokenResponse(token, user.ID, req.TenantID, string(user.Role)))
}

var sessionUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (h *Handler) handleSessionWS(c *gin.Context) {
	tenantID := strings.TrimSpace(c.Query("tenant_id"))
	userID := strings.TrimSpace(c.Query("user_id"))
	sessionID := strings.TrimSpace(c.Query("session_id"))
	sessionToken := strings.TrimSpace(c.Query("session_token"))

	ok, err := h.svc.ValidateSession(c.Request.Context(), tenantID, userID, sessionID, sessionToken)
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
		_, _ = h.svc.ValidateSession(c.Request.Context(), tenantID, userID, sessionID, sessionToken)
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

	session, err := h.svc.LoginDevice(c.Request.Context(), tenantID, userID, req.DeviceID, req.DeviceName, authToken, req.AllowedTenants)
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
	if err := h.svc.UpdateStatus(c.Request.Context(), tenantID, userID, req.Status, req.StatusNote); err != nil {
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
	note, err := h.svc.SendNote(c.Request.Context(), tenantID, userID, req)
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
	items, err := h.svc.ListInbox(c.Request.Context(), tenantID, userID, limit)
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
	if err := h.svc.MarkNoteRead(c.Request.Context(), tenantID, userID, noteID); err != nil {
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
	if err := h.svc.NotifyChat(c.Request.Context(), tenantID, userID, authToken, req); err != nil {
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

func actorRoleFromContext(c *gin.Context) (string, error) {
	rawRole, ok := c.Get("auth_role")
	if !ok {
		return "", http.ErrNoCookie
	}
	role, ok := rawRole.(string)
	if !ok || strings.TrimSpace(role) == "" {
		return "", http.ErrNoCookie
	}
	return strings.TrimSpace(role), nil
}

type PaginatedResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type AliasesResponse struct {
	Aliases []string `json:"aliases"`
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
