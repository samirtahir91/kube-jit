package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/middleware"
	"kube-jit/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Fetch GitHub teams for a user using their OAuth token
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
func HandleGitHubLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in GitHub login")
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

	var tokenData models.GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		logger.Error("Failed to decode GitHub token response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode access token"})
		return
	}

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
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData)

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	logger.Info("Session cookies set successfully for GitHub login", zap.String("user", githubUser.Login))

	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": tokenData.ExpiresIn,
	})
}

// GetGithubProfile gets the logged in users profile info
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
