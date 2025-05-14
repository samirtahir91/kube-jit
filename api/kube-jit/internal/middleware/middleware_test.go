package middleware

import (
	"encoding/gob"
	"encoding/json"
	"kube-jit/pkg/sessioncookie"
	"kube-jit/pkg/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
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

func TestSetupMiddleware_SessionSplitCombine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set required environment variables
	// Use a 32-byte secret for HMAC_SECRET as recommended by securecookie
	hmacSecret := "this-is-exactly-32-bytes-long!!!" // Corrected to be 32 bytes
	if len(hmacSecret) != 32 {
		t.Fatalf("HMAC_SECRET must be 32 bytes for this test, got %d", len(hmacSecret)) // More informative fatal message
	}
	t.Setenv("HMAC_SECRET", hmacSecret)
	t.Setenv("ALLOW_ORIGINS", `["http://localhost:7890"]`) // Needed by SetupMiddleware
	t.Setenv("COOKIE_SAMESITE", "Lax")                     // Explicitly set for predictability

	r := gin.New()
	SetupMiddleware(r) // Apply the middleware setup

	sessionKey := "testData"
	smallTestData := map[string]interface{}{"message": "hello", "count": 1}
	// Create data large enough to ensure splitting (maxCookieSize is 4000)
	// Encoded JSON and then securecookie encoding will add overhead.
	// A string of 3000 'a's, when JSON marshaled and encoded, should exceed 4000 bytes.
	largeString := strings.Repeat("a", 3000)
	largeTestData := map[string]interface{}{"largeMessage": largeString, "id": "large-id"}

	// Handler to set session data
	r.POST("/set-session", func(c *gin.Context) {
		var reqBody map[string]interface{}
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
			return
		}
		s := sessions.Default(c)
		s.Set(sessionKey, reqBody["payload"])
		// No s.Save() needed here, SplitAndCombineSessionMiddleware handles saving "data"
		// by reading from session.Get("data") after c.Next()
		// For this test, we are setting the "data" key directly as the custom middleware expects.
		// The custom middleware reads session.Get("data") and writes it to cookies.
		// And on incoming, it reads cookies and does session.Set("data", ...)
		// So, to test the writing part, we need to ensure "data" is set.
		// The auth middleware sets "sessionData", but the split/combine middleware uses "data".
		// Let's assume the "data" key is the one managed by SplitAndCombine.
		s.Set("data", reqBody["payload"]) // This is what SplitSessionData will look for
		c.Status(http.StatusOK)
	})

	// Handler to get session data
	r.GET("/get-session", func(c *gin.Context) {
		s := sessions.Default(c)
		// CombineSessionData should have populated session.Get("data")
		retrievedData := s.Get("data")
		if retrievedData == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session data not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"retrieved": retrievedData})
	})

	runSessionTest := func(payloadToSet map[string]interface{}, expectSplit bool) {
		// 1. Set session data
		setBody, _ := json.Marshal(map[string]interface{}{"payload": payloadToSet})
		setReq, _ := http.NewRequest(http.MethodPost, "/set-session", strings.NewReader(string(setBody)))
		setReq.Header.Set("Content-Type", "application/json")
		setW := httptest.NewRecorder()
		r.ServeHTTP(setW, setReq)
		assert.Equal(t, http.StatusOK, setW.Code)

		// 2. Extract cookies
		respCookies := setW.Result().Cookies()
		var sessionCookies []*http.Cookie
		var foundSessionCookie bool
		for _, cookie := range respCookies {
			if strings.HasPrefix(cookie.Name, sessioncookie.SessionPrefix) {
				sessionCookies = append(sessionCookies, cookie)
				foundSessionCookie = true
				assert.True(t, cookie.HttpOnly, "Cookie should be HttpOnly")
				assert.True(t, cookie.Secure, "Cookie should be Secure")
				assert.Equal(t, "/", cookie.Path, "Cookie path should be /")
				assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite, "Cookie SameSite mismatch") // Based on t.Setenv
				assert.Equal(t, 3600, cookie.MaxAge, "Cookie MaxAge mismatch")
			}
		}
		assert.True(t, foundSessionCookie, "Should find at least one session cookie")
		if expectSplit {
			assert.Greater(t, len(sessionCookies), 1, "Expected session data to be split into multiple cookies")
		} else {
			assert.LessOrEqual(t, len(sessionCookies), 1, "Expected session data in a single cookie or not split")
		}

		// 3. Get session data using the cookies
		getReq, _ := http.NewRequest(http.MethodGet, "/get-session", nil)
		for _, c := range sessionCookies {
			getReq.AddCookie(c)
		}
		getW := httptest.NewRecorder()
		r.ServeHTTP(getW, getReq)
		assert.Equal(t, http.StatusOK, getW.Code)

		var getRespBody map[string]interface{}
		err := json.Unmarshal(getW.Body.Bytes(), &getRespBody)
		assert.NoError(t, err)

		// Asserting map equality can be tricky with testify if types differ (e.g. int vs float64 from JSON)
		// We compare the marshaled JSON strings for a robust comparison of complex structures.
		expectedPayloadJSON, _ := json.Marshal(payloadToSet)
		retrievedPayloadJSON, _ := json.Marshal(getRespBody["retrieved"])
		assert.JSONEq(t, string(expectedPayloadJSON), string(retrievedPayloadJSON), "Retrieved session data does not match set data")
	}

	t.Run("SmallSessionData", func(t *testing.T) {
		runSessionTest(smallTestData, false)
	})

	t.Run("LargeSessionData", func(t *testing.T) {
		runSessionTest(largeTestData, true)
	})
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
