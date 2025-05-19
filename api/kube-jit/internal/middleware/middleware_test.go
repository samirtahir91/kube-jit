package middleware

import (
	"encoding/gob"
	"encoding/json"
	"kube-jit/pkg/sessioncookie"
	"kube-jit/pkg/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestMain sets up loggers and registers types for gob encoding.
func TestMain(m *testing.M) {
	nopLogger := zap.NewNop() // Use zap.NewDevelopment() for verbose logs during debugging

	InitLogger(nopLogger)               // For the current 'middleware' package
	sessioncookie.InitLogger(nopLogger) // For the 'sessioncookie' package
	utils.InitLogger(nopLogger)         // For the 'utils' package

	// Register types that might be used in sessions.
	// map[string]interface{} is used for the "data" key by sessioncookie middleware.
	gob.Register(map[string]interface{}{})

	os.Exit(m.Run())
}

func TestSetupMiddleware_CORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set required environment variables for CORS and session setup
	// HMAC_SECRET is needed by SetupMiddleware for the session store
	t.Setenv("HMAC_SECRET", "test-secret-for-middleware-test") // A simple secret for testing
	allowedOrigins := []string{"http://localhost:1234", "https://kube-jit.example.com"}
	allowedOriginsJSON, _ := json.Marshal(allowedOrigins)
	t.Setenv("ALLOW_ORIGINS", string(allowedOriginsJSON))

	r := gin.New()
	SetupMiddleware(r) // Apply the middleware setup

	// Add a dummy handler
	r.GET("/testcors", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	testCases := []struct {
		name                   string
		originHeader           string
		expectedHTTPStatus     int // Added to specify expected status for each case
		expectedAllowOrigin    string
		expectAllowCredentials string
	}{
		{
			name:                   "Allowed origin localhost",
			originHeader:           "http://localhost:1234",
			expectedHTTPStatus:     http.StatusOK,
			expectedAllowOrigin:    "http://localhost:1234",
			expectAllowCredentials: "true",
		},
		{
			name:                   "Allowed origin example.com",
			originHeader:           "https://kube-jit.example.com",
			expectedHTTPStatus:     http.StatusOK,
			expectedAllowOrigin:    "https://kube-jit.example.com",
			expectAllowCredentials: "true",
		},
		{
			name:                   "Disallowed origin",
			originHeader:           "http://disallowed.com",
			expectedHTTPStatus:     http.StatusForbidden, // Corrected expected status
			expectedAllowOrigin:    "",
			expectAllowCredentials: "",
		},
		{
			name:                   "No origin header",
			originHeader:           "",
			expectedHTTPStatus:     http.StatusOK, // No origin usually means not a cross-origin request, so OK
			expectedAllowOrigin:    "",
			expectAllowCredentials: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/testcors", nil)
			if tc.originHeader != "" {
				req.Header.Set("Origin", tc.originHeader)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedHTTPStatus, w.Code) // Use tc.expectedHTTPStatus

			if tc.expectedHTTPStatus == http.StatusOK { // Only check these headers if request was not forbidden
				if tc.expectedAllowOrigin != "" {
					assert.Equal(t, tc.expectedAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))
					assert.Equal(t, tc.expectAllowCredentials, w.Header().Get("Access-Control-Allow-Credentials"))
				} else if tc.originHeader != "" && !contains(allowedOrigins, tc.originHeader) {
					assert.NotEqual(t, tc.originHeader, w.Header().Get("Access-Control-Allow-Origin"), "ACAO should not reflect a disallowed origin")
				}
			}

			// Test OPTIONS request for preflight
			optionsReq, _ := http.NewRequest(http.MethodOptions, "/testcors", nil)
			if tc.originHeader != "" {
				optionsReq.Header.Set("Origin", tc.originHeader)
				optionsReq.Header.Set("Access-Control-Request-Method", "GET")
			}
			optionsW := httptest.NewRecorder()
			r.ServeHTTP(optionsW, optionsReq)

			if tc.originHeader != "" && contains(allowedOrigins, tc.originHeader) {
				assert.Equal(t, http.StatusNoContent, optionsW.Code, "OPTIONS request status mismatch")
				assert.Equal(t, tc.expectedAllowOrigin, optionsW.Header().Get("Access-Control-Allow-Origin"))
				assert.NotEmpty(t, optionsW.Header().Get("Access-Control-Allow-Methods"))
			} else if tc.originHeader != "" && !contains(allowedOrigins, tc.originHeader) {
				// For a disallowed origin, an OPTIONS request might also be forbidden or return no CORS headers.
				// The gin-contrib/cors default is to return 403 for disallowed origins on OPTIONS requests too.
				assert.Equal(t, http.StatusForbidden, optionsW.Code, "OPTIONS request for disallowed origin should be forbidden")
			}
		})
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
