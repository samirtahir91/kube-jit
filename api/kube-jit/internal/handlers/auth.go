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

// GetSessionData retrieves session data from the context or panics
func GetSessionData(c *gin.Context) map[string]interface{} {
	sessionData := c.MustGet("sessionData").(map[string]interface{})

	return sessionData
}

// Logout godoc
// @Summary Log out and clear all session cookies
// @Description Clears all session cookies with the session prefix and logs the user out.
// @Tags auth
// @Accept  json
// @Produce  json
// @Success 200 {object} models.SimpleMessageResponse "Logged out successfully"
// @Router /logout [post]
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
	c.JSON(http.StatusOK, models.SimpleMessageResponse{Message: "Logged out successfully"})
}

// CommonPermissionsResponse represents the response for CommonPermissions
type CommonPermissionsResponse struct {
	IsApprover             bool          `json:"isApprover"`
	ApproverGroups         []models.Team `json:"approverGroups"`
	IsAdmin                bool          `json:"isAdmin"`
	IsPlatformApprover     bool          `json:"isPlatformApprover"`
	AdminGroups            []models.Team `json:"adminGroups"`
	PlatformApproverGroups []models.Team `json:"platformApproverGroups"`
}

// CommonPermissionsRequest represents the request payload for CommonPermissions
type CommonPermissionsRequest struct {
	Provider string `json:"provider" example:"github"`
}

// CommonPermissions godoc
// @Summary Get common permissions for the logged in user
// @Description Returns the user's permissions and group memberships for the specified provider (GitHub, Google, Azure).
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Param   request body handlers.CommonPermissionsRequest true "Provider payload"
// @Success 200 {object} handlers.CommonPermissionsResponse "User permissions and groups"
// @Failure 400 {object} models.SimpleMessageResponse "Missing or invalid provider"
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Failure 500 {object} models.SimpleMessageResponse "Failed to fetch user groups"
// @Router /permissions [post]
// CommonPermissions checks if the user has common permissions
func CommonPermissions(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	// Parse provider from payload
	var payload struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Provider == "" {
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Missing or invalid provider"})
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
		userGroups, err = GetGithubTeams(token, reqLogger)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch GitHub teams"})
			return
		}
	case "google": // Google provider
		userEmail, _ := sessionData["email"].(string)
		userGroups, err = GetGoogleGroupsWithWorkloadIdentity(userEmail, reqLogger)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch Google groups"})
			return
		}
	case "azure": // Azure provider
		userGroups, err = GetAzureGroups(token, reqLogger)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch Azure groups"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Unknown provider"})
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
			reqLogger.Error("Error fetching JitGroups for cluster", zap.String("clusterName", clusterName), zap.Error(err))
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

	session := sessions.Default(c)
	session.Set("data", sessionData)
	sessioncookie.SplitSessionData(c)

	c.JSON(http.StatusOK, CommonPermissionsResponse{
		IsApprover:             isApprover,
		ApproverGroups:         matchedApproverGroups,
		IsAdmin:                isAdmin,
		IsPlatformApprover:     isPlatformApprover,
		AdminGroups:            matchedAdminGroups,
		PlatformApproverGroups: matchedPlatformGroups,
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
