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
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Fetch GitHub teams for a user using their OAuth token
func GetGithubTeams(token string) ([]models.Team, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching teams from GitHub: %s", string(body))
	}

	var githubTeams []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&githubTeams); err != nil {
		return nil, err
	}

	var teams []models.Team
	for _, t := range githubTeams {
		teams = append(teams, models.Team{
			ID:   strconv.Itoa(t.ID),
			Name: t.Name,
		})
	}
	return teams, nil
}

// HandleGitHubLogin handles the GitHub OAuth callback
func HandleGitHubLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	ctx := context.Background()
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching access token from GitHub"})
		return
	}

	var tokenData models.GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req, err = http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)

	userResp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Response Body: %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var githubUser models.GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     "", // GitHub API may not return email by default
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	// Prepare session data
	sessionData := map[string]interface{}{
		"token": tokenData.AccessToken,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData) // Store as a map, not a JSON string

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	log.Println("Session cookies set successfully")

	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": tokenData.ExpiresIn,
	})
}

// GetGithubProfile gets the logged in users profile info
func GetGithubProfile(c *gin.Context) {

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

	// Fetch the user's profile from GitHub
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Response Body: %s", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var githubUser models.GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     "", // GitHub API may not return email by default
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	c.JSON(http.StatusOK, normalizedUserData)
}
