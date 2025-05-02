package routes

import (
	"kube-jit/internal/handlers"
	"kube-jit/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Routes that require session handling
	apiWithSession := r.Group("/kube-jit-api")
	apiWithSession.Use(middleware.SplitAndCombineSessionMiddleware())
	{
		apiWithSession.GET("/approving-groups", handlers.GetApprovingGroups)
		apiWithSession.GET("/roles-and-clusters", handlers.GetClustersAndRoles)
		apiWithSession.GET("/github/profile", handlers.GetGithubProfile)
		apiWithSession.GET("/google/profile", handlers.GetGoogleProfile)
		apiWithSession.GET("/azure/profile", handlers.GetAzureProfile)
		apiWithSession.POST("/submit-request", handlers.SubmitRequest)
		apiWithSession.GET("/history", handlers.GetRecords)
		apiWithSession.GET("/approvals", handlers.GetPendingApprovals)
		apiWithSession.POST("/approve-reject", handlers.ApproveOrRejectRequests)
		apiWithSession.GET("/github/permissions", handlers.GithubPermissions)
		apiWithSession.GET("/google/permissions", handlers.GooglePermissions)
		apiWithSession.GET("/azure/permissions", handlers.AzurePermissions)
	}

	// Routes that do NOT require session handling
	r.GET("/kube-jit-api/oauth/github/callback", handlers.HandleGitHubLogin)
	r.GET("/kube-jit-api/oauth/google/callback", handlers.HandleGoogleLogin)
	r.GET("/kube-jit-api/oauth/azure/callback", handlers.HandleAzureLogin)
	r.GET("/kube-jit-api/healthz", handlers.HealthCheck)
	r.GET("/kube-jit-api/client_id", handlers.GetOauthClientId)
	r.POST("/k8s-callback", handlers.K8sCallback)
	r.POST("/kube-jit-api/logout", handlers.Logout)
}
