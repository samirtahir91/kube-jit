package routes

import (
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Set required env vars for middleware and handlers
	os.Setenv("HMAC_SECRET", "this-is-a-32-byte-secret-key!!")
	os.Setenv("ALLOW_ORIGINS", `["http://localhost:1234"]`)
	os.Setenv("OAUTH_PROVIDER", "azure")
	os.Setenv("OAUTH_AZURE_CLIENT_ID", "test-azure-client-id")
	os.Setenv("AZURE_AUTH_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/authorize") // <-- Add this line
	os.Setenv("AZURE_TOKEN_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/token")

	// Register gob type for session
	gob.Register(map[string]interface{}{})

	// Add session store middleware (needed for sessioncookie and RequireAuth)
	store := cookie.NewStore([]byte("test-session-secret"))
	r.Use(sessions.Sessions("mysession", store))

	SetupRoutes(r)
	return r
}

func Test_UnauthenticatedRoutes_Accessible(t *testing.T) {
	r := setupTestRouter()
	unauthRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/kube-jit-api/healthz"},
		{"GET", "/kube-jit-api/client_id"},
		{"POST", "/kube-jit-api/logout"},
		{"GET", "/kube-jit-api/build-sha"},
	}

	for _, route := range unauthRoutes {
		t.Run(route.method+"_"+strings.ReplaceAll(route.path, "/", "_"), func(t *testing.T) {
			req, _ := http.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Route %s %s should be registered", route.method, route.path)
		})
	}
}

func Test_AuthenticatedRoutes_RequireAuth(t *testing.T) {
	r := setupTestRouter()
	authRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/kube-jit-api/approving-groups"},
		{"GET", "/kube-jit-api/roles-and-clusters"},
		{"GET", "/kube-jit-api/github/profile"},
		{"GET", "/kube-jit-api/google/profile"},
		{"GET", "/kube-jit-api/azure/profile"},
		{"POST", "/kube-jit-api/submit-request"},
		{"GET", "/kube-jit-api/history"},
		{"GET", "/kube-jit-api/approvals"},
		{"POST", "/kube-jit-api/approve-reject"},
		{"POST", "/kube-jit-api/permissions"},
		{"POST", "/kube-jit-api/admin/clean-expired"},
	}

	for _, route := range authRoutes {
		t.Run(route.method+"_"+strings.ReplaceAll(route.path, "/", "_"), func(t *testing.T) {
			req, _ := http.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code, "Route %s %s should require authentication", route.method, route.path)
		})
	}
}
