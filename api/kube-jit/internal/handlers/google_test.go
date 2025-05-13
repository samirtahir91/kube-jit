package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func setupGoogleTestEnv() (restoreFunc func()) {
	originalOauthProvider := oauthProvider
	originalClientID := clientID
	originalClientSecret := clientSecret
	originalRedirectURI := redirectUri
	originalAllowedDomain := allowedDomain
	originalAllowedOrg := allowedOrg
	originalAdminEmail := adminEmail

	originalOauthProviderEnv := os.Getenv("OAUTH_PROVIDER")
	originalOauthClientIDEnv := os.Getenv("OAUTH_CLIENT_ID")
	originalOauthClientSecretEnv := os.Getenv("OAUTH_CLIENT_SECRET")
	originalOauthRedirectURIEnv := os.Getenv("OAUTH_REDIRECT_URI")
	originalAllowedDomainEnv := os.Getenv("ALLOWED_DOMAIN")
	originalAllowedGithubOrgEnv := os.Getenv("ALLOWED_GITHUB_ORG")
	originalGoogleAdminEmailEnv := os.Getenv("GOOGLE_ADMIN_EMAIL")

	oauthProvider = "google"
	clientID = "test-google-client-id"
	clientSecret = "test-google-client-secret"
	redirectUri = "http://localhost/test/google/callback"
	allowedDomain = "example.com"
	allowedOrg = ""
	adminEmail = "test-admin@example.com"

	os.Setenv("OAUTH_PROVIDER", "google")
	os.Setenv("OAUTH_CLIENT_ID", "test-google-client-id")
	os.Setenv("OAUTH_CLIENT_SECRET", "test-google-client-secret")
	os.Setenv("OAUTH_REDIRECT_URI", "http://localhost/test/google/callback")
	os.Setenv("ALLOWED_DOMAIN", "example.com")
	os.Setenv("ALLOWED_GITHUB_ORG", "")
	os.Setenv("GOOGLE_ADMIN_EMAIL", "test-admin@example.com")

	return func() {
		oauthProvider = originalOauthProvider
		clientID = originalClientID
		clientSecret = originalClientSecret
		redirectUri = originalRedirectURI
		allowedDomain = originalAllowedDomain
		allowedOrg = originalAllowedOrg
		adminEmail = originalAdminEmail

		os.Setenv("OAUTH_PROVIDER", originalOauthProviderEnv)
		os.Setenv("OAUTH_CLIENT_ID", originalOauthClientIDEnv)
		os.Setenv("OAUTH_CLIENT_SECRET", originalOauthClientSecretEnv)
		os.Setenv("OAUTH_REDIRECT_URI", originalOauthRedirectURIEnv)
		os.Setenv("ALLOWED_DOMAIN", originalAllowedDomainEnv)
		os.Setenv("ALLOWED_GITHUB_ORG", originalAllowedGithubOrgEnv)
		os.Setenv("GOOGLE_ADMIN_EMAIL", originalGoogleAdminEmailEnv)
	}
}

var originalFetchGoogleUserProfile func(token string) (*models.GoogleUser, error)

func mockFetchGoogleUserProfile(user *models.GoogleUser, err error) {
	originalFetchGoogleUserProfile = fetchGoogleUserProfile
	fetchGoogleUserProfile = func(token string) (*models.GoogleUser, error) {
		return user, err
	}
}

