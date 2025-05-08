package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// RequireAuth is a middleware that checks if the user is authenticated.
// It retrieves session data from cookies and sets it in the context.
// If the session data is not found or invalid, it returns an unauthorized response.
// It also sets user ID and username in the context for logging purposes.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		combinedData := session.Get("data")
		if combinedData == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no session data in cookies"})
			c.Abort()
			return
		}
		sessionData, ok := combinedData.(map[string]interface{})
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session data format"})
			c.Abort()
			return
		}
		c.Set("sessionData", sessionData)
		if id, ok := sessionData["id"].(string); ok {
			c.Set("userID", id)
		}
		if name, ok := sessionData["name"].(string); ok {
			c.Set("username", name)
		}
		c.Next()
	}
}
