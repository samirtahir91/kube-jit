package handlers

import (
	"encoding/json"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// TestHandleGitHubLogin tests the GitHub login handler
func TestHandleGitHubLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// --- Original values storage and deferred restoration ---
	// Package variables
	originalOauthProvider := oauthProvider
	originalClientID := clientID
	originalClientSecret := clientSecret
	originalRedirectURI := redirectUri // Though GitHub handler doesn't use this one directly from var
	originalAllowedOrg := allowedOrg
	originalAllowedDomain := allowedDomain // To restore Azure's setting

	// Environment variables
	originalOauthProviderEnv := os.Getenv("OAUTH_PROVIDER")
	originalOauthClientIDEnv := os.Getenv("OAUTH_CLIENT_ID")
	originalOauthClientSecretEnv := os.Getenv("OAUTH_CLIENT_SECRET")
	originalOauthRedirectURIEnv := os.Getenv("OAUTH_REDIRECT_URI")
	originalAllowedGithubOrgEnv := os.Getenv("ALLOWED_GITHUB_ORG")
	originalAllowedDomainEnv := os.Getenv("ALLOWED_DOMAIN")

	// Mocked functions' original states
	defer func() {
		// Restore package variables
		oauthProvider = originalOauthProvider
		clientID = originalClientID
		clientSecret = originalClientSecret
		redirectUri = originalRedirectURI
		allowedOrg = originalAllowedOrg
		allowedDomain = originalAllowedDomain

		// Restore environment variables
		os.Setenv("OAUTH_PROVIDER", originalOauthProviderEnv)
		os.Setenv("OAUTH_CLIENT_ID", originalOauthClientIDEnv)
		os.Setenv("OAUTH_CLIENT_SECRET", originalOauthClientSecretEnv)
		os.Setenv("OAUTH_REDIRECT_URI", originalOauthRedirectURIEnv)
		os.Setenv("ALLOWED_GITHUB_ORG", originalAllowedGithubOrgEnv)
		os.Setenv("ALLOWED_DOMAIN", originalAllowedDomainEnv)

		httpmock.DeactivateAndReset()
	}()

	// --- Setup for GitHub tests ---
	oauthProvider = "github"
	clientID = "test-github-client-id"
	clientSecret = "test-github-client-secret"
	// redirectUri is not directly used by HandleGitHubLogin's oauth flow in the same way as Azure's oauth2.Config
	allowedOrg = "test-valid-org"
	allowedDomain = "" // Clear Azure's setting for this test scope

	os.Setenv("OAUTH_PROVIDER", "github")
	os.Setenv("OAUTH_CLIENT_ID", "test-github-client-id")
	os.Setenv("OAUTH_CLIENT_SECRET", "test-github-client-secret")
	os.Setenv("ALLOWED_GITHUB_ORG", "test-valid-org") // For common.go init() and isAllowedUser
	os.Setenv("ALLOWED_DOMAIN", "")                   // Ensure Azure's domain doesn't interfere

	// Activate httpmock
	httpmock.Activate()
	// Use httpClient from common.go for httpmock
	httpmock.ActivateNonDefault(httpClient)

	// --- Test Cases ---
	t.Run("success", func(t *testing.T) {
		// 1. Mock GitHub token exchange
		httpmock.RegisterResponder("POST", "https://github.com/login/oauth/access_token",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				params, _ := url.ParseQuery(string(bodyBytes))
				assert.Equal(t, clientID, params.Get("client_id"))
				assert.Equal(t, clientSecret, params.Get("client_secret"))
				assert.Equal(t, "test_code", params.Get("code"))

				respBody := models.GitHubTokenResponse{
					AccessToken: "gh_mock_access_token",
					TokenType:   "Bearer",
					Scope:       "user,repo",
					ExpiresIn:   3600, // Example value
				}
				return httpmock.NewJsonResponse(200, respBody)
			},
		)

		// 2. Mock GitHub user profile fetch
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "Bearer gh_mock_access_token", req.Header.Get("Authorization"))
				userProfile := models.GitHubUser{
					ID:        12345,
					Login:     "testuser",
					Email:     "testuser@example.com", // Assume email is in primary profile
					AvatarURL: "https://avatar.example.com/testuser",
				}
				return httpmock.NewJsonResponse(200, userProfile)
			},
		)

		// 3. Mock GitHub user orgs fetch (for isAllowedUser)
		httpmock.RegisterResponder("GET", "https://api.github.com/user/orgs",
			func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "Bearer gh_mock_access_token", req.Header.Get("Authorization"))
				orgs := []struct {
					Login string `json:"login"`
				}{
					{Login: "test-valid-org"},
					{Login: "another-org"},
				}
				return httpmock.NewJsonResponse(200, orgs)
			},
		)

		// Setup router and make request
		router := setupTestRouter() // Assumes setupTestRouter is available and includes session middleware
		router.GET("/oauth/github/callback", HandleGitHubLogin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/github/callback?code=test_code", nil)
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)
		var respData models.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)
		assert.Equal(t, "12345", respData.UserData.ID)
		assert.Equal(t, "testuser", respData.UserData.Name)
		assert.Equal(t, "testuser@example.com", respData.UserData.Email)
		assert.Equal(t, "github", respData.UserData.Provider)
		assert.Equal(t, 3600, respData.ExpiresIn)

		cookies := w.Result().Cookies()
		sessionCookieFound := false
		for _, cookie := range cookies {
			if strings.HasPrefix(cookie.Name, sessioncookie.SessionPrefix) {
				sessionCookieFound = true
				break
			}
		}
		assert.True(t, sessionCookieFound, "Session cookie should be set")
		httpmock.Reset() // Reset mocks for the next sub-test if any, or rely on outer defer
	})

	t.Run("missing code query parameter", func(t *testing.T) {
		router := setupTestRouter()
		router.GET("/oauth/github/callback", HandleGitHubLogin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/github/callback", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var respData models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)
		assert.Equal(t, "Code query parameter is required", respData.Error)
		httpmock.Reset()
	})

	t.Run("token exchange fails with GitHub", func(t *testing.T) {
		httpmock.RegisterResponder("POST", "https://github.com/login/oauth/access_token",
			httpmock.NewStringResponder(500, "GitHub internal server error"),
		)

		router := setupTestRouter()
		router.GET("/oauth/github/callback", HandleGitHubLogin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/github/callback?code=bad_code", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code) // HandleGitHubLogin returns 400 for this
		var respData models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)
		assert.Equal(t, "Error fetching access token from GitHub", respData.Error)
		httpmock.Reset()
	})

	t.Run("fetch user profile fails", func(t *testing.T) {
		// 1. Mock GitHub token exchange (success)
		httpmock.RegisterResponder("POST", "https://github.com/login/oauth/access_token",
			func(req *http.Request) (*http.Response, error) {
				respBody := models.GitHubTokenResponse{AccessToken: "gh_mock_access_token", TokenType: "Bearer"}
				return httpmock.NewJsonResponse(200, respBody)
			},
		)
		// 2. Mock GitHub user profile fetch (failure)
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			httpmock.NewStringResponder(500, "GitHub API error for user"),
		)

		router := setupTestRouter()
		router.GET("/oauth/github/callback", HandleGitHubLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/github/callback?code=test_code", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var respData models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)
		assert.Contains(t, respData.Error, "error fetching user data from GitHub")
		httpmock.Reset()
	})

	t.Run("unauthorized organization", func(t *testing.T) {
		// Setup: change allowedOrg for this sub-test, then restore
		originalTestAllowedOrg := allowedOrg
		allowedOrg = "strictly-this-org-only"
		os.Setenv("ALLOWED_GITHUB_ORG", "strictly-this-org-only")
		defer func() {
			allowedOrg = originalTestAllowedOrg
			os.Setenv("ALLOWED_GITHUB_ORG", originalTestAllowedOrg)
		}()

		// 1. Mock GitHub token exchange
		httpmock.RegisterResponder("POST", "https://github.com/login/oauth/access_token",
			func(req *http.Request) (*http.Response, error) {
				respBody := models.GitHubTokenResponse{AccessToken: "gh_mock_access_token", TokenType: "Bearer"}
				return httpmock.NewJsonResponse(200, respBody)
			},
		)
		// 2. Mock GitHub user profile fetch
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			func(req *http.Request) (*http.Response, error) {
				userProfile := models.GitHubUser{ID: 12345, Login: "testuser", Email: "testuser@example.com"}
				return httpmock.NewJsonResponse(200, userProfile)
			},
		)
		// 3. Mock GitHub user orgs fetch (user is NOT in the strictly-this-org-only)
		httpmock.RegisterResponder("GET", "https://api.github.com/user/orgs",
			func(req *http.Request) (*http.Response, error) {
				orgs := []struct {
					Login string `json:"login"`
				}{{Login: "another-org"}}
				return httpmock.NewJsonResponse(200, orgs)
			},
		)

		router := setupTestRouter()
		router.GET("/oauth/github/callback", HandleGitHubLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/oauth/github/callback?code=test_code", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		var respData models.SimpleMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &respData)
		assert.NoError(t, err)
		assert.Equal(t, "Unauthorized org", respData.Error)
		httpmock.Reset()
	})

	// Add more test cases:
	// - Email not in primary profile, successfully fetched from /user/emails
	// - Email fetch from /user/emails fails
	// - No verified email found
	// - Fetch orgs fails
}
