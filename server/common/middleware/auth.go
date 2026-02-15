package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"msg_server/server/common/transport/httpresp"
)

type tokenAuth interface {
	ParseAuthContext(token string) (userID, tenantID, role string, err error)
}

func AuthRequired(auth tokenAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrMissingBearerToken))
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		userID, tenantID, role, err := auth.ParseAuthContext(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, httpresp.NewErrorResponse(httpresp.ErrInvalidToken))
			return
		}
		c.Set("auth_access_token", token)
		c.Set("auth_user_id", userID)
		c.Set("auth_tenant_id", tenantID)
		c.Set("auth_role", role)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[strings.TrimSpace(role)] = struct{}{}
	}
	return func(c *gin.Context) {
		rawRole, ok := c.Get("auth_role")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, httpresp.NewErrorResponse(httpresp.ErrForbidden))
			return
		}
		role, ok := rawRole.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, httpresp.NewErrorResponse(httpresp.ErrForbidden))
			return
		}
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, httpresp.NewErrorResponse(httpresp.ErrInsufficientRole))
			return
		}
		c.Next()
	}
}
