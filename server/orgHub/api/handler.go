package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	chatdomain "msg_server/server/chat/domain"
	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
	"msg_server/server/common/transport/httpresp"
	orgHub "msg_server/server/orgHub"
)

type Handler struct {
	users *orgHub.Service
	auth  *commonauth.Service
}

func NewHandler(users *orgHub.Service, auth *commonauth.Service) *Handler {
	return &Handler{users: users, auth: auth}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/v1/auth/login", h.loginAuth)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		adminOrManager := api.Group("")
		adminOrManager.Use(middleware.RequireRoles(string(chatdomain.UserRoleAdmin), string(chatdomain.UserRoleManager)))
		adminOrManager.POST("/org-units", h.createOrgUnit)
		adminOrManager.GET("/org-units", h.listOrgUnits)
		adminOrManager.POST("/users", h.createUser)

		api.PATCH("/users/:id/status", h.updateUserStatus)
		api.GET("/users/search", h.searchUsers)
		api.GET("/users/me/aliases", h.listMyAliases)
		api.POST("/users/me/aliases", h.addMyAlias)
		api.DELETE("/users/me/aliases", h.deleteMyAlias)
		api.GET("/users/me/aliases/audit", h.listMyAliasAudit)
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
