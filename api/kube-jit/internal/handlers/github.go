package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Fetch GitHub teams for a user using their OAuth token
// This function is used to get the teams associated with the authenticated user
// It sends a GET request to the GitHub API endpoint for user teams
// and returns a slice of models.Team
// Each team is represented by its ID and name
// It returns an error if the request fails or if the response is not as expected
func GetGithubTeams(token string) ([]models.Team, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		logger.Error("Failed to create request for GitHub teams", zap.Error(err))
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to fetch GitHub teams", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Error fetching teams from GitHub", zap.String("response", string(body)))
		return nil, fmt.Errorf("error fetching teams from GitHub: %s", string(body))
	}

	var githubTeams []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&githubTeams); err != nil {
		logger.Error("Failed to decode GitHub teams", zap.Error(err))
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
// It retrieves the access token and user information from GitHub
// and sets the session data for the user
// It also normalizes the user data and returns it in the response
// It returns a JSON response with the user data and expiration time
// or an error message if something goes wrong
func HandleGitHubLogin(c *gin.Context) {
	// Check for the presence of the 'code' query parameter
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in GitHub login")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	// Get the client ID and secret from global variables
	ctx := context.Background()
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	// Send a POST request to GitHub to exchange the code for an access token
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		logger.Error("Failed to create request for GitHub access token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request for access token"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to fetch GitHub access token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch access token"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Error fetching access token from GitHub", zap.Int("status", resp.StatusCode))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching access token from GitHub"})
		return
	}

	// Decode the response body to get the access token
	var tokenData models.GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		logger.Error("Failed to decode GitHub token response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode access token"})
		return
	}

	// Get the user information using the access token
	req, err = http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		logger.Error("Failed to create request for GitHub user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request for user"})
		return
	}
	req.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)

	userResp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to fetch GitHub user data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user data"})
		return
	}
	defer userResp.Body.Close()

	// Check if the response status code is OK
	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(userResp.Body)
		logger.Warn("Error fetching user data from GitHub", zap.String("response", string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var githubUser models.GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
		logger.Error("Failed to decode GitHub user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user data"})
		return
	}

	// Fetch email from GitHub user profile
	// If email is not present, fetch it from the emails endpoint
	email := githubUser.Email
	if email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err == nil {
			req.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)
			resp, err := httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var emails []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&emails); err == nil {
					for _, e := range emails {
						if e.Primary && e.Verified {
							email = e.Email
							break
						}
					}
					// fallback: use first verified email if no primary found
					if email == "" {
						for _, e := range emails {
							if e.Verified {
								email = e.Email
								break
							}
						}
					}
				}
			}
		}
	}

	// Normalize the user data
	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     email,
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	// Prepare session data
	sessionData := map[string]interface{}{
		"token": tokenData.AccessToken,
		"email": email,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData)

	// Split the session data into cookies
	sessioncookie.SplitSessionData(c)

	logger.Info("Session cookies set successfully for GitHub login", zap.String("user", githubUser.Login))

	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": tokenData.ExpiresIn,
	})

	// Fetch orgs for the user
	orgReq, err := http.NewRequest("GET", "https://api.github.com/user/orgs", nil)
	if err != nil {
		logger.Error("Failed to create request for GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request for orgs"})
		return
	}
	orgReq.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)
	orgResp, err := httpClient.Do(orgReq)
	if err != nil {
		logger.Error("Failed to fetch GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orgs"})
		return
	}
	defer orgResp.Body.Close()

	var orgs []struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(orgResp.Body).Decode(&orgs); err != nil {
		logger.Error("Failed to decode GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode orgs"})
		return
	}
	orgNames := []string{}
	for _, org := range orgs {
		orgNames = append(orgNames, org.Login)
	}
	extraInfo := map[string]any{"orgs": orgNames}

	if !isAllowedUser("github", email, extraInfo) {
		logger.Warn("Login attempt from unauthorized GitHub org", zap.String("email", email))
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized org"})
		return
	}
}

// GetGithubProfile gets the logged in users profile info
// from GitHub using the access token stored in the session
// It returns a JSON response with the user data or an error message if something goes wrong
func GetGithubProfile(c *gin.Context) {
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		logger.Warn("No token in session data for GitHub profile")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		logger.Error("Failed to create request for GitHub user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request for user profile"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to fetch GitHub user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Error fetching user profile from GitHub", zap.String("response", string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var githubUser models.GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		logger.Error("Failed to decode GitHub user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user profile"})
		return
	}

	// Fetch email from GitHub user profile
	email := githubUser.Email
	// If email is not present, fetch it from the emails endpoint
	if email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var emails []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&emails); err == nil {
					for _, e := range emails {
						if e.Primary && e.Verified {
							email = e.Email
							break
						}
					}
					// fallback: use first verified email if no primary found
					if email == "" {
						for _, e := range emails {
							if e.Verified {
								email = e.Email
								break
							}
						}
					}
				}
			}
		}
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     email,
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	c.JSON(http.StatusOK, normalizedUserData)
}
