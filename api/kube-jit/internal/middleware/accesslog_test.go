package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// setupObservedLogger creates a zap logger that captures logs for testing.
func setupObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, observedLogs := observer.New(zapcore.InfoLevel)
	return zap.New(core), observedLogs
}

func TestAccessLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name              string
		userIDInContext   interface{}
		usernameInContext interface{}
		expectedUserID    string
		expectedUsername  string
		setupContext      func(c *gin.Context)
	}{
		{
			name:              "With userID and username in context",
			userIDInContext:   "testUser123",
			usernameInContext: "Test User Name",
			expectedUserID:    "testUser123",
			expectedUsername:  "Test User Name",
			setupContext: func(c *gin.Context) {
				c.Set("userID", "testUser123")
				c.Set("username", "Test User Name")
			},
		},
		{
			name:              "Without userID and username in context",
			userIDInContext:   nil,
			usernameInContext: nil,
			expectedUserID:    "",
			expectedUsername:  "",
			setupContext:      func(c *gin.Context) {},
		},
		{
			name:              "With non-string userID and username in context",
			userIDInContext:   123,
			usernameInContext: true,
			expectedUserID:    "",
			expectedUsername:  "",
			setupContext: func(c *gin.Context) {
				c.Set("userID", 123)
				c.Set("username", true)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, observedLogs := setupObservedLogger()
			r := gin.New()
			r.Use(AccessLogger(logger))

			var nextCalled bool
			r.GET("/test", func(c *gin.Context) {
				if tc.setupContext != nil {
					tc.setupContext(c)
				}
				nextCalled = true
				c.String(http.StatusOK, "OK")
			})

			req, _ := http.NewRequest(http.MethodGet, "/test?query=param", nil)
			req.Header.Set("User-Agent", "test-agent")
			req.RemoteAddr = "192.0.2.1:12345" // For c.ClientIP()

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.True(t, nextCalled, "c.Next() was not called")
			assert.Equal(t, http.StatusOK, w.Code)

			logs := observedLogs.All()
			assert.Len(t, logs, 1, "Expected one log entry")

			logEntry := logs[0]
			assert.Equal(t, "access", logEntry.Message, "Log message mismatch")

			fields := logEntry.ContextMap() // Use ContextMap() for reliable field retrieval

			// Zap often stores ints as int64 when they become interface{}
			assert.Equal(t, int64(http.StatusOK), fields["status"], "Status code mismatch")
			assert.Equal(t, http.MethodGet, fields["method"], "HTTP method mismatch")
			assert.Equal(t, "/test", fields["path"], "Path mismatch")
			assert.Equal(t, "query=param", fields["query"], "Query mismatch")
			assert.Equal(t, "192.0.2.1", fields["ip"], "Client IP mismatch")
			assert.Equal(t, "test-agent", fields["user-agent"], "User agent mismatch")

			assert.Contains(t, fields, "latency", "Latency field missing")
			if latency, ok := fields["latency"].(time.Duration); ok {
				assert.True(t, latency >= 0, "Latency should be non-negative")
			} else if latencyInt, ok := fields["latency"].(int64); ok { // Fallback for some observer behaviors
				assert.True(t, latencyInt >= 0, "Latency (as int64) should be non-negative")
			} else {
				t.Errorf("Latency field is not a time.Duration or int64: %T", fields["latency"])
			}

			assert.Equal(t, tc.expectedUserID, fields["userID"], "UserID mismatch")
			assert.Equal(t, tc.expectedUsername, fields["username"], "Username mismatch")

			assert.Contains(t, fields, "time", "Time field missing")
			if logTime, ok := fields["time"].(time.Time); ok {
				assert.WithinDuration(t, time.Now(), logTime, 5*time.Second, "Log time is too far off")
			} else {
				t.Errorf("Time field is not a time.Time: %T", fields["time"])
			}
		})
	}
}

func TestUserIDString(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"String input", "testID", "testID"},
		{"Integer input", 123, ""},
		{"Boolean input", true, ""},
		{"Nil input", nil, ""},
		{"Empty string input", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, userIDString(tc.input))
		})
	}
}

func TestUsernameString(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"String input", "Test User", "Test User"},
		{"Integer input", 456, ""},
		{"Boolean input", false, ""},
		{"Nil input", nil, ""},
		{"Empty string input", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, usernameString(tc.input))
		})
	}
}
