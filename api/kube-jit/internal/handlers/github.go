package handlers

import (
	"context"
	"encoding/json"
	"io"
	"kube-jit/internal/middleware"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// IsGithubApprover uses the GitHub API to fetch the user's teams and checks if they belong to any approver teams
// It returns true if the user is an approver, false otherwise
func IsGithubApprover(c *gin.Context) {
	session := sessions.Default(c)

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	// Check if isApprover, isAdmin, approverGroups, and adminGroups are already in the session cookie
	isApprover, isApproverOk := sessionData["isApprover"].(bool)
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	approverGroups, approverGroupsOk := sessionData["approverGroups"]
	adminGroups, adminGroupsOk := sessionData["adminGroups"]
	if isApproverOk && isAdminOk && approverGroupsOk && adminGroupsOk {
		// Return cached values
		c.JSON(http.StatusOK, gin.H{
			"isApprover":     isApprover,
			"approverGroups": approverGroups,
			"isAdmin":        isAdmin,
			"adminGroups":    adminGroups,
		})
		return
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Fetch user's teams from GitHub
	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Error fetching teams from GitHub"})
		return
	}

	// Decode the response and convert IDs to strings
	var githubTeams []struct {
		ID   int    `json:"id"`   // GitHub returns numeric IDs
		Name string `json:"name"` // Team name
	}
	if err := json.NewDecoder(resp.Body).Decode(&githubTeams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert GitHub teams to the Team struct
	var userTeams []models.Team
	for _, githubTeam := range githubTeams {
		userTeams = append(userTeams, models.Team{
			ID:   strconv.Itoa(githubTeam.ID), // Convert numeric ID to string
			Name: githubTeam.Name,
		})
	}

	// Check if the user belongs to any approver or admin groups
	var matchedApproverGroups []string
	var matchedAdminGroups []string
	for _, group := range userTeams {
		for _, approverGroup := range k8s.ApproverTeams {
			if group.ID == approverGroup.ID && group.Name == approverGroup.Name {
				matchedApproverGroups = append(matchedApproverGroups, group.ID)
			}
		}
		for _, adminGroup := range k8s.AdminTeams {
			if group.ID == adminGroup.ID && group.Name == adminGroup.Name {
				matchedAdminGroups = append(matchedAdminGroups, group.ID)
			}
		}
	}

	isApprover = len(matchedApproverGroups) > 0
	isAdmin = len(matchedAdminGroups) > 0

	// Update the session data with isApprover, isAdmin, approverGroups, and adminGroups
	sessionData["isApprover"] = isApprover
	sessionData["approverGroups"] = matchedApproverGroups
	sessionData["isAdmin"] = isAdmin
	sessionData["adminGroups"] = matchedAdminGroups

	// Save the updated session data
	session.Set("data", sessionData)

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	// Respond with the result
	c.JSON(http.StatusOK, gin.H{
		"isApprover":     isApprover,
		"approverGroups": matchedApproverGroups,
		"isAdmin":        isAdmin,
		"adminGroups":    matchedAdminGroups,
	})
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
