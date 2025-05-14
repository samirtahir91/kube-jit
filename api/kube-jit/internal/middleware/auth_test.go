package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireAuth(t *testing.T) {
	testCases := []struct {
		name                string
		setupSession        func(s sessions.Session)
		expectedStatus      int
		expectedBody        gin.H
		expectNextCalled    bool
		expectedUserID      interface{}
		expectedUsername    interface{}
		expectedSessionData interface{}
	}{
		{
			name: "Successful authentication",
			setupSession: func(s sessions.Session) {
				s.Set("data", map[string]interface{}{
					"id":   "user123",
					"name": "Test User",
					"role": "admin",
				})
			},
			expectedStatus:   http.StatusOK,
			expectNextCalled: true,
			expectedUserID:   "user123",
			expectedUsername: "Test User",
			expectedSessionData: map[string]interface{}{
				"id":   "user123",
				"name": "Test User",
				"role": "admin",
			},
		},
		{
			name: "No session data in cookies",
			setupSession: func(s sessions.Session) {
				// Do nothing
			},
			expectedStatus:      http.StatusUnauthorized,
			expectedBody:        gin.H{"error": "Unauthorized: no session data in cookies"},
			expectNextCalled:    false,
			expectedUserID:      nil,
			expectedUsername:    nil,
			expectedSessionData: nil,
		},
		{
			name: "Invalid session data format (not a map)",
			setupSession: func(s sessions.Session) {
				s.Set("data", "this is not a map")
			},
			expectedStatus:      http.StatusInternalServerError,
			expectedBody:        gin.H{"error": "Invalid session data format"},
			expectNextCalled:    false,
			expectedUserID:      nil,
			expectedUsername:    nil,
			expectedSessionData: nil,
		},
		{
			name: "Session data exists but id is not a string",
			setupSession: func(s sessions.Session) {
				s.Set("data", map[string]interface{}{
					"id":   123,
					"name": "Test User",
				})
			},
			expectedStatus:   http.StatusOK,
			expectNextCalled: true,
			expectedUserID:   nil,
			expectedUsername: "Test User",
			expectedSessionData: map[string]interface{}{
				"id":   123,
				"name": "Test User",
			},
		},
		{
			name: "Session data exists but name is not a string",
			setupSession: func(s sessions.Session) {
				s.Set("data", map[string]interface{}{
					"id":   "user123",
					"name": true,
				})
			},
			expectedStatus:   http.StatusOK,
			expectNextCalled: true,
			expectedUserID:   "user123",
			expectedUsername: nil,
			expectedSessionData: map[string]interface{}{
				"id":   "user123",
				"name": true,
			},
		},
		{
			name: "Session data exists but id and name are missing",
			setupSession: func(s sessions.Session) {
				s.Set("data", map[string]interface{}{
					"role": "guest",
				})
			},
			expectedStatus:      http.StatusOK,
			expectNextCalled:    true,
			expectedUserID:      nil,
			expectedUsername:    nil,
			expectedSessionData: map[string]interface{}{"role": "guest"},
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			store := cookie.NewStore([]byte("secret"))
			r.Use(sessions.Sessions("mysession", store))

			if tc.setupSession != nil {
				r.Use(func(c *gin.Context) {
					s := sessions.Default(c)
					tc.setupSession(s)
					c.Next()
				})
			}
			r.Use(RequireAuth())

			nextCalled := false
			var actualSessionData, actualUserID, actualUsername interface{}
			var sessionDataExists, userIDExists, usernameExists bool

			r.GET("/testauth", func(c *gin.Context) {
				nextCalled = true

				actualSessionData, sessionDataExists = c.Get("sessionData")
				actualUserID, userIDExists = c.Get("userID")
				actualUsername, usernameExists = c.Get("username")

				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/testauth", nil)

			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code, "HTTP status code mismatch for test '%s'", tc.name)

			if w.Code != http.StatusOK && tc.expectedBody != nil {
				var responseBody gin.H
				err := json.Unmarshal(w.Body.Bytes(), &responseBody)
				assert.NoError(t, err, "Failed to unmarshal response body for test '%s'", tc.name)
				assert.Equal(t, tc.expectedBody, responseBody, "Response body mismatch for test '%s'", tc.name)
			}

			assert.Equal(t, tc.expectNextCalled, nextCalled, "c.Next() call expectation mismatch for test '%s'", tc.name)

			if nextCalled {
				if tc.expectedSessionData != nil {
					assert.True(t, sessionDataExists, "sessionData should exist in context for test '%s'", tc.name)
					assert.Equal(t, tc.expectedSessionData, actualSessionData, "sessionData content mismatch for test '%s'", tc.name)
				} else {
					assert.False(t, sessionDataExists, "sessionData should not exist in context for test '%s'", tc.name)
				}

				if tc.expectedUserID != nil {
					assert.True(t, userIDExists, "userID should exist in context for test '%s'", tc.name)
					assert.Equal(t, tc.expectedUserID, actualUserID, "userID content mismatch for test '%s'", tc.name)
				} else {
					assert.False(t, userIDExists, "userID should not exist in context for test '%s'", tc.name)
				}

				if tc.expectedUsername != nil {
					assert.True(t, usernameExists, "username should exist in context for test '%s'", tc.name)
					assert.Equal(t, tc.expectedUsername, actualUsername, "username content mismatch for test '%s'", tc.name)
				} else {
					assert.False(t, usernameExists, "username should not exist in context for test '%s'", tc.name)
				}
			}
		})
	}
}
