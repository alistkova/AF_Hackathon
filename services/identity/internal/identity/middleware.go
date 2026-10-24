package identity

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(store Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		if h := c.GetHeader("Authorization"); h != "" {
			parts := strings.Fields(h)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
		if token == "" {
			for _, name := range []string{"session", "session_token", "token"} {
				if v, err := c.Cookie(name); err == nil && v != "" {
					token = v
					break
				}
			}
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized"})
			c.Abort()
			return
		}
		u, err := store.Me(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized"})
			c.Abort()
			return
		}
		c.Set("token", token)
		c.Set("userID", u.ID)
		c.Next()
	}
}
