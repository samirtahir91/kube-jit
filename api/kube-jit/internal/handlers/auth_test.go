package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s" // Required for mocking k8s package variables and functions
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured" // Required for mocking GetJitGroups
)

// Mock logger setup
func getTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment() // Use Development logger for more verbose test output if needed, or Nop
	// logger := zap.NewNop()
	return logger
}

// Helper to setup Gin engine with session middleware for handler tests
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))
	return r
}

func TestGetSessionData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Successfully get session data", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		expectedData := map[string]interface{}{"key": "value", "number": 123}
		c.Set("sessionData", expectedData)

		data := GetSessionData(c)
		assert.Equal(t, expectedData, data, "Should retrieve the session data map")
	})

	t.Run("Panic if sessionData is not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.Panics(t, func() {
			GetSessionData(c)
		}, "Should panic if sessionData is not found in context")
	})

	t.Run("Panic if sessionData is wrong type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("sessionData", "not a map") // Set wrong type
		assert.Panics(t, func() {
			GetSessionData(c)
		}, "Should panic if sessionData is not of type map[string]interface{}")
	})
}

func TestLogout(t *testing.T) {
	r := setupTestRouter() // Router with session middleware
	r.POST("/logout", Logout)

	t.Run("No session cookies to clear", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Logged out successfully", resp.Message)
		assert.Empty(t, w.Header().Values("Set-Cookie"), "No Set-Cookie headers should be present if no cookies were found")
	})

	t.Run("Clear existing session cookies", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
		req.AddCookie(&http.Cookie{Name: fmt.Sprintf("%s0", sessioncookie.SessionPrefix), Value: "sessionpart1"})
		req.AddCookie(&http.Cookie{Name: fmt.Sprintf("%s1", sessioncookie.SessionPrefix), Value: "sessionpart2"})
		req.AddCookie(&http.Cookie{Name: "other_cookie", Value: "somevalue"})

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Logged out successfully", resp.Message)

		setCookies := w.Result().Cookies()
		clearedSessionCookies := 0
		for _, cookie := range setCookies {
			if strings.HasPrefix(cookie.Name, sessioncookie.SessionPrefix) {
				assert.Equal(t, "", cookie.Value, "Cookie value should be empty for cleared cookie")
				assert.Equal(t, -1, cookie.MaxAge, "Cookie MaxAge should be -1 to clear it")
				assert.True(t, cookie.HttpOnly, "Cookie should be HttpOnly")
				clearedSessionCookies++
			}
		}
		assert.Equal(t, 2, clearedSessionCookies, "Expected 2 session cookies to be cleared")
	})
}

