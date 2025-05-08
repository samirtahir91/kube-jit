package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AccessLogger is a middleware that logs the access details of each request
// It logs the request method, path, query parameters, client IP, user agent, latency, and user information
func AccessLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		userID, _ := c.Get("userID")
		username, _ := c.Get("username")

		logger.Info("access",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
			zap.String("userID", userIDString(userID)),
			zap.String("username", usernameString(username)),
			zap.Time("time", time.Now()),
		)
	}
}

// userIDString converts the userID to a string, if possible
// Otherwise, it returns an empty string
func userIDString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// usernameString converts the username to a string, if possible
// Otherwise, it returns an empty string
func usernameString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}
