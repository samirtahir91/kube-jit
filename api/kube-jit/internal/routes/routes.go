package routes

import (
	"kube-jit/internal/handlers"
	"kube-jit/internal/middleware"
	"kube-jit/pkg/sessioncookie"

	_ "kube-jit/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(r *gin.Engine) {
	// Routes that require session handling - authenticated
	// This middleware will check for the session cookie and handle it accordingly
	// We are using a custoim middleware to split and combine the session cookie
	apiWithSession := r.Group("/kube-jit-api")
	apiWithSession.Use(sessioncookie.SplitAndCombineSessionMiddleware())
	apiWithSession.Use(middleware.RequireAuth())
	// log the user ID and username from the session data
	apiWithSession.Use(middleware.AccessLogger(handlers.Logger()))
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

	// Routes that do NOT require session handling - unauthenticated
	r.GET("/kube-jit-api/docs/openapi3.yaml", handlers.ServeOpenAPI3)
	r.GET("/kube-jit-api/oauth/github/callback", handlers.HandleGitHubLogin)
	r.GET("/kube-jit-api/oauth/google/callback", handlers.HandleGoogleLogin)
	r.GET("/kube-jit-api/oauth/azure/callback", handlers.HandleAzureLogin)
	r.GET("/kube-jit-api/healthz", handlers.HealthCheck)
	r.GET("/kube-jit-api/client_id", handlers.GetOauthClientId)
	r.POST("/k8s-callback", handlers.K8sCallback)
	r.POST("/kube-jit-api/logout", handlers.Logout)
	r.GET("/kube-jit-api/build-sha", handlers.GetBuildSha)
	// openapi v2
	r.GET("/kube-jit-api/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// openapi v3
	r.Static("/kube-jit-api/swagger-ui", "/swagger-ui")
}
