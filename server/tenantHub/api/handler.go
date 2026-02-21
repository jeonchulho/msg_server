package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	chatdomain "msg_server/server/chat/domain"
	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
	"msg_server/server/common/transport/httpresp"
	tenantHub "msg_server/server/tenantHub/service"
)

type Handler struct {
	tenant *tenantHub.Service
	auth   *commonauth.Service
}

func NewHandler(tenant *tenantHub.Service, auth *commonauth.Service) *Handler {
	return &Handler{tenant: tenant, auth: auth}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		adminOrManager := api.Group("")
		adminOrManager.Use(middleware.RequireRoles(string(chatdomain.UserRoleAdmin), string(chatdomain.UserRoleManager)))
		adminOrManager.GET("/tenants", h.listTenants)
		adminOrManager.POST("/tenants", h.createTenant)
		adminOrManager.PATCH("/tenants/:id", h.updateTenant)
	}
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