func TestIsAllowedUser(t *testing.T) {
	originalAllowedDomain := allowedDomain
	originalAllowedOrg := allowedOrg
	defer func() {
		allowedDomain = originalAllowedDomain
		allowedOrg = originalAllowedOrg
	}()

	allowedDomain = "testdomain.com"
	allowedOrg = "testorg"

	tests := []struct {
		name      string
		provider  string
		email     string
		extraInfo map[string]any
		want      bool
	}{
		{"Google_Allowed", "google", "user@testdomain.com", nil, true},
		{"Google_Disallowed", "google", "user@other.com", nil, false},
		{"Azure_Allowed", "azure", "user@testdomain.com", nil, true},
		{"Azure_Disallowed", "azure", "user@other.com", nil, false},
		{"GitHub_Allowed", "github", "ghuser", map[string]any{"orgs": []string{"testorg", "another"}}, true},
		{"GitHub_Disallowed_WrongOrg", "github", "ghuser", map[string]any{"orgs": []string{"another"}}, false},
		{"GitHub_Disallowed_NoExtraInfoOrgs", "github", "ghuser", nil, false},
		{"GitHub_Disallowed_MalformedOrgs", "github", "ghuser", map[string]any{"orgs": "not-a-slice"}, false},
		{"UnknownProvider", "unknownprovider", "user@testdomain.com", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedUser(tt.provider, tt.email, tt.extraInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCommonPermissions(t *testing.T) {
	defaultLogger := getTestLogger()

	t.Run("Missing or invalid provider", func(t *testing.T) {
		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", map[string]interface{}{"user": "test"})
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: ""}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Missing or invalid provider", resp.Error)
	})

	t.Run("Permissions already cached in session", func(t *testing.T) {
		cachedApproverGroups := []models.Team{{ID: "ag1", Name: "Approver Group 1"}}
		cachedAdminGroups := []models.Team{{ID: "adg1", Name: "Admin Group 1"}}
		cachedPlatformApproverGroups := []models.Team{{ID: "pag1", Name: "Platform Approver Group 1"}}
		sessionMapWithCache := map[string]interface{}{
			"user":                   "testuser_cached",
			"isApprover":             true,
			"isAdmin":                false,
			"isPlatformApprover":     true,
			"approverGroups":         cachedApproverGroups,
			"adminGroups":            cachedAdminGroups,
			"platformApproverGroups": cachedPlatformApproverGroups,
		}

		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionMapWithCache) // This middleware sets it up
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: "github"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp CommonPermissionsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.IsApprover)
		assert.False(t, resp.IsAdmin)
		assert.True(t, resp.IsPlatformApprover)
		assert.Equal(t, cachedApproverGroups, resp.ApproverGroups)
		assert.Equal(t, cachedAdminGroups, resp.AdminGroups)
		assert.Equal(t, cachedPlatformApproverGroups, resp.PlatformApproverGroups)
	})

	t.Run("Unknown provider when not cached", func(t *testing.T) {
		sessionMapNoCache := map[string]interface{}{
			"user":  "testuser_unknown",
			"token": "fake-token",
		}
		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionMapNoCache)
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: "unknownprovider"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var respMsg models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Unknown provider", respMsg.Error)
	})

	// --- Tests for non-cached scenarios with mocked external calls ---

	t.Run("GitHub provider - success - fetch and match permissions", func(t *testing.T) {
		// Mock external dependencies
		originalGetGithubTeams := GetGithubTeams
		GetGithubTeams = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			assert.Equal(t, "fake-github-token", token)
			return []models.Team{
				{ID: "gh-approver-team-id", Name: "GitHub Approver Team"},
				{ID: "gh-admin-team-id", Name: "Configured Admin Team"},
				{ID: "gh-platform-team-id", Name: "Configured Platform Team"},
				{ID: "gh-other-team-id", Name: "GitHub Other Team"},
			}, nil
		}
		defer func() { GetGithubTeams = originalGetGithubTeams }()

		originalGetJitGroups := k8s.GetJitGroups
		k8s.GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
			assert.Equal(t, "test-cluster", clusterName)
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"groups": []interface{}{
							map[string]interface{}{"groupID": "gh-approver-team-id", "groupName": "Mapped JIT Approver Team"},
						},
					},
				},
			}, nil
		}
		defer func() { k8s.GetJitGroups = originalGetJitGroups }()

		originalPlatformApproverTeams := k8s.PlatformApproverTeams
		k8s.PlatformApproverTeams = []models.Team{{ID: "gh-platform-team-id", Name: "Configured Platform Team"}}
		defer func() { k8s.PlatformApproverTeams = originalPlatformApproverTeams }()

		originalAdminTeams := k8s.AdminTeams
		k8s.AdminTeams = []models.Team{{ID: "gh-admin-team-id", Name: "Configured Admin Team"}}
		defer func() { k8s.AdminTeams = originalAdminTeams }()

		originalClusterNames := k8s.ClusterNames
		k8s.ClusterNames = []string{"test-cluster"}
		defer func() { k8s.ClusterNames = originalClusterNames }()

		// Setup router and session
		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "testuser_github", "token": "fake-github-token"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData) // Set initial session data
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		// Perform request
		payload := CommonPermissionsRequest{Provider: "github"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)
		var resp CommonPermissionsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		assert.True(t, resp.IsApprover)
		assert.ElementsMatch(t, []models.Team{{ID: "gh-approver-team-id", Name: "Mapped JIT Approver Team"}}, resp.ApproverGroups)
		assert.True(t, resp.IsAdmin)
		assert.ElementsMatch(t, []models.Team{{ID: "gh-admin-team-id", Name: "Configured Admin Team"}}, resp.AdminGroups)
		assert.True(t, resp.IsPlatformApprover)
		assert.ElementsMatch(t, []models.Team{{ID: "gh-platform-team-id", Name: "Configured Platform Team"}}, resp.PlatformApproverGroups)

		// Verify session update (by checking the map that was supposed to be saved)
		// Note: sessionData map is modified by CommonPermissions
		assert.True(t, sessionData["isApprover"].(bool))
		assert.ElementsMatch(t, []models.Team{{ID: "gh-approver-team-id", Name: "Mapped JIT Approver Team"}}, sessionData["approverGroups"])
		assert.True(t, sessionData["isAdmin"].(bool))
		assert.ElementsMatch(t, []models.Team{{ID: "gh-admin-team-id", Name: "Configured Admin Team"}}, sessionData["adminGroups"])
		assert.True(t, sessionData["isPlatformApprover"].(bool))
		assert.ElementsMatch(t, []models.Team{{ID: "gh-platform-team-id", Name: "Configured Platform Team"}}, sessionData["platformApproverGroups"])
	})

	t.Run("GitHub provider - error fetching teams", func(t *testing.T) {
		originalGetGithubTeams := GetGithubTeams
		GetGithubTeams = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			return nil, errors.New("github API error")
		}
		defer func() { GetGithubTeams = originalGetGithubTeams }()

		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "testuser_github_error", "token": "fake-token"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: "github"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var respMsg models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Failed to fetch GitHub teams", respMsg.Error)
	})

	t.Run("Google provider - success - fetch and match permissions", func(t *testing.T) {
		originalGetGoogleGroups := GetGoogleGroupsWithWorkloadIdentity
		GetGoogleGroupsWithWorkloadIdentity = func(userEmail string, reqLogger *zap.Logger) ([]models.Team, error) {
			assert.Equal(t, "user@example.com", userEmail)
			return []models.Team{
				{ID: "google-approver-group-id", Name: "Configured Approver Group"},
				{ID: "google-admin-group-id", Name: "Configured Google Admin Group"},
			}, nil
		}
		defer func() { GetGoogleGroupsWithWorkloadIdentity = originalGetGoogleGroups }()

		originalGetJitGroups := k8s.GetJitGroups
		k8s.GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"groups": []interface{}{
							map[string]interface{}{"groupID": "google-approver-group-id", "groupName": "Mapped JIT Google Approver"},
						},
					},
				},
			}, nil
		}
		defer func() { k8s.GetJitGroups = originalGetJitGroups }()

		originalAdminTeams := k8s.AdminTeams
		k8s.AdminTeams = []models.Team{{ID: "google-admin-group-id", Name: "Configured Google Admin Group"}}
		defer func() { k8s.AdminTeams = originalAdminTeams }()
		originalPlatformApproverTeams := k8s.PlatformApproverTeams
		k8s.PlatformApproverTeams = []models.Team{} // No platform approver for this test
		defer func() { k8s.PlatformApproverTeams = originalPlatformApproverTeams }()
		originalClusterNames := k8s.ClusterNames
		k8s.ClusterNames = []string{"test-cluster-google"}
		defer func() { k8s.ClusterNames = originalClusterNames }()

		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "g_user", "token": "fake-g-token", "email": "user@example.com"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payloadReq := CommonPermissionsRequest{Provider: "google"}
		jsonBody, _ := json.Marshal(payloadReq)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp CommonPermissionsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.IsApprover)
		assert.ElementsMatch(t, []models.Team{{ID: "google-approver-group-id", Name: "Mapped JIT Google Approver"}}, resp.ApproverGroups)
		assert.True(t, resp.IsAdmin)
		assert.ElementsMatch(t, []models.Team{{ID: "google-admin-group-id", Name: "Configured Google Admin Group"}}, resp.AdminGroups)
		assert.False(t, resp.IsPlatformApprover)
		assert.Empty(t, resp.PlatformApproverGroups)
	})

	t.Run("Azure provider - success - fetch and match permissions", func(t *testing.T) {
		// Mock external dependencies
		originalGetAzureGroups := GetAzureGroups
		GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			assert.Equal(t, "fake-azure-token", token)
			return []models.Team{
				{ID: "azure-approver-group-id", Name: "Azure Approver Group (from Azure)"},
				{ID: "azure-admin-group-id", Name: "Azure Admin Group (from Azure)"},
			}, nil
		}
		defer func() { GetAzureGroups = originalGetAzureGroups }()

		originalGetJitGroups := k8s.GetJitGroups
		k8s.GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
			assert.Equal(t, "test-cluster-azure", clusterName)
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"groups": []interface{}{
							map[string]interface{}{"groupID": "azure-approver-group-id", "groupName": "Mapped JIT Azure Approver"},
						},
					},
				},
			}, nil
		}
		defer func() { k8s.GetJitGroups = originalGetJitGroups }()

		originalAdminTeams := k8s.AdminTeams
		k8s.AdminTeams = []models.Team{{ID: "azure-admin-group-id", Name: "Azure Admin Group (from Azure)"}}
		defer func() { k8s.AdminTeams = originalAdminTeams }()

		originalPlatformApproverTeams := k8s.PlatformApproverTeams
		k8s.PlatformApproverTeams = []models.Team{} // No platform approver role for Azure in this test
		defer func() { k8s.PlatformApproverTeams = originalPlatformApproverTeams }()

		originalClusterNames := k8s.ClusterNames
		k8s.ClusterNames = []string{"test-cluster-azure"}
		defer func() { k8s.ClusterNames = originalClusterNames }()

		// Setup router and session
		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "azure_user", "token": "fake-azure-token"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData) // Set initial session data
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		// Perform request
		payloadReq := CommonPermissionsRequest{Provider: "azure"}
		jsonBody, _ := json.Marshal(payloadReq)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)
		var resp CommonPermissionsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		assert.True(t, resp.IsApprover)
		assert.ElementsMatch(t, []models.Team{{ID: "azure-approver-group-id", Name: "Mapped JIT Azure Approver"}}, resp.ApproverGroups)
		assert.True(t, resp.IsAdmin)
		// Assuming MatchUserGroups returns the configured name for admin/platform groups
		assert.ElementsMatch(t, []models.Team{{ID: "azure-admin-group-id", Name: "Azure Admin Group (from Azure)"}}, resp.AdminGroups)
		assert.False(t, resp.IsPlatformApprover)
		assert.Empty(t, resp.PlatformApproverGroups)

		// Verify session update (by checking the map that was supposed to be saved)
		assert.True(t, sessionData["isApprover"].(bool))
		assert.ElementsMatch(t, []models.Team{{ID: "azure-approver-group-id", Name: "Mapped JIT Azure Approver"}}, sessionData["approverGroups"])
		assert.True(t, sessionData["isAdmin"].(bool))
		assert.ElementsMatch(t, []models.Team{{ID: "azure-admin-group-id", Name: "Azure Admin Group (from Azure)"}}, sessionData["adminGroups"])
		assert.False(t, sessionData["isPlatformApprover"].(bool))
		assert.Empty(t, sessionData["platformApproverGroups"])
	})

	t.Run("Azure provider - error fetching groups", func(t *testing.T) {
		originalGetAzureGroups := GetAzureGroups
		GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			return nil, errors.New("azure API error")
		}
		defer func() { GetAzureGroups = originalGetAzureGroups }()

		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "testuser_azure_error", "token": "fake-azure-token"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: "azure"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var respMsg models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Failed to fetch Azure groups", respMsg.Error)
	})

	t.Run("Error fetching JitGroups", func(t *testing.T) {
		originalGetGithubTeams := GetGithubTeams // Use GitHub as an example provider
		GetGithubTeams = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			return []models.Team{{ID: "some-team", Name: "Some Team"}}, nil
		}
		defer func() { GetGithubTeams = originalGetGithubTeams }()

		originalGetJitGroups := k8s.GetJitGroups
		k8s.GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
			return nil, errors.New("k8s error fetching JitGroups")
		}
		defer func() { k8s.GetJitGroups = originalGetJitGroups }()

		originalClusterNames := k8s.ClusterNames
		k8s.ClusterNames = []string{"error-cluster"}
		defer func() { k8s.ClusterNames = originalClusterNames }()
		// Ensure other k8s teams are empty or don't match to isolate JitGroup impact
		originalPlatformApproverTeams := k8s.PlatformApproverTeams
		k8s.PlatformApproverTeams = []models.Team{}
		defer func() { k8s.PlatformApproverTeams = originalPlatformApproverTeams }()
		originalAdminTeams := k8s.AdminTeams
		k8s.AdminTeams = []models.Team{}
		defer func() { k8s.AdminTeams = originalAdminTeams }()

		r := setupTestRouter()
		sessionData := map[string]interface{}{"user": "testuser_jit_error", "token": "fake-token"}
		r.Use(func(c *gin.Context) {
			c.Set("logger", defaultLogger)
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.POST("/permissions", CommonPermissions)

		payload := CommonPermissionsRequest{Provider: "github"}
		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// The handler logs the error but continues, potentially with empty approver groups.
		// The overall request should still succeed.
		assert.Equal(t, http.StatusOK, w.Code)
		var resp CommonPermissionsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.IsApprover, "IsApprover should be false as JitGroups fetch failed")
		assert.Empty(t, resp.ApproverGroups, "ApproverGroups should be empty")
		// Admin and PlatformApprover status depends on k8s.AdminTeams/PlatformApproverTeams and userGroups
		assert.False(t, resp.IsAdmin)
		assert.Empty(t, resp.AdminGroups)
		assert.False(t, resp.IsPlatformApprover)
		assert.Empty(t, resp.PlatformApproverGroups)
	})
}
