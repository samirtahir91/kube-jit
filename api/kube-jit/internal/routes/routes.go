package routes

import (
	"kube-jit/internal/handlers"
	"kube-jit/pkg/sessioncookie"
	"os"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Routes that require session handling
	// This middleware will check for the session cookie and handle it accordingly
	// We are using a custoim middleware to split and combine the session cookie
	apiWithSession := r.Group("/kube-jit-api")
	apiWithSession.Use(sessioncookie.SplitAndCombineSessionMiddleware())
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
		apiWithSession.POST("/permissions", handlers.CommonPermissions)
		apiWithSession.POST("/admin/clean-expired", handlers.CleanExpiredRequests)
	}

	// Routes that do NOT require session handling
	r.GET("/kube-jit-api/oauth/github/callback", handlers.HandleGitHubLogin)
	r.GET("/kube-jit-api/oauth/google/callback", handlers.HandleGoogleLogin)
	r.GET("/kube-jit-api/oauth/azure/callback", handlers.HandleAzureLogin)
	r.GET("/kube-jit-api/healthz", handlers.HealthCheck)
	r.GET("/kube-jit-api/client_id", handlers.GetOauthClientId)
	r.POST("/k8s-callback", handlers.K8sCallback)
	r.POST("/kube-jit-api/logout", handlers.Logout)
	r.GET("/kube-jit-api/build-sha", func(c *gin.Context) {
		c.JSON(200, gin.H{"sha": os.Getenv("BUILD_SHA")})
	})
}
