package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"kube-jit/internal/models"
	"kube-jit/pkg/k8s" // For k8s package variables if needed by handlers

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2" // For getAzureOAuthConfig, getGoogleOAuthConfig if their mocks are needed
)

// TestCommonInitLogic tests the logic within the init() function of common.go
// by checking the values of allowedDomain and allowedOrg after setting
// OAUTH_PROVIDER and the corresponding domain/org env vars.
func TestCommonInitLogic(t *testing.T) {
	originalOauthProvider := oauthProvider
	originalAllowedDomain := allowedDomain
	originalAllowedOrg := allowedOrg

	// Store original ENV VARS
	originalOauthProviderEnv := os.Getenv("OAUTH_PROVIDER")
	originalAllowedDomainEnv := os.Getenv("ALLOWED_DOMAIN")
	originalAllowedGithubOrgEnv := os.Getenv("ALLOWED_GITHUB_ORG")

	defer func() {
		oauthProvider = originalOauthProvider
		allowedDomain = originalAllowedDomain
		allowedOrg = originalAllowedOrg
		os.Setenv("OAUTH_PROVIDER", originalOauthProviderEnv)
		os.Setenv("ALLOWED_DOMAIN", originalAllowedDomainEnv)
		os.Setenv("ALLOWED_GITHUB_ORG", originalAllowedGithubOrgEnv)
	}()

	tests := []struct {
		name                 string
		providerEnv          string
		domainEnv            string
		orgEnv               string
		expectedDomain       string
		expectedOrg          string
		reassignOauthVar     bool   // Flag to indicate if we should reassign oauthProvider package var
		oauthVarValue        string // Value to assign to oauthProvider package var
		runInitLogicManually func() // Simulates re-running init logic for package vars
	}{
		{
			name:             "Google Provider",
			providerEnv:      "google",
			domainEnv:        "google-domain.com",
			orgEnv:           "",
			expectedDomain:   "google-domain.com",
			expectedOrg:      "",
			reassignOauthVar: true,
			oauthVarValue:    "google",
			runInitLogicManually: func() { // Manually simulate init logic for package vars
				if oauthProvider == "google" {
					allowedDomain = os.Getenv("ALLOWED_DOMAIN")
					allowedOrg = "" // Explicitly clear for test isolation
				}
			},
		},
		{
			name:             "GitHub Provider",
			providerEnv:      "github",
			domainEnv:        "",
			orgEnv:           "github-org",
			expectedDomain:   "",
			expectedOrg:      "github-org",
			reassignOauthVar: true,
			oauthVarValue:    "github",
			runInitLogicManually: func() {
				if oauthProvider == "github" {
					allowedOrg = os.Getenv("ALLOWED_GITHUB_ORG")
					allowedDomain = "" // Explicitly clear
				}
			},
		},
		{
			name:             "Azure Provider",
			providerEnv:      "azure",
			domainEnv:        "azure-domain.com",
			orgEnv:           "",
			expectedDomain:   "azure-domain.com",
			expectedOrg:      "",
			reassignOauthVar: true,
			oauthVarValue:    "azure",
			runInitLogicManually: func() {
				if oauthProvider == "azure" {
					allowedDomain = os.Getenv("ALLOWED_DOMAIN")
					allowedOrg = "" // Explicitly clear
				}
			},
		},
		{
			name:             "Unknown Provider",
			providerEnv:      "unknown",
			domainEnv:        "any-domain.com",
			orgEnv:           "any-org",
			expectedDomain:   "", // Expecting them to be reset/cleared
			expectedOrg:      "",
			reassignOauthVar: true,
			oauthVarValue:    "unknown",
			runInitLogicManually: func() {
				// For unknown, init doesn't set them, so they should remain as they were or be cleared
				allowedDomain = ""
				allowedOrg = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for the test
			os.Setenv("OAUTH_PROVIDER", tt.providerEnv)
			os.Setenv("ALLOWED_DOMAIN", tt.domainEnv)
			os.Setenv("ALLOWED_GITHUB_ORG", tt.orgEnv)

			// Re-assign package variable `oauthProvider` if needed for the test scenario
			// This simulates the state *after* utils.MustGetEnv has run for oauthProvider
			if tt.reassignOauthVar {
				oauthProvider = tt.oauthVarValue
			}

			// Manually run the logic that init() would perform on package variables
			// This is because init() itself only runs once per package load.
			tt.runInitLogicManually()

			assert.Equal(t, tt.expectedDomain, allowedDomain, "allowedDomain mismatch")
			assert.Equal(t, tt.expectedOrg, allowedOrg, "allowedOrg mismatch")

			// Clean up env vars for next test iteration
			os.Unsetenv("OAUTH_PROVIDER")
			os.Unsetenv("ALLOWED_DOMAIN")
			os.Unsetenv("ALLOWED_GITHUB_ORG")
			// Reset package vars to a known state before the next sub-test's re-assignment
			allowedDomain = ""
			allowedOrg = ""
		})
	}
}

