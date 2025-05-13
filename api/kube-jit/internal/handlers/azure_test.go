package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// --- Test Setup/Teardown for Azure specific tests ---
func setupAzureTestEnv() (restoreFunc func()) {
	// Store original package variables
	originalOauthProvider := oauthProvider
	originalClientID := clientID
	originalClientSecret := clientSecret
	originalRedirectURI := redirectUri
	originalAllowedDomain := allowedDomain
	originalAllowedOrg := allowedOrg // In case other tests (like GitHub) set it

	// Store original environment variables
	originalOauthProviderEnv := os.Getenv("OAUTH_PROVIDER")
	originalOauthClientIDEnv := os.Getenv("OAUTH_CLIENT_ID")
	originalOauthClientSecretEnv := os.Getenv("OAUTH_CLIENT_SECRET")
	originalOauthRedirectURIEnv := os.Getenv("OAUTH_REDIRECT_URI")
	originalAllowedDomainEnv := os.Getenv("ALLOWED_DOMAIN")
	originalAllowedGithubOrgEnv := os.Getenv("ALLOWED_GITHUB_ORG") // In case GitHub tests set it
	originalAzureAuthURLEnv := os.Getenv("AZURE_AUTH_URL")
	originalAzureTokenURLEnv := os.Getenv("AZURE_TOKEN_URL")

	// --- Setup for Azure tests ---
	oauthProvider = "azure"
	clientID = "test-azure-client-id"
	clientSecret = "test-azure-client-secret"
	redirectUri = "http://localhost/test/azure/callback"
	allowedDomain = "example.com"
	allowedOrg = "" // Clear GitHub's setting if any

	os.Setenv("OAUTH_PROVIDER", "azure")
	os.Setenv("OAUTH_CLIENT_ID", "test-azure-client-id")
	os.Setenv("OAUTH_CLIENT_SECRET", "test-azure-client-secret")
	os.Setenv("OAUTH_REDIRECT_URI", "http://localhost/test/azure/callback")
	os.Setenv("ALLOWED_DOMAIN", "example.com")
	os.Setenv("ALLOWED_GITHUB_ORG", "") // Ensure GitHub specific env is cleared
	os.Setenv("AZURE_AUTH_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/authorize")
	os.Setenv("AZURE_TOKEN_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/token") // Default, can be overridden by mock servers

	return func() {
		// Restore package variables
		oauthProvider = originalOauthProvider
		clientID = originalClientID
		clientSecret = originalClientSecret
		redirectUri = originalRedirectURI
		allowedDomain = originalAllowedDomain
		allowedOrg = originalAllowedOrg

		// Restore environment variables
		os.Setenv("OAUTH_PROVIDER", originalOauthProviderEnv)
		os.Setenv("OAUTH_CLIENT_ID", originalOauthClientIDEnv)
		os.Setenv("OAUTH_CLIENT_SECRET", originalOauthClientSecretEnv)
		os.Setenv("OAUTH_REDIRECT_URI", originalOauthRedirectURIEnv)
		os.Setenv("ALLOWED_DOMAIN", originalAllowedDomainEnv)
		os.Setenv("ALLOWED_GITHUB_ORG", originalAllowedGithubOrgEnv)
		os.Setenv("AZURE_AUTH_URL", originalAzureAuthURLEnv)
		os.Setenv("AZURE_TOKEN_URL", originalAzureTokenURLEnv)
	}
}

// --- Mock for isAllowedUser ---
var originalIsAllowedUser func(provider, email string, extraInfo map[string]any) bool

func mockIsAllowedUser(allowed bool) {
	originalIsAllowedUser = isAllowedUser
	isAllowedUser = func(provider, email string, extraInfo map[string]any) bool {
		return allowed
	}
}

func restoreIsAllowedUser() {
	if originalIsAllowedUser != nil {
		isAllowedUser = originalIsAllowedUser
	}
}

// --- Mock for fetchAzureUserProfile ---
var originalFetchAzureUserProfile func(token string) (*models.AzureUser, error)

func mockFetchAzureUserProfile(user *models.AzureUser, err error) {
	originalFetchAzureUserProfile = fetchAzureUserProfile
	fetchAzureUserProfile = func(token string) (*models.AzureUser, error) {
		return user, err
	}
}

func restoreFetchAzureUserProfile() {
	if originalFetchAzureUserProfile != nil {
		fetchAzureUserProfile = originalFetchAzureUserProfile
	}
}

func TestHandleAzureLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreEnv := setupAzureTestEnv()
	defer restoreEnv()

	t.Run("success", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			assert.Equal(t, "auth_code", r.Form.Get("code"))
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "mocked_access_token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		}))
		defer mockTokenServer.Close()

		os.Setenv("AZURE_TOKEN_URL", mockTokenServer.URL)

		mockIsAllowedUser(true)
		defer restoreIsAllowedUser()

		mockedAzureUser := &models.AzureUser{
			ID:                "azure123",
			DisplayName:       "Azure Test User",
			Mail:              "azure.user@example.com",
			UserPrincipalName: "azure.user@example.com",
		}
		mockFetchAzureUserProfile(mockedAzureUser, nil)
		defer restoreFetchAzureUserProfile()

		r := setupTestRouter()
		r.GET("/oauth/azure/callback", HandleAzureLogin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/azure/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "azure123", resp.UserData.ID)
		assert.Equal(t, "Azure Test User", resp.UserData.Name)
		assert.Equal(t, "azure.user@example.com", resp.UserData.Email)
		assert.Equal(t, "azure", resp.UserData.Provider)
		assert.True(t, resp.ExpiresIn > 0 && resp.ExpiresIn <= 3600)

		cookies := w.Result().Cookies()
		sessionCookieFound := false
		for _, cookie := range cookies {
			if strings.HasPrefix(cookie.Name, sessioncookie.SessionPrefix) {
				sessionCookieFound = true
				break
			}
		}
		assert.True(t, sessionCookieFound, "Session cookie should be set")
	})

	t.Run("missing code", func(t *testing.T) {
		r := setupTestRouter()
		r.GET("/oauth/azure/callback", HandleAzureLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/azure/callback", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Code query parameter is required", resp.Error)
	})

	t.Run("token exchange fails", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer mockTokenServer.Close()
		os.Setenv("AZURE_TOKEN_URL", mockTokenServer.URL)

		r := setupTestRouter()
		r.GET("/oauth/azure/callback", HandleAzureLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/azure/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var resp models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Failed to exchange token", resp.Error)
	})

	t.Run("fetch user profile fails", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "valid_token_for_profile_fail",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		}))
		defer mockTokenServer.Close()
		os.Setenv("AZURE_TOKEN_URL", mockTokenServer.URL)

		mockFetchAzureUserProfile(nil, fmt.Errorf("graph API error"))
		defer restoreFetchAzureUserProfile()

		r := setupTestRouter()
		r.GET("/oauth/azure/callback", HandleAzureLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/azure/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var resp models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "graph API error", resp.Error)
	})

	t.Run("unauthorized domain", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "valid_token_for_unauth_domain",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		}))
		defer mockTokenServer.Close()
		os.Setenv("AZURE_TOKEN_URL", mockTokenServer.URL)

		mockedAzureUser := &models.AzureUser{Mail: "user@otherdomain.com", UserPrincipalName: "user@otherdomain.com"}
		mockFetchAzureUserProfile(mockedAzureUser, nil)
		defer restoreFetchAzureUserProfile()

		mockIsAllowedUser(false)
		defer restoreIsAllowedUser()

		r := setupTestRouter()
		r.GET("/oauth/azure/callback", HandleAzureLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/azure/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		var resp models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Unauthorized domain", resp.Error)
	})
}

func TestGetAzureProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreEnv := setupAzureTestEnv()
	defer restoreEnv()

	t.Run("success", func(t *testing.T) {
		mockedAzureUser := &models.AzureUser{
			ID:          "azure123",
			DisplayName: "Azure Test User",
			Mail:        "azure.user@example.com",
		}
		mockFetchAzureUserProfile(mockedAzureUser, nil)
		defer restoreFetchAzureUserProfile()

		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			sessionData := map[string]interface{}{
				"token": "fake-azure-profile-token",
				"id":    "azure123",
				"name":  "Azure Test User",
			}
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.GET("/azure/profile", GetAzureProfile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/azure/profile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "azure123", resp["id"])
		assert.Equal(t, "Azure Test User", resp["name"])
		assert.Equal(t, "azure.user@example.com", resp["email"])
		assert.Equal(t, "azure", resp["provider"])
	})

	t.Run("no token in session", func(t *testing.T) {
		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			c.Set("sessionData", map[string]interface{}{"id": "azure123", "name": "Test"})
			c.Next()
		})
		r.GET("/azure/profile", GetAzureProfile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/azure/profile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var respMsg models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &respMsg)
		assert.Equal(t, "Unauthorized: no token in session data", respMsg.Error)
	})

	t.Run("fetchAzureUserProfile fails", func(t *testing.T) {
		mockFetchAzureUserProfile(nil, fmt.Errorf("graph API error"))
		defer restoreFetchAzureUserProfile()

		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			c.Set("sessionData", map[string]interface{}{"token": "valid-token", "id": "azure123", "name": "Test"})
			c.Next()
		})
		r.GET("/azure/profile", GetAzureProfile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/azure/profile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var respMsg models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &respMsg)
		assert.Equal(t, "graph API error", respMsg.Error)
	})
}

func TestFetchAzureUserProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedUser := models.AzureUser{
			ID:                "azure-user-id",
			DisplayName:       "Azure Test User",
			Mail:              "azure.test@example.com",
			UserPrincipalName: "azure.test@example.com",
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1.0/me", r.URL.Path)
			assert.Equal(t, "Bearer fake-access-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(expectedUser)
		}))
		defer server.Close()

		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-access-token"}))

		resp, err := client.Get(server.URL + "/v1.0/me")
		assert.NoError(t, err)
		defer resp.Body.Close()

		var user models.AzureUser
		err = json.NewDecoder(resp.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, expectedUser, user)
	})

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
	})
}

func TestGetAzureGroups(t *testing.T) {
	logger := getTestLogger()

	t.Run("success", func(t *testing.T) {
		expectedGroups := []models.Team{
			{ID: "group1", Name: "Azure Group One"},
			{ID: "group2", Name: "Azure Group Two"},
		}
		mockResponse := struct {
			Value []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"value"`
		}{
			Value: []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			}{
				{ID: "group1", DisplayName: "Azure Group One"},
				{ID: "group2", DisplayName: "Azure Group Two"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1.0/me/memberOf", r.URL.Path)
			assert.Equal(t, "Bearer fake-access-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		originalGetAzureGroups := GetAzureGroups
		defer func() { GetAzureGroups = originalGetAzureGroups }()

		GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())
			client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
			resp, err := client.Get(server.URL + "/v1.0/me/memberOf")
			if err != nil {
				reqLogger.Error("Failed to fetch Azure groups", zap.Error(err))
				return nil, fmt.Errorf("failed to fetch groups from Azure AD")
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				bodyBytes := make([]byte, 1024)
				n, _ := resp.Body.Read(bodyBytes)
				reqLogger.Warn("Error fetching Azure groups", zap.String("response", string(bodyBytes[:n])))
				return nil, fmt.Errorf("error fetching groups from Azure AD: status %d", resp.StatusCode)
			}

			var groupsResponse struct {
				Value []struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"value"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&groupsResponse); err != nil {
				reqLogger.Error("Failed to decode Azure groups response", zap.Error(err))
				return nil, fmt.Errorf("failed to decode groups response")
			}

			var teams []models.Team
			for _, g := range groupsResponse.Value {
				teams = append(teams, models.Team{
					ID:   g.ID,
					Name: g.DisplayName,
				})
			}
			return teams, nil
		}

		groups, err := GetAzureGroups("fake-access-token", logger)
		assert.NoError(t, err)
		assert.Equal(t, expectedGroups, groups)
	})

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		originalGetAzureGroups := GetAzureGroups
		defer func() { GetAzureGroups = originalGetAzureGroups }()
		GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())
			client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
			resp, err := client.Get(server.URL + "/v1.0/me/memberOf")
			if err != nil {
				return nil, fmt.Errorf("failed to fetch groups from Azure AD")
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("error fetching groups from Azure AD")
			}
			return nil, nil
		}

		_, err := GetAzureGroups("fake-access-token", logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error fetching groups from Azure AD")
	})

	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"value": "not an array"`)
		}))
		defer server.Close()

		originalGetAzureGroups := GetAzureGroups
		defer func() { GetAzureGroups = originalGetAzureGroups }()
		GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())
			client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
			resp, err := client.Get(server.URL + "/v1.0/me/memberOf")
			if err != nil {
				return nil, fmt.Errorf("failed to fetch groups from Azure AD")
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("error fetching groups from Azure AD")
			}
			if err := json.NewDecoder(resp.Body).Decode(&struct{}{}); err != nil {
				return nil, fmt.Errorf("failed to decode groups response")
			}
			return nil, nil
		}
		_, err := GetAzureGroups("fake-access-token", logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode groups response")
	})
}
