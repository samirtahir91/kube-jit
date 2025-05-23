package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"kube-jit/pkg/utils"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// getAzureOAuthConfig constructs and returns the Azure OAuth2 config.
// This ensures it uses the current values of clientID, clientSecret, etc.
var getAzureOAuthConfig = func() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,     // Reads current package-level clientID from common.go
		ClientSecret: clientSecret, // Reads current package-level clientSecret from common.go
		RedirectURL:  redirectUri,  // Reads current package-level redirectUri from common.go
		Scopes:       []string{"openid", "email", "profile", "User.Read", "Directory.Read.All"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  utils.MustGetEnv("AZURE_AUTH_URL"),
			TokenURL: utils.MustGetEnv("AZURE_TOKEN_URL"),
		},
	}
}

// Helper to fetch and decode Azure user profile
var fetchAzureUserProfile = func(token string) (*models.AzureUser, error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Azure user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching user profile from Azure AD: %s", string(body))
	}

	var azureUser models.AzureUser
	if err := json.NewDecoder(resp.Body).Decode(&azureUser); err != nil {
		return nil, fmt.Errorf("failed to decode Azure user info: %w", err)
	}
	return &azureUser, nil
}

// HandleAzureLogin godoc
// @Summary Azure OAuth callback
// @Description Handles the Azure OAuth callback, exchanges the code for an access token, fetches user info, sets session data, and returns normalized user data and expiration time.
// @Tags azure
// @Accept  json
// @Produce  json
// @Param   code query string true "Azure OAuth authorization code"
// @Success 200 {object} models.LoginResponse "Normalized user data and expiration time"
// @Failure 400 {object} models.SimpleMessageResponse "Missing or invalid code"
// @Failure 403 {object} models.SimpleMessageResponse "Unauthorized domain"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /oauth/azure/callback [get]
func HandleAzureLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in Azure login")
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Code query parameter is required"})
		return
	}

	currentAzureOAuthConfig := getAzureOAuthConfig() // Use the function here

	// Exchange the authorization code for a token
	token, err := currentAzureOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		// Log the detailed error from the Exchange call
		logger.Error("Failed to exchange Azure token",
			zap.Error(err), // This will print the underlying error from the oauth2 library
			zap.String("codeUsed", code),
			zap.String("clientIDUsed", currentAzureOAuthConfig.ClientID),
			zap.String("clientSecretUsed", currentAzureOAuthConfig.ClientSecret), // Be careful logging secrets, even in tests
			zap.String("redirectURIUsed", currentAzureOAuthConfig.RedirectURL),
			zap.String("tokenURLUsed", currentAzureOAuthConfig.Endpoint.TokenURL),
		)
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to exchange token"})
		return
	}

	// Fetch user info using the helper
	azureUser, err := fetchAzureUserProfile(token.AccessToken)
	if err != nil {
		logger.Error("Failed to fetch Azure user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Use Mail if present, otherwise fallback to UserPrincipalName
	email := azureUser.Mail
	if email == "" {
		email = azureUser.UserPrincipalName
	}

	// Check if the user is allowed to log in
	if !isAllowedUser("azure", email, nil) {
		logger.Warn("Login attempt from unauthorized Azure domain", zap.String("email", email))
		c.JSON(http.StatusForbidden, models.SimpleMessageResponse{Error: "Unauthorized domain"})
		return
	}

	// Normalize the user data
	normalizedUserData := models.NormalizedUserData{
		ID:        azureUser.ID,
		Name:      azureUser.DisplayName,
		Email:     email,
		AvatarURL: "", // Azure AD doesn't provide an avatar URL by default
		Provider:  "azure",
	}

	// Prepare session data
	sessionData := map[string]interface{}{
		"email": email,
		"token": token.AccessToken,
		"id":    azureUser.ID,
		"name":  azureUser.DisplayName,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData)

	// Split the session data into cookies
	sessioncookie.SplitSessionData(c)

	logger.Debug("Session cookies set successfully for Azure login", zap.String("name", azureUser.DisplayName))

	// Respond with the normalized user data
	c.JSON(http.StatusOK, models.LoginResponse{
		UserData:  normalizedUserData,
		ExpiresIn: int(time.Until(token.Expiry).Seconds()),
	})
}

// GetAzureProfile godoc
// @Summary Get the logged in user's Azure profile
// @Description Returns the normalized Azure user profile for the authenticated user.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags azure
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} models.NormalizedUserData
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /azure/profile [get]
func GetAzureProfile(c *gin.Context) {
	// Check if the user is logged in
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	reqLogger.Debug("User authenticated", zap.String("userID", sessionData["id"].(string)))

	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		reqLogger.Warn("No token in session data for Azure profile")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no token in session data"})
		return
	}

	// Fetch user info using the helper
	azureUser, err := fetchAzureUserProfile(token)
	if err != nil {
		reqLogger.Error("Failed to fetch Azure user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Normalize the response to match the structure of other providers
	normalizedUserData := map[string]any{
		"id":         azureUser.ID,
		"name":       azureUser.DisplayName,
		"email":      azureUser.Mail,
		"avatar_url": "",
		"provider":   "azure",
	}

	c.JSON(http.StatusOK, normalizedUserData)
}

// Fetch Azure AD groups for a user using their OAuth token
var GetAzureGroups = func(token string, reqLogger *zap.Logger) ([]models.Team, error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me/memberOf")
	if err != nil {
		reqLogger.Error("Failed to fetch Azure groups", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch groups from Azure AD")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		reqLogger.Warn("Error fetching Azure groups", zap.String("response", string(body)))
		return nil, fmt.Errorf("error fetching groups from Azure AD")
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