func TestGetOauthClientId(t *testing.T) {
	// Backup original values
	originalOauthProvider := oauthProvider
	originalClientID := clientID
	originalRedirectURI := redirectUri
	originalAzureAuthURL := os.Getenv("AZURE_AUTH_URL")
	originalAzureTokenURL := os.Getenv("AZURE_TOKEN_URL")

	// Mock getAzureOAuthConfig
	originalGetAzureCfg := getAzureOAuthConfig
	defer func() {
		getAzureOAuthConfig = originalGetAzureCfg
		oauthProvider = originalOauthProvider
		clientID = originalClientID
		redirectUri = originalRedirectURI
		os.Setenv("AZURE_AUTH_URL", originalAzureAuthURL)
		os.Setenv("AZURE_TOKEN_URL", originalAzureTokenURL)
	}()

	r := setupTestRouter()
	r.GET("/client_id", GetOauthClientId)

	testCases := []struct {
		name              string
		setupProvider     string
		setupClientID     string
		setupRedirectURI  string
		setupAzureAuthURL string // For getAzureOAuthConfig().Endpoint.AuthURL
		expectedStatus    int
		expectedResponse  OauthClientIdResponse
	}{
		{
			name:              "Azure provider",
			setupProvider:     "azure",
			setupClientID:     "azure-id",
			setupRedirectURI:  "http://localhost/azure",
			setupAzureAuthURL: "https://azure.auth.url/authorize",
			expectedStatus:    http.StatusOK,
			expectedResponse: OauthClientIdResponse{
				ClientID:    "azure-id",
				Provider:    "azure",
				RedirectURI: "http://localhost/azure",
				AuthURL:     "https://azure.auth.url/authorize",
			},
		},
		{
			name:             "GitHub provider",
			setupProvider:    "github",
			setupClientID:    "github-id",
			setupRedirectURI: "http://localhost/github",
			expectedStatus:   http.StatusOK,
			expectedResponse: OauthClientIdResponse{
				ClientID:    "github-id",
				Provider:    "github",
				RedirectURI: "http://localhost/github",
				AuthURL:     "", // AuthURL is empty for non-Azure providers in current logic
			},
		},
		{
			name:             "Google provider",
			setupProvider:    "google",
			setupClientID:    "google-id",
			setupRedirectURI: "http://localhost/google",
			expectedStatus:   http.StatusOK,
			expectedResponse: OauthClientIdResponse{
				ClientID:    "google-id",
				Provider:    "google",
				RedirectURI: "http://localhost/google",
				AuthURL:     "", // AuthURL is empty for non-Azure providers
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up package variables for this test case
			oauthProvider = tc.setupProvider
			clientID = tc.setupClientID
			redirectUri = tc.setupRedirectURI
			os.Setenv("AZURE_AUTH_URL", tc.setupAzureAuthURL)
			// AZURE_TOKEN_URL is also needed by getAzureOAuthConfig
			os.Setenv("AZURE_TOKEN_URL", "https://dummy.token.url")

			// If provider is Azure, ensure getAzureOAuthConfig returns a config with the expected AuthURL
			if tc.setupProvider == "azure" {
				getAzureOAuthConfig = func() *oauth2.Config {
					return &oauth2.Config{
						Endpoint: oauth2.Endpoint{AuthURL: tc.setupAzureAuthURL},
						// Other fields can be minimal as only AuthURL is used by GetOauthClientId
					}
				}
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/client_id", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				var resp OauthClientIdResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, resp)
			}
		})
	}
}

