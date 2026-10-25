package identity

import (
	"net/http"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(store Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}

		user, err := store.Me(c.Request.Context(), token)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Сессия недействительна")
			return
		}

		c.Set("token", token)
		c.Set("userID", user.ID)
		c.Set("userEmail", user.Email)
		c.Set("role", user.Role)
		c.Set("primaryCompanyID", user.PrimaryCompanyID)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != RoleAdmin {
			httpx.Error(c, http.StatusForbidden, "forbidden", "Недостаточно прав")
			return
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	if header := c.GetHeader("Authorization"); header != "" {
		parts := strings.Fields(header)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	if value, err := c.Cookie(cookieName()); err == nil {
		return value
	}
	return ""
}