func restoreFetchGoogleUserProfile() {
	if originalFetchGoogleUserProfile != nil {
		fetchGoogleUserProfile = originalFetchGoogleUserProfile
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreEnv := setupGoogleTestEnv()
	defer restoreEnv()

	const testClientID = "test-google-client-id"
	const testClientSecret = "test-google-client-secret"
	const testRedirectURI = "http://localhost/test/google/callback"

	t.Run("success", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// --- BEGIN REQUEST DUMP ---
			t.Logf("--- MOCK TOKEN SERVER REQUEST ---")
			t.Logf("Method: %s", r.Method)
			t.Logf("URL: %s", r.URL.String())
			t.Logf("Authorization Header: %s", r.Header.Get("Authorization")) // Log Authorization header
			t.Logf("Content-Type Header: %s", r.Header.Get("Content-Type"))

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Logf("Error reading request body: %v", err)
			} else {
				t.Logf("Request Body: %s", string(bodyBytes))
			}
			r.Body = io.NopCloser(strings.NewReader(string(bodyBytes))) // Restore body for ParseForm

			err = r.ParseForm()
			if err != nil {
				t.Logf("Error parsing form: %v", err)
			}
			t.Logf("Form Values After ParseForm:")
			for key, values := range r.Form {
				for _, value := range values {
					t.Logf("  Form['%s']: %s", key, value)
				}
			}
			t.Logf("--- END MOCK TOKEN SERVER REQUEST ---")

			// Check for HTTP Basic Authentication
			username, password, ok := r.BasicAuth()
			if !ok {
				t.Errorf("Basic auth credentials not provided or malformed")
			}
			assert.True(t, ok, "Basic auth should be present")
			assert.Equal(t, testClientID, username, "Basic auth username should be client_id")
			assert.Equal(t, testClientSecret, password, "Basic auth password should be client_secret")

			// Check form values (these should still be present)
			assert.Equal(t, "auth_code", r.Form.Get("code"))
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, testRedirectURI, r.Form.Get("redirect_uri"))

			// Ensure client_id and client_secret are NOT in the form body if sent via Basic Auth
			// (This is typical for Google)
			assert.Empty(t, r.Form.Get("client_id"), "client_id should not be in form body when using Basic Auth")
			assert.Empty(t, r.Form.Get("client_secret"), "client_secret should not be in form body when using Basic Auth")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "mocked_google_access_token",
				"token_type":   "Bearer",
				"expires_in":   3600,
				"id_token":     "mocked_id_token",
			})
		}))
		defer mockTokenServer.Close()

		originalGetConfigFunc := getGoogleOAuthConfig
		getGoogleOAuthConfig = func() *oauth2.Config {
			t.Logf("DEBUG: Swapped getGoogleOAuthConfig is being called! Using ClientID: %s, ClientSecret: %s", testClientID, testClientSecret)
			return &oauth2.Config{
				ClientID:     testClientID,
				ClientSecret: testClientSecret,
				RedirectURL:  testRedirectURI,
				Scopes: []string{
					"https://www.googleapis.com/auth/userinfo.profile",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://accounts.google.com/o/oauth2/auth",
					TokenURL: mockTokenServer.URL,
				},
			}
		}
		defer func() { getGoogleOAuthConfig = originalGetConfigFunc }()

		mockIsAllowedUser(true)
		defer restoreIsAllowedUser()

		mockedGoogleUser := &models.GoogleUser{
			ID:      "google123",
			Name:    "Google Test User",
			Email:   "google.user@example.com",
			Picture: "http://example.com/avatar.jpg",
		}
		mockFetchGoogleUserProfile(mockedGoogleUser, nil)
		defer restoreFetchGoogleUserProfile()

		r := setupTestRouter()
		r.GET("/oauth/google/callback", HandleGoogleLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/google/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp models.LoginResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "google123", resp.UserData.ID)
		assert.Equal(t, "Google Test User", resp.UserData.Name)
		assert.Equal(t, "google.user@example.com", resp.UserData.Email)
		assert.Equal(t, "http://example.com/avatar.jpg", resp.UserData.AvatarURL)
		assert.Equal(t, "google", resp.UserData.Provider)
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

	t.Run("token exchange fails", func(t *testing.T) {
		mockTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer mockTokenServer.Close()

		originalGetConfigFunc := getGoogleOAuthConfig
		getGoogleOAuthConfig = func() *oauth2.Config {
			t.Logf("DEBUG: Swapped getGoogleOAuthConfig (for token exchange fails) is being called! Using ClientID: %s", testClientID)
			return &oauth2.Config{
				ClientID:     testClientID,
				ClientSecret: testClientSecret,
				RedirectURL:  testRedirectURI,
				Scopes: []string{
					"https://www.googleapis.com/auth/userinfo.profile",
					"https://www.googleapis.com/auth/userinfo.email",
				},
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://accounts.google.com/o/oauth2/auth",
					TokenURL: mockTokenServer.URL,
				},
			}
		}
		defer func() { getGoogleOAuthConfig = originalGetConfigFunc }()

		r := setupTestRouter()
		r.GET("/oauth/google/callback", HandleGoogleLogin)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/oauth/google/callback?code=auth_code", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var resp models.SimpleMessageResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Failed to exchange token", resp.Error)
	})
}

func TestGetGoogleProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreEnv := setupGoogleTestEnv()
	defer restoreEnv()

	t.Run("success", func(t *testing.T) {
		mockedGoogleUser := &models.GoogleUser{
			ID:      "google123",
			Name:    "Google Test User",
			Email:   "google.user@example.com",
			Picture: "http://example.com/avatar.jpg",
		}
		mockFetchGoogleUserProfile(mockedGoogleUser, nil)
		defer restoreFetchGoogleUserProfile()

		r := setupTestRouter()
		r.Use(func(c *gin.Context) {
			sessionData := map[string]interface{}{
				"token": "fake-google-profile-token",
				"id":    "google123",
				"name":  "Google Test User",
			}
			c.Set("sessionData", sessionData)
			c.Next()
		})
		r.GET("/google/profile", GetGoogleProfile)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/google/profile", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "google123", resp["id"])
		assert.Equal(t, "Google Test User", resp["name"])
		assert.Equal(t, "google.user@example.com", resp["email"])
		assert.Equal(t, "http://example.com/avatar.jpg", resp["avatar_url"])
		assert.Equal(t, "google", resp["provider"])
	})
}

func TestFetchGoogleUserProfileHelper(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedUser := models.GoogleUser{
			ID:      "google-user-id",
			Name:    "Google Test User",
			Email:   "google.test@example.com",
			Picture: "http://example.com/pic.jpg",
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/oauth2/v2/userinfo", r.URL.Path)
			assert.Equal(t, "Bearer fake-access-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(expectedUser)
		}))
		defer server.Close()

		originalFunc := fetchGoogleUserProfile
		fetchGoogleUserProfile = func(token string) (*models.GoogleUser, error) {
			client := server.Client()
			req, _ := http.NewRequest("GET", server.URL+"/oauth2/v2/userinfo", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
			}
			var user models.GoogleUser
			if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
				return nil, err
			}
			return &user, nil
		}
		defer func() { fetchGoogleUserProfile = originalFunc }()

		user, err := fetchGoogleUserProfile("fake-access-token")
		assert.NoError(t, err)
		assert.Equal(t, &expectedUser, user)
	})
}