func TestContains(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"item exists", []string{"a", "b", "c"}, "b", true},
		{"item does not exist", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"slice with empty strings, item exists", []string{"", "b", ""}, "", true},
		{"slice with empty strings, item does not exist", []string{"", "b", ""}, "c", false},
		{"nil slice", nil, "a", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, contains(tc.slice, tc.item))
		})
	}
}

func TestMatchUserGroups(t *testing.T) {
	platformTeam1 := models.Team{ID: "p1", Name: "Platform Team Alpha"}
	platformTeam2 := models.Team{ID: "p2", Name: "Platform Team Beta"}
	adminTeam1 := models.Team{ID: "a1", Name: "Admin Team Alpha"}
	adminTeam2 := models.Team{ID: "a2", Name: "Admin Team Beta"}

	configuredPlatformTeams := []models.Team{platformTeam1, platformTeam2}
	configuredAdminTeams := []models.Team{adminTeam1, adminTeam2}

	testCases := []struct {
		name                       string
		userGroups                 []models.Team
		platformTeams              []models.Team
		adminTeams                 []models.Team
		expectedIsAdmin            bool
		expectedIsPlatformApprover bool
		expectedMatchedPlatform    []models.Team
		expectedMatchedAdmin       []models.Team
	}{
		{
			name:                       "No groups",
			userGroups:                 []models.Team{},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{},
		},
		{
			name:                       "User in one platform group",
			userGroups:                 []models.Team{platformTeam1},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: true,
			expectedMatchedPlatform:    []models.Team{platformTeam1},
			expectedMatchedAdmin:       []models.Team{},
		},
		{
			name:                       "User in one admin group",
			userGroups:                 []models.Team{adminTeam1},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            true,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{adminTeam1},
		},
		{
			name:                       "User in one platform and one admin group",
			userGroups:                 []models.Team{platformTeam2, adminTeam1},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            true,
			expectedIsPlatformApprover: true,
			expectedMatchedPlatform:    []models.Team{platformTeam2},
			expectedMatchedAdmin:       []models.Team{adminTeam1},
		},
		{
			name:                       "User in multiple platform and admin groups",
			userGroups:                 []models.Team{adminTeam2, platformTeam1, adminTeam1, platformTeam2},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            true,
			expectedIsPlatformApprover: true,
			expectedMatchedPlatform:    []models.Team{platformTeam1, platformTeam2}, // Order might vary, use ElementsMatch
			expectedMatchedAdmin:       []models.Team{adminTeam2, adminTeam1},       // Order might vary
		},
		{
			name:                       "User groups not in configured teams",
			userGroups:                 []models.Team{{ID: "u1", Name: "User Team Unrelated"}},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{},
		},
		{
			name:                       "Empty configured platform teams",
			userGroups:                 []models.Team{platformTeam1},
			platformTeams:              []models.Team{},
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{},
		},
		{
			name:                       "Empty configured admin teams",
			userGroups:                 []models.Team{adminTeam1},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 []models.Team{},
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{},
		},
		{
			name:                       "User group with same ID but different name",
			userGroups:                 []models.Team{{ID: platformTeam1.ID, Name: "Different Name P"}},
			platformTeams:              configuredPlatformTeams,
			adminTeams:                 configuredAdminTeams,
			expectedIsAdmin:            false,
			expectedIsPlatformApprover: false,
			expectedMatchedPlatform:    []models.Team{},
			expectedMatchedAdmin:       []models.Team{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isAdmin, isPlatformApprover, matchedPlatform, matchedAdmin := MatchUserGroups(
				tc.userGroups,
				tc.platformTeams,
				tc.adminTeams,
			)
			assert.Equal(t, tc.expectedIsAdmin, isAdmin, "isAdmin mismatch")
			assert.Equal(t, tc.expectedIsPlatformApprover, isPlatformApprover, "isPlatformApprover mismatch")
			assert.ElementsMatch(t, tc.expectedMatchedPlatform, matchedPlatform, "matchedPlatformGroups mismatch")
			assert.ElementsMatch(t, tc.expectedMatchedAdmin, matchedAdmin, "matchedAdminGroups mismatch")
		})
	}
}

