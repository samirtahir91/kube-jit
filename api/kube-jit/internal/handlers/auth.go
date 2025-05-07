package handlers

import (
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// checkLoggedIn verifies if the user is logged in by checking session data.
// Returns the session data if valid, or sends an unauthorized response and aborts the request.
func checkLoggedIn(c *gin.Context) (map[string]interface{}, bool) {
	session := sessions.Default(c)
	combinedData := session.Get("data")
	if combinedData == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no session data in cookies"})
		c.Abort() // Stop further processing of the request
		return nil, false
	}

	// Ensure the session data is a map[string]interface{}
	sessionData, ok := combinedData.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session data format"})
		c.Abort() // Stop further processing of the request
		return nil, false
	}

	return sessionData, true
}

// Logout clears all session cookies with the sessionPrefix
func Logout(c *gin.Context) {
	// Iterate through cookies with the session prefix
	for i := 0; ; i++ {
		cookieName := fmt.Sprintf("%s%d", sessioncookie.SessionPrefix, i)
		_, err := c.Cookie(cookieName)
		if err != nil {
			break // Stop when no more cookies are found
		}

		// Clear the cookie by setting its MaxAge to -1
		c.SetCookie(cookieName, "", -1, "/", "", true, true)
	}

	// Respond with a success message
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// CommonPermissions checks if the user has common permissions
// It checks if the user is logged in and retrieves their permissions
// It fetches the user's groups from the specified provider (GitHub, Google, Azure)
// It matches the user groups to the approver and admin teams
// It updates the session with the user's permissions
// It returns the permissions as JSON
func CommonPermissions(c *gin.Context) {
	session := sessions.Default(c)

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	// Parse provider from payload
	var payload struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid provider"})
		return
	}

	// Check if cached in session
	isApprover, isApproverOk := sessionData["isApprover"].(bool)
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	isPlatformApprover, isPlatformApproverOk := sessionData["isPlatformApprover"].(bool)
	approverGroups, approverGroupsOk := sessionData["approverGroups"]
	adminGroups, adminGroupsOk := sessionData["adminGroups"]
	platformApproverGroups, platformApproverGroupsOk := sessionData["platformApproverGroups"]
	if isApproverOk && isAdminOk && isPlatformApproverOk && approverGroupsOk && adminGroupsOk && platformApproverGroupsOk {
		c.JSON(http.StatusOK, gin.H{
			"isApprover":             isApprover,
			"approverGroups":         approverGroups,
			"isAdmin":                isAdmin,
			"isPlatformApprover":     isPlatformApprover,
			"adminGroups":            adminGroups,
			"platformApproverGroups": platformApproverGroups,
		})
		return
	}

	// Get token from session
	token, _ := sessionData["token"].(string)

	var userGroups []models.Team
	var err error

	// Fetch user groups based on the provider
	switch payload.Provider {
	case "github": // GitHub provider
		userGroups, err = GetGithubTeams(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch GitHub teams"})
			return
		}
	case "google": // Google provider
		userEmail, _ := sessionData["email"].(string)
		userGroups, err = GetGoogleGroupsWithWorkloadIdentity(userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Google groups"})
			return
		}
	case "azure": // Azure provider
		userGroups, err = GetAzureGroups(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Azure groups"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown provider"})
		return
	}

	// Match user groups to approver/admin teams
	isAdmin, isPlatformApprover, matchedPlatformGroups, matchedAdminGroups := MatchUserGroups(
		userGroups,
		k8s.PlatformApproverTeams,
		k8s.AdminTeams,
	)

	// Check and append if user is in any JitGroup for any cluster
	var matchedApproverGroups []models.Team
	for _, clusterName := range k8s.ClusterNames {
		jitGroups, err := k8s.GetJitGroups(clusterName)
		if err != nil {
			logger.Error("Error fetching JitGroups for cluster", zap.String("clusterName", clusterName), zap.Error(err))
			continue
		}
		groups, _, _ := unstructured.NestedSlice(jitGroups.Object, "spec", "groups")
		for _, group := range groups {
			groupMap, ok := group.(map[string]any)
			if !ok {
				continue
			}
			groupID, ok := groupMap["groupID"].(string)
			groupName, _ := groupMap["groupName"].(string)
			if ok {
				for _, userGroup := range userGroups {
					if userGroup.ID == groupID {
						matchedApproverGroups = append(matchedApproverGroups, models.Team{ID: groupID, Name: groupName})
					}
				}
			}
		}
	}

	// Check if the user is an approver
	isApprover = len(matchedApproverGroups) > 0

	// Update session
	sessionData["isApprover"] = isApprover
	sessionData["approverGroups"] = matchedApproverGroups
	sessionData["isAdmin"] = isAdmin
	sessionData["isPlatformApprover"] = isPlatformApprover
	sessionData["adminGroups"] = matchedAdminGroups
	sessionData["platformApproverGroups"] = matchedPlatformGroups
	session.Set("data", sessionData)
	sessioncookie.SplitSessionData(c)

	c.JSON(http.StatusOK, gin.H{
		"isApprover":             isApprover,
		"approverGroups":         matchedApproverGroups,
		"isAdmin":                isAdmin,
		"isPlatformApprover":     isPlatformApprover,
		"adminGroups":            matchedAdminGroups,
		"platformApproverGroups": matchedPlatformGroups,
	})
}

// isAllowedUser checks if the user is allowed to access the api
// It checks the provider and email domain for Google and Azure
// and checks the organization membership for GitHub
// It returns true if the user is allowed, false otherwise
func isAllowedUser(provider, email string, extraInfo map[string]any) bool {
	switch provider {
	case "google", "azure":
		return strings.HasSuffix(email, "@"+allowedDomain)
	case "github":
		orgs, ok := extraInfo["orgs"].([]string)
		if !ok {
			return false
		}
		for _, org := range orgs {
			if org == allowedOrg {
				return true
			}
		}
		return false
	default:
		return false
	}
}
