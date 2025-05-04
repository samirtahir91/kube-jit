package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/middleware"
	"kube-jit/internal/models"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

var (
	azureOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Scopes:       []string{"openid", "email", "profile", "User.Read", "Directory.Read.All"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  os.Getenv("AZURE_AUTH_URL"),  // Use the configurable authorization endpoint
			TokenURL: os.Getenv("AZURE_TOKEN_URL"), // Use the configurable token endpoint
		},
	}
)

// HandleAzureLogin handles the Azure AD OAuth callback
func HandleAzureLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		log.Println("Missing 'code' query parameter")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	log.Println("Received authorization code:", code)

	// Exchange the authorization code for a token
	token, err := azureOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("Failed to exchange token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	log.Printf("Token received: %+v", token)

	// Use the token to fetch user info
	client := azureOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		log.Printf("Failed to fetch user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user info"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error fetching user profile from Azure AD: %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Azure AD"})
		return
	}

	// Decode the user info into the AzureUser struct
	var azureUser models.AzureUser
	if err := json.NewDecoder(resp.Body).Decode(&azureUser); err != nil {
		log.Printf("Failed to decode user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
		return
	}

	log.Printf("Azure user info: %+v", azureUser)

	// Normalize the user data
	normalizedUserData := models.NormalizedUserData{
		ID:        azureUser.ID,
		Name:      azureUser.DisplayName,
		Email:     azureUser.Mail,
		AvatarURL: "", // Azure AD doesn't provide an avatar URL by default
		Provider:  "azure",
	}

	// Prepare session data
	sessionData := map[string]interface{}{
		"email": azureUser.Mail,
		"token": token.AccessToken,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData) // Store as a map, not a JSON string

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	log.Println("Session cookies set successfully")

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
		return // The response has already been sent by CheckLoggedIn
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Use the token to fetch the user's profile from Azure's API
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Azure"})
		return
	}

	// Decode the response into a struct
	var azureUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&azureUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user profile"})
		return
	}

	// Normalize the response to match the structure of other providers
	normalizedUserData := map[string]interface{}{
		"id":         azureUser.ID,
		"name":       azureUser.DisplayName,
		"email":      azureUser.Mail,
		"avatar_url": "", // Azure AD doesn't provide an avatar URL by default
		"provider":   "azure",
	}

	// Return the normalized user data
	c.JSON(http.StatusOK, normalizedUserData)
}

// Fetch Azure AD groups for a user using their OAuth token
func GetAzureGroups(token string) ([]models.Team, error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me/memberOf")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching groups from Azure AD: %s", string(body))
	}

	var groupsResponse struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groupsResponse); err != nil {
		return nil, err
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
