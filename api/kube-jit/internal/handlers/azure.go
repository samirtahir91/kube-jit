package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var (
	azureOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Scopes:       []string{"openid", "email", "profile", "User.Read", "Directory.Read.All"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  os.Getenv("AZURE_AUTH_URL"),
			TokenURL: os.Getenv("AZURE_TOKEN_URL"),
		},
	}
)

// HandleAzureLogin handles the Azure AD OAuth callback
func HandleAzureLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in Azure login")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	// Exchange the authorization code for a token
	token, err := azureOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("Failed to exchange Azure token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	// Use the token to fetch user info
	client := azureOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		logger.Error("Failed to fetch Azure user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user info"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Error fetching user profile from Azure AD", zap.String("response", string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Azure AD"})
		return
	}

	// Decode the user info into the AzureUser struct
	var azureUser models.AzureUser
	if err := json.NewDecoder(resp.Body).Decode(&azureUser); err != nil {
		logger.Error("Failed to decode Azure user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized domain"})
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
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData)

	// Split the session data into cookies
	sessioncookie.SplitSessionData(c)

	logger.Info("Session cookies set successfully for Azure login", zap.String("name", azureUser.DisplayName))

	// Respond with the normalized user data
	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": int(time.Until(token.Expiry).Seconds()),
	})
}

// GetAzureProfile retrieves the logged-in user's profile info from Azure
func GetAzureProfile(c *gin.Context) {

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		logger.Warn("No token in session data for Azure profile")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Use the token to fetch the user's profile from Azure's API
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		logger.Error("Failed to fetch Azure user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Error fetching user profile from Azure", zap.Int("status", resp.StatusCode))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Azure"})
		return
	}

	var azureUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&azureUser); err != nil {
		logger.Error("Failed to decode Azure user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user profile"})
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
func GetAzureGroups(token string) ([]models.Team, error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me/memberOf")
	if err != nil {
		logger.Error("Failed to fetch Azure groups", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch groups from Azure AD")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Error fetching Azure groups", zap.String("response", string(body)))
		return nil, fmt.Errorf("error fetching groups from Azure AD")
	}

	var groupsResponse struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groupsResponse); err != nil {
		logger.Error("Failed to decode Azure groups response", zap.Error(err))
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