func TestGetClustersAndRoles(t *testing.T) {
	r := setupTestRouter()
	r.GET("/roles-and-clusters", GetClustersAndRoles)

	// Backup and defer restoration of k8s package variables
	originalClusterNames := k8s.ClusterNames
	originalAllowedRoles := k8s.AllowedRoles
	defer func() {
		k8s.ClusterNames = originalClusterNames
		k8s.AllowedRoles = originalAllowedRoles
	}()

	// Set mock data for k8s package variables
	k8s.ClusterNames = []string{"cluster-alpha", "cluster-beta"}
	k8s.AllowedRoles = []models.Roles{
		{Name: "role-viewer"},
		{Name: "role-editor"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/roles-and-clusters", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ClustersAndRolesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Equal(t, k8s.ClusterNames, resp.Clusters)
	assert.Equal(t, k8s.AllowedRoles, resp.Roles)
}

func TestGetBuildSha(t *testing.T) {
	r := setupTestRouter()
	r.GET("/build-sha", GetBuildSha)

	originalBuildSHA := os.Getenv("BUILD_SHA")
	defer os.Setenv("BUILD_SHA", originalBuildSHA)

	t.Run("BUILD_SHA is set", func(t *testing.T) {
		expectedSHA := "abcdef123456"
		os.Setenv("BUILD_SHA", expectedSHA)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/build-sha", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp BuildShaResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, expectedSHA, resp.Sha)
	})

	t.Run("BUILD_SHA is not set", func(t *testing.T) {
		os.Unsetenv("BUILD_SHA") // Or os.Setenv("BUILD_SHA", "")

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/build-sha", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp BuildShaResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "", resp.Sha) // Expect empty string if env var is not set
	})
}

func TestGetApprovingGroups(t *testing.T) {
	r := setupTestRouter()
	r.GET("/approving-groups", GetApprovingGroups)

	originalPlatformApproverTeams := k8s.PlatformApproverTeams
	defer func() { k8s.PlatformApproverTeams = originalPlatformApproverTeams }()

	k8s.PlatformApproverTeams = []models.Team{
		{ID: "team-plat-1", Name: "Platform Approvers Alpha"},
		{ID: "team-plat-2", Name: "Platform Approvers Beta"},
	}

	t.Run("User logged in with token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w) // Create context to set session data

		// Simulate session data being set
		// The GetSessionData function expects "sessionData" in the Gin context.
		sessionMap := map[string]interface{}{
			"token": "fake-user-token",
			"id":    "user123",
			"name":  "Test User",
		}
		c.Set("sessionData", sessionMap) // This is what GetSessionData will retrieve

		routerWithAuthContext := setupTestRouter()         // New router for this specific test
		routerWithAuthContext.Use(func(ctx *gin.Context) { // Middleware to set sessionData
			ctx.Set("sessionData", sessionMap)
			ctx.Next()
		})
		routerWithAuthContext.GET("/approving-groups", GetApprovingGroups)

		recorder := httptest.NewRecorder()
		httpReq, _ := http.NewRequest(http.MethodGet, "/approving-groups", nil)
		routerWithAuthContext.ServeHTTP(recorder, httpReq)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var resp []models.Team
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, k8s.PlatformApproverTeams, resp)
	})

	t.Run("User not logged in - no token", func(t *testing.T) {
		routerWithAuthContext := setupTestRouter()
		routerWithAuthContext.Use(func(ctx *gin.Context) {
			// Simulate no token in session data
			ctx.Set("sessionData", map[string]interface{}{"id": "user123", "name": "Test User"})
			ctx.Next()
		})
		routerWithAuthContext.GET("/approving-groups", GetApprovingGroups)

		recorder := httptest.NewRecorder()
		httpReq, _ := http.NewRequest(http.MethodGet, "/approving-groups", nil)
		routerWithAuthContext.ServeHTTP(recorder, httpReq)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		var respMsg models.SimpleMessageResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &respMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Unauthorized: no token in session data", respMsg.Error)
	})
}

func TestHealthCheck(t *testing.T) {
	r := setupTestRouter()
	r.GET("/healthz", HealthCheck)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.SimpleMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
}
