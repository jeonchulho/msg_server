package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	commonauth "msg_server/server/common/auth"
	"msg_server/server/common/middleware"
	"msg_server/server/common/transport/httpresp"
	"msg_server/server/fileman/domain"
	"msg_server/server/fileman/service"
)

type Handler struct {
	files *service.FileService
	auth  *commonauth.Service
}

func NewHandler(files *service.FileService, auth *commonauth.Service) *Handler {
	return &Handler{files: files, auth: auth}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(h.auth))
	{
		api.POST("/files/presign-upload", h.presignUpload)
		api.POST("/files/presign-download", h.presignDownload)
		api.POST("/files/register", h.registerFile)
	}
}

func (h *Handler) presignUpload(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		ObjectKey string `json:"object_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	url, err := h.files.PresignUpload(c.Request.Context(), tenantID, req.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewURLResponse(url))
}

func (h *Handler) presignDownload(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		ObjectKey string `json:"object_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	url, err := h.files.PresignDownload(c.Request.Context(), tenantID, req.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, httpresp.NewURLResponse(url))
}

func (h *Handler) registerFile(c *gin.Context) {
	tenantID, err := tenantFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	uploaderID, err := actorFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrUnauthorized))
		return
	}
	var req struct {
		RoomID       string `json:"room_id" binding:"required"`
		ObjectKey    string `json:"object_key" binding:"required"`
		ContentType  string `json:"content_type" binding:"required"`
		SizeBytes    int64  `json:"size_bytes" binding:"required"`
		OriginalName string `json:"original_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpresp.NewErrorResponse(err.Error()))
		return
	}
	item, err := h.files.RegisterAndMaybeThumbnail(c.Request.Context(), domain.FileObject{
		TenantID:     tenantID,
		RoomID:       req.RoomID,
		UploaderID:   uploaderID,
		ObjectKey:    req.ObjectKey,
		ContentType:  req.ContentType,
		SizeBytes:    req.SizeBytes,
		OriginalName: req.OriginalName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpresp.NewErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, item)
}

func tenantFromContext(c *gin.Context) (string, error) {
	rawTenantID, ok := c.Get("auth_tenant_id")
	if !ok {
		return "", http.ErrNoCookie
	}
	tenantID, ok := rawTenantID.(string)
	if !ok || tenantID == "" {
		return "", http.ErrNoCookie
	}
	return tenantID, nil
}

func actorFromContext(c *gin.Context) (string, error) {
	rawUserID, ok := c.Get("auth_user_id")
	if !ok {
		return "", http.ErrNoCookie
	}
	userID, ok := rawUserID.(string)
	if !ok || userID == "" {
		return "", http.ErrNoCookie
	}
	return userID, nil
}
